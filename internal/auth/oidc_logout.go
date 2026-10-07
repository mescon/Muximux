package auth

import (
	"context"
	"net/url"
	"strings"

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
