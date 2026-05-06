package auth

import (
	"errors"
	"sync"

	"golang.org/x/crypto/bcrypt"

	"github.com/mescon/muximux/v3/internal/logging"
)

// Role constants
const (
	RoleAdmin     = "admin"
	RolePowerUser = "power-user"
	RoleUser      = "user"
)

// roleLevels maps each role to a numeric level for comparison.
var roleLevels = map[string]int{
	RoleUser:      1,
	RolePowerUser: 2,
	RoleAdmin:     3,
}

// RoleLevel returns the numeric level for a role (0 for unknown).
func RoleLevel(role string) int {
	return roleLevels[role]
}

// HasMinRole checks if userRole meets or exceeds minRole in the hierarchy.
func HasMinRole(userRole, minRole string) bool {
	return RoleLevel(userRole) >= RoleLevel(minRole)
}

// User represents an authenticated user
type User struct {
	ID           string
	Username     string
	PasswordHash string
	Role         string
	Email        string
	DisplayName  string
	// Groups are the user's group memberships used for app-level
	// allowed_groups filtering. For built-in users, the operator sets
	// this in config.yaml or the Settings UI. For OIDC users this is
	// populated from the configured groups_claim each login. For
	// forward-auth users it is populated from the Remote-Groups header
	// each request. The list is the source of truth at the time of
	// session creation; it does not auto-refresh until the user logs
	// in again.
	Groups []string
}

// UserConfig represents user configuration from YAML, mirrored on the
// auth-package side so handlers/server can pass user records into the
// user store without the auth package importing config.
type UserConfig struct {
	Username     string   `yaml:"username"`
	PasswordHash string   `yaml:"password_hash"`
	Role         string   `yaml:"role"`
	Email        string   `yaml:"email,omitempty"`
	DisplayName  string   `yaml:"display_name,omitempty"`
	Groups       []string `yaml:"groups,omitempty"`
}

// UserStore manages users
type UserStore struct {
	users map[string]*User
	mu    sync.RWMutex

	// Timing-dummy hash cache, sized to the *minimum* bcrypt cost
	// across the live user store (or bcryptTargetCost when the store
	// is empty). Without this, the unknown-user path always runs at
	// bcryptTargetCost (12) while the wrong-password path runs at
	// whatever the user's stored cost is - so a measurable wall-clock
	// gap distinguishes "user does not exist" from "user exists,
	// wrong password" against any pre-rehash account (findings
	// codebase-review H6). Recomputed under dummyMu when the
	// minimum cost shifts.
	dummyMu      sync.Mutex
	dummyCost    int
	dummyHashRaw []byte
}

// NewUserStore creates a new user store
func NewUserStore() *UserStore {
	return &UserStore{
		users: make(map[string]*User),
	}
}

// LoadFromConfig loads users from configuration
func (s *UserStore) LoadFromConfig(configs []UserConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.users = make(map[string]*User)
	for _, cfg := range configs {
		role := cfg.Role
		if role == "" {
			role = RoleUser
		}
		s.users[cfg.Username] = &User{
			ID:           cfg.Username, // Use username as ID for simplicity
			Username:     cfg.Username,
			PasswordHash: cfg.PasswordHash,
			Role:         role,
			Email:        cfg.Email,
			DisplayName:  cfg.DisplayName,
			Groups:       append([]string(nil), cfg.Groups...), // defensive copy so later mutations to cfg.Groups don't leak in
		}
	}
	logging.Debug("User store loaded", "source", "auth", "count", len(configs))
}

// Get retrieves a copy of a user by username.
func (s *UserStore) Get(username string) *User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.copyUser(s.users[username])
}

// GetByID retrieves a copy of a user by ID.
func (s *UserStore) GetByID(id string) *User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// For now, ID == username
	return s.copyUser(s.users[id])
}

func (s *UserStore) copyUser(u *User) *User {
	if u == nil {
		return nil
	}
	copy := *u
	// Slice is shared by value-copy of the struct above; clone it so
	// callers cannot mutate the user store's internal state by writing
	// to the returned User's Groups field.
	if u.Groups != nil {
		copy.Groups = append([]string(nil), u.Groups...)
	}
	return &copy
}

// bcryptTargetCost is the bcrypt work factor Authenticate rehashes to
// on a successful login when the stored hash is weaker. DefaultCost
// (10) is dated; 12 is the 2024-era recommendation and takes ~250 ms
// on a modest CPU. Raising this transparently on each login lets old
// accounts migrate forward without operator action (findings.md M4).
const bcryptTargetCost = 12

