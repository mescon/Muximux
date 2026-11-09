package auth

import (
	"context"
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
	if binding == nil || binding.Value != entry.binding || !binding.HttpOnly || binding.SameSite != http.SameSiteLaxMode || binding.Path != "/api/auth/oidc/" {
		t.Fatalf("binding cookie = %+v", binding)
	}

	// A second sign-in from the same browser reuses the binding.
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/login", nil)
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
