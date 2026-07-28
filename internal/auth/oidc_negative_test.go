package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ID token rejection cases.
//
// The positive path is covered elsewhere; what matters for an auth provider
// is that every malformed or hostile token is refused. go-oidc performs the
// signature, issuer, audience and expiry checks and HandleCallback adds the
// nonce comparison, but none of that was asserted -- a future change to the
// verifier configuration (a wrong ClientID, a SkipIssuerCheck, an added
// SkipExpiryCheck) would have gone unnoticed. Each case below fails closed
// with 401 and is a regression guard for one specific check.

// signIDTokenWith signs claims with an arbitrary key, so a case can present
// a token whose signature does not match the issuer's published JWKS.
func signIDTokenWith(t *testing.T, claims map[string]interface{}, key *rsa.PrivateKey) string {
	t.Helper()
	header := map[string]interface{}{"alg": "RS256", "typ": "JWT", "kid": "test-kid"}
	hj, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	cj, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	input := base64.RawURLEncoding.EncodeToString(hj) + "." + base64.RawURLEncoding.EncodeToString(cj)
	sum := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// unsignedIDToken builds an alg=none token: a well-formed JWT with an empty
// signature. Accepting one would let anybody mint an administrator.
func unsignedIDToken(t *testing.T, claims map[string]interface{}) string {
	t.Helper()
	header := map[string]interface{}{"alg": "none", "typ": "JWT"}
	hj, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	cj, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(hj) + "." + base64.RawURLEncoding.EncodeToString(cj) + "."
}

// mockIDPWithToken serves discovery, JWKS and userinfo honestly, but hands
// the token endpoint's ID token to the caller so each case can decide what
// the IdP returns. issuerOverride, when non-empty, is published as the
// discovery issuer.
func mockIDPWithToken(t *testing.T, buildToken func(issuer, nonce string) string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var lastNonce string

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 base,
			"authorization_endpoint": base + "/authorize",
			"token_endpoint":         base + "/token",
			"userinfo_endpoint":      base + "/userinfo",
			"jwks_uri":               base + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(testJWKSResponse(t))
	})
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		lastNonce = r.URL.Query().Get("nonce")
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"sub": "test-sub"})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		issuer := "http://" + r.Host
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(TokenResponse{ //nolint:gosec // G117: test mock IdP, not a real credential
			AccessToken: "access-token",
			TokenType:   "Bearer",
			IDToken:     buildToken(issuer, lastNonce),
		})
	})
	return httptest.NewServer(mux)
}

func TestHandleCallback_RejectsInvalidIDTokens(t *testing.T) {
	const goodNonce = "nonce-abc"

	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate attacker key: %v", err)
	}

	baseClaims := func(issuer string) map[string]interface{} {
		now := time.Now()
		return map[string]interface{}{
			"iss":   issuer,
			"aud":   "test-client-id",
			"sub":   "test-sub",
			"nonce": goodNonce,
			"iat":   now.Unix(),
			"exp":   now.Add(time.Hour).Unix(),
		}
	}

	cases := []struct {
		name  string
		build func(issuer, nonce string) string
		why   string
	}{
		{
			name: "issuer mismatch",
			why:  "a token minted by a different IdP must not authenticate",
			build: func(issuer, _ string) string {
				c := baseClaims(issuer)
				c["iss"] = "http://attacker.example.com"
				return signTestIDToken(t, c)
			},
		},
		{
			name: "audience mismatch",
			why:  "a token issued for another client must not be replayable here",
			build: func(issuer, _ string) string {
				c := baseClaims(issuer)
				c["aud"] = "some-other-client"
				return signTestIDToken(t, c)
			},
		},
		{
			name: "expired",
			why:  "expiry must be enforced, not merely present",
			build: func(issuer, _ string) string {
				c := baseClaims(issuer)
				c["exp"] = time.Now().Add(-time.Hour).Unix()
				c["iat"] = time.Now().Add(-2 * time.Hour).Unix()
				return signTestIDToken(t, c)
			},
		},
		{
			name: "signature from an unknown key",
			why:  "the signature must be checked against the issuer's JWKS",
			build: func(issuer, _ string) string {
				return signIDTokenWith(t, baseClaims(issuer), otherKey)
			},
		},
		{
			name: "alg none",
			why:  "an unsigned token must never be accepted",
			build: func(issuer, _ string) string {
				return unsignedIDToken(t, baseClaims(issuer))
			},
		},
		{
			name: "nonce mismatch",
			why:  "replaying a token from another login attempt must fail",
			build: func(issuer, _ string) string {
				c := baseClaims(issuer)
				c["nonce"] = "some-other-nonce"
				return signTestIDToken(t, c)
			},
		},
		{
			name: "nonce absent",
			why:  "a token with no nonce must not satisfy the stored nonce",
			build: func(issuer, _ string) string {
				c := baseClaims(issuer)
				delete(c, "nonce")
				return signTestIDToken(t, c)
			},
		},
		{
			name:  "malformed token",
			why:   "a non-JWT must be refused rather than panic",
			build: func(_, _ string) string { return "not-a-jwt" },
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := mockIDPWithToken(t, tc.build)
			defer srv.Close()

			p, ss := newTestOIDCProvider(t, srv.URL)
			if err := p.loadDiscovery(context.Background()); err != nil {
				t.Fatalf("loadDiscovery: %v", err)
			}

			// Index rather than tc.name: the names contain spaces, which
			// would not survive being pasted into a query string.
			state := fmt.Sprintf("state-%d", i)
			p.statesMu.Lock()
			p.states[state] = stateEntry{createdAt: time.Now(), redirectURL: "/", nonce: goodNonce}
			p.statesMu.Unlock()

			req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?code=x&state="+state, nil)
			rec := httptest.NewRecorder()
			p.HandleCallback(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401 (%s)\n  body: %s", rec.Code, tc.why, rec.Body.String())
			}
			// A rejected login must not leave a usable session behind.
			for _, c := range rec.Result().Cookies() {
				if c.Name == ss.cookieName && c.Value != "" {
					t.Errorf("a session cookie was issued despite rejection (%s)", tc.why)
				}
			}
		})
	}
}

// TestHandleCallback_AcceptsValidIDToken is the control for the rejection
// table above. Without it, a mock IdP broken in some unrelated way would
// make every negative case "pass" by failing for the wrong reason. A token
// that is correct in every respect must NOT produce 401 through the same
// harness.
func TestHandleCallback_AcceptsValidIDToken(t *testing.T) {
	const goodNonce = "nonce-abc"

	srv := mockIDPWithToken(t, func(issuer, _ string) string {
		now := time.Now()
		return signTestIDToken(t, map[string]interface{}{
			"iss":                issuer,
			"aud":                "test-client-id",
			"sub":                "test-sub",
			"nonce":              goodNonce,
			"iat":                now.Unix(),
			"exp":                now.Add(time.Hour).Unix(),
			"preferred_username": "alice",
		})
	})
	defer srv.Close()

	p, _ := newTestOIDCProvider(t, srv.URL)
	if err := p.loadDiscovery(context.Background()); err != nil {
		t.Fatalf("loadDiscovery: %v", err)
	}

	p.statesMu.Lock()
	p.states["ok-state"] = stateEntry{createdAt: time.Now(), redirectURL: "/", nonce: goodNonce}
	p.statesMu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?code=x&state=ok-state", nil)
	rec := httptest.NewRecorder()
	p.HandleCallback(rec, req)

	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("a fully valid token was rejected, so the rejection table above "+
			"proves nothing: status=%d body=%s", rec.Code, rec.Body.String())
	}
}