// fallbackTimingDummyHash is the at-target-cost dummy used when the
// user store is empty (no hashes to read a min cost from). Tests and
// fresh installs land here before any users are added.
var fallbackTimingDummyHash = func() []byte {
	h, err := bcrypt.GenerateFromPassword([]byte("muximux-timing-dummy-not-a-real-password"), bcryptTargetCost)
	if err != nil {
		panic("auth: failed to pre-compute timing dummy hash: " + err.Error())
	}
	return h
}()

// minHashCostLocked returns the lowest bcrypt cost present across all
// stored hashes. Caller must hold s.mu (read or write). Returns
// bcryptTargetCost when the store is empty or contains only hashes
// whose cost cannot be parsed.
func (s *UserStore) minHashCostLocked() int {
	min := 0
	for _, u := range s.users {
		c, err := bcrypt.Cost([]byte(u.PasswordHash))
		if err != nil {
			continue
		}
		if min == 0 || c < min {
			min = c
		}
	}
	if min == 0 {
		return bcryptTargetCost
	}
	return min
}

// timingDummy returns a bcrypt hash sized to the lowest stored hash
// cost so the unknown-user compare takes roughly the same wall time
// as a real wrong-password compare against any user. Cached and
// re-computed lazily when the floor shifts.
func (s *UserStore) timingDummy() []byte {
	s.mu.RLock()
	cost := s.minHashCostLocked()
	s.mu.RUnlock()

	s.dummyMu.Lock()
	defer s.dummyMu.Unlock()
	if s.dummyHashRaw != nil && s.dummyCost == cost {
		return s.dummyHashRaw
	}
	if cost == bcryptTargetCost {
		// Reuse the package-level fallback at target cost.
		s.dummyHashRaw = fallbackTimingDummyHash
		s.dummyCost = bcryptTargetCost
		return s.dummyHashRaw
	}
	h, err := bcrypt.GenerateFromPassword([]byte("muximux-timing-dummy-not-a-real-password"), cost)
	if err != nil {
		// Rare: bcrypt only errors on cost out of range, which
		// minHashCostLocked has already filtered. Fall back to the
		// at-target-cost dummy rather than skipping the compare.
		logging.Warn("Failed to compute timing dummy hash; using fallback", "source", "auth", "cost", cost, "error", err)
		return fallbackTimingDummyHash
	}
	s.dummyHashRaw = h
	s.dummyCost = cost
	return s.dummyHashRaw
}

