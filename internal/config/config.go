package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ThemeConfig holds theme selection
type ThemeConfig struct {
	Family  string `yaml:"family" json:"family"`   // "default", "nord", "dracula", etc.
	Variant string `yaml:"variant" json:"variant"` // "dark", "light", "system"
}

// Config holds all application configuration
type Config struct {
	ConfigVersion int               `yaml:"config_version"`
	Server        ServerConfig      `yaml:"server"`
	Auth          AuthConfig        `yaml:"auth"`
	Navigation    NavigationConfig  `yaml:"navigation"`
	Theme         ThemeConfig       `yaml:"theme" json:"theme"`
	Icons         IconsConfig       `yaml:"icons"`
	Health        HealthConfig      `yaml:"health"`
	Keybindings   KeybindingsConfig `yaml:"keybindings" json:"keybindings"`
	Discovery     DiscoveryConfig   `yaml:"discovery"`
	Groups        []GroupConfig     `yaml:"groups"`
	Apps          []AppConfig       `yaml:"apps"`

	// MissingEnvVars carries the names of any ${VAR} references the
	// loader could not resolve. Populated by Load; not serialised to
	// YAML/JSON. Callers (typically cmd/muximux/main.go after logging
	// is initialised) surface the list via Warn so a missing env var
	// doesn't silently leave a literal ${VAR} in a config field.
	MissingEnvVars []string `yaml:"-" json:"-"`
}

// KeybindingsConfig holds custom keyboard shortcut overrides
// Only stores customized bindings; defaults are managed client-side
type KeybindingsConfig struct {
	// Each key is an action name (e.g., "search", "refresh")
	// Each value is an array of key combos that trigger that action
	Bindings map[string][]KeyCombo `yaml:"bindings,omitempty" json:"bindings,omitempty"`
}

// KeyCombo represents a keyboard shortcut combination
type KeyCombo struct {
	Key   string `yaml:"key" json:"key"`
	Ctrl  bool   `yaml:"ctrl,omitempty" json:"ctrl,omitempty"`
	Alt   bool   `yaml:"alt,omitempty" json:"alt,omitempty"`
	Shift bool   `yaml:"shift,omitempty" json:"shift,omitempty"`
	Meta  bool   `yaml:"meta,omitempty" json:"meta,omitempty"`
}

// HealthConfig holds health monitoring settings
type HealthConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Interval string `yaml:"interval"` // Check interval, e.g., "30s", "1m"
	Timeout  string `yaml:"timeout"`  // Request timeout, e.g., "5s"
}

// DiscoveryConfig is the top-level container for service-discovery
// integrations. Today it carries Docker; future entries (Podman,
// nerdctl, ...) would sit alongside under their own sub-key.
type DiscoveryConfig struct {
	Docker DiscoveryDockerConfig `yaml:"docker"`
}

// DiscoveryDockerConfig configures the Docker engine API connection
// used by the discovery service to enumerate containers and refresh
// tracked-app URLs. Off by default; opt-in because it requires a
// privileged socket / network endpoint.
type DiscoveryDockerConfig struct {
	Enabled         bool               `yaml:"enabled" json:"enabled"`
	Endpoint        string             `yaml:"endpoint" json:"endpoint"` // unix:///... or tcp://host:port
	TLS             DiscoveryTLSConfig `yaml:"tls" json:"tls"`
	NetworkStrategy NetworkStrategy    `yaml:"network_strategy" json:"network_strategy"` // see NetworkStrategy constants
	HostIP          string             `yaml:"host_ip,omitempty" json:"host_ip,omitempty"`
	NetworkFilter   string             `yaml:"network_filter,omitempty" json:"network_filter,omitempty"`
	RefreshInterval string             `yaml:"refresh_interval" json:"refresh_interval"` // e.g. "60s"
}

// NetworkStrategy picks how a container's URL is constructed when
// the discovery scan + refresh poller resolve it. The defined type
// catches typos at the dozen-plus comparison sites across discovery/
// and config/ that would otherwise compile silently.
type NetworkStrategy string

const (
	// StrategyContainerIP: read the IP off the docker-network the
	// container is attached to; reachable from Muximux when it
	// runs on the same network or self-detect succeeds.
	StrategyContainerIP NetworkStrategy = "container_ip"
	// StrategyContainerDNS: hit the container by name through
	// docker's internal DNS; needs Muximux on a shared network
	// where docker dns resolves.
	StrategyContainerDNS NetworkStrategy = "container_dns"
	// StrategyHostPort: use the published host port mapping;
	// works from anywhere with reachability to the host IP.
	StrategyHostPort NetworkStrategy = "host_port"
	// StrategyHostDockerInternal: address the host via the
	// special docker DNS name; useful when Muximux itself runs
	// in a Desktop / WSL container.
	StrategyHostDockerInternal NetworkStrategy = "host_docker_internal"
)

// DiscoveryTLSConfig is the mTLS configuration used when the Docker
// endpoint is a tcp:// address requiring certificate authentication.
// Empty paths are tolerated only when Enabled is false.
type DiscoveryTLSConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	CACert     string `yaml:"ca_cert,omitempty" json:"ca_cert,omitempty"`
	ClientCert string `yaml:"client_cert,omitempty" json:"client_cert,omitempty"`
	ClientKey  string `yaml:"client_key,omitempty" json:"client_key,omitempty"`
}

// TLSConfig holds TLS/HTTPS settings
type TLSConfig struct {
	Domain string `yaml:"domain" json:"domain"`
	Email  string `yaml:"email" json:"email"`
	Cert   string `yaml:"cert" json:"cert"`
	Key    string `yaml:"key" json:"key"`
}

// ServerConfig holds HTTP server settings
// TLSMode names the four valid values of GatewaySite.TLS. The type is
// `string` underneath so YAML / JSON round-trip identically to the
// previous bare-string field, but using the named type at the call
// site lets readers see the valid range without consulting the
// validator.
type TLSMode string

