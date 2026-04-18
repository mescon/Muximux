package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/logging"
)

// OIDCProvider handles OIDC authentication
type OIDCProvider struct {
	config       config.OIDCConfig
	basePath     string // e.g. "/muximux" — prepended to fallback redirect
	httpClient   *http.Client
	sessionStore *SessionStore
	userStore    *UserStore

	// Discovery endpoints (cached)
	mu                    sync.RWMutex
	discoveryLoaded       bool
	authorizationEndpoint string
	tokenEndpoint         string
	userinfoEndpoint      string
	jwksURI               string

	// State storage (for CSRF protection)
	states   map[string]stateEntry
	statesMu sync.Mutex

	// Cleanup goroutine lifecycle. Close signals done so Close()
	// actually stops the cleanup ticker instead of leaking the
	// goroutine past a provider reload (findings.md M13).
	done chan struct{}
}

type stateEntry struct {
	createdAt    time.Time
	redirectURL  string
	nonce        string
	codeVerifier string // PKCE verifier; sent with code exchange to defend against code interception
}

// NewOIDCProvider creates a new OIDC provider
func NewOIDCProvider(cfg *config.OIDCConfig, basePath string, sessionStore *SessionStore, userStore *UserStore) *OIDCProvider {
	p := &OIDCProvider{
		config:       *cfg,
		basePath:     basePath,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		sessionStore: sessionStore,
		userStore:    userStore,
		states:       make(map[string]stateEntry),
		done:         make(chan struct{}),
	}

	// Set default scopes
	if len(p.config.Scopes) == 0 {
		p.config.Scopes = []string{"openid", "profile", "email"}
	}

	// Set default claims
	if p.config.UsernameClaim == "" {
		p.config.UsernameClaim = "preferred_username"
	}
	if p.config.EmailClaim == "" {
		p.config.EmailClaim = "email"
	}
	if p.config.DisplayNameClaim == "" {
		p.config.DisplayNameClaim = "name"
	}
	if p.config.GroupsClaim == "" {
		p.config.GroupsClaim = "groups"
	}

	// Start state cleanup goroutine
	go p.cleanupStates()

	return p
}

