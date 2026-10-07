package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"

	"github.com/mescon/muximux/v3/internal/config"
)

// Discover loads the provider's discovery document. Used to validate new
// settings before they replace a working provider.
func (p *OIDCProvider) Discover(ctx context.Context) error {
	return p.loadDiscovery(ctx)
}

// Config returns a copy of the provider's settings with defaults applied.
func (p *OIDCProvider) Config() config.OIDCConfig {
	return p.config
}

// Capabilities reports what the discovery document advertised.
func (p *OIDCProvider) Capabilities() (endSession, backchannel bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.endSessionEndpoint != "", p.backchannelSupported
}

// EndSessionURL builds the RP-initiated logout URL for an OIDC session.
// Returns false when provider logout is off, the session is not an OIDC
// session, or no end-session address is known.
func (p *OIDCProvider) EndSessionURL(s *Session) (string, bool) {
	if !p.config.ProviderLogout || s == nil {
		return "", false
	}
	hint, _ := s.Data["oidc_id_token"].(string)
	if hint == "" {
		return "", false
	}
	base := p.config.LogoutURL
	if base == "" {
		p.mu.RLock()
		base = p.endSessionEndpoint
		p.mu.RUnlock()
	}
	u, err := url.Parse(base)
	if base == "" || err != nil {
		return "", false
	}
	state, err := generateRandomString()
	if err != nil {
		return "", false
	}
	q := u.Query()
	q.Set("id_token_hint", hint)
	q.Set("client_id", p.config.ClientID)
	q.Set("post_logout_redirect_uri", p.postLogoutRedirect())
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), true
}

// postLogoutRedirect is the configured return address, or the origin of
// the callback URL plus the base path and /login?logged_out=1. The marker
// stops auto_redirect from starting a new SSO login on return.
func (p *OIDCProvider) postLogoutRedirect() string {
	if p.config.PostLogoutRedirectURL != "" {
		return p.config.PostLogoutRedirectURL
	}
	cb, err := url.Parse(p.config.RedirectURL)
	if err != nil || cb.Host == "" {
		return strings.TrimRight(p.basePath, "/") + "/login?logged_out=1"
	}
	return cb.Scheme + "://" + cb.Host + strings.TrimRight(p.basePath, "/") + "/login?logged_out=1"
}

const (
	backchannelEventKey = "http://schemas.openid.net/event/backchannel-logout"
	logoutTokenMaxAge   = 10 * time.Minute
	logoutTokenSkew     = 2 * time.Minute
	maxSeenJTI          = 10000
)

// goOIDCProvider returns the cached go-oidc provider (discovery + keyset),
// creating it on first use.
func (p *OIDCProvider) goOIDCProvider(ctx context.Context) (*gooidc.Provider, error) {
	p.verifierMu.Lock()
	defer p.verifierMu.Unlock()
	if p.verifier != nil {
		return p.verifier, nil
	}
	prov, err := gooidc.NewProvider(ctx, p.config.IssuerURL)
	if err != nil {
		return nil, err
	}
	p.verifier = prov
	return prov, nil
}

// VerifyLogoutToken validates an OpenID Connect Back-Channel Logout token
// and returns its subject and session id.
func (p *OIDCProvider) VerifyLogoutToken(ctx context.Context, raw string) (string, string, error) {
	prov, err := p.goOIDCProvider(ctx)
	if err != nil {
		return "", "", fmt.Errorf("provider: %w", err)
	}
	// exp is optional on logout tokens; freshness is checked via iat below.
	tok, err := prov.Verifier(&gooidc.Config{ClientID: p.config.ClientID, SkipExpiryCheck: true}).Verify(ctx, raw)
	if err != nil {
		return "", "", fmt.Errorf("verify: %w", err)
	}
	var c struct {
		Sid    string                     `json:"sid"`
		JTI    string                     `json:"jti"`
		Nonce  json.RawMessage            `json:"nonce"`
		Iat    *int64                     `json:"iat"`
		Events map[string]json.RawMessage `json:"events"`
	}
	if err := tok.Claims(&c); err != nil {
		return "", "", fmt.Errorf("claims: %w", err)
	}
	if _, ok := c.Events[backchannelEventKey]; !ok {
		return "", "", errors.New("missing back-channel logout event")
	}
	if c.Nonce != nil { // present, even as null
		return "", "", errors.New("logout token must not carry a nonce")
	}
	if tok.Subject == "" && c.Sid == "" {
		return "", "", errors.New("logout token has neither sub nor sid")
	}
	if c.JTI == "" {
		return "", "", errors.New("logout token has no jti")
	}
	if c.Iat == nil {
		return "", "", errors.New("logout token has no iat")
	}
	issued := time.Unix(*c.Iat, 0)
	now := time.Now()
	if issued.After(now.Add(logoutTokenSkew)) || now.Sub(issued) > logoutTokenMaxAge {
		return "", "", errors.New("logout token is not fresh")
	}
	if !p.rememberJTI(c.JTI, now) {
		return "", "", errors.New("logout token replayed")
	}
	return tok.Subject, c.Sid, nil
}

// rememberJTI records jti and reports false when it was already seen
// within the freshness window. Old entries are pruned; the map is capped.
func (p *OIDCProvider) rememberJTI(jti string, now time.Time) bool {
	p.jtiMu.Lock()
	defer p.jtiMu.Unlock()
	for k, at := range p.seenJTI {
		if now.Sub(at) > logoutTokenMaxAge+logoutTokenSkew {
			delete(p.seenJTI, k)
		}
	}
	if _, dup := p.seenJTI[jti]; dup {
		return false
	}
	if len(p.seenJTI) >= maxSeenJTI {
		var oldestK string
		var oldestT time.Time
		for k, at := range p.seenJTI {
			if oldestK == "" || at.Before(oldestT) {
				oldestK, oldestT = k, at
			}
		}
		delete(p.seenJTI, oldestK)
	}
	p.seenJTI[jti] = now
	return true
}

// EndSessions ends the Muximux sessions a logout token names: by sid when
// present, otherwise every session of the subject.
func (p *OIDCProvider) EndSessions(sub, sid string) int {
	if sub == "" && sid == "" {
		return 0
	}
	return p.sessionStore.DeleteMatching(func(s *Session) bool {
		if sid != "" {
			v, _ := s.Data["oidc_sid"].(string)
			return v == sid
		}
		v, _ := s.Data["oidc_sub"].(string)
		return v == sub
	})
}