const (
	// TLSModeDefault is the empty string and means "auto" by virtue
	// of the default-value rule applied in validateGatewaySite. It
	// is exposed as a named constant so a switch can compare against
	// it without sprinkling the literal `""` around.
	TLSModeDefault TLSMode = ""
	// TLSModeAuto issues a Let's Encrypt cert for the site.
	TLSModeAuto TLSMode = "auto"
	// TLSModeCustom serves the site with the operator-supplied cert
	// and key paths.
	TLSModeCustom TLSMode = "custom"
	// TLSModeNone serves the site over plain HTTP. Typically used
	// when Muximux is fronted by another reverse proxy that handles
	// TLS termination upstream.
	TLSModeNone TLSMode = "none"
)

// GatewaySite is a single subdomain/host served via Muximux's embedded
// Caddy. Replaces the legacy server.gateway: <path-to-file> approach
// with a structured representation editable through the Settings UI.
//
// Each site emits a Caddy site block at the public Domain, reverse-
// proxying to BackendURL. WebSocket upgrades, HTTP/2, and large
// uploads are inherited from Caddy's defaults — no flags required.
//
// The optional remedies (StripFrameBlockers, Streaming, ProxyHeaders)
// translate to response- and request-header tweaks in the generated
// Caddyfile so that subdomain-hosted apps can be embedded in
// Muximux's dashboard without going through the /proxy/{slug}/
// path-prefix rewriting layer.
type GatewaySite struct {
	// Domain is the public hostname Caddy will listen for (e.g.,
	// "sonarr.example.com"). Required.
	Domain string `yaml:"domain" json:"domain"`

	// BackendURL is the upstream the site forwards to (e.g.,
	// "http://sonarr:8989"). Required. Scheme must be http or https;
	// host must be non-empty; loopback paired with Muximux's own listen
	// port is rejected to prevent self-loops.
	BackendURL string `yaml:"backend_url" json:"backend_url"`

	// TLS controls how Caddy serves this site:
	//   TLSModeAuto    - Let's Encrypt (also the default when empty)
	//   TLSModeCustom  - serve with TLSCert + TLSKey
	//   TLSModeNone    - HTTP only (typical when fronted by another proxy)
	TLS     TLSMode `yaml:"tls,omitempty" json:"tls,omitempty"`
	TLSCert string  `yaml:"tls_cert,omitempty" json:"tls_cert,omitempty"`
	TLSKey  string  `yaml:"tls_key,omitempty" json:"tls_key,omitempty"`

	// StripFrameBlockers removes X-Frame-Options on the response and
	// splices Muximux's origin into a Content-Security-Policy
	// frame-ancestors directive, so the dashboard can iframe this
	// subdomain even if the backend serves restrictive headers. Off
	// by default; only enable for self-hosted backends you trust.
	StripFrameBlockers bool `yaml:"strip_frame_blockers,omitempty" json:"strip_frame_blockers,omitempty"`

	// Streaming sets `flush_interval -1` on the reverse_proxy so
	// long-lived response streams (Server-Sent Events, video
	// transcodes, live dashboards) are flushed continuously. Plex,
	// Jellyfin, Grafana, Home Assistant: on. Most apps: leave off.
	Streaming bool `yaml:"streaming,omitempty" json:"streaming,omitempty"`

	// ProxyHeaders are HTTP headers injected on the upstream request.
	// Use to forward backend API keys, Authorization tokens, etc.
	ProxyHeaders map[string]string `yaml:"proxy_headers,omitempty" json:"proxy_headers,omitempty"`

	// ForwardedHeaders defaults to true: emit X-Forwarded-Proto,
	// X-Forwarded-Host, X-Forwarded-For, X-Real-IP. Set false for
	// backends that reject those headers.
	ForwardedHeaders *bool `yaml:"forwarded_headers,omitempty" json:"forwarded_headers,omitempty"`

	// AppName, when set, links this site to apps[].name. The paired
	// App's URL is derived from this site's Domain and locked in
	// the App form so the two stay in sync.
	AppName string `yaml:"app_name,omitempty" json:"app_name,omitempty"`

	// Docker auto-management. Same semantics as AppConfig: BackendURL
	// is refreshed by the discovery poller while DockerKey is set.
	DockerKey      string `yaml:"docker_key,omitempty" json:"docker_key,omitempty"`
	DockerEndpoint string `yaml:"docker_endpoint,omitempty" json:"docker_endpoint,omitempty"`
	DockerStrategy string `yaml:"docker_strategy,omitempty" json:"docker_strategy,omitempty"`

	// Gateway auth gate. When RequireAuth is true, Caddy's
	// forward_auth directive routes every request to this site
	// through Muximux's session check before forwarding to the
	// backend. Anonymous clients are redirected to /login;
	// authenticated clients are checked against MinRole + AllowedGroups
	// using the same rules AppConfig uses for dashboard access.
	//
	// Requires server.session_cookie_domain to be set so the
	// session cookie crosses subdomain boundaries. Validated at
	// config load.
	RequireAuth bool `yaml:"require_auth,omitempty" json:"require_auth,omitempty"`
	// MinRole is one of "user", "power-user", "admin". Empty means
	// any authenticated user passes. Ignored when RequireAuth is
	// false. Same semantics as AppConfig.MinRole.
	MinRole string `yaml:"min_role,omitempty" json:"min_role,omitempty"`
	// AllowedGroups limits gate access to users in at least one of
	// these groups (case-insensitive). Admins bypass the check.
	// Empty means no group restriction. Ignored when RequireAuth is
	// false. Same semantics as AppConfig.AllowedGroups.
	AllowedGroups []string `yaml:"allowed_groups,omitempty" json:"allowed_groups,omitempty"`
}