// loadDiscovery fetches the OIDC discovery document using double-check locking
// so the write lock is not held during the (potentially slow) HTTP call.
// Accepts a context so an in-flight OIDC login that the user abandons
// cancels the discovery fetch instead of pinning a goroutine against
// a slow IdP (findings.md M14).
func (p *OIDCProvider) loadDiscovery(ctx context.Context) error {
	// Fast path: already loaded
	p.mu.RLock()
	loaded := p.discoveryLoaded
	p.mu.RUnlock()
	if loaded {
		return nil
	}

	discoveryURL := strings.TrimSuffix(p.config.IssuerURL, "/") + "/.well-known/openid-configuration"

	// Fetch outside lock — network I/O can take seconds
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return fmt.Errorf("failed to build discovery request: %w", err)
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		logging.Error("OIDC discovery failed", "source", "auth", "url", discoveryURL, "error", err)
		return fmt.Errorf("failed to fetch discovery document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("discovery endpoint returned %d", resp.StatusCode)
	}

	var doc struct {
		AuthorizationEndpoint string `json:"authorization_endpoint"`
		TokenEndpoint         string `json:"token_endpoint"`
		UserinfoEndpoint      string `json:"userinfo_endpoint"`
		JwksURI               string `json:"jwks_uri"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return fmt.Errorf("failed to parse discovery document: %w", err)
	}

	// Re-check under write lock before assigning
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.discoveryLoaded {
		return nil // another goroutine beat us
	}

	p.authorizationEndpoint = doc.AuthorizationEndpoint
	p.tokenEndpoint = doc.TokenEndpoint
	p.userinfoEndpoint = doc.UserinfoEndpoint
	p.jwksURI = doc.JwksURI
	p.discoveryLoaded = true
	logging.Debug("OIDC discovery loaded", "source", "auth", "issuer", p.config.IssuerURL)

	return nil
}

// GetAuthorizationURL returns the URL to redirect the user to for authentication.
// ctx is used to cancel the discovery fetch if the caller goes away.
func (p *OIDCProvider) GetAuthorizationURL(ctx context.Context, redirectAfterLogin string) (string, error) {
	if err := p.loadDiscovery(ctx); err != nil {
		return "", err
	}

	// Generate state parameter for CSRF protection
	state, err := generateRandomString()
	if err != nil {
		return "", fmt.Errorf("failed to generate state: %w", err)
	}

	// Generate nonce for ID token replay protection
	nonce, err := generateRandomString()
	if err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// PKCE: generate a code verifier and derive an S256 challenge so an
	// attacker who intercepts the authorization code cannot redeem it
	// without also possessing the verifier we kept on the server.
	codeVerifier, err := generateRandomString()
	if err != nil {
		return "", fmt.Errorf("failed to generate PKCE verifier: %w", err)
	}
	codeChallenge := pkceS256Challenge(codeVerifier)

	// Store state
	p.statesMu.Lock()
	p.states[state] = stateEntry{
		createdAt:    time.Now(),
		redirectURL:  redirectAfterLogin,
		nonce:        nonce,
		codeVerifier: codeVerifier,
	}
	p.statesMu.Unlock()

	// Build authorization URL
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", p.config.ClientID)
	params.Set("redirect_uri", p.config.RedirectURL)
	params.Set("scope", strings.Join(p.config.Scopes, " "))
	params.Set("state", state)
	params.Set("nonce", nonce)
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")

	return p.authorizationEndpoint + "?" + params.Encode(), nil
}

// pkceS256Challenge returns the base64url-encoded SHA-256 of the verifier
// per RFC 7636 §4.2, with no padding.
func pkceS256Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// HandleCallback processes the OIDC callback
func (p *OIDCProvider) HandleCallback(w http.ResponseWriter, r *http.Request) {
	// Check for errors from provider
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		errDesc := r.URL.Query().Get("error_description")
		logging.From(r.Context()).Warn("OIDC authentication error", "source", "auth", "error", errParam, "description", errDesc)
		http.Error(w, errAuthFailed, http.StatusUnauthorized)
		return
	}

	// Verify state parameter
	state := r.URL.Query().Get("state")
	p.statesMu.Lock()
	entry, ok := p.states[state]
	if ok {
		delete(p.states, state)
	}
	p.statesMu.Unlock()

	if !ok {
		logging.From(r.Context()).Warn("OIDC callback: invalid state parameter", "source", "auth")
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	// Get authorization code
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	// Exchange code for tokens. The PKCE verifier stashed in the state
	// entry accompanies the exchange so an attacker who snatched the
	// authorization code out of a redirect URL cannot redeem it.
	// Use the request's context so a slow IdP cannot pin this goroutine
	// past the client disconnect (findings.md M14).
	ctx := r.Context()
	tokens, err := p.exchangeCode(ctx, code, entry.codeVerifier)
	if err != nil {
		logging.From(r.Context()).Error("OIDC code exchange failed", "source", "auth", "error", err)
		http.Error(w, errAuthFailed, http.StatusInternalServerError)
		return
	}

	// ID token is required. Userinfo alone is unauthenticated from the
	// client's perspective: without a verified ID token we cannot prove
	// the tokens came from the configured IdP, cannot bind them to the
	// nonce we sent, and cannot check audience/expiry. An IdP that omits
	// id_token (misconfig, bug, MITM) must be rejected outright.
	if tokens.IDToken == "" {
		logging.From(r.Context()).Warn("OIDC: token endpoint returned no id_token; rejecting login", "source", "audit")
		http.Error(w, errAuthFailed, http.StatusUnauthorized)
		return
	}
	provider, err := gooidc.NewProvider(ctx, p.config.IssuerURL)
	if err != nil {
		logging.From(r.Context()).Error("OIDC: failed to create provider for token verification", "source", "auth", "error", err)
		http.Error(w, errAuthFailed, http.StatusInternalServerError)
		return
	}
	verifier := provider.Verifier(&gooidc.Config{ClientID: p.config.ClientID})
	idToken, err := verifier.Verify(ctx, tokens.IDToken)
	if err != nil {
		logging.From(r.Context()).Info("OIDC: ID token verification failed", "source", "audit", "error", err.Error())
		http.Error(w, errAuthFailed, http.StatusUnauthorized)
		return
	}
	if idToken.Nonce != entry.nonce {
		logging.From(r.Context()).Info("OIDC: nonce mismatch", "source", "audit")
		http.Error(w, errAuthFailed, http.StatusUnauthorized)
		return
	}

	// Get user info
	userInfo, err := p.getUserInfo(ctx, tokens.AccessToken)
	if err != nil {
		logging.From(r.Context()).Error("OIDC user info retrieval failed", "source", "auth", "error", err)
		http.Error(w, errAuthFailed, http.StatusInternalServerError)
		return
	}

	// Extract user details from claims
	username := getStringClaim(userInfo, p.config.UsernameClaim)
	if username == "" {
		username = getStringClaim(userInfo, "sub") // Fallback to subject
	}
	email := getStringClaim(userInfo, p.config.EmailClaim)
	displayName := getStringClaim(userInfo, p.config.DisplayNameClaim)
	groups := getStringListClaim(userInfo, p.config.GroupsClaim)

	// Determine role
	role := determineOIDCRole(groups, p.config.AdminGroups)

	// Create or update user
	user := &User{
		ID:          username,
		Username:    username,
		Email:       email,
		DisplayName: displayName,
		Role:        role,
	}

	// Create session
	session, err := p.sessionStore.Create(user.ID, user.Username, user.Role)
	if err != nil {
		logging.From(r.Context()).Error("OIDC: failed to create session", "source", "auth", "user", username, "error", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Store OIDC claims in session data so the auth middleware can
	// reconstruct the full User without a UserStore lookup (OIDC
	// users are session-only, not persisted to the config-based store).
	session.Data["email"] = email
	session.Data["display_name"] = displayName

	// Set session cookie
	p.sessionStore.SetCookie(w, session)
	logging.From(r.Context()).Info("OIDC user logged in", "source", "audit", "user", username, "role", role)

	// Redirect to original destination or home
	redirectURL := sanitizeRedirectURL(entry.redirectURL, p.basePath)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// TokenResponse represents the token endpoint response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
}

// exchangeCode exchanges an authorization code for tokens. codeVerifier is
// the PKCE verifier originally paired with the code_challenge sent on the
// authorization request; it must be presented here or the IdP rejects the
// exchange.
func (p *OIDCProvider) exchangeCode(ctx context.Context, code, codeVerifier string) (*TokenResponse, error) {
	if err := p.loadDiscovery(ctx); err != nil {
		return nil, err
	}

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", p.config.RedirectURL)
	data.Set("client_id", p.config.ClientID)
	data.Set("client_secret", p.config.ClientSecret)
	if codeVerifier != "" {
		data.Set("code_verifier", codeVerifier)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokens TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, err
	}

	return &tokens, nil
}

// getUserInfo fetches user info from the userinfo endpoint
func (p *OIDCProvider) getUserInfo(ctx context.Context, accessToken string) (map[string]interface{}, error) {
	if err := p.loadDiscovery(ctx); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.userinfoEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("userinfo endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var claims map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&claims); err != nil {
		return nil, err
	}

	return claims, nil
}

// cleanupStates periodically removes expired states. Exits when Close
// is called so a provider reload does not leak this goroutine
// (findings.md M13).
func (p *OIDCProvider) cleanupStates() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-p.done:
			return
		case <-ticker.C:
			p.statesMu.Lock()
			now := time.Now()
			for state, entry := range p.states {
				// States expire after 10 minutes
				if now.Sub(entry.createdAt) > 10*time.Minute {
					delete(p.states, state)
				}
			}
			p.statesMu.Unlock()
		}
	}
}

// generateRandomString generates a cryptographically secure random string of length 32.
func generateRandomString() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b)[:32], nil
}

// getStringClaim extracts a string claim from user info
func getStringClaim(claims map[string]interface{}, key string) string {
	if v, ok := claims[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// getStringListClaim extracts a string list claim from user info
func getStringListClaim(claims map[string]interface{}, key string) []string {
	if v, ok := claims[key]; ok {
		switch val := v.(type) {
		case []interface{}:
			result := make([]string, 0, len(val))
			for _, item := range val {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		case []string:
			return val
		case string:
			// Some providers return groups as space-separated string
			return strings.Fields(val)
		}
	}
	return nil
}

// determineOIDCRole checks if any of the user's groups match the admin groups
// and returns the appropriate role
func determineOIDCRole(groups []string, adminGroups []string) string {
	for _, group := range groups {
		for _, adminGroup := range adminGroups {
			if strings.EqualFold(group, adminGroup) {
				return RoleAdmin
			}
		}
	}
	return RoleUser
}

// sanitizeRedirectURL validates that the redirect URL is a safe relative
// path so the OIDC login callback cannot be tricked into bouncing the user
// to an attacker-controlled origin. Accepted: a path starting with "/"
// followed by a non-slash, non-backslash character. Rejected: empty
// input, protocol-relative URLs ("//evil"), backslash prefixes ("/\evil"
// which some browsers normalize to "//evil"), and any input containing
// control characters or a CRLF that could smuggle a second header.
func sanitizeRedirectURL(redirectURL, basePath string) string {
	fallback := basePath + "/"
	if redirectURL == "" || !strings.HasPrefix(redirectURL, "/") {
		return fallback
	}
	if strings.HasPrefix(redirectURL, "//") || strings.HasPrefix(redirectURL, "/\\") {
		return fallback
	}
	for _, r := range redirectURL {
		if r < 0x20 || r == 0x7f {
			return fallback
		}
	}
	return redirectURL
}

// HandleLogin redirects to the OIDC provider for authentication
func (p *OIDCProvider) HandleLogin(w http.ResponseWriter, r *http.Request) {
	redirectAfter := sanitizeRedirectURL(r.URL.Query().Get("redirect"), p.basePath)

	authURL, err := p.GetAuthorizationURL(r.Context(), redirectAfter)
	if err != nil {
		logging.From(r.Context()).Warn("OIDC: failed to generate authorization URL", "source", "auth", "error", err)
		http.Error(w, "Failed to get authorization URL: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

// Close signals the cleanup goroutine to exit. Safe to call multiple
// times (findings.md M13).
func (p *OIDCProvider) Close() error {
	select {
	case <-p.done:
		// already closed
	default:
		close(p.done)
	}
	return nil
}

// Enabled returns whether OIDC is enabled
func (p *OIDCProvider) Enabled() bool {
	return p.config.Enabled && p.config.IssuerURL != "" && p.config.ClientID != ""
}

// GetConfig returns the OIDC configuration (without secrets)
func (p *OIDCProvider) GetConfig() config.OIDCConfig {
	cfg := p.config
	cfg.ClientSecret = "" // Don't expose secret
	return cfg
}

// Verify checks if the OIDC configuration is valid
func (p *OIDCProvider) Verify(ctx context.Context) error {
	if !p.Enabled() {
		return nil
	}
	return p.loadDiscovery(ctx)
}
