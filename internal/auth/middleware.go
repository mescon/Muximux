package auth

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync/atomic"

	"golang.org/x/crypto/bcrypt"

	"github.com/mescon/muximux/v3/internal/logging"
)

// ContextKey is a type for context keys
type ContextKey string

const (
	// ContextKeyUser is the context key for the authenticated user
	ContextKeyUser ContextKey = "user"
	// ContextKeySession is the context key for the session
	ContextKeySession ContextKey = "session"
	// ContextKeyClientIP is the context key for the real client IP
	// (resolved from X-Forwarded-For / X-Real-IP when behind a trusted proxy).
	ContextKeyClientIP ContextKey = "client_ip"
	// ContextKeyClientScheme is the context key for the scheme the client
	// used to reach Muximux ("http" or "https"). Derived from r.TLS and,
	// when the request arrived from a trusted proxy, X-Forwarded-Proto.
	ContextKeyClientScheme ContextKey = "client_scheme"
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
	APIKeyHash     string // bcrypt hash of API key
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

// parseTrustedProxies converts a list of CIDR strings or IP addresses into
// net.IPNet entries. Invalid entries are logged and skipped so a
// misconfiguration is visible rather than silently widening or narrowing the
// trust boundary.
func parseTrustedProxies(proxies []string) []*net.IPNet {
	var nets []*net.IPNet
	for _, entry := range proxies {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		_, network, err := net.ParseCIDR(trimmed)
		if err != nil {
			ip := net.ParseIP(trimmed)
			if ip == nil {
				logging.Warn("Invalid trusted_proxy entry ignored", "source", "auth", "value", entry)
				continue
			}
			suffix := "/128"
			if ip.To4() != nil {
				suffix = "/32"
			}
			_, network, err = net.ParseCIDR(trimmed + suffix)
			if err != nil {
				logging.Warn("Invalid trusted_proxy entry ignored", "source", "auth", "value", entry, "error", err)
				continue
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

// ResolveClientIP returns middleware that stores the real client IP and the
// client-facing scheme (http/https) in the request context. It must be
// chained OUTSIDE the logging middleware so that log entries see the
// resolved IP instead of the proxy's address. Downstream code that needs to
// decide cookie Secure flags or build redirects should read the scheme from
// context rather than trusting X-Forwarded-Proto directly.
func (m *Middleware) ResolveClientIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		snap := m.snapshot()
		ctx := r.Context()
		ctx = context.WithValue(ctx, ContextKeyClientIP, getClientIP(r, snap))
		ctx = context.WithValue(ctx, ContextKeyClientScheme, getClientScheme(r, snap))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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
			ctx = logging.SetUser(ctx, virtualAdmin.Username)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Check bypass rules — still attempt best-effort auth so that
		// bypassed endpoints (e.g. /api/auth/status) can see the user.
		if shouldBypass(r, snap) {
			logging.From(r.Context()).Debug("Auth bypassed", "source", "auth", "path", r.URL.Path)
			user, session := m.authenticateRequest(r, snap)
			if user != nil {
				ctx := context.WithValue(r.Context(), ContextKeyUser, user)
				ctx = logging.SetUser(ctx, user.Username)
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
			logging.From(r.Context()).Debug("Unauthenticated request", "source", "auth", "path", r.URL.Path, "method", r.Method)
			handleUnauthenticated(w, r, snap)
			return
		}

		logging.From(r.Context()).Debug("Authenticated request", "source", "auth", "user", user.Username, "path", r.URL.Path)

		ctx := context.WithValue(r.Context(), ContextKeyUser, user)
		ctx = logging.SetUser(ctx, user.Username)
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
			if user == nil {
				// Session-only user (e.g. OIDC) — not in the config-based
				// UserStore. Reconstruct from session data.
				user = userFromSession(session)
			}
			return user, session
		}
		return nil, nil

	case AuthMethodForwardAuth:
		return authenticateForwardAuth(r, snap), nil

	default:
		return nil, nil
	}
}

// userFromSession constructs a User from session data. This is used for
// session-only users (OIDC) that aren't persisted to the config-based UserStore.
func userFromSession(s *Session) *User {
	u := &User{
		ID:       s.UserID,
		Username: s.Username,
		Role:     s.Role,
	}
	if email, ok := s.Data["email"].(string); ok {
		u.Email = email
	}
	if dn, ok := s.Data["display_name"].(string); ok {
		u.DisplayName = dn
	}
	return u
}

// RequireRole returns middleware that requires a specific role
func (m *Middleware) RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUserFromContext(r.Context())
			if user == nil {
				logging.From(r.Context()).Warn("Access denied: no user in context", "source", "auth", "path", r.URL.Path)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			for _, role := range roles {
				if user.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}

			logging.From(r.Context()).Warn("Access denied: insufficient role", "source", "auth", "user", user.Username, "role", user.Role, "path", r.URL.Path)
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
	if provided == "" || snap.config.APIKeyHash == "" {
		return false
	}
	if bcrypt.CompareHashAndPassword([]byte(snap.config.APIKeyHash), []byte(provided)) != nil {
		logging.From(r.Context()).Info("API key authentication failed", "source", "audit", "path", r.URL.Path)
		return false
	}
	return true
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
		logging.From(r.Context()).Warn("Forward auth request not from trusted proxy", "source", "auth", "client_ip", getClientIP(r, snap))
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
			// Case-insensitive compare to match OIDC's behaviour.
			// Authelia/Authentik etc. tend to preserve the operator's
			// configured casing ("Admins", "ADMIN"), and a silently
			// case-sensitive check here was a common misconfig trap
			// (findings.md M2).
			g = strings.ToLower(strings.TrimSpace(g))
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
		logging.From(r.Context()).Warn("Forward auth enabled but no trusted_proxies configured; rejecting request", "source", "auth")
		return false
	}
	return ipIsTrusted(getDirectIP(r), snap)
}

// ipIsTrusted reports whether the given IP string falls within any of the
// configured trusted-proxy networks. Used both for direct-hop checks and for
// walking the X-Forwarded-For chain.
func ipIsTrusted(ipStr string, snap *authSnapshot) bool {
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil {
		return false
	}
	for _, network := range snap.trustedNets {
		if network.Contains(ip) {
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

// GetClientIP returns the real client IP for the given request,
// respecting X-Forwarded-For / X-Real-IP when the request arrives
// from a trusted proxy.
func (m *Middleware) GetClientIP(r *http.Request) string {
	return getClientIP(r, m.snapshot())
}

// getClientIP resolves the real client IP by walking X-Forwarded-For from
// right to left, skipping hops that are themselves trusted proxies. The
// first untrusted hop is the real client. This prevents an attacker from
// forging their own IP by prepending values to X-Forwarded-For; the leftmost
// (attacker-controlled) entry is only returned when every upstream hop is
// inside the configured trusted_proxies set, which is the expected layout.
func getClientIP(r *http.Request, snap *authSnapshot) string {
	direct := getDirectIP(r)
	if !ipIsTrusted(direct, snap) {
		return direct
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			ip := strings.TrimSpace(parts[i])
			if ip == "" {
				continue
			}
			if !ipIsTrusted(ip, snap) {
				return ip
			}
		}
		// Every XFF hop is a trusted proxy; the leftmost hop is the
		// closest thing we have to the originator.
		for _, part := range parts {
			if ip := strings.TrimSpace(part); ip != "" {
				return ip
			}
		}
	}

	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		return xri
	}

	return direct
}

// getClientScheme returns "https" if the client reached Muximux over TLS,
// either directly or via a trusted proxy that reported it with
// X-Forwarded-Proto. Returns "http" otherwise. An X-Forwarded-Proto header
// from an untrusted direct caller is ignored.
func getClientScheme(r *http.Request, snap *authSnapshot) string {
	if r.TLS != nil {
		return "https"
	}
	if ipIsTrusted(getDirectIP(r), snap) {
		if xfp := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); xfp != "" {
			if strings.EqualFold(xfp, "https") {
				return "https"
			}
			return "http"
		}
	}
	return "http"
}

// ClientSchemeFromContext returns the scheme the client used to reach
// Muximux ("http" or "https") as resolved by ResolveClientIP middleware.
// Returns an empty string when the middleware has not run.
func ClientSchemeFromContext(ctx context.Context) string {
	scheme, _ := ctx.Value(ContextKeyClientScheme).(string)
	return scheme
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
