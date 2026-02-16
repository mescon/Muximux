package auth

import (
	"context"
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/mescon/muximux/v3/internal/logging"
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
	APIKey         string
	BasePath       string // e.g. "/muximux" — prepended to login redirect
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
	mu           sync.RWMutex
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

// UpdateConfig replaces the auth configuration and re-parses trusted proxy networks.
func (m *Middleware) UpdateConfig(config AuthConfig) {
	var trustedNets []*net.IPNet
	for _, cidr := range config.TrustedProxies {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
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
			trustedNets = append(trustedNets, network)
		}
	}

	m.mu.Lock()
	m.config = config
	m.trustedNets = trustedNets
	m.mu.Unlock()
}

// RequireAuth returns middleware that requires authentication
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.RLock()
		method := m.config.Method
		m.mu.RUnlock()

		// Check if auth is disabled — inject virtual admin so downstream
		// handlers (e.g. RequireRole) always find a user in context.
		if method == AuthMethodNone {
			virtualAdmin := &User{ID: "admin", Username: "admin", Role: RoleAdmin}
			ctx := context.WithValue(r.Context(), ContextKeyUser, virtualAdmin)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Check bypass rules
		if m.shouldBypass(r) {
			logging.Debug("Auth bypassed", "source", "auth", "path", r.URL.Path)
			next.ServeHTTP(w, r)
			return
		}

		user, session := m.authenticateRequest(r)
		if user == nil {
			logging.Debug("Unauthenticated request", "source", "auth", "path", r.URL.Path, "method", r.Method)
			m.handleUnauthenticated(w, r)
			return
		}

		logging.Debug("Authenticated request", "source", "auth", "user", user.Username, "path", r.URL.Path)

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

// authenticateRequest attempts to authenticate the request based on the configured auth method.
// Returns the authenticated user and session (if applicable).
func (m *Middleware) authenticateRequest(r *http.Request) (*User, *Session) {
	switch m.config.Method {
	case AuthMethodBuiltin, AuthMethodOIDC:
		session := m.sessionStore.GetFromRequest(r)
		if session != nil {
			user := m.userStore.GetByID(session.UserID)
			return user, session
		}
		return nil, nil

	case AuthMethodForwardAuth:
		return m.authenticateForwardAuth(r), nil

	default:
		return nil, nil
	}
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
	if !matchPath(r.URL.Path, rule) {
		return false
	}
	if !matchMethod(r.Method, rule) {
		return false
	}
	if !m.matchAPIKey(r, rule) {
		return false
	}
	if !m.matchAllowedIPs(r, rule) {
		return false
	}
	return true
}

// matchPath checks if the request path matches the rule path (supports wildcard suffix)
func matchPath(requestPath string, rule BypassRule) bool {
	if rule.Path == "" {
		return true
	}
	if strings.HasSuffix(rule.Path, "*") {
		prefix := strings.TrimSuffix(rule.Path, "*")
		return strings.HasPrefix(requestPath, prefix)
	}
	return requestPath == rule.Path
}

// matchMethod checks if the HTTP method matches the rule methods
func matchMethod(method string, rule BypassRule) bool {
	if len(rule.Methods) == 0 {
		return true
	}
	for _, m := range rule.Methods {
		if strings.EqualFold(method, m) {
			return true
		}
	}
	return false
}

// matchAPIKey verifies the API key using constant-time comparison
func (m *Middleware) matchAPIKey(r *http.Request, rule BypassRule) bool {
	if !rule.RequireAPIKey {
		return true
	}
	provided := r.Header.Get("X-Api-Key")
	if provided == "" || m.config.APIKey == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(m.config.APIKey)) == 1
}

// matchAllowedIPs checks the IP allowlist with CIDR support
func (m *Middleware) matchAllowedIPs(r *http.Request, rule BypassRule) bool {
	if len(rule.AllowedIPs) == 0 {
		return true
	}
	clientIP := m.getClientIP(r)
	for _, allowed := range rule.AllowedIPs {
		if clientIP == allowed {
			return true
		}
		// Check CIDR
		_, network, err := net.ParseCIDR(allowed)
		if err == nil && network.Contains(net.ParseIP(clientIP)) {
			return true
		}
	}
	return false
}

// authenticateForwardAuth extracts user info from forward auth headers
func (m *Middleware) authenticateForwardAuth(r *http.Request) *User {
	// Verify request is from trusted proxy
	if !m.isFromTrustedProxy(r) {
		logging.Warn("Forward auth request not from trusted proxy", "source", "auth", "client_ip", m.getClientIP(r))
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
// Uses the direct connection IP (RemoteAddr), not forwarded headers
func (m *Middleware) isFromTrustedProxy(r *http.Request) bool {
	if len(m.trustedNets) == 0 {
		// No trusted proxies configured — fail closed for security
		logging.Warn("Forward auth enabled but no trusted_proxies configured; rejecting request", "source", "auth")
		return false
	}

	directIP := net.ParseIP(m.getDirectIP(r))
	if directIP == nil {
		return false
	}

	for _, network := range m.trustedNets {
		if network.Contains(directIP) {
			return true
		}
	}

	return false
}

// getDirectIP returns the IP from RemoteAddr (the actual TCP connection, not forwarded headers)
func (m *Middleware) getDirectIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// getClientIP extracts the client IP from the request.
// Only trusts X-Forwarded-For / X-Real-IP when the direct connection is from a trusted proxy.
func (m *Middleware) getClientIP(r *http.Request) string {
	// Only trust forwarded headers from verified trusted proxies
	if m.isFromTrustedProxy(r) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			ips := strings.Split(xff, ",")
			if len(ips) > 0 {
				return strings.TrimSpace(ips[0])
			}
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return xri
		}
	}

	// Fall back to direct connection IP
	return m.getDirectIP(r)
}

// handleUnauthenticated handles unauthenticated requests
func (m *Middleware) handleUnauthenticated(w http.ResponseWriter, r *http.Request) {
	// Check if it's an API request
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Redirect to login page for browser requests
	http.Redirect(w, r, m.config.BasePath+"/login", http.StatusFound)
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