// Authenticate verifies username and password. Three failure modes are
// distinguished in the logs so an operator can tell an unknown user
// from a wrong password from a corrupt stored hash, while callers
// still see the same error shape for the two user-facing failures
// (findings.md M3). A successful login silently re-hashes the
// password if the stored hash is below bcryptTargetCost
// (findings.md M4); the rehash is best-effort and does not fail the
// login if it cannot be persisted. Returns rehashed=true so the
// caller can sync the user store to disk after a successful upgrade
// (codebase review C4-shf) - prior callers used `_, _, err :=` which
// is fine; new callers can pin the upgrade-on-disk flow.
func (s *UserStore) Authenticate(username, password string) (*User, bool, error) {
	user := s.Get(username)
	if user == nil {
		// Perform a throwaway bcrypt compare against a dummy hash sized
		// to the *minimum* cost in the user store. If we always used
		// bcryptTargetCost here while users still had cost-10 hashes,
		// the wall-clock gap (250 ms vs 70 ms) would distinguish
		// unknown-user from wrong-password against pre-rehash accounts
		// (codebase review H6).
		_ = bcrypt.CompareHashAndPassword(s.timingDummy(), []byte(password))
		logging.Debug("Auth attempt for unknown user", "source", "auth", "username", username)
		return nil, false, errors.New(errUserNotFound)
	}

	// Pre-check hash validity so a corrupt stored hash does not masquerade
	// as "wrong password" forever. bcrypt.Cost returns an error only for
	// malformed input; a valid hash at any cost returns nil here.
	if _, err := bcrypt.Cost([]byte(user.PasswordHash)); err != nil {
		logging.Warn("Auth attempt against corrupt password hash", "source", "audit", "username", username)
		return nil, false, errors.New("invalid password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		logging.Debug("Auth attempt: wrong password", "source", "auth", "username", username)
		return nil, false, errors.New("invalid password")
	}

	rehashed := s.maybeUpgradeHash(user.Username, user.PasswordHash, password)
	return user, rehashed, nil
}

// maybeUpgradeHash rehashes the password at bcryptTargetCost in place
// when the stored hash is below the target. Returns true when an
// upgrade actually happened so the caller can trigger a persist
// (codebase review C4-shf): without that, a successful rehash lives
// only in memory until the next config save and is lost on restart.
// A bcrypt failure now logs Warn instead of silently swallowing the
// error - a degraded crypto runtime is something the operator should
// see.
func (s *UserStore) maybeUpgradeHash(username, oldHash, password string) bool {
	cost, err := bcrypt.Cost([]byte(oldHash))
	if err != nil || cost >= bcryptTargetCost {
		return false
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptTargetCost)
	if err != nil {
		logging.Warn("Failed to upgrade bcrypt hash on login; running at degraded cost",
			"source", "audit",
			"user", username,
			"target_cost", bcryptTargetCost,
			"error", err)
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if u, ok := s.users[username]; ok && u.PasswordHash == oldHash {
		u.PasswordHash = string(newHash)
		return true
	}
	return false
}

// HashPassword creates a bcrypt hash of a password at the current
// target cost.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptTargetCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// Add adds a new user
func (s *UserStore) Add(user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[user.Username]; exists {
		return errors.New("user already exists")
	}

	s.users[user.Username] = user
	return nil
}

// Update updates an existing user. The atomic last-admin guard in
// UpdateIfNotLastAdminDemotion should be used when the caller is an
// admin API that must not leave the instance with zero admins.
func (s *UserStore) Update(user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[user.Username]; !exists {
		return errors.New(errUserNotFound)
	}

	s.users[user.Username] = user
	return nil
}

// UpdateIfNotLastAdminDemotion atomically checks that the update would
// not leave the instance without any admin user, then writes it.
// Complements DeleteIfNotLastAdmin (findings.md H11): without this, the
// admin API let an admin demote the only remaining admin and lock the
// instance out of its own role-gated endpoints.
func (s *UserStore) UpdateIfNotLastAdminDemotion(user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	prev, exists := s.users[user.Username]
	if !exists {
		return errors.New(errUserNotFound)
	}

	if prev.Role == RoleAdmin && user.Role != RoleAdmin {
		adminCount := 0
		for _, u := range s.users {
			if u.Role == RoleAdmin {
				adminCount++
			}
		}
		if adminCount <= 1 {
			return errors.New("cannot demote the last admin user")
		}
	}

	s.users[user.Username] = user
	return nil
}

// Delete removes a user
func (s *UserStore) Delete(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[username]; !exists {
		return errors.New(errUserNotFound)
	}

	delete(s.users, username)
	return nil
}

// DeleteIfNotLastAdmin atomically checks that deleting the user would not
// remove the last admin, then deletes. Returns an error if the user is the
// last admin or doesn't exist.
func (s *UserStore) DeleteIfNotLastAdmin(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	target, exists := s.users[username]
	if !exists {
		return errors.New(errUserNotFound)
	}

	if target.Role == RoleAdmin {
		adminCount := 0
		for _, u := range s.users {
			if u.Role == RoleAdmin {
				adminCount++
			}
		}
		if adminCount <= 1 {
			return errors.New("cannot delete the last admin user")
		}
	}

	delete(s.users, username)
	return nil
}

// List returns all users (without password hashes)
func (s *UserStore) List() []*User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]*User, 0, len(s.users))
	for _, user := range s.users {
		// Return copy without password hash; clone Groups so callers
		// cannot mutate the user store's internal slice.
		var groups []string
		if user.Groups != nil {
			groups = append([]string(nil), user.Groups...)
		}
		users = append(users, &User{
			ID:          user.ID,
			Username:    user.Username,
			Role:        user.Role,
			Email:       user.Email,
			DisplayName: user.DisplayName,
			Groups:      groups,
		})
	}
	return users
}

// ListWithHashes returns all users including password hashes.
// This is intended for server-side config persistence only.
func (s *UserStore) ListWithHashes() []*User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]*User, 0, len(s.users))
	for _, user := range s.users {
		var groups []string
		if user.Groups != nil {
			groups = append([]string(nil), user.Groups...)
		}
		users = append(users, &User{
			ID:           user.ID,
			Username:     user.Username,
			PasswordHash: user.PasswordHash,
			Role:         user.Role,
			Email:        user.Email,
			DisplayName:  user.DisplayName,
			Groups:       groups,
		})
	}
	return users
}

// Count returns the number of users
func (s *UserStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.users)
}

// HasAdmin checks if at least one admin user exists
func (s *UserStore) HasAdmin() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, user := range s.users {
		if user.Role == RoleAdmin {
			return true
		}
	}
	return false
}