type ServerConfig struct {
	Listen       string    `yaml:"listen" json:"listen"`
	BasePath     string    `yaml:"base_path" json:"base_path"` // e.g. "/muximux" — for serving behind a reverse proxy subpath
	Title        string    `yaml:"title" json:"title"`
	Language     string    `yaml:"language" json:"language"` // BCP 47 tag: "en", "sv", "ar", etc.
	LogLevel     string    `yaml:"log_level" json:"log_level"`
	LogFormat    string    `yaml:"log_format" json:"log_format"`       // "text" or "json" (default: "text")
	ProxyTimeout string    `yaml:"proxy_timeout" json:"proxy_timeout"` // e.g. "30s", "1m" — timeout for proxied requests
	TLS          TLSConfig `yaml:"tls" json:"tls"`
	// Gateway was the path to an operator-written Caddyfile of extra
	// sites. Removed in v3.1.0; the field is kept on the struct so
	// strict YAML decode does not reject it with an unhelpful "unknown
	// field" message. Any non-empty value is rejected at startup with
	// a migration message pointing at `gateway_sites` and the
	// `muximux migrate-gateway` CLI helper.
	Gateway      string        `yaml:"gateway,omitempty" json:"gateway,omitempty"`
	GatewaySites []GatewaySite `yaml:"gateway_sites,omitempty" json:"gateway_sites,omitempty"`
	// GatewayListen overrides the default Caddy bind for gateway
	// sites. Empty (default) means Caddy binds the privileged ports
	// 80 + 443 with auto-HTTPS - the standard topology for hosts that
	// own the public IP. Non-empty (e.g. ":8443") binds that address
	// instead and serves all gateway sites as plain HTTP unless the
	// site has TLS=custom (operator-supplied cert). Use this when
	// Muximux runs behind another reverse proxy that handles TLS
	// termination, or when you don't want to grant the binary
	// CAP_NET_BIND_SERVICE / run it as root.
	//
	// Format: ":<port>" or "<host>:<port>" (anything net.Listen
	// accepts). Validated at config load.
	GatewayListen string `yaml:"gateway_listen,omitempty" json:"gateway_listen,omitempty"`

	// SessionCookieDomain, when non-empty, sets the Domain attribute
	// on the Muximux session cookie so it crosses subdomain
	// boundaries. Required when any gateway site has require_auth=true
	// because the gate uses the Muximux session as the auth source
	// and the session cookie must be visible at both the dashboard
	// host AND each gated site.
	//
	// Format: ".example.com" (leading dot, browser-canonical) or
	// "example.com" (browser normalises). The dashboard host and
	// every gateway site with require_auth=true must be a subdomain
	// of this value. Validated at config load.
	SessionCookieDomain string `yaml:"session_cookie_domain,omitempty" json:"session_cookie_domain,omitempty"`
}

// NormalizedBasePath returns the base path with a leading slash and no trailing slash.
// Returns "" if no base path is configured.
func (c *ServerConfig) NormalizedBasePath() string {
	p := strings.TrimRight(c.BasePath, "/")
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

// NeedsCaddy returns true if TLS, the legacy gateway file, or any
// structured gateway site is configured, meaning Caddy should start
// to handle the user-facing port. The legacy file branch is kept so
// callers that run before c.validate() (rare) still report correctly;
// validate() rejects that field separately at startup.
func (c *ServerConfig) NeedsCaddy() bool {
	return c.TLS.Domain != "" || c.TLS.Cert != "" || c.Gateway != "" || len(c.GatewaySites) > 0
}

// AuthConfig holds authentication settings
type AuthConfig struct {
	Method         string            `yaml:"method"` // none, builtin, forward_auth, oidc
	Users          []UserConfig      `yaml:"users"`
	TrustedProxies []string          `yaml:"trusted_proxies"`
	Headers        map[string]string `yaml:"headers"`
	LogoutURL      string            `yaml:"logout_url"` // External auth provider logout URL (forward_auth)
	// ForwardAuthAdminGroups names the upstream groups that elevate a
	// forward-auth user to admin. Empty falls back to the
	// case-insensitive ["admin", "admins", "administrators"] default,
	// which preserves prior behaviour. Mirrors auth.oidc.admin_groups
	// so an operator running Authelia / Authentik with a custom
	// admin-group name (e.g. "dashboard-admins") can grant admin
	// without renaming their IdP groups.
	ForwardAuthAdminGroups []string   `yaml:"forward_auth_admin_groups,omitempty"`
	OIDC                   OIDCConfig `yaml:"oidc"`
	SessionMaxAge          string     `yaml:"session_max_age"` // e.g., "24h", "7d"
	SecureCookies          bool       `yaml:"secure_cookies"`
	APIKeyHash             string     `yaml:"api_key_hash,omitempty"` // bcrypt hash of API key
	SetupComplete          bool       `yaml:"setup_complete"`
}

// UserConfig holds local user credentials
type UserConfig struct {
	Username     string   `yaml:"username"`
	PasswordHash string   `yaml:"password_hash"`
	Role         string   `yaml:"role"`
	Email        string   `yaml:"email,omitempty"`
	DisplayName  string   `yaml:"display_name,omitempty"`
	Groups       []string `yaml:"groups,omitempty"` // optional group memberships used by app-level allowed_groups filtering
}

// OIDCConfig holds OIDC provider settings
type OIDCConfig struct {
	Enabled          bool     `yaml:"enabled"`
	IssuerURL        string   `yaml:"issuer_url"`
	ClientID         string   `yaml:"client_id"`
	ClientSecret     string   `yaml:"client_secret"`
	RedirectURL      string   `yaml:"redirect_url"`
	Scopes           []string `yaml:"scopes"`
	UsernameClaim    string   `yaml:"username_claim"`
	EmailClaim       string   `yaml:"email_claim"`
	GroupsClaim      string   `yaml:"groups_claim"`
	DisplayNameClaim string   `yaml:"display_name_claim"`
	AdminGroups      []string `yaml:"admin_groups"`
}

// NavigationConfig holds navigation layout settings
type NavigationConfig struct {
	Position           string         `yaml:"position" json:"position"` // top, left, right, bottom, floating
	Width              string         `yaml:"width" json:"width"`
	AutoHide           bool           `yaml:"auto_hide" json:"auto_hide"`
	AutoHideDelay      string         `yaml:"auto_hide_delay" json:"auto_hide_delay"`
	ShowOnHover        bool           `yaml:"show_on_hover" json:"show_on_hover"`
	ShowLabels         bool           `yaml:"show_labels" json:"show_labels"`
	ShowLogo           bool           `yaml:"show_logo" json:"show_logo"`
	ShowHomeButton     bool           `yaml:"show_home_button" json:"show_home_button"`
	HomeIcon           *AppIconConfig `yaml:"home_icon,omitempty" json:"home_icon,omitempty"`
	ShowAppColors      bool           `yaml:"show_app_colors" json:"show_app_colors"`
	ShowIconBackground bool           `yaml:"show_icon_background" json:"show_icon_background"`
	IconScale          float64        `yaml:"icon_scale" json:"icon_scale"`
	ShowSplashOnStart  bool           `yaml:"show_splash_on_startup" json:"show_splash_on_startup"`
	ShowShadow         bool           `yaml:"show_shadow" json:"show_shadow"`
	FloatingPosition   string         `yaml:"floating_position" json:"floating_position"` // bottom-right, bottom-left, top-right, top-left
	BarStyle           string         `yaml:"bar_style" json:"bar_style"`                 // grouped, flat (top/bottom bars only)
	HideSidebarFooter  bool           `yaml:"hide_sidebar_footer" json:"hide_sidebar_footer"`
	MaxOpenTabs        int            `yaml:"max_open_tabs" json:"max_open_tabs"`
}

// IconsConfig holds icon settings
type IconsConfig struct {
	DashboardIcons DashboardIconsConfig `yaml:"dashboard_icons"`
}

// DashboardIconsConfig holds Dashboard Icons integration settings
type DashboardIconsConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Mode     string `yaml:"mode"` // on_demand, prefetch, offline
	CacheDir string `yaml:"cache_dir"`
	CacheTTL string `yaml:"cache_ttl"`
}

