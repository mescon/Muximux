package auth

import (
	"errors"
	"sync"

	"golang.org/x/crypto/bcrypt"
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
}

// UserConfig represents user configuration from YAML
type UserConfig struct {
	Username     string `yaml:"username"`
	PasswordHash string `yaml:"password_hash"`
	Role         string `yaml:"role"`
	Email        string `yaml:"email,omitempty"`
	DisplayName  string `yaml:"display_name,omitempty"`
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
		}
	}
}

// Get retrieves a user by username
func (s *UserStore) Get(username string) *User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.users[username]
}

// GetByID retrieves a user by ID
func (s *UserStore) GetByID(id string) *User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// For now, ID == username
	return s.users[id]
}

// Authenticate verifies username and password
func (s *UserStore) Authenticate(username, password string) (*User, error) {
	user := s.Get(username)
	if user == nil {
		return nil, errors.New(errUserNotFound)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid password")
	}

	return user, nil
}

// HashPassword creates a bcrypt hash of a password
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
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

// Update updates an existing user
func (s *UserStore) Update(user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[user.Username]; !exists {
		return errors.New(errUserNotFound)
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

// List returns all users (without password hashes)
func (s *UserStore) List() []*User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]*User, 0, len(s.users))
	for _, user := range s.users {
		// Return copy without password hash
		users = append(users, &User{
			ID:          user.ID,
			Username:    user.Username,
			Role:        user.Role,
			Email:       user.Email,
			DisplayName: user.DisplayName,
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
		users = append(users, &User{
			ID:           user.ID,
			Username:     user.Username,
			PasswordHash: user.PasswordHash,
			Role:         user.Role,
			Email:        user.Email,
			DisplayName:  user.DisplayName,
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
