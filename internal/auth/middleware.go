package auth

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
)

// ContextKey is a type for context keys
type ContextKey string

const (
	// ContextKeyUser is the context key for the authenticated user
	ContextKeyUser ContextKey = "user"
	// ContextKeySession is the context key for the session
	ContextKeySession ContextKey = "session"
)

// AuthMethod defines the authentication method
type AuthMethod string

const (
	AuthMethodNone        AuthMethod = "none"
	AuthMethodBuiltin     AuthMethod = "builtin"
	AuthMethodForwardAuth AuthMethod = "forward_auth"
	AuthMethodOIDC        AuthMethod = "oidc"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Method         AuthMethod
	TrustedProxies []string
	Headers        ForwardAuthHeaders
	BypassRules    []BypassRule
}

// ForwardAuthHeaders defines the header names for forward auth
type ForwardAuthHeaders struct {
	User   string // Default: Remote-User
	Email  string // Default: Remote-Email
	Groups string // Default: Remote-Groups
	Name   string // Default: Remote-Name
}

// BypassRule defines a rule for bypassing authentication
type BypassRule struct {
	Path          string
	Methods       []string
	RequireAPIKey bool
	AllowedIPs    []string
}

// Middleware provides authentication middleware
type Middleware struct {
	config       AuthConfig
	sessionStore *SessionStore
	userStore    *UserStore
	trustedNets  []*net.IPNet
}

// NewMiddleware creates a new auth middleware
func NewMiddleware(config AuthConfig, sessionStore *SessionStore, userStore *UserStore) *Middleware {
	m := &Middleware{
		config:       config,
		sessionStore: sessionStore,
		userStore:    userStore,
	}

	// Parse trusted proxy networks
	for _, cidr := range config.TrustedProxies {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			// Try as single IP
			ip := net.ParseIP(cidr)
			if ip != nil {
				if ip4 := ip.To4(); ip4 != nil {
					_, network, _ = net.ParseCIDR(cidr + "/32")
				} else {
					_, network, _ = net.ParseCIDR(cidr + "/128")
				}
			}
		}
		if network != nil {
			m.trustedNets = append(m.trustedNets, network)
		}
	}

	return m
}

// RequireAuth returns middleware that requires authentication
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if auth is disabled
		if m.config.Method == AuthMethodNone {
			next.ServeHTTP(w, r)
			return
		}

		// Check bypass rules
		if m.shouldBypass(r) {
			next.ServeHTTP(w, r)
			return
		}

		// Authenticate based on method
		var user *User
		var session *Session

		switch m.config.Method {
		case AuthMethodBuiltin:
			session = m.sessionStore.GetFromRequest(r)
			if session != nil {
				user = m.userStore.GetByID(session.UserID)
			}

		case AuthMethodForwardAuth:
			user = m.authenticateForwardAuth(r)

		case AuthMethodOIDC:
			// OIDC handled separately via callback
			session = m.sessionStore.GetFromRequest(r)
			if session != nil {
				user = m.userStore.GetByID(session.UserID)
			}
		}

		if user == nil {
			m.handleUnauthenticated(w, r)
			return
		}

		// Add user and session to context
		ctx := context.WithValue(r.Context(), ContextKeyUser, user)
		if session != nil {
			ctx = context.WithValue(ctx, ContextKeySession, session)
			// Refresh session on activity
			m.sessionStore.Refresh(session.ID)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole returns middleware that requires a specific role
func (m *Middleware) RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUserFromContext(r.Context())
			if user == nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			for _, role := range roles {
				if user.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, "Forbidden", http.StatusForbidden)
		})
	}
}

// shouldBypass checks if the request should bypass authentication
func (m *Middleware) shouldBypass(r *http.Request) bool {
	for _, rule := range m.config.BypassRules {
		if m.matchBypassRule(r, rule) {
			return true
		}
	}
	return false
}

