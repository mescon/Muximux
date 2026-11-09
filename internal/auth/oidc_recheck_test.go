package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// recheckRequest builds a callback request that carries the session
// cookie of sess.
func recheckRequest(query string, sess *Session) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?"+query, nil)
	if sess != nil {
		req.AddCookie(&http.Cookie{Name: "test_session", Value: sess.ID})
	}
	return req
}

// addState registers entry under the state "st" used by these tests.
func addState(p *OIDCProvider, entry *stateEntry) {
	p.statesMu.Lock()
	p.states["st"] = *entry
	p.statesMu.Unlock()
}

func TestAuthorizationURL_SilentAddsPromptNone(t *testing.T) {
	srv := mockOIDCServer(t, nil)
	defer srv.Close()
	p, _ := newTestOIDCProvider(t, srv.URL)

	for _, silent := range []bool{false, true} {
		raw, err := p.authorizationURL(context.Background(), "/", loginOptions{silent: silent})
		if err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(raw)
		if got := u.Query().Get("prompt"); (got == "none") != silent {
			t.Errorf("silent=%v: prompt=%q", silent, got)
		}
		p.statesMu.Lock()
		entry := p.states[u.Query().Get("state")]
		p.statesMu.Unlock()
		if entry.silent != silent {
			t.Errorf("silent=%v: state entry silent=%v", silent, entry.silent)
		}
	}
}

func TestHandleLogin_PromptNoneStartsSilentRecheck(t *testing.T) {
	srv := mockOIDCServer(t, nil)
	defer srv.Close()
	p, ss := newTestOIDCProvider(t, srv.URL)
	p.config.RevalidateAfter = "1h"
	sess, _ := ss.CreateWithData("u", "u", RoleUser, map[string]interface{}{"oidc_iss": srv.URL})

	rec := httptest.NewRecorder()
	p.HandleLogin(rec, recheckRequestTo("/api/auth/oidc/login?prompt=none&redirect=/", sess))
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if loc.Query().Get("prompt") != "none" {
		t.Fatalf("expected prompt=none in %s", loc)
	}
	p.statesMu.Lock()
	entry := p.states[loc.Query().Get("state")]
	p.statesMu.Unlock()
	if entry.rechecked != sess.ID || entry.binding == "" {
		t.Fatalf("entry = %+v, want rechecked=%s and a binding", entry, sess.ID)
	}
	var binding *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == oidcBindingCookie {
			binding = c
		}
	}
	if binding == nil || binding.Value != entry.binding || !binding.HttpOnly || binding.SameSite != http.SameSiteLaxMode || binding.Path != "/" {
		t.Fatalf("binding cookie = %+v", binding)
	}

	// A second sign-in from the same browser reuses the binding.
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/login", nil)
	req.Host = "localhost:3000"
	req.AddCookie(binding)
	rec2 := httptest.NewRecorder()
	p.HandleLogin(rec2, req)
	for _, c := range rec2.Result().Cookies() {
		if c.Name == oidcBindingCookie && c.Value != binding.Value {
			t.Fatal("binding changed between sign-ins in one browser")
		}
	}
}

func recheckRequestTo(target string, sess *Session) *http.Request {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Host = "localhost:3000" // the host the test provider's redirect_url names
	if sess != nil {
		req.AddCookie(&http.Cookie{Name: "test_session", Value: sess.ID})
	}
	return req
}

func TestHandleCallback_RejectsCallbackFromAnotherBrowser(t *testing.T) {
	p, ss := newTestOIDCProvider(t, "http://idp.invalid")
	victim, _ := ss.CreateWithData("v", "v", RoleUser, map[string]interface{}{"oidc_iss": "http://idp.invalid"})
	// The attacker started the recheck; the entry carries the attacker's
	// binding and session.
	for _, cookie := range []string{"", "victim-binding-0123456789abcdef0"} {
		addState(p, &stateEntry{createdAt: time.Now(), redirectURL: "/", silent: true, rechecked: "attacker-session", binding: "attacker-binding-0123456789abcdef"})
		req := recheckRequest("error=login_required&state=st", victim)
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: oidcBindingCookie, Value: cookie})
		}
		rec := httptest.NewRecorder()
		p.HandleCallback(rec, req)
		if ss.Get(victim.ID) == nil {
			t.Fatalf("cookie %q: a callback from another browser ended the victim's session", cookie)
		}
	}

	// Same for a code callback: refused with the state error.
	addState(p, &stateEntry{createdAt: time.Now(), redirectURL: "/", binding: "attacker-binding-0123456789abcdef"})
	rec := httptest.NewRecorder()
	p.HandleCallback(rec, recheckRequest("code=c&state=st", nil))
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "error="+callbackErrState) {
		t.Fatalf("code callback from another browser: Location = %q", loc)
	}
}

