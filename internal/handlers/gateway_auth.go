package handlers

import (
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/logging"
)

// GatewayAuthHandler implements the receive side of Caddy's
// forward_auth directive: every request to a require_auth=true
// gateway site arrives here over the loopback, this handler decides
// whether the upstream Muximux session permits the request.
//
// The handler must be registered on the INTERNAL mux only (the one
// bound to 127.0.0.1:<internal-port>). Exposing it on the external
// mux would let any anonymous client probe for session validity by
// reading the response shape.
type GatewayAuthHandler struct {
	sessionStore *auth.SessionStore
	cfg          *config.Config
	configMu     *sync.RWMutex
	// dashboardURL is the absolute URL the operator's browser uses
	// to reach the Muximux dashboard, used as the destination for
	// the login redirect when an anonymous client hits a gated
	// site. Computed from server.tls.domain when set, otherwise
	// from the Host header of the original request via X-Forwarded-
	// Proto + the configured listen port.
	dashboardURL string
}

// NewGatewayAuthHandler binds the handler to its dependencies. The
// dashboard URL is supplied at construction time so the handler does
// not have to re-derive it on every request. Pass empty when the
// operator has not configured server.tls.domain; the handler will
// build a relative redirect that works inside a browser visiting the
// gated subdomain (the browser fills in the host).
func NewGatewayAuthHandler(sessionStore *auth.SessionStore, cfg *config.Config, configMu *sync.RWMutex, dashboardURL string) *GatewayAuthHandler {
	return &GatewayAuthHandler{
		sessionStore: sessionStore,
		cfg:          cfg,
		configMu:     configMu,
		dashboardURL: strings.TrimRight(dashboardURL, "/"),
	}
}

// Forward implements GET /api/auth/forward. Caddy calls this once per
// incoming request to a gated site, supplying X-Forwarded-Host and
// X-Forwarded-Uri. We return:
//
//	200 OK + X-Muximux-User + X-Muximux-Role  - allowed
//	302 -> /login?next=<original>             - no session
//	403 + small explanatory body              - signed in but not permitted
//	500                                       - misconfiguration (no site match etc.)
//
// The 401-vs-403 split matters: anonymous gets a login redirect
// (logging in fixes the problem); permission-denied gets a "you're
// in but not allowed" page (logging in again won't help, but
// signing out + back in as someone else might).
func (h *GatewayAuthHandler) Forward(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		// Caddy issues a GET; reject anything else so a hostile
		// internal caller can't probe alternate semantics.
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		// Caddy always sets this on forward_auth requests. Empty
		// means either misconfiguration or a non-Caddy caller; fail
		// closed.
		logging.Warn("forward-auth received empty X-Forwarded-Host",
			"source", "audit", "remote", r.RemoteAddr)
		respondError(w, r, http.StatusBadRequest, "X-Forwarded-Host header is required")
		return
	}
	requestURI := r.Header.Get("X-Forwarded-Uri")

	// Strip the port from X-Forwarded-Host: when gateway_listen is
	// set Caddy passes "sonarr.example.com:8443" but our config
	// only stores the hostname.
	hostOnly := host
	if i := strings.IndexByte(host, ':'); i >= 0 {
		hostOnly = host[:i]
	}

	h.configMu.RLock()
	site := h.findGatewaySite(hostOnly)
	h.configMu.RUnlock()

	if site == nil {
		// Caddy is calling us for an unknown host. Either the
		// operator removed the site mid-flight (config edit
		// raced ahead of a Caddy reload) or someone is probing.
		// Fail closed; this is a server misconfiguration.
		logging.Warn("forward-auth called for unknown host",
			"source", "audit", "host", hostOnly, "remote", r.RemoteAddr)
		respondError(w, r, http.StatusInternalServerError, "gateway site not found for host "+hostOnly)
		return
	}
	if !site.RequireAuth {
		// Caddy shouldn't be calling us for an ungated site. Fail
		// open with 200 so the operator's traffic isn't blocked,
		// but log loudly so the operator notices the misconfig.
		logging.Warn("forward-auth called for ungated site",
			"source", "audit", "host", hostOnly)
		w.WriteHeader(http.StatusOK)
		return
	}

	session := h.sessionStore.GetFromRequest(r)
	if session == nil || session.IsExpired() {
		// No session: redirect to login with a validated next= param
		// pointing back at the original gated URL.
		h.redirectToLogin(w, r, host, requestURI)
		logging.Audit("forward-auth denied",
			"host", hostOnly, "user", "", "result", "no_session")
		return
	}

	// Permission check: same rules as the dashboard App access
	// gate so operators only learn one mental model.
	if denial := h.checkPermissions(session, site); denial != "" {
		h.serveForbidden(w, session.Username, site.Domain, denial)
		logging.Audit("forward-auth denied",
			"host", hostOnly, "user", session.Username,
			"role", session.Role, "result", denial)
		return
	}

	// Allowed. Stamp the user identity headers so backends that
	// honour them can correlate; backends that don't care will
	// ignore them.
	w.Header().Set("X-Muximux-User", session.Username)
	w.Header().Set("X-Muximux-Role", session.Role)
	w.WriteHeader(http.StatusOK)
	logging.Audit("forward-auth allowed",
		"host", hostOnly, "user", session.Username, "role", session.Role)
}