// matchBypassRule checks if a request matches a bypass rule
func (m *Middleware) matchBypassRule(r *http.Request, rule BypassRule) bool {
	// Check path
	if rule.Path != "" {
		if strings.HasSuffix(rule.Path, "*") {
			prefix := strings.TrimSuffix(rule.Path, "*")
			if !strings.HasPrefix(r.URL.Path, prefix) {
				return false
			}
		} else if r.URL.Path != rule.Path {
			return false
		}
	}

	// Check method
	if len(rule.Methods) > 0 {
		methodMatch := false
		for _, method := range rule.Methods {
			if strings.EqualFold(r.Method, method) {
				methodMatch = true
				break
			}
		}
		if !methodMatch {
			return false
		}
	}

	// Check API key requirement
	if rule.RequireAPIKey {
		if r.Header.Get("X-Api-Key") == "" {
			return false
		}
	}

	// Check IP allowlist
	if len(rule.AllowedIPs) > 0 {
		clientIP := m.getClientIP(r)
		ipAllowed := false
		for _, allowed := range rule.AllowedIPs {
			if clientIP == allowed {
				ipAllowed = true
				break
			}
			// Check CIDR
			_, network, err := net.ParseCIDR(allowed)
			if err == nil && network.Contains(net.ParseIP(clientIP)) {
				ipAllowed = true
				break
			}
		}
		if !ipAllowed {
			return false
		}
	}

	return true
}

// authenticateForwardAuth extracts user info from forward auth headers
func (m *Middleware) authenticateForwardAuth(r *http.Request) *User {
	// Verify request is from trusted proxy
	if !m.isFromTrustedProxy(r) {
		log.Printf("Forward auth request not from trusted proxy: %s", m.getClientIP(r))
		return nil
	}

	// Get header names (with defaults)
	userHeader := m.config.Headers.User
	if userHeader == "" {
		userHeader = "Remote-User"
	}
	emailHeader := m.config.Headers.Email
	if emailHeader == "" {
		emailHeader = "Remote-Email"
	}
	nameHeader := m.config.Headers.Name
	if nameHeader == "" {
		nameHeader = "Remote-Name"
	}
	groupsHeader := m.config.Headers.Groups
	if groupsHeader == "" {
		groupsHeader = "Remote-Groups"
	}

	// Extract user info
	username := r.Header.Get(userHeader)
	if username == "" {
		return nil
	}

	email := r.Header.Get(emailHeader)
	displayName := r.Header.Get(nameHeader)
	groups := r.Header.Get(groupsHeader)

	// Determine role from groups
	role := RoleUser
	if groups != "" {
		groupList := strings.Split(groups, ",")
		for _, g := range groupList {
			g = strings.TrimSpace(g)
			if g == "admin" || g == "admins" || g == "administrators" {
				role = RoleAdmin
				break
			}
		}
	}

	// Return user (auto-created from headers)
	return &User{
		ID:          username,
		Username:    username,
		Email:       email,
		DisplayName: displayName,
		Role:        role,
	}
}

// isFromTrustedProxy checks if the request is from a trusted proxy
func (m *Middleware) isFromTrustedProxy(r *http.Request) bool {
	if len(m.trustedNets) == 0 {
		// No trusted proxies configured, trust all (not recommended)
		return true
	}

	clientIP := net.ParseIP(m.getClientIP(r))
	if clientIP == nil {
		return false
	}

	for _, network := range m.trustedNets {
		if network.Contains(clientIP) {
			return true
		}
	}

	return false
}

// getClientIP extracts the client IP from the request
func (m *Middleware) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For first (if from trusted proxy)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// handleUnauthenticated handles unauthenticated requests
func (m *Middleware) handleUnauthenticated(w http.ResponseWriter, r *http.Request) {
	// Check if it's an API request
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Redirect to login page for browser requests
	http.Redirect(w, r, "/login", http.StatusFound)
}

// GetUserFromContext extracts the user from request context
func GetUserFromContext(ctx context.Context) *User {
	user, _ := ctx.Value(ContextKeyUser).(*User)
	return user
}

// GetSessionFromContext extracts the session from request context
func GetSessionFromContext(ctx context.Context) *Session {
	session, _ := ctx.Value(ContextKeySession).(*Session)
	return session
}
