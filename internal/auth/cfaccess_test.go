package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/mescon/muximux/v3/internal/config"
)

// cfAccessFixture is a fake Access team: a key server plus a signer.
type cfAccessFixture struct {
	srv *httptest.Server
	key *rsa.PrivateKey
}

func newCFAccessFixture(t *testing.T) *cfAccessFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	set := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(set)
	}))
	t.Cleanup(srv.Close)
	return &cfAccessFixture{srv: srv, key: key}
}

func (f *cfAccessFixture) token(t *testing.T, key *rsa.PrivateKey, aud, email string, exp time.Time, extra ...map[string]any) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "k1"))
	if err != nil {
		t.Fatal(err)
	}
	claims := map[string]any{
		"iss": f.srv.URL, "aud": []string{aud}, "email": email,
		"iat": time.Now().Add(-time.Minute).Unix(), "exp": exp.Unix(),
	}
	for _, e := range extra {
		for k, v := range e {
			claims[k] = v
		}
	}
	tok, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestForwardAuth_CloudflareAccessToken(t *testing.T) {
	f := newCFAccessFixture(t)
	m, _, _ := newTestMiddleware(&AuthConfig{
		Method:         AuthMethodForwardAuth,
		TrustedProxies: []string{"10.0.0.0/8"},
		Headers:        ForwardAuthHeaders{User: "Cf-Access-Authenticated-User-Email", Email: "Cf-Access-Authenticated-User-Email"},
	})
	m.snap.Load().cfAccess = newCFAccessVerifierFor(f.srv.URL, f.srv.URL, "aud-123")

	var got *User
	h := m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = GetUserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	do := func(token, headerEmail string) int {
		got = nil
		req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
		req.RemoteAddr = "10.0.0.1:80"
		req.Header.Set("Cf-Access-Authenticated-User-Email", headerEmail)
		// A client-supplied groups and name header must never count.
		req.Header.Set("Remote-Groups", "admin")
		req.Header.Set("Remote-Name", "Spoofed Name")
		if token != "" {
			req.Header.Set(cfAccessJWTHeader, token)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	future := time.Now().Add(time.Hour)

	// A valid token wins over a spoofed header.
	if code := do(f.token(t, f.key, "aud-123", "alice@example.com", future), "mallory@example.com"); code != http.StatusOK || got == nil {
		t.Fatalf("valid token: status %d, user %v", code, got)
	}
	if got.Username != "alice@example.com" || got.Email != "alice@example.com" {
		t.Errorf("identity must come from the token: got %+v", got)
	}
	if got.Role != RoleUser || len(got.Groups) != 0 || got.DisplayName != "alice@example.com" {
		t.Errorf("a spoofed Remote-Groups/Remote-Name header must not count: got %+v", got)
	}

	// Groups and name inside the signed token do count.
	for name, extra := range map[string]map[string]any{
		"top-level groups": {"groups": []string{"admins"}, "name": "Alice"},
		"custom groups":    {"custom": map[string]any{"groups": []any{"admins"}}, "name": "Alice"},
	} {
		tok := f.token(t, f.key, "aud-123", "alice@example.com", future, extra)
		if code := do(tok, "alice@example.com"); code != http.StatusOK || got == nil || got.Role != RoleAdmin || got.DisplayName != "Alice" {
			t.Errorf("%s: status %d, user %+v; want admin named Alice", name, code, got)
		}
	}

	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	for name, tok := range map[string]string{
		"missing token":  "",
		"wrong audience": f.token(t, f.key, "other-aud", "alice@example.com", future),
		"expired":        f.token(t, f.key, "aud-123", "alice@example.com", time.Now().Add(-time.Minute)),
		"wrong key":      f.token(t, other, "aud-123", "alice@example.com", future),
		"no email":       f.token(t, f.key, "aud-123", "", future),
		"garbage":        "not.a.jwt",
	} {
		if code := do(tok, "alice@example.com"); code == http.StatusOK || got != nil {
			t.Errorf("%s: status %d, user %v; want the request refused", name, code, got)
		}
	}
}

func TestCloudflareAccessFromConfig(t *testing.T) {
	if CloudflareAccessFromConfig(nil) != nil {
		t.Error("nil config must stay nil")
	}
	c := CloudflareAccessFromConfig(&config.CloudflareAccessConfig{TeamDomain: "team.cloudflareaccess.com", Audience: "a"})
	if c == nil || c.TeamDomain != "team.cloudflareaccess.com" || c.Audience != "a" {
		t.Errorf("got %+v", c)
	}
	if newCFAccessVerifier(nil) != nil {
		t.Error("no config must give no verifier")
	}
	if newCFAccessVerifier(c) == nil {
		t.Error("a config must give a verifier")
	}
}