// GroupConfig holds app group settings
type GroupConfig struct {
	Name     string        `yaml:"name" json:"name"`
	Icon     AppIconConfig `yaml:"icon" json:"icon"`
	Color    string        `yaml:"color" json:"color"`
	Order    int           `yaml:"order" json:"order"`
	Expanded bool          `yaml:"expanded" json:"expanded"`
}

// AppConfig holds individual app settings
type AppConfig struct {
	Name                string            `yaml:"name"`
	URL                 string            `yaml:"url"`
	HealthURL           string            `yaml:"health_url,omitempty"` // Optional custom health check URL
	Icon                AppIconConfig     `yaml:"icon"`
	Color               string            `yaml:"color"`
	Group               string            `yaml:"group"`
	Order               int               `yaml:"order"`
	Enabled             bool              `yaml:"enabled"`
	Default             bool              `yaml:"default"`
	OpenMode            string            `yaml:"open_mode"`                                            // iframe, new_tab, new_window, redirect
	HealthCheck         *bool             `yaml:"health_check,omitempty" json:"health_check,omitempty"` // opt-in: nil/false = disabled, true = enabled
	Proxy               bool              `yaml:"proxy"`
	ProxySkipTLSVerify  *bool             `yaml:"proxy_skip_tls_verify,omitempty"` // nil = true (default: skip)
	ProxyHeaders        map[string]string `yaml:"proxy_headers,omitempty"`         // custom headers sent to backend
	Scale               float64           `yaml:"scale"`
	Shortcut            *int              `yaml:"shortcut,omitempty" json:"shortcut,omitempty"` // 1-9 keyboard shortcut slot
	MinRole             string            `yaml:"min_role,omitempty"`                           // minimum role to see this app (default: "user")
	AllowedGroups       []string          `yaml:"allowed_groups,omitempty"`                     // optional group allowlist; user must be in at least one of these groups (empty = no group gate)
	ForceIconBackground bool              `yaml:"force_icon_background,omitempty"`              // show icon background even when global setting is off
	AuthBypass          []AuthBypassRule  `yaml:"auth_bypass"`
	Access              AppAccessConfig   `yaml:"access"`
	// Permissions is the list of browser feature policy permissions delegated to
	// the iframe (camera, microphone, geolocation, fullscreen, display-capture,
	// clipboard-read, clipboard-write, autoplay, midi, payment). Empty = no
	// permissions delegated (browser default for cross-origin iframes).
	Permissions []string `yaml:"permissions,omitempty" json:"permissions,omitempty"`
	// AllowNotifications enables the Muximux notification bridge for this app:
	// the embedded iframe can postMessage {type: 'muximux:notify', title, body, tag}
	// to request a browser notification via Muximux's top-level origin.
	AllowNotifications bool `yaml:"allow_notifications,omitempty" json:"allow_notifications,omitempty"`

	// Docker auto-management. When DockerKey is non-empty, the URL of
	// this app is updated by the discovery poller from the current
	// state of the matching container. Editing the URL through the
	// API or YAML detaches the app via the dedicated DELETE endpoint
	// (the SaveConfig path preserves these fields when omitted, so a
	// frontend bug or scripted PUT cannot accidentally clear tracking).
	DockerKey      string `yaml:"docker_key,omitempty" json:"docker_key,omitempty"`
	DockerEndpoint string `yaml:"docker_endpoint,omitempty" json:"docker_endpoint,omitempty"`
	DockerStrategy string `yaml:"docker_strategy,omitempty" json:"docker_strategy,omitempty"`
}

// AppIconConfig holds app icon settings
type AppIconConfig struct {
	Type       string `yaml:"type" json:"type"` // dashboard, lucide, custom, url
	Name       string `yaml:"name,omitempty" json:"name,omitempty"`
	File       string `yaml:"file,omitempty" json:"file,omitempty"`
	URL        string `yaml:"url,omitempty" json:"url,omitempty"`
	Variant    string `yaml:"variant,omitempty" json:"variant,omitempty"`
	Color      string `yaml:"color,omitempty" json:"color"`
	Background string `yaml:"background,omitempty" json:"background"`
	Invert     bool   `yaml:"invert,omitempty" json:"invert,omitempty"`
}

// AuthBypassRule defines a path that bypasses auth
type AuthBypassRule struct {
	Path          string   `yaml:"path"`
	Methods       []string `yaml:"methods"`
	RequireAPIKey bool     `yaml:"require_api_key"`
	AllowedIPs    []string `yaml:"allowed_ips"`
}

// AppAccessConfig defines who can access an app
type AppAccessConfig struct {
	Roles []string `yaml:"roles"`
	Users []string `yaml:"users"`
}

