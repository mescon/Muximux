package auth

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"
	"time"

	"github.com/mescon/muximux/v3/internal/logging"
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
	sessions       map[string]*Session
	mu             sync.RWMutex
	cookieName     string
	cookieDomain   string // empty = host-scoped; non-empty applies Domain= attribute so the cookie crosses subdomains
	maxAge         time.Duration
	absoluteMaxAge time.Duration // hard cap on total session lifetime; 0 disables the cap
	secure         bool
	done           chan struct{}
}

// SetCookieDomain configures the Domain attribute applied to every
// session cookie this store issues from now on. Empty (the default)
// keeps cookies host-scoped to the dashboard, which is correct for
// single-domain deployments. Set to ".example.com" (or "example.com",
// browser normalises) when the gateway auth gate is in use so the
// session cookie is visible at all gated subdomains.
//
// Existing in-flight cookies are unaffected; the change takes effect
// on the next Create / SetCookie call.
func (s *SessionStore) SetCookieDomain(domain string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cookieDomain = domain
}

// defaultSessionAbsoluteMaxAge is the wall-clock maximum lifetime of any
// session, irrespective of how often Refresh extends it. Prevents the
// "infinite rolling session for any active cookie" outcome flagged in
// findings.md H21.
const defaultSessionAbsoluteMaxAge = 7 * 24 * time.Hour

// NewSessionStore creates a new session store
func NewSessionStore(cookieName string, maxAge time.Duration, secure bool) *SessionStore {
	absolute := defaultSessionAbsoluteMaxAge
	if maxAge > absolute {
		// A rolling session cannot usefully live longer than the hard
		// cap, so use the rolling window as the ceiling when that is
		// larger (operator has deliberately configured a long session).
		absolute = maxAge
	}
	store := &SessionStore{
		sessions:       make(map[string]*Session),
		cookieName:     cookieName,
		maxAge:         maxAge,
		absoluteMaxAge: absolute,
		secure:         secure,
		done:           make(chan struct{}),
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

// Get retrieves a session by ID, returning a copy.
func (s *SessionStore) Get(id string) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[id]
	if !exists || session.IsExpired() {
		return nil
	}

	// Return a copy so callers don't race with Refresh() on the live session.
	copy := *session
	return &copy
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

// Refresh extends the session expiration. The extension is capped by the
// store's absolute maximum lifetime (CreatedAt + absoluteMaxAge), so an
// attacker who steals an active cookie cannot keep it alive forever just
// by making a request before each expiration.
func (s *SessionStore) Refresh(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, exists := s.sessions[id]
	if !exists {
		return
	}
	newExpiry := time.Now().Add(s.maxAge)
	if s.absoluteMaxAge > 0 {
		deadline := session.CreatedAt.Add(s.absoluteMaxAge)
		if newExpiry.After(deadline) {
			newExpiry = deadline
		}
	}
	session.ExpiresAt = newExpiry
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
	s.mu.RLock()
	domain := s.cookieDomain
	s.mu.RUnlock()
	// gosec G124: Secure is intentionally configurable via secure_cookies so
	// Muximux can run behind plain HTTP (documented in security.md). HttpOnly
	// and SameSite are always set; operators on HTTPS set secure_cookies: true.
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure is operator-configurable by design
		Name:     s.cookieName,
		Value:    session.ID,
		Path:     "/",
		Domain:   domain,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(s.maxAge.Seconds()),
	})
}

// ClearCookie removes the session cookie. Must mirror SetCookie's
// Domain attribute exactly or browsers will refuse to clear the
// original cookie (host-scoped clear of a domain-scoped cookie is
// silently ignored).
func (s *SessionStore) ClearCookie(w http.ResponseWriter) {
	s.mu.RLock()
	domain := s.cookieDomain
	s.mu.RUnlock()
	// gosec G124: mirrors SetCookie - Secure tracks the operator-configured
	// secure_cookies flag by design; HttpOnly and SameSite are always set.
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure is operator-configurable by design
		Name:     s.cookieName,
		Value:    "",
		Path:     "/",
		Domain:   domain,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// Close stops the cleanup goroutine. Safe to call multiple times -
// guards close(s.done) with a select on its already-closed shape so a
// double-Stop (test harnesses, future "stop on second SIGINT" paths)
// does not panic. Mirrors websocket.Hub.Close.
func (s *SessionStore) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.done:
		// already closed
	default:
		close(s.done)
	}
}

// cleanup periodically removes expired sessions
func (s *SessionStore) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			count := 0
			for id, session := range s.sessions {
				if session.IsExpired() {
					delete(s.sessions, id)
					count++
				}
			}
			s.mu.Unlock()
			if count > 0 {
				logging.Debug("Session cleanup", "source", "auth", "expired", count)
			}
		case <-s.done:
			return
		}
	}
}

// Count returns the number of active sessions
func (s *SessionStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}
