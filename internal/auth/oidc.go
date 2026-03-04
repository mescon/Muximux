package auth

import (
	"context"
	"crypto/rand"
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
}

type stateEntry struct {
	createdAt   time.Time
	redirectURL string
	nonce       string
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
func (p *OIDCProvider) loadDiscovery() error {
	// Fast path: already loaded
	p.mu.RLock()
	loaded := p.discoveryLoaded
	p.mu.RUnlock()
	if loaded {
		return nil
	}

	discoveryURL := strings.TrimSuffix(p.config.IssuerURL, "/") + "/.well-known/openid-configuration"

	// Fetch outside lock — network I/O can take seconds
	resp, err := p.httpClient.Get(discoveryURL)
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

// GetAuthorizationURL returns the URL to redirect the user to for authentication
func (p *OIDCProvider) GetAuthorizationURL(redirectAfterLogin string) (string, error) {
	if err := p.loadDiscovery(); err != nil {
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

	// Store state
	p.statesMu.Lock()
	p.states[state] = stateEntry{
		createdAt:   time.Now(),
		redirectURL: redirectAfterLogin,
		nonce:       nonce,
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

	return p.authorizationEndpoint + "?" + params.Encode(), nil
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

	// Exchange code for tokens
	tokens, err := p.exchangeCode(code)
	if err != nil {
		logging.From(r.Context()).Error("OIDC code exchange failed", "source", "auth", "error", err)
		http.Error(w, errAuthFailed, http.StatusInternalServerError)
		return
	}

	// Validate ID token if present (signature, issuer, audience, expiry, nonce)
	if tokens.IDToken != "" {
		ctx := context.Background()
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
	}

	// Get user info
	userInfo, err := p.getUserInfo(tokens.AccessToken)
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

// exchangeCode exchanges an authorization code for tokens
func (p *OIDCProvider) exchangeCode(code string) (*TokenResponse, error) {
	if err := p.loadDiscovery(); err != nil {
		return nil, err
	}

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", p.config.RedirectURL)
	data.Set("client_id", p.config.ClientID)
	data.Set("client_secret", p.config.ClientSecret)

	req, err := http.NewRequest("POST", p.tokenEndpoint, strings.NewReader(data.Encode()))
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
func (p *OIDCProvider) getUserInfo(accessToken string) (map[string]interface{}, error) {
	if err := p.loadDiscovery(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", p.userinfoEndpoint, nil)
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

// cleanupStates periodically removes expired states
func (p *OIDCProvider) cleanupStates() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
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

// sanitizeRedirectURL validates that the redirect URL is a safe relative path
// to prevent open redirect vulnerabilities
func sanitizeRedirectURL(redirectURL, basePath string) string {
	if redirectURL == "" || !strings.HasPrefix(redirectURL, "/") || strings.HasPrefix(redirectURL, "//") {
		return basePath + "/"
	}
	return redirectURL
}

// HandleLogin redirects to the OIDC provider for authentication
func (p *OIDCProvider) HandleLogin(w http.ResponseWriter, r *http.Request) {
	redirectAfter := sanitizeRedirectURL(r.URL.Query().Get("redirect"), p.basePath)

	authURL, err := p.GetAuthorizationURL(redirectAfter)
	if err != nil {
		logging.From(r.Context()).Warn("OIDC: failed to generate authorization URL", "source", "auth", "error", err)
		http.Error(w, "Failed to get authorization URL: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

// Close cleans up resources
func (p *OIDCProvider) Close() error {
	// Nothing to clean up currently
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
	return p.loadDiscovery()
}