// Load reads configuration from a YAML file.
// Environment variables referenced as ${VAR} in config values are expanded.
// Bare $VAR references are NOT expanded to avoid corrupting bcrypt hashes.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return defaultConfig(), nil
		}
		return nil, err
	}

	// Expand only ${VAR} (braced) environment variables — bare $VAR is NOT
	// expanded because bcrypt hashes like $2a$10$... would be corrupted.
	// The list of unresolved names is surfaced through MissingEnvVars
	// so the caller can warn after logging is initialised.
	expanded, missingEnv := expandBracedEnv(string(data))

	cfg := defaultConfig()
	// Strict decode: reject YAML that contains fields not declared on
	// our types. Without this, a hand-edited file with a typo'd field
	// (or a stale field carried over from a prior version) is silently
	// dropped and the operator wonders why their setting "did
	// nothing." Mirrors what the import/restore endpoints already
	// do, so startup and import behave consistently.
	dec := yaml.NewDecoder(strings.NewReader(expanded))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil {
		return nil, err
	}
	cfg.MissingEnvVars = missingEnv

	// Normalize zero-value fields that have non-zero defaults
	if cfg.Navigation.IconScale <= 0 {
		cfg.Navigation.IconScale = 1.0
	}

	// Splash on startup requires the home button to be visible,
	// otherwise the user has no way to get back to the overview.
	if !cfg.Navigation.ShowHomeButton {
		cfg.Navigation.ShowSplashOnStart = false
	}

	// Discovery defaults: when the operator enables Docker discovery
	// without spelling out an endpoint or a network strategy we pick
	// the most common safe values (unix socket, container_ip). When
	// disabled, the zero values are fine and we don't touch them.
	if cfg.Discovery.Docker.Enabled {
		if cfg.Discovery.Docker.Endpoint == "" {
			cfg.Discovery.Docker.Endpoint = "unix:///var/run/docker.sock"
		}
		if cfg.Discovery.Docker.NetworkStrategy == "" {
			cfg.Discovery.Docker.NetworkStrategy = StrategyContainerIP
		}
		if cfg.Discovery.Docker.RefreshInterval == "" {
			cfg.Discovery.Docker.RefreshInterval = "60s"
		}
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// expandBracedEnv replaces ${VAR} references with environment variable values.
// Unlike os.ExpandEnv, bare $VAR references are left untouched so that values
// like bcrypt hashes ($2a$10$...) are not corrupted.
var bracedEnvRe = regexp.MustCompile(`\$\{([^}]+)\}`)

// IsBracedEnvRef reports whether s looks like a ${VAR} environment variable reference.
func IsBracedEnvRef(s string) bool {
	return bracedEnvRe.MatchString(s)
}

// expandBracedEnv expands ${VAR} references and returns the expanded
// text plus a deduplicated list of any variables that were referenced
// but not present in the environment. Callers can surface that list
// to the operator so a missing env var doesn't silently leave a
// literal ${VAR} in a config field (which then fails confusingly at
// the IdP / TLS provisioning layer).
func expandBracedEnv(s string) (string, []string) {
	missing := map[string]struct{}{}
	expanded := bracedEnvRe.ReplaceAllStringFunc(s, func(match string) string {
		key := match[2 : len(match)-1] // strip ${ and }
		if val, ok := os.LookupEnv(key); ok {
			return val
		}
		missing[key] = struct{}{}
		return match // leave ${VAR} literal if env var is not set
	})
	if len(missing) == 0 {
		return expanded, nil
	}
	out := make([]string, 0, len(missing))
	for k := range missing {
		out = append(out, k)
	}
	return expanded, out
}

// Validate is the public entry point for the same invariant
// checks Load runs at startup. SaveConfig calls it so a bad
// runtime mutation is rejected with a 400 before being persisted
// rather than silently breaking the next boot.
func (c *Config) Validate() error {
	return c.validate()
}

// validate checks the configuration for contradictory or incomplete settings.
func (c *Config) validate() error {
	tls := c.Server.TLS

	if tls.Domain != "" && tls.Email == "" {
		return fmt.Errorf("tls.email is required when tls.domain is set")
	}
	// Reject tls.email without tls.domain: there is no flow that
	// consumes the address (ACME issuance is what would use it, and
	// that requires tls.domain). Silently accepting it leaves the
	// operator believing ACME is wired up when nothing has changed.
	if tls.Email != "" && tls.Domain == "" {
		return fmt.Errorf("tls.email is set but tls.domain is empty; ACME issuance only runs when tls.domain is configured")
	}
	if (tls.Cert != "") != (tls.Key != "") {
		return fmt.Errorf("tls.cert and tls.key must both be set, or both empty")
	}
	if tls.Domain != "" && tls.Cert != "" {
		return fmt.Errorf("use tls.domain or tls.cert/tls.key, not both")
	}
	if c.Server.Gateway != "" {
		// Removed in v3.1.0: the file-based gateway is no longer
		// supported. Surface a migration message rather than letting
		// the operator's instance start with a partial configuration.
		return fmt.Errorf("server.gateway is no longer supported (removed in v3.1.0).\n\n"+
			"Migrate your existing Caddyfile to gateway_sites:\n\n"+
			"  muximux migrate-gateway %s\n\n"+
			"Then paste the printed YAML under server.gateway_sites: in your\n"+
			"config.yaml and remove the legacy server.gateway: line.\n\n"+
			"See https://github.com/mescon/Muximux/wiki/tls-and-gateway#migration",
			c.Server.Gateway)
	}

	if err := validateGatewayListen(c.Server.GatewayListen); err != nil {
		return err
	}

	if err := validateSessionCookieDomain(c); err != nil {
		return err
	}

	if err := validateGatewaySites(c.Server.GatewaySites, c); err != nil {
		return err
	}

	return nil
}

// validateSessionCookieDomain enforces the cross-subdomain cookie
// rules required by the gateway auth gate. The dashboard host and
// every gateway site with require_auth=true must be a subdomain of
// the configured value, so the session cookie reaches all of them.
//
// Empty SessionCookieDomain is allowed for deployments that don't
// use the auth gate; in that case any gateway site with require_auth
// would still need to be rejected (handled here too).
func validateSessionCookieDomain(c *Config) error {
	gateAuthSites := []string{}
	for i := range c.Server.GatewaySites {
		if c.Server.GatewaySites[i].RequireAuth {
			gateAuthSites = append(gateAuthSites, c.Server.GatewaySites[i].Domain)
		}
	}

	if c.Server.SessionCookieDomain == "" {
		if len(gateAuthSites) > 0 {
			return fmt.Errorf("server.session_cookie_domain is required when any gateway site has require_auth=true (gated sites: %v)", gateAuthSites)
		}
		return nil
	}

	// Normalise: trim leading dot for the subdomain check so
	// ".example.com" and "example.com" both compare against
	// "sonarr.example.com" the same way.
	parent := strings.TrimPrefix(c.Server.SessionCookieDomain, ".")
	if parent == "" {
		return fmt.Errorf("server.session_cookie_domain %q is invalid (empty after trimming dot)", c.Server.SessionCookieDomain)
	}
	// The TLS domain (or each gateway site) must end with the
	// parent. Use a labelled-suffix match so "evil-example.com"
	// does not satisfy parent="example.com".
	mustBeUnder := func(host, label string) error {
		if host == "" {
			return nil
		}
		if !hostIsUnderParent(host, parent) {
			return fmt.Errorf("%s %q is not under server.session_cookie_domain %q; either change %s or unset session_cookie_domain", label, host, c.Server.SessionCookieDomain, label)
		}
		return nil
	}

	if err := mustBeUnder(c.Server.TLS.Domain, "server.tls.domain"); err != nil {
		return err
	}
	for _, dom := range gateAuthSites {
		if err := mustBeUnder(dom, "gateway site "+dom); err != nil {
			return err
		}
	}
	return nil
}

// hostIsUnderParent returns true when host == parent OR host ends
// with ".parent". Avoids the substring trap where "evil-example.com"
// would otherwise pass a naive HasSuffix("example.com") check.
func hostIsUnderParent(host, parent string) bool {
	host = strings.ToLower(host)
	parent = strings.ToLower(parent)
	if host == parent {
		return true
	}
	return strings.HasSuffix(host, "."+parent)
}

// validateGatewayListen accepts an empty string (use defaults) or any
// address that net.Listen would accept. Rejects malformed values up
// front so the operator gets a clear error before Caddy is asked to
// bind something it cannot parse.
func validateGatewayListen(addr string) error {
	if addr == "" {
		return nil
	}
	// Accept ":port" or "host:port"; net.SplitHostPort handles both.
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("server.gateway_listen %q is not a valid bind address (want \":port\" or \"host:port\"): %w", addr, err)
	}
	if port == "" {
		return fmt.Errorf("server.gateway_listen %q must include a port number", addr)
	}
	if _, perr := net.LookupPort("tcp", port); perr != nil {
		return fmt.Errorf("server.gateway_listen %q has an invalid port %q: %w", addr, port, perr)
	}
	// Empty host is fine - means "all interfaces". Reject only
	// obviously broken non-numeric ports above; literal IPv6
	// addresses are caller's responsibility.
	_ = host
	return nil
}