// findGatewaySite locates the GatewaySite for a host (case-
// insensitive). Returns nil when no site matches. Caller holds
// configMu (read or write).
func (h *GatewayAuthHandler) findGatewaySite(host string) *config.GatewaySite {
	for i := range h.cfg.Server.GatewaySites {
		if strings.EqualFold(h.cfg.Server.GatewaySites[i].Domain, host) {
			return &h.cfg.Server.GatewaySites[i]
		}
	}
	return nil
}

// checkPermissions returns "" when the session is allowed to access
// site, or a short label describing the denial reason (used as the
// audit-log result tag). Mirrors the App access gate exactly so the
// behaviour is shared between dashboard-embedded apps and gate-
// protected sites.
func (h *GatewayAuthHandler) checkPermissions(session *auth.Session, site *config.GatewaySite) string {
	// Admins bypass the role + group checks.
	if session.Role == auth.RoleAdmin {
		return ""
	}
	if site.MinRole != "" && !auth.HasMinRole(session.Role, site.MinRole) {
		return "role_insufficient"
	}
	if len(site.AllowedGroups) > 0 && !sessionInAllowedGroups(session, site.AllowedGroups) {
		return "group_mismatch"
	}
	return ""
}

// sessionInAllowedGroups checks the session's groups (case-
// insensitive) against the site's allow-list. Pulled out so a
// future change to where group state is stored (Session.Data
// vs a dedicated field) is a single-place edit.
func sessionInAllowedGroups(session *auth.Session, allowed []string) bool {
	if session.Data == nil {
		return false
	}
	raw, ok := session.Data["groups"]
	if !ok {
		return false
	}
	userGroups, ok := raw.([]string)
	if !ok {
		return false
	}
	for _, want := range allowed {
		for _, have := range userGroups {
			if strings.EqualFold(have, want) {
				return true
			}
		}
	}
	return false
}

// redirectToLogin issues a 302 to the dashboard's /login endpoint
// with a validated next= parameter so the user lands back on the
// originally-requested gated URL after signing in.
func (h *GatewayAuthHandler) redirectToLogin(w http.ResponseWriter, r *http.Request, host, requestURI string) {
	scheme := "https"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	originalURL := scheme + "://" + host + requestURI

	loginURL := h.dashboardURL + "/login"
	if h.dashboardURL == "" {
		// No configured dashboard URL means we have to send the
		// user to a relative path. Browsers will resolve against
		// the gated site's origin - which won't work because /login
		// lives on the dashboard, not the gated app. Fail closed
		// with a 503 telling the operator to configure tls.domain.
		respondError(w, r, http.StatusServiceUnavailable, "Muximux session expired; configure server.tls.domain so the auth gate can route operators to /login")
		return
	}

	u, err := url.Parse(loginURL)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "invalid dashboard URL: "+err.Error())
		return
	}
	q := u.Query()
	q.Set("next", originalURL)
	u.RawQuery = q.Encode()

	w.Header().Set("Location", u.String())
	w.WriteHeader(http.StatusFound)
}

// serveForbidden returns a 403 with a tiny HTML body. The body
// names the host and the denial reason so the operator can see at
// a glance why their session can't reach this site, and offers a
// sign-out link that lets them switch users without a new browser
// session.
func (h *GatewayAuthHandler) serveForbidden(w http.ResponseWriter, username, host, reason string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	// Minimal HTML; no styling beyond inline defaults. Operators
	// who want this prettier can put a proper edge proxy in front
	// or live with the plain page.
	body := `<!doctype html>
<html><head><title>403 Forbidden</title></head>
<body style="font-family:sans-serif;max-width:40em;margin:4em auto;padding:0 1em">
<h1>Forbidden</h1>
<p>You are signed in as <strong>` + htmlEscape(username) + `</strong> but your account
does not have permission to access <strong>` + htmlEscape(host) + `</strong>
(reason: <code>` + htmlEscape(reason) + `</code>).</p>
<p>Sign out and back in as a user with the right role or group membership.</p>
<p><a href="/logout">Sign out</a></p>
</body></html>`
	// G705 false-positive: every interpolated value runs through
	// htmlEscape above. The static template fragments are author-
	// controlled and contain no taint sink.
	_, _ = w.Write([]byte(body)) //nolint:gosec // values pre-escaped, see htmlEscape
}

// htmlEscaper is a package-level Replacer reused for every
// htmlEscape call. NewReplacer is cheap individually but allocates
// each time it runs, and the forbidden page fires htmlEscape three
// times per 403 response - so the hot path is "every gated request
// that ends up rejected". Promoting to package scope makes the
// escaper a constant.
var htmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"'", "&#39;",
)

// htmlEscape is a tiny inline escaper for the four characters that
// can break HTML when interpolated naively. The forbidden page only
// interpolates server-controlled values (username from session,
// host from config), but defence in depth is cheap here.
func htmlEscape(s string) string {
	return htmlEscaper.Replace(s)
}