func TestHandleCallback_SilentErrorOnlyTouchesRecheckedSession(t *testing.T) {
	p, ss := newTestOIDCProvider(t, "http://idp.invalid")
	other, _ := ss.CreateWithData("o", "o", RoleUser, map[string]interface{}{"oidc_iss": "http://idp.invalid"})
	addState(p, &stateEntry{createdAt: time.Now(), redirectURL: "/", silent: true, rechecked: "some-other-session"})

	rec := httptest.NewRecorder()
	p.HandleCallback(rec, recheckRequest("error=login_required&state=st", other))
	if ss.Get(other.ID) == nil {
		t.Fatal("a recheck for another session ended this one")
	}
}

func TestHandleCallback_SilentLoginRequiredEndsSessionAndSignsInAgain(t *testing.T) {
	for _, errCode := range []string{"login_required", "interaction_required", "consent_required", "account_selection_required", "access_denied"} {
		t.Run(errCode, func(t *testing.T) {
			p, ss := newTestOIDCProvider(t, "http://idp.invalid")
			p.basePath = "/mux"
			sess, _ := ss.CreateWithData("u", "u", RoleUser, map[string]interface{}{"oidc_iss": "http://idp.invalid"})
			addState(p, &stateEntry{createdAt: time.Now(), redirectURL: "/mux/", silent: true, rechecked: sess.ID})

			rec := httptest.NewRecorder()
			p.HandleCallback(rec, recheckRequest("error="+errCode+"&state=st", sess))

			if ss.Get(sess.ID) != nil {
				t.Fatal("local session should end when the provider session has ended")
			}
			want := "/mux/api/auth/oidc/login?redirect=" + url.QueryEscape("/mux/")
			if got := rec.Header().Get("Location"); got != want {
				t.Fatalf("Location = %q, want %q", got, want)
			}
			if strings.Contains(rec.Header().Get("Location"), "prompt=none") {
				t.Fatal("the follow-up sign-in must be interactive")
			}
			cleared := false
			for _, c := range rec.Result().Cookies() {
				if c.Name == "test_session" && c.MaxAge < 0 {
					cleared = true
				}
			}
			if !cleared {
				t.Fatal("session cookie should be cleared")
			}
		})
	}
}

func TestHandleCallback_SilentProviderErrorKeepsSession(t *testing.T) {
	p, ss := newTestOIDCProvider(t, "http://idp.invalid")
	sess, _ := ss.CreateWithData("u", "u", RoleUser, map[string]interface{}{"oidc_iss": "http://idp.invalid"})
	ss.mu.Lock()
	ss.sessions[sess.ID].ProviderCheckedAt = time.Now().Add(-time.Hour)
	ss.mu.Unlock()
	addState(p, &stateEntry{createdAt: time.Now(), redirectURL: "/#emby", silent: true, rechecked: sess.ID})

	rec := httptest.NewRecorder()
	p.HandleCallback(rec, recheckRequest("error=temporarily_unavailable&state=st", sess))

	got := ss.Get(sess.ID)
	if got == nil {
		t.Fatal("a provider error must not end the session")
	}
	if time.Since(got.ProviderCheckedAt) > time.Minute {
		t.Fatal("the recheck clock should be reset so the dashboard does not retry in a loop")
	}
	if loc := rec.Header().Get("Location"); loc != "/#emby" {
		t.Fatalf("Location = %q, want the original destination", loc)
	}
}

func TestHandleCallback_NonSilentErrorUnchanged(t *testing.T) {
	p, ss := newTestOIDCProvider(t, "http://idp.invalid")
	sess, _ := ss.Create("u", "u", RoleUser)
	addState(p, &stateEntry{createdAt: time.Now(), redirectURL: "/"})

	rec := httptest.NewRecorder()
	p.HandleCallback(rec, recheckRequest("error=login_required&state=st", sess))
	if ss.Get(sess.ID) == nil {
		t.Fatal("an interactive sign-in error must not touch an existing session")
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "/login?error=") {
		t.Fatalf("Location = %q", loc)
	}
}

func TestHandleCallback_SilentSuccessReplacesSession(t *testing.T) {
	userinfo := map[string]interface{}{"sub": "user-1", "preferred_username": "oidcuser", "email": "o@example.com"}
	srv := mockOIDCServerWithIDClaims(t, userinfo, nil)
	defer srv.Close()
	p, ss := newTestOIDCProvider(t, srv.URL)
	if err := p.loadDiscovery(context.Background()); err != nil {
		t.Fatal(err)
	}
	old, _ := ss.CreateWithData("oidcuser", "oidcuser", RoleUser, map[string]interface{}{"oidc_iss": srv.URL})
	addState(p, &stateEntry{createdAt: time.Now(), redirectURL: "/", silent: true, rechecked: old.ID})

	rec := httptest.NewRecorder()
	p.HandleCallback(rec, recheckRequest("code=test-code&state=st", old))
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/" {
		t.Fatalf("status %d Location %q", rec.Code, rec.Header().Get("Location"))
	}
	if ss.Get(old.ID) != nil {
		t.Fatal("the rechecked session should be replaced")
	}
	fresh := ss.GetFromRequest(requestWithCookies(rec.Result().Cookies()))
	if fresh == nil || time.Since(fresh.ProviderCheckedAt) > time.Minute {
		t.Fatalf("expected a fresh session, got %+v", fresh)
	}
}

