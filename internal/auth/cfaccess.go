package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	gooidc "github.com/coreos/go-oidc/v3/oidc"

	"github.com/mescon/muximux/v3/internal/config"
)

// CloudflareAccessConfig identifies the Cloudflare Access application whose
// tokens forward auth accepts.
type CloudflareAccessConfig struct {
	TeamDomain string // e.g. "example.cloudflareaccess.com"
	Audience   string // the application's AUD tag
}

// cfAccessJWTHeader carries the token Cloudflare Access signs for each
// request it lets through.
const cfAccessJWTHeader = "Cf-Access-Jwt-Assertion"

// cfAccessVerifier checks Cloudflare Access tokens: the signature against
// the team's published keys (fetched and cached by go-oidc, which follows
// key rotation), the issuer, the audience and the expiry.
type cfAccessVerifier struct {
	verifier *gooidc.IDTokenVerifier
}

// newCFAccessVerifier returns nil when c is nil (verification off).
func newCFAccessVerifier(c *CloudflareAccessConfig) *cfAccessVerifier {
	if c == nil {
		return nil
	}
	domain := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(c.TeamDomain), "https://"), "/")
	issuer := "https://" + domain
	return newCFAccessVerifierFor(issuer, issuer+"/cdn-cgi/access/certs", c.Audience)
}

// newCFAccessVerifierFor builds a verifier for an explicit issuer and key
// URL; tests point both at a local server.
func newCFAccessVerifierFor(issuer, certsURL, audience string) *cfAccessVerifier {
	// The key set outlives any one request, so it gets its own context.
	keys := gooidc.NewRemoteKeySet(context.Background(), certsURL)
	return &cfAccessVerifier{verifier: gooidc.NewVerifier(issuer, keys, &gooidc.Config{
		ClientID:             strings.TrimSpace(audience),
		SupportedSigningAlgs: []string{gooidc.RS256},
	})}
}

// cfAccessIdentity is what a verified token says about the user.
type cfAccessIdentity struct {
	Email  string
	Name   string
	Groups []string
}

// verify checks token and returns the identity it was issued for. Groups
// come from a "groups" claim, at the top level or under "custom" (where
// Access puts identity-provider attributes passed through to the app).
func (v *cfAccessVerifier) verify(ctx context.Context, token string) (*cfAccessIdentity, error) {
	if token == "" {
		return nil, errors.New("missing " + cfAccessJWTHeader)
	}
	idt, err := v.verifier.Verify(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	var claims struct {
		Email  string         `json:"email"`
		Name   string         `json:"name"`
		Groups []string       `json:"groups"`
		Custom map[string]any `json:"custom"`
	}
	if err := idt.Claims(&claims); err != nil || claims.Email == "" {
		return nil, errors.New("token has no email claim")
	}
	groups := claims.Groups
	if len(groups) == 0 {
		groups = stringList(claims.Custom["groups"])
	}
	return &cfAccessIdentity{Email: claims.Email, Name: claims.Name, Groups: groups}, nil
}

// stringList reads a JSON array of strings; anything else gives nil.
func stringList(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// CloudflareAccessFromConfig converts the YAML block; nil stays nil.
func CloudflareAccessFromConfig(c *config.CloudflareAccessConfig) *CloudflareAccessConfig {
	if c == nil {
		return nil
	}
	return &CloudflareAccessConfig{TeamDomain: c.TeamDomain, Audience: c.Audience}
}