// ValidateGatewaySites is the public entry point for the REST handler's
// dry-run validation path. It re-uses the same rules the YAML loader
// applies, so a candidate config that passes ValidateGatewaySites is
// guaranteed to load cleanly when persisted to disk and reloaded.
//
// `cfg` provides both the server-level fields (TLS domain collision
// check) and the apps list (AppName cross-reference); pass the live
// Config or a synthesized one with at least the relevant fields set.
// Passing nil disables the cross-app check, which is appropriate for
// callers that are validating in isolation (e.g. unit tests) but is
// generally a smell — the loader and the REST handler always have a
// real Config available.
func ValidateGatewaySites(sites []GatewaySite, cfg *Config) error {
	return validateGatewaySites(sites, cfg)
}

// validateGatewaySites is exported via ValidateGatewaySites; this name
// is kept for the existing internal callsites in this file.
func validateGatewaySites(sites []GatewaySite, cfg *Config) error {
	if len(sites) == 0 {
		return nil
	}
	// Pre-compute the app-name set once so the per-site loop is O(N)
	// rather than O(N*M).
	var appNames map[string]struct{}
	if cfg != nil && len(cfg.Apps) > 0 {
		appNames = make(map[string]struct{}, len(cfg.Apps))
		for i := range cfg.Apps {
			appNames[cfg.Apps[i].Name] = struct{}{}
		}
	}
	// Track app_name use across sites so we can flag duplicate
	// pairings (two sites both claiming the same app — the type
	// analyzer's last-write-wins concern).
	appNameSeen := make(map[string]int, len(sites))
	seen := make(map[string]struct{}, len(sites))
	for i := range sites {
		s := &sites[i]
		var srv *ServerConfig
		if cfg != nil {
			srv = &cfg.Server
		}
		if err := validateGatewaySite(s, srv); err != nil {
			return fmt.Errorf("gateway_sites[%d] (%q): %w", i, s.Domain, err)
		}
		// Domain matching is case-insensitive in DNS land; canonicalise
		// before the duplicate check so "Sonarr.example.com" and
		// "sonarr.example.com" are treated as one.
		key := strings.ToLower(s.Domain)
		if _, dup := seen[key]; dup {
			return fmt.Errorf("gateway_sites[%d]: duplicate domain %q", i, s.Domain)
		}
		seen[key] = struct{}{}

		if s.AppName != "" {
			// Cross-reference against the apps list. A dangling
			// app_name silently desyncs the Settings UI badge; we
			// reject loudly at validation time. The cleanup path in
			// DeleteApp / UpdateApp keeps this invariant impossible
			// to trip during normal use.
			if appNames != nil {
				if _, ok := appNames[s.AppName]; !ok {
					return fmt.Errorf("gateway_sites[%d]: app_name %q does not match any apps[].name", i, s.AppName)
				}
			}
			if firstIdx, dup := appNameSeen[s.AppName]; dup {
				return fmt.Errorf("gateway_sites[%d]: app_name %q is already used by gateway_sites[%d] (%q)", i, s.AppName, firstIdx, sites[firstIdx].Domain)
			}
			appNameSeen[s.AppName] = i
		}
	}
	return nil
}

