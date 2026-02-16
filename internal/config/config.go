package config

import (
	"fmt"
	"os"
	"path/filepath"
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
	Server      ServerConfig      `yaml:"server"`
	Auth        AuthConfig        `yaml:"auth"`
	Navigation  NavigationConfig  `yaml:"navigation"`
	Theme       ThemeConfig       `yaml:"theme" json:"theme"`
	Icons       IconsConfig       `yaml:"icons"`
	Health      HealthConfig      `yaml:"health"`
	Keybindings KeybindingsConfig `yaml:"keybindings" json:"keybindings"`
	Groups      []GroupConfig     `yaml:"groups"`
	Apps        []AppConfig       `yaml:"apps"`
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

// TLSConfig holds TLS/HTTPS settings
type TLSConfig struct {
	Domain string `yaml:"domain" json:"domain"`
	Email  string `yaml:"email" json:"email"`
	Cert   string `yaml:"cert" json:"cert"`
	Key    string `yaml:"key" json:"key"`
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Listen       string    `yaml:"listen" json:"listen"`
	BasePath     string    `yaml:"base_path" json:"base_path"` // e.g. "/muximux" — for serving behind a reverse proxy subpath
	Title        string    `yaml:"title" json:"title"`
	LogLevel     string    `yaml:"log_level" json:"log_level"`
	ProxyTimeout string    `yaml:"proxy_timeout" json:"proxy_timeout"` // e.g. "30s", "1m" — timeout for proxied requests
	TLS          TLSConfig `yaml:"tls" json:"tls"`
	Gateway      string    `yaml:"gateway" json:"gateway"`
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

// NeedsCaddy returns true if TLS or Gateway is configured, meaning Caddy
// should start to handle the user-facing port.
func (c *ServerConfig) NeedsCaddy() bool {
	return c.TLS.Domain != "" || c.TLS.Cert != "" || c.Gateway != ""
}

// AuthConfig holds authentication settings
type AuthConfig struct {
	Method         string            `yaml:"method"` // none, builtin, forward_auth, oidc
	Users          []UserConfig      `yaml:"users"`
	TrustedProxies []string          `yaml:"trusted_proxies"`
	Headers        map[string]string `yaml:"headers"`
	OIDC           OIDCConfig        `yaml:"oidc"`
	SessionMaxAge  string            `yaml:"session_max_age"` // e.g., "24h", "7d"
	SecureCookies  bool              `yaml:"secure_cookies"`
	APIKey         string            `yaml:"api_key"`
	SetupComplete  bool              `yaml:"setup_complete"`
}

// UserConfig holds local user credentials
type UserConfig struct {
	Username     string `yaml:"username"`
	PasswordHash string `yaml:"password_hash"`
	Role         string `yaml:"role"`
	Email        string `yaml:"email,omitempty"`
	DisplayName  string `yaml:"display_name,omitempty"`
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
	Position           string  `yaml:"position" json:"position"` // top, left, right, bottom, floating
	Width              string  `yaml:"width" json:"width"`
	AutoHide           bool    `yaml:"auto_hide" json:"auto_hide"`
	AutoHideDelay      string  `yaml:"auto_hide_delay" json:"auto_hide_delay"`
	ShowOnHover        bool    `yaml:"show_on_hover" json:"show_on_hover"`
	ShowLabels         bool    `yaml:"show_labels" json:"show_labels"`
	ShowLogo           bool    `yaml:"show_logo" json:"show_logo"`
	ShowAppColors      bool    `yaml:"show_app_colors" json:"show_app_colors"`
	ShowIconBackground bool    `yaml:"show_icon_background" json:"show_icon_background"`
	IconScale          float64 `yaml:"icon_scale" json:"icon_scale"`
	ShowSplashOnStart  bool    `yaml:"show_splash_on_startup" json:"show_splash_on_startup"`
	ShowShadow         bool    `yaml:"show_shadow" json:"show_shadow"`
	FloatingPosition   string  `yaml:"floating_position" json:"floating_position"` // bottom-right, bottom-left, top-right, top-left
	BarStyle           string  `yaml:"bar_style" json:"bar_style"`                 // grouped, flat (top/bottom bars only)
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
	Name                     string            `yaml:"name"`
	URL                      string            `yaml:"url"`
	HealthURL                string            `yaml:"health_url,omitempty"` // Optional custom health check URL
	Icon                     AppIconConfig     `yaml:"icon"`
	Color                    string            `yaml:"color"`
	Group                    string            `yaml:"group"`
	Order                    int               `yaml:"order"`
	Enabled                  bool              `yaml:"enabled"`
	Default                  bool              `yaml:"default"`
	OpenMode                 string            `yaml:"open_mode"`                                            // iframe, new_tab, new_window, redirect
	HealthCheck              *bool             `yaml:"health_check,omitempty" json:"health_check,omitempty"` // nil/true = enabled, false = disabled
	Proxy                    bool              `yaml:"proxy"`
	ProxySkipTLSVerify       *bool             `yaml:"proxy_skip_tls_verify,omitempty"` // nil = true (default: skip)
	ProxyHeaders             map[string]string `yaml:"proxy_headers,omitempty"`         // custom headers sent to backend
	Scale                    float64           `yaml:"scale"`
	Shortcut                 *int              `yaml:"shortcut,omitempty" json:"shortcut,omitempty"` // 1-9 keyboard shortcut slot
	DisableKeyboardShortcuts bool              `yaml:"disable_keyboard_shortcuts,omitempty"`
	AuthBypass               []AuthBypassRule  `yaml:"auth_bypass"`
	Access                   AppAccessConfig   `yaml:"access"`
}

// AppIconConfig holds app icon settings
type AppIconConfig struct {
	Type       string `yaml:"type" json:"type"` // dashboard, lucide, custom, url
	Name       string `yaml:"name" json:"name"`
	File       string `yaml:"file" json:"file"`
	URL        string `yaml:"url" json:"url"`
	Variant    string `yaml:"variant" json:"variant"`
	Color      string `yaml:"color,omitempty" json:"color"`
	Background string `yaml:"background,omitempty" json:"background"`
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
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return defaultConfig(), nil
		}
		return nil, err
	}

	// Expand environment variables in config values
	expanded := os.ExpandEnv(string(data))

	cfg := defaultConfig()
	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, err
	}

	// Normalize zero-value fields that have non-zero defaults
	if cfg.Navigation.IconScale <= 0 {
		cfg.Navigation.IconScale = 1.0
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validate checks the configuration for contradictory or incomplete settings.
func (c *Config) validate() error {
	tls := c.Server.TLS

	if tls.Domain != "" && tls.Email == "" {
		return fmt.Errorf("tls.email is required when tls.domain is set")
	}
	if (tls.Cert != "") != (tls.Key != "") {
		return fmt.Errorf("tls.cert and tls.key must both be set, or both empty")
	}
	if tls.Domain != "" && tls.Cert != "" {
		return fmt.Errorf("use tls.domain or tls.cert/tls.key, not both")
	}
	if c.Server.Gateway != "" {
		if _, err := os.Stat(c.Server.Gateway); err != nil {
			return fmt.Errorf("gateway file not found: %s", c.Server.Gateway)
		}
	}

	return nil
}

// Save writes configuration to a YAML file
func (c *Config) Save(path string) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
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
			LogLevel:     "info",
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
