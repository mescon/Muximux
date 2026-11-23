package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/logging"
)

// contentTypeManifest is the IANA type for Web App Manifests. Chrome only
// applies installability heuristics to responses served as such.
const contentTypeManifest = "application/manifest+json"

// slugPattern matches the output alphabet of config.Slugify (lowercase
// alphanumerics, interior dashes). Anything else under /manifest/ is not a
// slug and answers 404 before config is consulted.
var slugPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// themeColorPattern accepts #rgb / #rrggbb for the per-app theme_color.
// Anything else falls back to the dashboard color so a config typo cannot
// emit an invalid manifest member.
var themeColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{3}([0-9a-fA-F]{3})?$`)

// webManifestIcon is one entry of the manifest icons array.
type webManifestIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type"`
	Purpose string `json:"purpose,omitempty"`
}

// webManifest is the per-app Web App Manifest (M-536). Every URL is relative
// so installs keep working under a base_path (#436): the manifest is served
// from /manifest/<slug>.webmanifest, so one level up (../) is the dashboard
// root the start_url, scope, id and icons resolve against.
type webManifest struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	ShortName       string            `json:"short_name"`
	StartURL        string            `json:"start_url"`
	Scope           string            `json:"scope"`
	Display         string            `json:"display"`
	ThemeColor      string            `json:"theme_color"`
	BackgroundColor string            `json:"background_color"`
	Icons           []webManifestIcon `json:"icons"`
}

// muximuxManifestIcons are the dashboard PNGs every per-app manifest ends
// with. Chrome skips icons it cannot fetch or that are too small, so the
// fallbacks keep the install prompt working when the app's own icon is
// missing, SVG-only, or undersized.
var muximuxManifestIcons = []webManifestIcon{
	{Src: "../android-chrome-192x192.png", Sizes: "192x192", Type: "image/png", Purpose: "any maskable"},
	{Src: "../android-chrome-256x256.png", Sizes: "256x256", Type: "image/png", Purpose: "any maskable"},
	{Src: "../android-chrome-512x512.png", Sizes: "512x512", Type: "image/png", Purpose: "any maskable"},
}

// handleAppManifest serves GET /manifest/<slug>.webmanifest, a Web App
// Manifest that installs one dashboard app as its own standalone PWA instead
// of the whole dashboard. Unknown slugs, non-slug paths, and apps the caller
// may not see answer 404 (mirroring GET /api/app/<name>, so the endpoint
// cannot probe for gated apps). The route bypasses auth like /manifest.json
// — Chrome fetches the manifest on install — but per-app visibility still
// applies to the caller's session, if any.
func (s *Server) handleAppManifest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}
	rest, ok := strings.CutPrefix(r.URL.Path, "/manifest/")
	if !ok {
		http.NotFound(w, r)
		return
	}
	slug, ok := strings.CutSuffix(rest, ".webmanifest")
	if !ok || slug == "" || strings.Contains(slug, "/") || !slugPattern.MatchString(slug) {
		http.NotFound(w, r)
		return
	}
	s.configMu.RLock()
	var app *config.AppConfig
	for i := range s.config.Apps {
		if config.Slugify(s.config.Apps[i].Name) == slug {
			cp := s.config.Apps[i]
			app = &cp
			break
		}
	}
	s.configMu.RUnlock()
	if app == nil {
		http.NotFound(w, r)
		return
	}
	if !appManifestVisibleTo(app, auth.GetUserFromContext(r.Context())) {
		http.NotFound(w, r)
		return
	}
	manifest := webManifest{
		ID:              "../?app=" + slug,
		Name:            app.Name,
		ShortName:       app.Name,
		StartURL:        "../#" + slug,
		Scope:           "../",
		Display:         "standalone",
		ThemeColor:      manifestThemeColor(app.Color),
		BackgroundColor: "#09090b",
		Icons:           manifestIconsFor(app),
	}
	w.Header().Set(headerContentType, contentTypeManifest)
	// Like the other root static files, the manifest is content-addressed
	// nowhere, so it must revalidate: an app rename or icon change takes
	// effect on the next install check instead of sticking for a year.
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		// Encode errors are logged, not recovered: headers are committed
		// by now, matching the writeJSON convention (findings.md H8).
		if err := json.NewEncoder(w).Encode(manifest); err != nil {
			logging.Warn("Failed to write per-app manifest", "source", "server", "slug", slug, "error", err)
		}
	}
}

// appManifestVisibleTo is the visibility rule for the per-app manifest,
// mirroring handlers.appVisibleTo but hardened for an auth-bypassed route:
// with no session (Chrome's install-time fetch) only enabled apps without
// any gate are visible — never the setup-preview convention (empty role
// sees everything) the API list uses for pre-setup reads.
func appManifestVisibleTo(app *config.AppConfig, user *auth.User) bool {
	if user == nil {
		return app.Enabled && app.MinRole == "" && len(app.AllowedGroups) == 0 && !app.Access.Restricted()
	}
	role, username := user.Role, user.Username
	isAdmin := role == auth.RoleAdmin
	if !isAdmin && !app.Enabled {
		return false
	}
	if app.MinRole != "" && !auth.HasMinRole(role, app.MinRole) {
		return false
	}
	if !isAdmin && len(app.AllowedGroups) > 0 && !auth.InAnyGroup(user, app.AllowedGroups) {
		return false
	}
	if !isAdmin && app.Access.Restricted() && !app.Access.Allows(username, role) {
		return false
	}
	return true
}

// manifestThemeColor tints the installed window chrome with the app's color,
// falling back to the dashboard color on anything that is not a hex color.
func manifestThemeColor(color string) string {
	if themeColorPattern.MatchString(color) {
		return color
	}
	return "#09090b"
}

// manifestIconsFor lists the app's own raster icon first (Chrome prefers the
// first usable entry), always followed by the Muximux fallbacks. Lucide and
// remote (url) icons have no servable PNG at install sizes, so they resolve
// to the fallbacks alone rather than a URL Chrome would reject.
func manifestIconsFor(app *config.AppConfig) []webManifestIcon {
	icons := make([]webManifestIcon, 0, len(muximuxManifestIcons)+2)
	switch app.Icon.Type {
	case "custom":
		if isManifestRaster(app.Icon.File) {
			src := "../icons/custom/" + url.PathEscape(app.Icon.File)
			icons = append(icons,
				webManifestIcon{Src: src, Sizes: "192x192", Type: "image/png"},
				webManifestIcon{Src: src, Sizes: "512x512", Type: "image/png"},
			)
		}
	case "dashboard":
		if app.Icon.Name != "" {
			src := "../icons/dashboard/" + url.PathEscape(app.Icon.Name) + ".png"
			icons = append(icons,
				webManifestIcon{Src: src, Sizes: "192x192", Type: "image/png"},
				webManifestIcon{Src: src, Sizes: "512x512", Type: "image/png"},
			)
		}
	}
	return append(icons, muximuxManifestIcons...)
}

// isManifestRaster reports whether a custom icon file can plausibly back the
// 192/512 PNG entries: a raster extension Chrome installs from. SVG, extensionless
// uploads, and empty names fall back to the Muximux icons.
func isManifestRaster(file string) bool {
	if file == "" {
		return false
	}
	ext := strings.ToLower(file[strings.LastIndex(file, ".")+1:])
	switch ext {
	case "png", "jpg", "jpeg", "webp":
		return strings.Contains(file, ".")
	default:
		return false
	}
}
