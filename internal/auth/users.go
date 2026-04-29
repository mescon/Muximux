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

// timingDummyHash is a pre-computed bcrypt hash used to absorb the
// bcrypt compare cost when the supplied username does not match any
// user. It is generated at bcryptTargetCost so the unknown-user and
// wrong-password paths take the same wall-clock time (findings.md H2).
var timingDummyHash = func() []byte {
	h, err := bcrypt.GenerateFromPassword([]byte("muximux-timing-dummy-not-a-real-password"), bcryptTargetCost)
	if err != nil {
		panic("auth: failed to pre-compute timing dummy hash: " + err.Error())
	}
	return h
}()

// Authenticate verifies username and password. Three failure modes are
// distinguished in the logs so an operator can tell an unknown user
// from a wrong password from a corrupt stored hash, while callers
// still see the same error shape for the two user-facing failures
// (findings.md M3). A successful login silently re-hashes the
// password if the stored hash is below bcryptTargetCost
// (findings.md M4); the rehash is best-effort and does not fail the
// login if it cannot be persisted.
func (s *UserStore) Authenticate(username, password string) (*User, error) {
	user := s.Get(username)
	if user == nil {
		// Perform a throwaway bcrypt compare so the failure path takes
		// roughly as long as a real mismatch. Err is ignored; we always
		// return the same error shape as a real bad-password case.
		_ = bcrypt.CompareHashAndPassword(timingDummyHash, []byte(password))
		logging.Debug("Auth attempt for unknown user", "source", "auth", "username", username)
		return nil, errors.New(errUserNotFound)
	}

	// Pre-check hash validity so a corrupt stored hash does not masquerade
	// as "wrong password" forever. bcrypt.Cost returns an error only for
	// malformed input; a valid hash at any cost returns nil here.
	if _, err := bcrypt.Cost([]byte(user.PasswordHash)); err != nil {
		logging.Warn("Auth attempt against corrupt password hash", "source", "audit", "username", username)
		return nil, errors.New("invalid password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		logging.Debug("Auth attempt: wrong password", "source", "auth", "username", username)
		return nil, errors.New("invalid password")
	}

	s.maybeUpgradeHash(user.Username, user.PasswordHash, password)
	return user, nil
}

// maybeUpgradeHash rehashes the password at bcryptTargetCost in place
// when the stored hash is below the target. Silent: a failure to
// compute or store the new hash does not break the current login.
// Callers are expected to persist the UserStore to disk shortly after
// (syncUsersToConfig runs on password change and user update paths;
// login alone currently does not sync, so the upgraded hash survives
// in memory until the next config save).
func (s *UserStore) maybeUpgradeHash(username, oldHash, password string) {
	cost, err := bcrypt.Cost([]byte(oldHash))
	if err != nil || cost >= bcryptTargetCost {
		return
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptTargetCost)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if u, ok := s.users[username]; ok && u.PasswordHash == oldHash {
		u.PasswordHash = string(newHash)
	}
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
