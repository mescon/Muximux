package auth

import (
	"context"
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
	"sync/atomic"

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

// authSnapshot is a point-in-time copy of config and trusted networks,
// captured once under the lock at the start of each request.
type authSnapshot struct {
	config      AuthConfig
	trustedNets []*net.IPNet
}

// Middleware provides authentication middleware
type Middleware struct {
	snap         atomic.Pointer[authSnapshot]
	sessionStore *SessionStore
	userStore    *UserStore
}

// NewMiddleware creates a new auth middleware
func NewMiddleware(config *AuthConfig, sessionStore *SessionStore, userStore *UserStore) *Middleware {
	m := &Middleware{
		sessionStore: sessionStore,
		userStore:    userStore,
	}
	m.snap.Store(&authSnapshot{
		config:      *config,
		trustedNets: parseTrustedProxies(config.TrustedProxies),
	})
	return m
}

// UpdateConfig replaces the auth configuration and re-parses trusted proxy networks.
func (m *Middleware) UpdateConfig(config *AuthConfig) {
	trustedNets := parseTrustedProxies(config.TrustedProxies)
	m.snap.Store(&authSnapshot{
		config:      *config,
		trustedNets: trustedNets,
	})
	logging.Info("Auth config updated", "source", "auth", "method", string(config.Method))
}

// parseTrustedProxies converts a list of CIDR strings or IP addresses into net.IPNet entries.
func parseTrustedProxies(proxies []string) []*net.IPNet {
	var nets []*net.IPNet
	for _, cidr := range proxies {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			ip := net.ParseIP(cidr)
			if ip != nil {
				suffix := "/128"
				if ip.To4() != nil {
					suffix = "/32"
				}
				_, network, _ = net.ParseCIDR(cidr + suffix)
			}
		}
		if network != nil {
			nets = append(nets, network)
		}
	}
	return nets
}

// snapshot returns the current auth config snapshot (lock-free via atomic.Pointer).
func (m *Middleware) snapshot() *authSnapshot {
	return m.snap.Load()
}

// RequireAuth returns middleware that requires authentication
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		snap := m.snapshot()

		// Check if auth is disabled — inject virtual admin so downstream
		// handlers (e.g. RequireRole) always find a user in context.
		if snap.config.Method == AuthMethodNone {
			virtualAdmin := &User{ID: "admin", Username: "admin", Role: RoleAdmin}
			ctx := context.WithValue(r.Context(), ContextKeyUser, virtualAdmin)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Check bypass rules — still attempt best-effort auth so that
		// bypassed endpoints (e.g. /api/auth/status) can see the user.
		if shouldBypass(r, snap) {
			logging.Debug("Auth bypassed", "source", "auth", "path", r.URL.Path)
			user, session := m.authenticateRequest(r, snap)
			if user != nil {
				ctx := context.WithValue(r.Context(), ContextKeyUser, user)
				if session != nil {
					ctx = context.WithValue(ctx, ContextKeySession, session)
					m.sessionStore.Refresh(session.ID)
				}
				next.ServeHTTP(w, r.WithContext(ctx))
			} else {
				next.ServeHTTP(w, r)
			}
			return
		}

		user, session := m.authenticateRequest(r, snap)
		if user == nil {
			logging.Debug("Unauthenticated request", "source", "auth", "path", r.URL.Path, "method", r.Method)
			handleUnauthenticated(w, r, snap)
			return
		}

		logging.Debug("Authenticated request", "source", "auth", "user", user.Username, "path", r.URL.Path)

		ctx := context.WithValue(r.Context(), ContextKeyUser, user)
		if session != nil {
			ctx = context.WithValue(ctx, ContextKeySession, session)
			m.sessionStore.Refresh(session.ID)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authenticateRequest attempts to authenticate the request using the snapshotted config.
func (m *Middleware) authenticateRequest(r *http.Request, snap *authSnapshot) (*User, *Session) {
	switch snap.config.Method {
	case AuthMethodBuiltin, AuthMethodOIDC:
		session := m.sessionStore.GetFromRequest(r)
		if session != nil {
			user := m.userStore.GetByID(session.UserID)
			return user, session
		}
		return nil, nil

	case AuthMethodForwardAuth:
		return authenticateForwardAuth(r, snap), nil

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

			logging.Warn("Access denied: insufficient role", "source", "auth", "user", user.Username, "role", user.Role, "path", r.URL.Path)
			http.Error(w, "Forbidden", http.StatusForbidden)
		})
	}
}