// validateGatewaySite enforces the per-site invariants. Validation is
// intentionally strict so problems are caught at config-load time
// rather than at Caddy-load time, where the error message is harder
// to act on.
func validateGatewaySite(s *GatewaySite, srv *ServerConfig) error {
	if s.Domain == "" {
		return fmt.Errorf("domain is required")
	}
	if !isValidGatewayDomain(s.Domain) {
		return fmt.Errorf("domain %q is not a valid hostname", s.Domain)
	}
	if srv != nil && srv.TLS.Domain != "" && strings.EqualFold(s.Domain, srv.TLS.Domain) {
		return fmt.Errorf("domain %q collides with server.tls.domain; gateway sites must use distinct hostnames", s.Domain)
	}

	if s.BackendURL == "" {
		return fmt.Errorf("backend_url is required")
	}
	u, err := url.Parse(s.BackendURL)
	if err != nil {
		return fmt.Errorf("backend_url %q is not a valid URL: %w", s.BackendURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("backend_url %q must use http or https", s.BackendURL)
	}
	if u.Host == "" {
		return fmt.Errorf("backend_url %q is missing a host", s.BackendURL)
	}
	// Caddy's reverse_proxy upstream syntax is scheme://host[:port] only:
	// path, query, and fragment are not allowed. Catching them here turns
	// a confusing late-stage Caddy parse error ("invalid upstream") into
	// a clear validator message at the originating field.
	if u.Path != "" && u.Path != "/" {
		return fmt.Errorf("backend_url %q must not include a path; the structured form forwards the inbound request path as-is", s.BackendURL)
	}
	if u.RawQuery != "" {
		return fmt.Errorf("backend_url %q must not include a query string", s.BackendURL)
	}
	if u.Fragment != "" {
		return fmt.Errorf("backend_url %q must not include a fragment", s.BackendURL)
	}
	// Reject 0.0.0.0 and IPv4 link-local; never legitimate as upstream
	// targets and easy to type by mistake. Private IPs (10/8, 172.16/12,
	// 192.168/16) are explicitly allowed because that is where most
	// homelab backends live.
	hostname := u.Hostname()
	if hostname == "0.0.0.0" {
		return fmt.Errorf("backend_url %q points at 0.0.0.0; specify a real host or 127.0.0.1", s.BackendURL)
	}
	if ip := net.ParseIP(hostname); ip != nil && ip.IsLinkLocalUnicast() {
		return fmt.Errorf("backend_url %q targets a link-local address; this is rarely intentional", s.BackendURL)
	}
	// Reject the obvious self-loop: backend pointing at the Muximux
	// HTTP listener itself. Letting this through would make every
	// gateway request bounce back into the Go server, blow stacks
	// quickly, and leak an unauthenticated proxy hop. We only
	// recognise the cases we can be *sure* about (wildcard-listen +
	// loopback-target, and exact host+port match) so a backend on a
	// different machine that happens to share a port is left alone.
	if srv != nil {
		backendPort := u.Port()
		if backendPort == "" {
			if u.Scheme == "https" {
				backendPort = "443"
			} else {
				backendPort = "80"
			}
		}
		if isSelfLoop(hostname, backendPort, srv.Listen) {
			return fmt.Errorf("backend_url %q points at Muximux's own listener %q; this would loop the proxy back on itself", s.BackendURL, srv.Listen)
		}
	}

	// Validate ProxyHeaders entries. Caddy emits these as `header_up
	// <name> <value>` directives; an unvalidated name would let an
	// admin-side typo (a space, a newline) corrupt the Caddyfile and
	// break Caddy's reload silently. RFC 7230 token grammar covers
	// real-world header names and rejects every shape that would
	// confuse the Caddyfile parser.
	for k, v := range s.ProxyHeaders {
		if !isValidHeaderName(k) {
			return fmt.Errorf("proxy_headers key %q is not a valid HTTP header name (RFC 7230 token)", k)
		}
		if !isValidHeaderValue(v) {
			return fmt.Errorf("proxy_headers value for %q contains an invalid character (CR / LF / NUL / quote)", k)
		}
	}

	switch s.TLS {
	case TLSModeDefault, TLSModeAuto:
		if s.TLSCert != "" || s.TLSKey != "" {
			return fmt.Errorf("tls_cert / tls_key are only valid when tls is %q", TLSModeCustom)
		}
	case TLSModeCustom:
		if s.TLSCert == "" || s.TLSKey == "" {
			return fmt.Errorf("tls=%q requires both tls_cert and tls_key", TLSModeCustom)
		}
		if _, err := os.Stat(s.TLSCert); err != nil {
			return fmt.Errorf("tls_cert %q not readable: %w", s.TLSCert, err)
		}
		if _, err := os.Stat(s.TLSKey); err != nil {
			return fmt.Errorf("tls_key %q not readable: %w", s.TLSKey, err)
		}
	case TLSModeNone:
		if s.TLSCert != "" || s.TLSKey != "" {
			return fmt.Errorf("tls_cert / tls_key are only valid when tls is %q", TLSModeCustom)
		}
	default:
		return fmt.Errorf("tls=%q is invalid; expected %q, %q, or %q", s.TLS, TLSModeAuto, TLSModeCustom, TLSModeNone)
	}

	// Gateway auth-gate fields. Skip the checks entirely when
	// require_auth is false - min_role and allowed_groups are
	// silently inert in that case (already documented on the
	// struct). When require_auth is true, min_role must be one
	// of the known role names so a typo doesn't silently grant
	// access. The three valid values are duplicated here from
	// internal/auth/users.go because config cannot import auth
	// (auth -> config).
	if s.RequireAuth {
		switch s.MinRole {
		case "", "user", "power-user", "admin":
			// ok
		default:
			return fmt.Errorf("min_role=%q is invalid; expected %q, %q, or %q (empty = any authenticated user)", s.MinRole, "user", "power-user", "admin")
		}
	}

	return nil
}

// isValidHeaderName checks that the given string is a valid RFC 7230
// `token` — the grammar HTTP header names must follow. This is the
// same shape Caddy's Caddyfile parser will accept on a `header_up`
// directive without quoting; rejecting anything outside it means our
// generator can write the name unquoted (`header_up X-Foo "value"`)
// without escape gymnastics.
//
// Token = 1*tchar; tchar = "!" / "#" / "$" / "%" / "&" / "'" / "*" /
// "+" / "-" / "." / "^" / "_" / "`" / "|" / "~" / DIGIT / ALPHA.
func isValidHeaderName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '!' || r == '#' || r == '$' || r == '%' || r == '&' || r == '\'':
		case r == '*' || r == '+' || r == '-' || r == '.' || r == '^' || r == '_':
		case r == '`' || r == '|' || r == '~':
		default:
			return false
		}
	}
	return true
}

