package auth

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"
	"time"
)

// Session represents a user session
type Session struct {
	ID        string
	UserID    string
	Username  string
	Role      string
	CreatedAt time.Time
	ExpiresAt time.Time
	Data      map[string]interface{}
}

// IsExpired checks if the session has expired
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// SessionStore manages sessions
type SessionStore struct {
	sessions   map[string]*Session
	mu         sync.RWMutex
	cookieName string
	maxAge     time.Duration
	secure     bool
}

// NewSessionStore creates a new session store
func NewSessionStore(cookieName string, maxAge time.Duration, secure bool) *SessionStore {
	store := &SessionStore{
		sessions:   make(map[string]*Session),
		cookieName: cookieName,
		maxAge:     maxAge,
		secure:     secure,
	}

	// Start cleanup goroutine
	go store.cleanup()

	return store
}

// generateSessionID creates a cryptographically secure session ID
func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// Create creates a new session for a user
func (s *SessionStore) Create(userID, username, role string) (*Session, error) {
	id, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	session := &Session{
		ID:        id,
		UserID:    userID,
		Username:  username,
		Role:      role,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(s.maxAge),
		Data:      make(map[string]interface{}),
	}

	s.mu.Lock()
	s.sessions[id] = session
	s.mu.Unlock()

	return session, nil
}

// Get retrieves a session by ID
func (s *SessionStore) Get(id string) *Session {
	s.mu.RLock()
	session, exists := s.sessions[id]
	s.mu.RUnlock()

	if !exists || session.IsExpired() {
		return nil
	}

	return session
}

// Delete removes a session
func (s *SessionStore) Delete(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

// DeleteByUserID removes all sessions for a user, optionally excluding one session
func (s *SessionStore) DeleteByUserID(userID string, exceptSessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, session := range s.sessions {
		if session.UserID == userID && id != exceptSessionID {
			delete(s.sessions, id)
		}
	}
}

// Refresh extends the session expiration
func (s *SessionStore) Refresh(id string) {
	s.mu.Lock()
	if session, exists := s.sessions[id]; exists {
		session.ExpiresAt = time.Now().Add(s.maxAge)
	}
	s.mu.Unlock()
}

// GetFromRequest extracts session from HTTP request cookie
func (s *SessionStore) GetFromRequest(r *http.Request) *Session {
	cookie, err := r.Cookie(s.cookieName)
	if err != nil {
		return nil
	}
	return s.Get(cookie.Value)
}

// SetCookie sets the session cookie on the response
func (s *SessionStore) SetCookie(w http.ResponseWriter, session *Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookieName,
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(s.maxAge.Seconds()),
	})
}

// ClearCookie removes the session cookie
func (s *SessionStore) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// cleanup periodically removes expired sessions
func (s *SessionStore) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		for id, session := range s.sessions {
			if session.IsExpired() {
				delete(s.sessions, id)
			}
		}
		s.mu.Unlock()
	}
}

// Count returns the number of active sessions
func (s *SessionStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}