func TestHandleLogin_PromptNoneIgnoredUnlessRecheckIsOnAndSignedIn(t *testing.T) {
	srv := mockOIDCServer(t, nil)
	defer srv.Close()

	check := func(t *testing.T, revalidate string, withSession bool) {
		t.Helper()
		p, ss := newTestOIDCProvider(t, srv.URL)
		p.config.RevalidateAfter = revalidate
		var sess *Session
		if withSession {
			sess, _ = ss.CreateWithData("u", "u", RoleUser, map[string]interface{}{"oidc_iss": srv.URL})
		}
		rec := httptest.NewRecorder()
		p.HandleLogin(rec, recheckRequestTo("/api/auth/oidc/login?prompt=none&redirect=/", sess))
		loc, _ := url.Parse(rec.Header().Get("Location"))
		if loc.Query().Get("prompt") != "" {
			t.Fatalf("prompt=none must not reach the provider (revalidate=%q, session=%v): %s", revalidate, withSession, loc)
		}
		p.statesMu.Lock()
		entry := p.states[loc.Query().Get("state")]
		p.statesMu.Unlock()
		if entry.silent || entry.rechecked != "" {
			t.Fatalf("entry = %+v, want a normal sign-in", entry)
		}
	}
	t.Run("recheck off", func(t *testing.T) { check(t, "", true) })
	t.Run("no session", func(t *testing.T) { check(t, "1h", false) })
}

func TestHandleLogin_MovesToTheCallbackHostOnce(t *testing.T) {
	srv := mockOIDCServer(t, nil)
	defer srv.Close()
	p, _ := newTestOIDCProvider(t, srv.URL)
	p.config.RedirectURL = "https://dash.example.com/mux/api/auth/oidc/callback"

	// Started on a LAN address: move to the public host first, keeping
	// the query, under the prefix redirect_url has.
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/login?redirect=/mux/", nil)
	req.Host = "192.168.1.10:8080"
	rec := httptest.NewRecorder()
	p.HandleLogin(rec, req)
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if loc.Host != "dash.example.com" || loc.Path != "/mux/api/auth/oidc/login" || loc.Query().Get("redirect") != "/mux/" || loc.Query().Get(oidcHopParam) != "1" {
		t.Fatalf("hop = %s", loc)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == oidcBindingCookie {
			t.Fatal("the binding cookie belongs on the callback host, not this one")
		}
	}

	// After the hop (or behind a proxy that rewrites Host) the sign-in
	// goes ahead, whatever Host says.
	req2 := httptest.NewRequest(http.MethodGet, loc.RequestURI(), nil)
	req2.Host = "muximux:8080"
	rec2 := httptest.NewRecorder()
	p.HandleLogin(rec2, req2)
	if got := rec2.Header().Get("Location"); !strings.Contains(got, "/authorize?") {
		t.Fatalf("second request should go to the provider, got %s", got)
	}

	// A redirect_url that does not end in /callback gives no safe hop.
	p.config.RedirectURL = "https://dash.example.com/custom"
	rec3 := httptest.NewRecorder()
	p.HandleLogin(rec3, req)
	if got := rec3.Header().Get("Location"); !strings.Contains(got, "/authorize?") {
		t.Fatalf("no hop expected, got %s", got)
	}
}

func TestHandleCallback_SilentFailureAfterCodeKeepsSession(t *testing.T) {
	// The token endpoint fails: an interactive sign-in shows the error, a
	// recheck keeps the session and returns the user where they were.
	var base string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			_ = json.NewEncoder(w).Encode(map[string]string{
				"issuer": base, "authorization_endpoint": base + "/authorize",
				"token_endpoint": base + "/token", "jwks_uri": base + "/jwks",
			})
			return
		}
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	base = srv.URL
	p, ss := newTestOIDCProvider(t, srv.URL)
	if err := p.loadDiscovery(context.Background()); err != nil {
		t.Fatal(err)
	}
	sess, _ := ss.CreateWithData("u", "u", RoleUser, map[string]interface{}{"oidc_iss": srv.URL})
	ss.mu.Lock()
	ss.sessions[sess.ID].ProviderCheckedAt = time.Now().Add(-time.Hour)
	ss.mu.Unlock()

	addState(p, &stateEntry{createdAt: time.Now(), redirectURL: "/", silent: true, rechecked: sess.ID})
	rec := httptest.NewRecorder()
	p.HandleCallback(rec, recheckRequest("code=c&state=st", sess))
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Fatalf("recheck: Location = %q, want the original page", loc)
	}
	if got := ss.Get(sess.ID); got == nil || time.Since(got.ProviderCheckedAt) > time.Minute {
		t.Fatal("recheck: session should be kept with its clock reset")
	}

	addState(p, &stateEntry{createdAt: time.Now(), redirectURL: "/"})
	rec2 := httptest.NewRecorder()
	p.HandleCallback(rec2, recheckRequest("code=c&state=st", nil))
	if loc := rec2.Header().Get("Location"); !strings.Contains(loc, "/login?error="+callbackErrFailed) {
		t.Fatalf("interactive: Location = %q", loc)
	}
}