// isValidHeaderValue rejects characters that cannot legitimately
// appear in an HTTP header value: control characters (which would
// truncate or smuggle subsequent headers), and the double quote
// (which our generator uses to delimit the value in the Caddyfile).
// Tab is allowed because RFC 7230 permits HTAB in header values.
func isValidHeaderValue(s string) bool {
	for _, r := range s {
		if r == '\t' {
			continue
		}
		if r < 0x20 || r == 0x7f {
			return false
		}
		if r == '"' {
			return false
		}
	}
	return true
}

// isValidGatewayDomain accepts RFC-1123-ish hostnames: labels of
// alphanumerics and hyphens, dot-separated, total length <= 253, no
// leading/trailing hyphens per label, no wildcards (we don't support
// wildcard certs in v1).
func isValidGatewayDomain(d string) bool {
	if d == "" || len(d) > 253 {
		return false
	}
	if strings.Contains(d, "*") {
		return false
	}
	for _, label := range strings.Split(d, ".") {
		if label == "" || len(label) > 63 {
			return false
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}

// isSelfLoop reports whether a backend at backendHost:backendPort would
// resolve to the Muximux listener `listen` (e.g. ":8080",
// "127.0.0.1:8080", "0.0.0.0:8080"). It only returns true for the cases
// we can be *certain* about so cross-host configs that legitimately
// share a port are left alone:
//
//   - wildcard listener (":port", "0.0.0.0:port", "[::]:port") +
//     loopback target ("localhost", "127.0.0.1", "::1");
//   - exact host+port match between listener and backend;
//   - both sides are loopback in any form.
//
// We do not perform DNS lookups -- a transient resolver failure must
// not turn a config into invalid one.
func isSelfLoop(backendHost, backendPort, listen string) bool {
	if listen == "" {
		return false
	}
	listenHost, listenPort, err := net.SplitHostPort(listen)
	if err != nil {
		return false
	}
	if listenPort != backendPort {
		return false
	}
	listenIP := net.ParseIP(listenHost)
	backendIP := net.ParseIP(backendHost)
	listenWildcard := listenHost == "" ||
		(listenIP != nil && listenIP.IsUnspecified())
	listenLoopback := strings.EqualFold(listenHost, "localhost") ||
		(listenIP != nil && listenIP.IsLoopback())
	backendLoopback := strings.EqualFold(backendHost, "localhost") ||
		(backendIP != nil && backendIP.IsLoopback())

	if listenWildcard && backendLoopback {
		return true
	}
	if listenLoopback && backendLoopback {
		return true
	}
	if listenHost != "" && strings.EqualFold(listenHost, backendHost) {
		return true
	}
	return false
}

// CurrentConfigVersion is the config schema version.
const CurrentConfigVersion = 1

// Save writes configuration to a YAML file
func (c *Config) Save(path string) error {
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".config-*.yaml")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0600); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	// fsync the parent directory so the rename hits stable storage
	// before Save returns. Without this, a power loss between rename
	// and the directory's eventual writeback can leave the filesystem
	// seeing the temp file removed but the rename not persisted
	// (findings.md L10). Best-effort: on filesystems that don't
	// support directory fsync the error is returned for visibility.
	if dir != "." {
		if d, err := os.Open(dir); err == nil {
			_ = d.Sync()
			_ = d.Close()
		}
	}
	return nil
}

// NeedsSetup returns true when the application has not been configured yet.
func (c *Config) NeedsSetup() bool {
	if c.Auth.SetupComplete {
		return false
	}
	if len(c.Apps) > 0 {
		return false
	}
	if c.Auth.Method != "" && c.Auth.Method != "none" {
		return false
	}
	if len(c.Auth.Users) > 0 {
		return false
	}
	return true
}

// defaultConfig returns sensible defaults
func defaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Listen:       ":8080",
			Title:        "Muximux",
			Language:     "en",
			LogLevel:     "info",
			LogFormat:    "text",
			ProxyTimeout: "30s",
		},
		Auth: AuthConfig{
			Method: "none",
		},
		Theme: ThemeConfig{
			Family:  "default",
			Variant: "system",
		},
		Navigation: NavigationConfig{
			Position:           "top",
			Width:              "220px",
			AutoHide:           false,
			AutoHideDelay:      "0.5s",
			ShowOnHover:        true,
			ShowLabels:         true,
			ShowLogo:           true,
			ShowHomeButton:     true,
			ShowAppColors:      true,
			ShowIconBackground: false,
			IconScale:          1.0,
			ShowSplashOnStart:  false,
			ShowShadow:         true,
			FloatingPosition:   "bottom-right",
			BarStyle:           "grouped",
		},
		Icons: IconsConfig{
			DashboardIcons: DashboardIconsConfig{
				Enabled:  true,
				Mode:     "on_demand",
				CacheDir: "icons/dashboard",
				CacheTTL: "7d",
			},
		},
		Health: HealthConfig{
			Enabled:  true,
			Interval: "30s",
			Timeout:  "5s",
		},
		Keybindings: KeybindingsConfig{
			Bindings: make(map[string][]KeyCombo),
		},
		Groups: []GroupConfig{},
		Apps:   []AppConfig{},
	}
}
