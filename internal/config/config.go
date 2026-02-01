package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Proxy      ProxyConfig      `yaml:"proxy"`
	Auth       AuthConfig       `yaml:"auth"`
	Navigation NavigationConfig `yaml:"navigation"`
	Icons      IconsConfig      `yaml:"icons"`
	Groups     []GroupConfig    `yaml:"groups"`
	Apps       []AppConfig      `yaml:"apps"`
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Listen string `yaml:"listen"`
	Title  string `yaml:"title"`
}

// ProxyConfig holds embedded Caddy proxy settings
type ProxyConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Listen    string `yaml:"listen"`
	AutoHTTPS bool   `yaml:"auto_https"`
	ACMEEmail string `yaml:"acme_email"`
	TLSCert   string `yaml:"tls_cert"`
	TLSKey    string `yaml:"tls_key"`
}

// AuthConfig holds authentication settings
type AuthConfig struct {
	Method         string            `yaml:"method"` // none, builtin, forward_auth, oidc
	Users          []UserConfig      `yaml:"users"`
	TrustedProxies []string          `yaml:"trusted_proxies"`
	Headers        map[string]string `yaml:"headers"`
	OIDC           OIDCConfig        `yaml:"oidc"`
}

// UserConfig holds local user credentials
type UserConfig struct {
	Username     string `yaml:"username"`
	PasswordHash string `yaml:"password_hash"`
	Role         string `yaml:"role"`
}

// OIDCConfig holds OIDC provider settings
type OIDCConfig struct {
	Issuer       string   `yaml:"issuer"`
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	RedirectURL  string   `yaml:"redirect_url"`
	Scopes       []string `yaml:"scopes"`
}

// NavigationConfig holds navigation layout settings
type NavigationConfig struct {
	Position      string `yaml:"position"` // top, left, right, bottom, floating
	Width         string `yaml:"width"`
	AutoHide      bool   `yaml:"auto_hide"`
	AutoHideDelay string `yaml:"auto_hide_delay"`
	ShowOnHover   bool   `yaml:"show_on_hover"`
	ShowLabels    bool   `yaml:"show_labels"`
	ShowLogo      bool   `yaml:"show_logo"`
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
	Name     string `yaml:"name"`
	Icon     string `yaml:"icon"`
	Color    string `yaml:"color"`
	Order    int    `yaml:"order"`
	Expanded bool   `yaml:"expanded"`
}

// AppConfig holds individual app settings
type AppConfig struct {
	Name       string           `yaml:"name"`
	URL        string           `yaml:"url"`
	Icon       AppIconConfig    `yaml:"icon"`
	Color      string           `yaml:"color"`
	Group      string           `yaml:"group"`
	Order      int              `yaml:"order"`
	Enabled    bool             `yaml:"enabled"`
	Default    bool             `yaml:"default"`
	OpenMode   string           `yaml:"open_mode"` // iframe, new_tab, new_window, redirect
	Proxy      bool             `yaml:"proxy"`
	Scale      float64          `yaml:"scale"`
	AuthBypass []AuthBypassRule `yaml:"auth_bypass"`
	Access     AppAccessConfig  `yaml:"access"`
}

// AppIconConfig holds app icon settings
type AppIconConfig struct {
	Type    string `yaml:"type"` // dashboard, builtin, custom, url
	Name    string `yaml:"name"`
	File    string `yaml:"file"`
	URL     string `yaml:"url"`
	Variant string `yaml:"variant"`
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

// Load reads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return defaultConfig(), nil
		}
		return nil, err
	}

	cfg := defaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save writes configuration to a YAML file
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// defaultConfig returns sensible defaults
func defaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Listen: ":8080",
			Title:  "Muximux",
		},
		Proxy: ProxyConfig{
			Enabled: false,
			Listen:  ":8443",
		},
		Auth: AuthConfig{
			Method: "none",
		},
		Navigation: NavigationConfig{
			Position:      "top",
			Width:         "220px",
			AutoHide:      false,
			AutoHideDelay: "3s",
			ShowOnHover:   true,
			ShowLabels:    true,
			ShowLogo:      true,
		},
		Icons: IconsConfig{
			DashboardIcons: DashboardIconsConfig{
				Enabled:  true,
				Mode:     "on_demand",
				CacheDir: "data/icons/dashboard",
				CacheTTL: "7d",
			},
		},
		Groups: []GroupConfig{},
		Apps:   []AppConfig{},
	}
}
