package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mescon/muximux/v3/internal/auth"
)

type oidcSettingsResponse struct {
	Enabled               bool              `json:"enabled"`
	IssuerURL             string            `json:"issuer_url"`
	ClientID              string            `json:"client_id"`
	ClientSecretSet       bool              `json:"client_secret_set"`
	RedirectURL           string            `json:"redirect_url"`
	Scopes                []string          `json:"scopes"`
	UsernameClaim         string            `json:"username_claim"`
	EmailClaim            string            `json:"email_claim"`
	GroupsClaim           string            `json:"groups_claim"`
	DisplayNameClaim      string            `json:"display_name_claim"`
	AdminGroups           []string          `json:"admin_groups"`
	ProviderLogout        bool              `json:"provider_logout"`
	PostLogoutRedirectURL string            `json:"post_logout_redirect_url"`
	LogoutURL             string            `json:"logout_url"`
	AutoRedirect          bool              `json:"auto_redirect"`
	DisableLocalLogin     bool              `json:"disable_local_login"`
	EnvFields             map[string]string `json:"env_fields"` // field -> variable name
	DefaultCallbackURL    string            `json:"default_callback_url"`
	BackchannelURL        string            `json:"backchannel_url"`
	CurrentSessionIsOIDC  bool              `json:"current_session_is_oidc"`
}

type oidcTestResponse struct {
	Reachable            bool   `json:"reachable"`
	Error                string `json:"error,omitempty"`
	Issuer               string `json:"issuer,omitempty"`
	Authorization        bool   `json:"authorization"`
	Token                bool   `json:"token"`
	Userinfo             bool   `json:"userinfo"`
	JWKS                 bool   `json:"jwks"`
	EndSession           bool   `json:"end_session"`
	BackchannelSupported bool   `json:"backchannel_supported"`
}

var oidcEnvFieldNames = []string{"issuer_url", "client_id", "client_secret", "redirect_url", "post_logout_redirect_url", "logout_url"}

const (
	oidcTestTimeout = 10 * time.Second
	oidcTestMaxBody = 1 << 20
)

// publicOrigin returns the scheme and host the browser used. The scheme
// comes from the trusted-proxy-aware value set by the ResolveClientIP
// middleware; the raw X-Forwarded-Proto header is never trusted.
func publicOrigin(r *http.Request) string {
	scheme := auth.ClientSchemeFromContext(r.Context())
	if scheme == "" {
		scheme = "http"
		if r.TLS != nil {
			scheme = "https"
		}
	}
	return scheme + "://" + r.Host
}

// requestIsOIDCSession reports whether the caller signed in through OIDC.
func requestIsOIDCSession(store *auth.SessionStore, r *http.Request) bool {
	s := store.GetFromRequest(r)
	if s == nil {
		return false
	}
	_, ok := s.Data["oidc_id_token"].(string)
	return ok
}

// GetOIDCSettings handles GET /api/auth/settings/oidc (admin).
func (h *AuthHandler) GetOIDCSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	if h.config == nil {
		respondError(w, r, http.StatusServiceUnavailable, "configuration unavailable")
		return
	}
	h.configMu.RLock()
	o := h.config.Auth.OIDC
	base := h.config.Server.NormalizedBasePath()
	env := map[string]string{}
	for _, f := range oidcEnvFieldNames {
		if v, ok := h.config.EnvRefVar("auth", "oidc", f); ok {
			env[f] = v
		}
	}
	h.configMu.RUnlock()

	origin := publicOrigin(r)
	resp := oidcSettingsResponse{
		Enabled: o.Enabled, IssuerURL: o.IssuerURL, ClientID: o.ClientID, ClientSecretSet: o.ClientSecret != "",
		RedirectURL: o.RedirectURL, Scopes: o.Scopes, UsernameClaim: o.UsernameClaim, EmailClaim: o.EmailClaim,
		GroupsClaim: o.GroupsClaim, DisplayNameClaim: o.DisplayNameClaim, AdminGroups: o.AdminGroups,
		ProviderLogout: o.ProviderLogout, PostLogoutRedirectURL: o.PostLogoutRedirectURL, LogoutURL: o.LogoutURL,
		AutoRedirect: o.AutoRedirect, DisableLocalLogin: o.DisableLocalLogin, EnvFields: env,
		DefaultCallbackURL: origin + base + "/api/auth/oidc/callback",
		BackchannelURL:     origin + base + "/api/auth/oidc/backchannel-logout",
	}
	resp.CurrentSessionIsOIDC = requestIsOIDCSession(h.sessionStore, r)
	sendJSON(w, http.StatusOK, resp)
}

// TestOIDCProvider handles POST /api/auth/settings/oidc/test (admin): it
// fetches the issuer's discovery document and reports what it offers.
// Private addresses are allowed (identity providers commonly run on the
// LAN); scheme, time, size and redirects are limited.
func (h *AuthHandler) TestOIDCProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	var req struct {
		IssuerURL string `json:"issuer_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidBody)
		return
	}
	sendJSON(w, http.StatusOK, probeOIDCIssuer(r.Context(), req.IssuerURL))
}

func probeOIDCIssuer(ctx context.Context, issuer string) oidcTestResponse {
	u, err := url.Parse(strings.TrimSpace(issuer))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return oidcTestResponse{Error: "issuer URL must be an absolute http(s) URL"}
	}
	discovery := strings.TrimRight(u.String(), "/") + "/.well-known/openid-configuration"
	client := &http.Client{
		Timeout: oidcTestTimeout,
		CheckRedirect: func(next *http.Request, _ []*http.Request) error {
			if !strings.EqualFold(next.URL.Host, u.Host) {
				return errors.New("redirect to another host refused")
			}
			return nil
		},
	}
	ctx, cancel := context.WithTimeout(ctx, oidcTestTimeout)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, discovery, http.NoBody)
	resp, err := client.Do(req)
	if err != nil {
		return oidcTestResponse{Error: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return oidcTestResponse{Error: "discovery returned HTTP " + resp.Status}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, oidcTestMaxBody+1))
	if err != nil {
		return oidcTestResponse{Error: err.Error()}
	}
	if len(body) > oidcTestMaxBody {
		return oidcTestResponse{Error: "discovery document is larger than 1 MB"}
	}
	var doc struct {
		Issuer                     string `json:"issuer"`
		AuthorizationEndpoint      string `json:"authorization_endpoint"`
		TokenEndpoint              string `json:"token_endpoint"`
		UserinfoEndpoint           string `json:"userinfo_endpoint"`
		JwksURI                    string `json:"jwks_uri"`
		EndSessionEndpoint         string `json:"end_session_endpoint"`
		BackchannelLogoutSupported bool   `json:"backchannel_logout_supported"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return oidcTestResponse{Error: "discovery document is not valid JSON"}
	}
	return oidcTestResponse{
		Reachable: true, Issuer: doc.Issuer,
		Authorization: doc.AuthorizationEndpoint != "", Token: doc.TokenEndpoint != "",
		Userinfo: doc.UserinfoEndpoint != "", JWKS: doc.JwksURI != "",
		EndSession: doc.EndSessionEndpoint != "", BackchannelSupported: doc.BackchannelLogoutSupported,
	}
}
