package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
	"golang.org/x/crypto/bcrypt"
)

// manifestApps is the shared fixture: one plain app, one gated app a normal
// user may not see, and one admins-only default app. Slugs come from
// config.Slugify, so the names exercise spacing and punctuation.
func manifestApps() []config.AppConfig {
	return []config.AppConfig{
		{
			Name: "Plex", URL: "http://plex:32400", Enabled: true, Color: "#e5a00d",
			Icon: config.AppIconConfig{Type: "dashboard", Name: "plex"},
		},
		{
			Name: "Secret Admin!", URL: "http://admin:1", Enabled: true,
			MinRole: "admin",
			Icon:    config.AppIconConfig{Type: "url", URL: "https://cdn.example/icon.png"},
		},
	}
}

// manifestServer builds a completed-setup server with manifestApps plus any
// extra apps the test needs.
func manifestServer(t *testing.T, extra ...config.AppConfig) *Server {
	t.Helper()
	return newServerForTest(t, func(cfg *config.Config) {
		setup := completeSetup(t)
		setup(cfg)
		cfg.Apps = append(manifestApps(), extra...)
	})
}

// decodeManifest GETs a manifest path anonymously and decodes a 200 body.
func decodeManifest(t *testing.T, s *Server, path string) (int, webManifest) {
	t.Helper()
	return decodeManifestWith(t, s, path, nil)
}

func decodeManifestWith(t *testing.T, s *Server, path string, cookies []*http.Cookie) (int, webManifest) {
	t.Helper()
	rec := doJSON(s, http.MethodGet, path, "", cookies, true)
	var m webManifest
	if rec.Code == http.StatusOK {
		if ct := rec.Header().Get("Content-Type"); ct != contentTypeManifest {
			t.Errorf("GET %s Content-Type = %q, want %q", path, ct, contentTypeManifest)
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
			t.Fatalf("GET %s: manifest does not decode: %v\n%s", path, err, rec.Body.String())
		}
	}
	return rec.Code, m
}

func TestAppManifest_PlainApp(t *testing.T) {
	s := manifestServer(t)
	code, m := decodeManifest(t, s, "/manifest/plex.webmanifest")
	if code != http.StatusOK {
		t.Fatalf("GET /manifest/plex.webmanifest = %d, want 200", code)
	}
	if m.Name != "Plex" || m.ShortName != "Plex" {
		t.Errorf("name = %q/%q, want Plex/Plex", m.Name, m.ShortName)
	}
	// Relative URLs keep the install working under a base_path (#436).
	for _, u := range []string{m.ID, m.StartURL, m.Scope} {
		if len(u) == 0 || u[0] == '/' {
			t.Errorf("manifest URL %q must be relative, not absolute", u)
		}
	}
	if m.StartURL != "../#plex" {
		t.Errorf("start_url = %q, want ../#plex", m.StartURL)
	}
	if m.Scope != "../" {
		t.Errorf("scope = %q, want ../", m.Scope)
	}
	if m.Display != "standalone" {
		t.Errorf("display = %q, want standalone", m.Display)
	}
	if m.ThemeColor != "#e5a00d" {
		t.Errorf("theme_color = %q, want the app color #e5a00d", m.ThemeColor)
	}
	if len(m.Icons) < 3 {
		t.Fatalf("got %d icons, want the app icon plus 3 Muximux fallbacks", len(m.Icons))
	}
	if m.Icons[0].Src != "../icons/dashboard/plex.png" {
		t.Errorf("first icon = %q, want the app's own ../icons/dashboard/plex.png", m.Icons[0].Src)
	}
	if m.Icons[0].Sizes != "192x192" {
		t.Errorf("first icon sizes = %q, want 192x192", m.Icons[0].Sizes)
	}
	sizes := map[string]bool{}
	for _, icon := range m.Icons {
		sizes[icon.Sizes] = true
		for _, src := range []string{icon.Src} {
			if len(src) == 0 || src[0] == '/' || src[:4] == "http" {
				t.Errorf("icon src %q must be relative", src)
			}
		}
	}
	if !sizes["192x192"] || !sizes["512x512"] {
		t.Errorf("sizes %v lack a 192x192 or 512x512 entry: not installable", sizes)
	}
}

func TestAppManifest_UnknownSlug404(t *testing.T) {
	s := manifestServer(t)
	for _, path := range []string{
		"/manifest/nope.webmanifest",
		"/manifest/plex.json",
		"/manifest/.webmanifest",
		"/manifest/PLEX.webmanifest",
		"/manifest/plex.webmanifest/extra",
	} {
		if code, _ := decodeManifest(t, s, path); code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, code)
		}
	}
	// net/http normalizes dot-segments and empty segments before routing,
	// so traversal-shaped URLs never reach the handler as a manifest slug:
	// they must fail, whether as redirect, bad request, or 404.
	for _, path := range []string{
		"/manifest//.webmanifest",
		"/manifest/../manifest.json.webmanifest",
	} {
		if code, _ := decodeManifest(t, s, path); code == http.StatusOK {
			t.Errorf("GET %s = 200, want a non-installable refusal", path)
		}
	}
}

func TestAppManifest_WrongMethod(t *testing.T) {
	s := manifestServer(t)
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		rec := doJSON(s, method, "/manifest/plex.webmanifest", "", nil, true)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s = %d, want 405", method, rec.Code)
		}
	}
}