func shouldBypass(r *http.Request, snap *authSnapshot) bool {
	for _, rule := range snap.config.BypassRules {
		if matchBypassRule(r, rule, snap) {
			return true
		}
	}
	return false
}

func matchBypassRule(r *http.Request, rule BypassRule, snap *authSnapshot) bool {
	return matchPath(r.URL.Path, rule) &&
		matchMethod(r.Method, rule) &&
		matchAPIKey(r, rule, snap) &&
		matchAllowedIPs(r, rule, snap)
}

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

func matchAPIKey(r *http.Request, rule BypassRule, snap *authSnapshot) bool {
	if !rule.RequireAPIKey {
		return true
	}
	provided := r.Header.Get("X-Api-Key")
	if provided == "" || snap.config.APIKey == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(snap.config.APIKey)) == 1
}

func matchAllowedIPs(r *http.Request, rule BypassRule, snap *authSnapshot) bool {
	if len(rule.AllowedIPs) == 0 {
		return true
	}
	clientIP := getClientIP(r, snap)
	for _, allowed := range rule.AllowedIPs {
		if clientIP == allowed {
			return true
		}
		_, network, err := net.ParseCIDR(allowed)
		if err == nil && network.Contains(net.ParseIP(clientIP)) {
			return true
		}
	}
	return false
}

func authenticateForwardAuth(r *http.Request, snap *authSnapshot) *User {
	if !isFromTrustedProxy(r, snap) {
		logging.Warn("Forward auth request not from trusted proxy", "source", "auth", "client_ip", getClientIP(r, snap))
		return nil
	}

	userHeader := headerWithDefault(snap.config.Headers.User, "Remote-User")
	emailHeader := headerWithDefault(snap.config.Headers.Email, "Remote-Email")
	nameHeader := headerWithDefault(snap.config.Headers.Name, "Remote-Name")
	groupsHeader := headerWithDefault(snap.config.Headers.Groups, "Remote-Groups")

	username := r.Header.Get(userHeader)
	if username == "" {
		return nil
	}

	email := r.Header.Get(emailHeader)
	displayName := r.Header.Get(nameHeader)
	groups := r.Header.Get(groupsHeader)

	role := RoleUser
	if groups != "" {
		for _, g := range strings.Split(groups, ",") {
			g = strings.TrimSpace(g)
			if g == "admin" || g == "admins" || g == "administrators" {
				role = RoleAdmin
				break
			}
		}
	}

	return &User{
		ID:          username,
		Username:    username,
		Email:       email,
		DisplayName: displayName,
		Role:        role,
	}
}

func isFromTrustedProxy(r *http.Request, snap *authSnapshot) bool {
	if len(snap.trustedNets) == 0 {
		logging.Warn("Forward auth enabled but no trusted_proxies configured; rejecting request", "source", "auth")
		return false
	}

	directIP := net.ParseIP(getDirectIP(r))
	if directIP == nil {
		return false
	}

	for _, network := range snap.trustedNets {
		if network.Contains(directIP) {
			return true
		}
	}

	return false
}

func getDirectIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func getClientIP(r *http.Request, snap *authSnapshot) string {
	if isFromTrustedProxy(r, snap) {
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

	return getDirectIP(r)
}

func handleUnauthenticated(w http.ResponseWriter, r *http.Request, snap *authSnapshot) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	http.Redirect(w, r, snap.config.BasePath+"/login", http.StatusFound)
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

func headerWithDefault(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