func TestAppManifest_UnauthenticatedSeesUngatedOnly(t *testing.T) {
	s := manifestServer(t)
	// Chrome fetches the manifest with no session on install, so the route
	// bypasses auth — but a caller with no session still gets only apps
	// without a visibility gate.
	if code, _ := decodeManifest(t, s, "/manifest/plex.webmanifest"); code != http.StatusOK {
		t.Errorf("anonymous plain app = %d, want 200", code)
	}
	if code, _ := decodeManifest(t, s, "/manifest/secret-admin.webmanifest"); code != http.StatusNotFound {
		t.Errorf("anonymous gated app = %d, want 404", code)
	}
}

func TestAppManifest_RoleGating(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct horse"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	s := newServerForTest(t, func(cfg *config.Config) {
		setup := completeSetup(t)
		setup(cfg)
		cfg.Apps = manifestApps()
		cfg.Auth.Users = append(cfg.Auth.Users,
			config.UserConfig{Username: "pleb", PasswordHash: string(hash), Role: "user"},
		)
	})
	userCookies := loginCookies(t, s, "pleb", "correct horse")
	adminCookies := loginCookies(t, s, "admin", "correct horse")

	if code, _ := decodeManifestWith(t, s, "/manifest/secret-admin.webmanifest", userCookies); code != http.StatusNotFound {
		t.Errorf("user on admin-gated app = %d, want 404", code)
	}
	if code, m := decodeManifestWith(t, s, "/manifest/secret-admin.webmanifest", adminCookies); code != http.StatusOK {
		t.Errorf("admin on admin-gated app = %d, want 200", code)
	} else if m.Name != "Secret Admin!" {
		t.Errorf("admin manifest name = %q, want the app name", m.Name)
	}
	if code, _ := decodeManifestWith(t, s, "/manifest/plex.webmanifest", userCookies); code != http.StatusOK {
		t.Errorf("user on plain app = %d, want 200", code)
	}
}

func TestAppManifest_DisabledApp404(t *testing.T) {
	s := manifestServer(t, config.AppConfig{
		Name: "Off", URL: "http://off:1", Enabled: false,
		Icon: config.AppIconConfig{Type: "dashboard", Name: "off"},
	})
	if code, _ := decodeManifest(t, s, "/manifest/off.webmanifest"); code != http.StatusNotFound {
		t.Errorf("disabled app = %d, want 404", code)
	}
}

func TestAppManifest_IconFallbacks(t *testing.T) {
	s := manifestServer(t,
		config.AppConfig{
			Name: "Lucide", URL: "http://l:1", Enabled: true,
			Icon: config.AppIconConfig{Type: "lucide", Name: "home"},
		},
		config.AppConfig{
			Name: "Custom SVG", URL: "http://c:1", Enabled: true,
			Icon: config.AppIconConfig{Type: "custom", File: "logo.svg"},
		},
		config.AppConfig{
			Name: "Custom PNG", URL: "http://p:1", Enabled: true,
			Icon: config.AppIconConfig{Type: "custom", File: "logo.png"},
		},
		config.AppConfig{
			Name: "Bad Color", URL: "http://b:1", Enabled: true, Color: "red",
			Icon: config.AppIconConfig{Type: "dashboard", Name: "bad"},
		},
	)
	// Lucide and remote-URL icons have no fetchable PNG: fallbacks only.
	if code, m := decodeManifest(t, s, "/manifest/secret-admin.webmanifest"); code == http.StatusOK {
		for _, icon := range m.Icons {
			if len(icon.Src) >= 4 && icon.Src[:4] == "http" {
				t.Errorf("url-icon manifest must not embed the remote URL %q", icon.Src)
			}
		}
	}
	if code, m := decodeManifest(t, s, "/manifest/lucide.webmanifest"); code != http.StatusOK {
		t.Fatalf("lucide app = %d, want 200", code)
	} else if len(m.Icons) != 3 {
		t.Errorf("lucide app has %d icons, want exactly the 3 fallbacks", len(m.Icons))
	}
	if code, m := decodeManifest(t, s, "/manifest/custom-svg.webmanifest"); code != http.StatusOK {
		t.Fatalf("custom SVG app = %d, want 200", code)
	} else if len(m.Icons) != 3 {
		t.Errorf("custom SVG app has %d icons, want exactly the 3 fallbacks", len(m.Icons))
	}
	if code, m := decodeManifest(t, s, "/manifest/custom-png.webmanifest"); code != http.StatusOK {
		t.Fatalf("custom PNG app = %d, want 200", code)
	} else if m.Icons[0].Src != "../icons/custom/logo.png" {
		t.Errorf("custom PNG first icon = %q, want ../icons/custom/logo.png", m.Icons[0].Src)
	}
	// A non-hex color falls back to the dashboard color, never an invalid member.
	if code, m := decodeManifest(t, s, "/manifest/bad-color.webmanifest"); code != http.StatusOK {
		t.Fatalf("bad color app = %d, want 200", code)
	} else if m.ThemeColor != "#09090b" {
		t.Errorf("theme_color = %q, want the #09090b fallback", m.ThemeColor)
	}
}
