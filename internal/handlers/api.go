package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/logging"
)

// APIHandler handles API requests
type APIHandler struct {
	config       *config.Config
	configPath   string
	mu           *sync.RWMutex
	onConfigSave func() // called after config is saved to trigger route rebuilds etc.
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(cfg *config.Config, configPath string, mu *sync.RWMutex) *APIHandler {
	return &APIHandler{
		config:     cfg,
		configPath: configPath,
		mu:         mu,
	}
}

// SetOnConfigSave sets a callback invoked after every config save.
func (h *APIHandler) SetOnConfigSave(fn func()) {
	h.onConfigSave = fn
}

func (h *APIHandler) notifyConfigSaved() {
	if h.onConfigSave != nil {
		h.onConfigSave()
	}
}

// Pre-marshaled response for the high-frequency health endpoint
var healthOKResponse = []byte("{\"status\":\"ok\"}\n")

// Health returns server health status
func (h *APIHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(headerContentType, contentTypeJSON)
	w.Write(healthOKResponse)
}

// GetConfig returns the current configuration (sanitized)
func (h *APIHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	userRole := getUserRole(r)
	sendJSON(w, http.StatusOK, buildClientConfigResponse(h.config, userRole))
}

// ExportConfig returns the full configuration as a downloadable YAML file,
// with sensitive auth fields (password hashes, secrets, API keys) stripped.
func (h *APIHandler) ExportConfig(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	cfg := *h.config
	h.mu.RUnlock()

	// Deep-copy slices that will be mutated to avoid writing through the
	// shared backing array into the live config.
	users := make([]config.UserConfig, len(cfg.Auth.Users))
	copy(users, cfg.Auth.Users)
	cfg.Auth.Users = users

	// Strip sensitive auth data
	for i := range cfg.Auth.Users {
		cfg.Auth.Users[i].PasswordHash = ""
	}
	cfg.Auth.APIKeyHash = ""
	cfg.Auth.OIDC.ClientSecret = ""

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "Failed to marshal config", "source", "config", "error", err)
		return
	}

	logging.From(r.Context()).Info("Config exported", "source", "audit")
	filename := fmt.Sprintf("muximux-config-%s.yaml", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Write(data)
}

// ParseImportedConfig accepts a YAML config file via POST, validates it, and
// returns the parsed config as JSON so the frontend can preview before applying.
func (h *APIHandler) ParseImportedConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB limit
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "Failed to read request body")
		return
	}

	// KnownFields(true) rejects any YAML field not declared on the
	// Config struct. Guards against mass-assignment: if a sensitive
	// field is ever added to Config, an older backup that happens to
	// contain a field of the same name cannot seed it without an
	// explicit code change acknowledging the import path
	// (findings.md M7).
	var cfg config.Config
	dec := yaml.NewDecoder(bytes.NewReader(body))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		respondError(w, r, http.StatusBadRequest, fmt.Sprintf("Invalid YAML: %s", err.Error()), "source", "config", "error", err)
		return
	}

	if len(cfg.Apps) == 0 {
		respondError(w, r, http.StatusBadRequest, "Config must contain at least one app")
		return
	}

	if err := validateImportedConfig(&cfg); err != nil {
		respondError(w, r, http.StatusBadRequest, err.Error(), "source", "config")
		return
	}

	// Return as the same sanitized JSON format the frontend expects
	sendJSON(w, http.StatusOK, buildClientConfigResponse(&cfg, ""))
}

// validateImportedConfig rejects backups that would leave the running
// instance in an opaque broken state (findings.md M20). Checks:
//   - every app has a Name and URL, and each URL is parseable http(s)
//   - known open_mode values
//   - min_role is either empty or a known role
//   - auth.method is one of the known values
//   - any duration fields parse
//
// Missing fields that Load() would silently default are left alone; the
// point here is to reject structurally invalid inputs, not to force
// every field to be explicit.
func validateImportedConfig(cfg *config.Config) error {
	knownOpenModes := map[string]bool{"": true, "iframe": true, "tab": true, "window": true, "popup": true}
	knownRoles := map[string]bool{"": true, auth.RoleAdmin: true, auth.RolePowerUser: true, auth.RoleUser: true}
	knownAuthMethods := map[string]bool{"": true, "none": true, "builtin": true, "forward_auth": true, "oidc": true}

	for i := range cfg.Apps {
		app := &cfg.Apps[i]
		if app.Name == "" {
			return fmt.Errorf("each app must have a name")
		}
		if app.URL == "" {
			return fmt.Errorf("app %q must have a URL", app.Name)
		}
		if u, err := parseURL(app.URL); err != nil || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "") {
			return fmt.Errorf("app %q has invalid URL %q", app.Name, app.URL)
		}
		if !knownOpenModes[app.OpenMode] {
			return fmt.Errorf("app %q has unknown open_mode %q", app.Name, app.OpenMode)
		}
		if !knownRoles[app.MinRole] {
			return fmt.Errorf("app %q has unknown min_role %q", app.Name, app.MinRole)
		}
	}

	if !knownAuthMethods[cfg.Auth.Method] {
		return fmt.Errorf("unknown auth.method %q", cfg.Auth.Method)
	}

	if cfg.Auth.SessionMaxAge != "" {
		if _, err := time.ParseDuration(cfg.Auth.SessionMaxAge); err != nil {
			return fmt.Errorf("invalid auth.session_max_age: %w", err)
		}
	}
	if cfg.Server.ProxyTimeout != "" {
		if _, err := time.ParseDuration(cfg.Server.ProxyTimeout); err != nil {
			return fmt.Errorf("invalid server.proxy_timeout: %w", err)
		}
	}

	return nil
}

func parseURL(raw string) (*url.URL, error) {
	return url.Parse(raw)
}

// clientConfigResponse is the sanitized config structure sent to the frontend.
type clientConfigResponse struct {
	Title        string                    `json:"title"`
	Language     string                    `json:"language"`
	LogLevel     string                    `json:"log_level"`
	ProxyTimeout string                    `json:"proxy_timeout,omitempty"`
	Navigation   config.NavigationConfig   `json:"navigation"`
	Theme        config.ThemeConfig        `json:"theme"`
	Health       *config.HealthConfig      `json:"health,omitempty"`
	Keybindings  *config.KeybindingsConfig `json:"keybindings,omitempty"`
	Auth         *clientAuthConfig         `json:"auth,omitempty"`
	Groups       []config.GroupConfig      `json:"groups"`
	Apps         []ClientAppConfig         `json:"apps"`
}

// clientAuthConfig is the sanitized auth config sent to the frontend.
// Excludes sensitive fields (users, api_key, etc).
type clientAuthConfig struct {
	Method         string            `json:"method"`
	TrustedProxies []string          `json:"trusted_proxies,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	LogoutURL      string            `json:"logout_url,omitempty"`
}

// buildClientConfigResponse creates a sanitized config response from the server config.
// userRole filters apps by minimum role; empty string means no filtering (e.g. import preview).
func buildClientConfigResponse(cfg *config.Config, userRole string) clientConfigResponse {
	language := cfg.Server.Language
	if language == "" {
		language = "en"
	}
	resp := clientConfigResponse{
		Title:        cfg.Server.Title,
		Language:     language,
		LogLevel:     cfg.Server.LogLevel,
		ProxyTimeout: cfg.Server.ProxyTimeout,
		Navigation:   cfg.Navigation,
		Theme:        cfg.Theme,
		Health:       &cfg.Health,
		Groups:       cfg.Groups,
		Apps:         sanitizeApps(cfg.Apps, userRole),
	}
	if len(cfg.Keybindings.Bindings) > 0 {
		resp.Keybindings = &cfg.Keybindings
	}
	if cfg.Auth.Method != "" {
		authCfg := &clientAuthConfig{Method: cfg.Auth.Method}
		if len(cfg.Auth.TrustedProxies) > 0 {
			authCfg.TrustedProxies = cfg.Auth.TrustedProxies
		}
		if len(cfg.Auth.Headers) > 0 {
			authCfg.Headers = cfg.Auth.Headers
		}
		authCfg.LogoutURL = cfg.Auth.LogoutURL
		resp.Auth = authCfg
	}
	return resp
}

// ClientConfigUpdate represents the configuration update from the frontend
type ClientConfigUpdate struct {
	Title        string                    `json:"title"`
	Language     string                    `json:"language"`
	LogLevel     string                    `json:"log_level"`
	ProxyTimeout string                    `json:"proxy_timeout"`
	Navigation   config.NavigationConfig   `json:"navigation"`
	Theme        config.ThemeConfig        `json:"theme"`
	Health       *config.HealthConfig      `json:"health,omitempty"`
	Keybindings  *config.KeybindingsConfig `json:"keybindings,omitempty"`
	Groups       []config.GroupConfig      `json:"groups"`
	Apps         []ClientAppConfig         `json:"apps"`
}

// SaveConfig updates and saves the configuration
func (h *APIHandler) SaveConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	var update ClientConfigUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	mergeConfigUpdate(h.config, &update)

	// Save to file
	if err := h.config.Save(h.configPath); err != nil {
		respondError(w, r, http.StatusInternalServerError, errFailedSaveConfig, "source", "config", "error", err)
		return
	}

	logging.From(r.Context()).Info("Configuration saved", "source", "audit")
	h.notifyConfigSaved()

	// Apply log level change at runtime
	if h.config.Server.LogLevel != "" {
		logging.SetLevel(logging.Level(h.config.Server.LogLevel))
		logging.From(r.Context()).Info("Log level changed", "source", "config", "level", h.config.Server.LogLevel)
	}

	sendJSON(w, http.StatusOK, buildClientConfigResponse(h.config, auth.RoleAdmin))
}

// mergeConfigUpdate applies a client config update to the server config,
// preserving sensitive fields (auth bypass, access rules, original proxy URLs).
func mergeConfigUpdate(cfg *config.Config, update *ClientConfigUpdate) {
	cfg.Server.Title = update.Title
	cfg.Server.Language = update.Language
	cfg.Server.LogLevel = update.LogLevel
	if update.ProxyTimeout != "" {
		cfg.Server.ProxyTimeout = update.ProxyTimeout
	}
	cfg.Navigation = update.Navigation
	cfg.Theme = update.Theme
	if update.Health != nil {
		cfg.Health = *update.Health
	}
	cfg.Groups = update.Groups
	if update.Keybindings != nil {
		cfg.Keybindings = *update.Keybindings
	}

	// Build lookup of existing apps by name to preserve sensitive data.
	existingApps := make(map[string]config.AppConfig)
	for i := range cfg.Apps {
		existingApps[cfg.Apps[i].Name] = cfg.Apps[i]
	}

	newApps := make([]config.AppConfig, 0, len(update.Apps))
	for i := range update.Apps {
		app := mergeClientApp(&update.Apps[i], existingApps)
		newApps = append(newApps, app)
	}
	cfg.Apps = newApps
}

// clientAppToConfig converts a client app payload to a full AppConfig.
func clientAppToConfig(c *ClientAppConfig) config.AppConfig {
	return config.AppConfig{
		Name:                c.Name,
		URL:                 c.URL,
		HealthURL:           c.HealthURL,
		Icon:                c.Icon,
		Color:               c.Color,
		Group:               c.Group,
		Order:               c.Order,
		Enabled:             c.Enabled,
		Default:             c.Default,
		OpenMode:            c.OpenMode,
		Proxy:               c.Proxy,
		HealthCheck:         c.HealthCheck,
		ProxySkipTLSVerify:  c.ProxySkipTLSVerify,
		ProxyHeaders:        c.ProxyHeaders,
		Scale:               c.Scale,
		Shortcut:            c.Shortcut,
		MinRole:             c.MinRole,
		ForceIconBackground: c.ForceIconBackground,
		Permissions:         c.Permissions,
		AllowNotifications:  c.AllowNotifications,
	}
}

// mergeClientApp converts a client app config back to a full app config,
// preserving sensitive fields from the existing app if it was previously configured.
func mergeClientApp(clientApp *ClientAppConfig, existingApps map[string]config.AppConfig) config.AppConfig {
	app := clientAppToConfig(clientApp)

	// Preserve auth bypass and access rules if app existed before
	if existing, ok := existingApps[clientApp.Name]; ok {
		app.AuthBypass = existing.AuthBypass
		app.Access = existing.Access
	}

	return app
}

// GetApps returns the list of apps
func (h *APIHandler) GetApps(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	userRole := getUserRole(r)
	sendJSON(w, http.StatusOK, sanitizeApps(h.config.Apps, userRole))
}

// GetGroups returns the list of groups
func (h *APIHandler) GetGroups(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	sendJSON(w, http.StatusOK, h.config.Groups)
}

// GetApp returns a single app by name
func (h *APIHandler) GetApp(w http.ResponseWriter, r *http.Request, name string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for i := range h.config.Apps {
		if h.config.Apps[i].Name == name {
			sendJSON(w, http.StatusOK, sanitizeApp(&h.config.Apps[i]))
			return
		}
	}

	respondError(w, r, http.StatusNotFound, errAppNotFound)
}

// CreateApp creates a new app
func (h *APIHandler) CreateApp(w http.ResponseWriter, r *http.Request) {
	var clientApp ClientAppConfig
	if err := json.NewDecoder(r.Body).Decode(&clientApp); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}

	if clientApp.Name == "" {
		respondError(w, r, http.StatusBadRequest, "App name is required")
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if app already exists
	for i := range h.config.Apps {
		if h.config.Apps[i].Name == clientApp.Name {
			respondError(w, r, http.StatusConflict, "App already exists")
			return
		}
	}

	// Create new app config
	newApp := clientAppToConfig(&clientApp)
	newApp.Order = len(h.config.Apps) // Add at end

	h.config.Apps = append(h.config.Apps, newApp)

	// Save config
	if err := h.config.Save(h.configPath); err != nil {
		respondError(w, r, http.StatusInternalServerError, errFailedSaveConfig, "source", "config", "app", newApp.Name, "error", err)
		return
	}

	logging.From(r.Context()).Info("App created", "source", "audit", "app", newApp.Name)
	h.notifyConfigSaved()
	sendJSON(w, http.StatusCreated, sanitizeApp(&newApp))
}

// UpdateApp updates an existing app
func (h *APIHandler) UpdateApp(w http.ResponseWriter, r *http.Request, name string) {
	var clientApp ClientAppConfig
	if err := json.NewDecoder(r.Body).Decode(&clientApp); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Find the app
	idx := -1
	for i := range h.config.Apps {
		if h.config.Apps[i].Name == name {
			idx = i
			break
		}
	}

	if idx == -1 {
		respondError(w, r, http.StatusNotFound, errAppNotFound)
		return
	}

	// Update app config, preserving sensitive fields
	existing := h.config.Apps[idx]
	updated := clientAppToConfig(&clientApp)
	updated.AuthBypass = existing.AuthBypass
	updated.Access = existing.Access
	h.config.Apps[idx] = updated

	// Save config
	if err := h.config.Save(h.configPath); err != nil {
		respondError(w, r, http.StatusInternalServerError, errFailedSaveConfig, "source", "config", "app", clientApp.Name, "error", err)
		return
	}

	logging.From(r.Context()).Info("App updated", "source", "audit", "app", clientApp.Name)
	h.notifyConfigSaved()
	sendJSON(w, http.StatusOK, sanitizeApp(&h.config.Apps[idx]))
}

// DeleteApp removes an app
func (h *APIHandler) DeleteApp(w http.ResponseWriter, r *http.Request, name string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Find and remove the app
	idx := -1
	for i := range h.config.Apps {
		if h.config.Apps[i].Name == name {
			idx = i
			break
		}
	}

	if idx == -1 {
		respondError(w, r, http.StatusNotFound, errAppNotFound)
		return
	}

	h.config.Apps = append(h.config.Apps[:idx], h.config.Apps[idx+1:]...)

	// Save config
	if err := h.config.Save(h.configPath); err != nil {
		respondError(w, r, http.StatusInternalServerError, errFailedSaveConfig, "source", "config", "app", name, "error", err)
		return
	}

	logging.From(r.Context()).Info("App deleted", "source", "audit", "app", name)
	h.notifyConfigSaved()
	w.WriteHeader(http.StatusNoContent)
}

// GetGroup returns a single group by name
func (h *APIHandler) GetGroup(w http.ResponseWriter, r *http.Request, name string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for i := range h.config.Groups {
		if h.config.Groups[i].Name == name {
			sendJSON(w, http.StatusOK, h.config.Groups[i])
			return
		}
	}

	respondError(w, r, http.StatusNotFound, errGroupNotFound)
}

// CreateGroup creates a new group
func (h *APIHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var group config.GroupConfig
	if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}

	if group.Name == "" {
		respondError(w, r, http.StatusBadRequest, "Group name is required")
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if group already exists
	for i := range h.config.Groups {
		if h.config.Groups[i].Name == group.Name {
			respondError(w, r, http.StatusConflict, "Group already exists")
			return
		}
	}

	group.Order = len(h.config.Groups)
	h.config.Groups = append(h.config.Groups, group)

	// Save config
	if err := h.config.Save(h.configPath); err != nil {
		respondError(w, r, http.StatusInternalServerError, errFailedSaveConfig, "source", "config", "group", group.Name, "error", err)
		return
	}

	logging.From(r.Context()).Info("Group created", "source", "audit", "group", group.Name)
	sendJSON(w, http.StatusCreated, group)
}

// UpdateGroup updates an existing group
func (h *APIHandler) UpdateGroup(w http.ResponseWriter, r *http.Request, name string) {
	var group config.GroupConfig
	if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Find the group
	idx := -1
	for i := range h.config.Groups {
		if h.config.Groups[i].Name == name {
			idx = i
			break
		}
	}

	if idx == -1 {
		respondError(w, r, http.StatusNotFound, errGroupNotFound)
		return
	}

	h.config.Groups[idx] = group

	// Save config
	if err := h.config.Save(h.configPath); err != nil {
		respondError(w, r, http.StatusInternalServerError, errFailedSaveConfig, "source", "config", "group", group.Name, "error", err)
		return
	}

	logging.From(r.Context()).Info("Group updated", "source", "audit", "group", group.Name)
	sendJSON(w, http.StatusOK, group)
}

// DeleteGroup removes a group
func (h *APIHandler) DeleteGroup(w http.ResponseWriter, r *http.Request, name string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Find and remove the group
	idx := -1
	for i := range h.config.Groups {
		if h.config.Groups[i].Name == name {
			idx = i
			break
		}
	}

	if idx == -1 {
		respondError(w, r, http.StatusNotFound, errGroupNotFound)
		return
	}

	h.config.Groups = append(h.config.Groups[:idx], h.config.Groups[idx+1:]...)

	for i := range h.config.Apps {
		if h.config.Apps[i].Group == name {
			h.config.Apps[i].Group = ""
		}
	}

	// Save config
	if err := h.config.Save(h.configPath); err != nil {
		respondError(w, r, http.StatusInternalServerError, errFailedSaveConfig, "source", "config", "group", name, "error", err)
		return
	}

	logging.From(r.Context()).Info("Group deleted", "source", "audit", "group", name)
	w.WriteHeader(http.StatusNoContent)
}

// sanitizeApp converts a single app config to client format. It assumes
// an admin-level caller: use sanitizeAppForRole instead when returning
// config to non-admins so embedded URL credentials
// (https://user:token@host/) and per-app injected headers (Authorization
// / X-Api-Key) don't leak to everyone who can read /api/config.
func sanitizeApp(app *config.AppConfig) ClientAppConfig {
	return sanitizeAppForRole(app, true)
}

// sanitizeAppForRole builds a client-facing app config with sensitive
// fields stripped when isAdmin is false. Non-admins see app.URL with
// any userinfo removed and never receive ProxyHeaders.
func sanitizeAppForRole(app *config.AppConfig, isAdmin bool) ClientAppConfig {
	var proxyURL string
	if app.Proxy {
		proxyURL = proxyPathPrefix + Slugify(app.Name) + "/"
	}
	url := app.URL
	var proxyHeaders map[string]string
	if isAdmin {
		proxyHeaders = app.ProxyHeaders
	} else {
		url = stripURLCredentials(url)
	}
	return ClientAppConfig{
		Name:                app.Name,
		URL:                 url,
		HealthURL:           stripURLCredentialsIf(!isAdmin, app.HealthURL),
		ProxyURL:            proxyURL,
		Icon:                app.Icon,
		Color:               app.Color,
		Group:               app.Group,
		Order:               app.Order,
		Enabled:             app.Enabled,
		Default:             app.Default,
		OpenMode:            app.OpenMode,
		Proxy:               app.Proxy,
		HealthCheck:         app.HealthCheck,
		ProxySkipTLSVerify:  app.ProxySkipTLSVerify,
		ProxyHeaders:        proxyHeaders,
		Scale:               app.Scale,
		Shortcut:            app.Shortcut,
		MinRole:             app.MinRole,
		ForceIconBackground: app.ForceIconBackground,
		Permissions:         app.Permissions,
		AllowNotifications:  app.AllowNotifications,
	}
}

// stripURLCredentials removes any userinfo component (user / user:pass)
// from a URL string so non-admin clients cannot read admin-embedded
// credentials out of the app config (findings.md H12). Returns the
// input unchanged if parsing fails, since an unparseable URL cannot
// leak a structured credential.
func stripURLCredentials(raw string) string {
	if raw == "" {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	u.User = nil
	return u.String()
}

func stripURLCredentialsIf(strip bool, raw string) string {
	if !strip {
		return raw
	}
	return stripURLCredentials(raw)
}

// ClientAppConfig is the app config sent to the frontend (no sensitive data)
type ClientAppConfig struct {
	Name                string               `json:"name"`
	URL                 string               `json:"url"` // Original target URL (for editing/config)
	HealthURL           string               `json:"health_url,omitempty"`
	ProxyURL            string               `json:"proxyUrl,omitempty"` // Proxy path for iframe loading (when proxy enabled)
	Icon                config.AppIconConfig `json:"icon"`
	Color               string               `json:"color"`
	Group               string               `json:"group"`
	Order               int                  `json:"order"`
	Enabled             bool                 `json:"enabled"`
	Default             bool                 `json:"default"`
	OpenMode            string               `json:"open_mode"`
	Proxy               bool                 `json:"proxy"`
	HealthCheck         *bool                `json:"health_check,omitempty"`          // nil/true = enabled, false = disabled
	ProxySkipTLSVerify  *bool                `json:"proxy_skip_tls_verify,omitempty"` // nil = true (default)
	ProxyHeaders        map[string]string    `json:"proxy_headers,omitempty"`
	Scale               float64              `json:"scale"`
	Shortcut            *int                 `json:"shortcut,omitempty"`
	MinRole             string               `json:"min_role,omitempty"`
	ForceIconBackground bool                 `json:"force_icon_background,omitempty"`
	Permissions         []string             `json:"permissions,omitempty"`
	AllowNotifications  bool                 `json:"allow_notifications,omitempty"`
}

// sanitizeApps removes sensitive fields and filters by role.
// userRole is the requesting user's role; empty string disables filtering
// AND is treated as admin-level for compatibility with callers that never
// carried a role (e.g. unauthenticated setup previews).
func sanitizeApps(apps []config.AppConfig, userRole string) []ClientAppConfig {
	isAdmin := userRole == "" || userRole == auth.RoleAdmin
	result := make([]ClientAppConfig, 0, len(apps))
	for i := range apps {
		if !apps[i].Enabled {
			continue
		}
		// Filter by minimum role if a user role is provided
		if userRole != "" && apps[i].MinRole != "" {
			if !auth.HasMinRole(userRole, apps[i].MinRole) {
				continue
			}
		}
		result = append(result, sanitizeAppForRole(&apps[i], isAdmin))
	}
	return result
}

// Slugify converts a name to a URL-safe slug
func Slugify(name string) string {
	// Slugify: lowercase, keep alphanumeric, collapse separators to single dash, trim edges
	result := make([]byte, 0, len(name))
	lastDash := true // start true to suppress leading dash
	for _, c := range name {
		switch {
		case c >= 'A' && c <= 'Z':
			result = append(result, byte(c+32)) // lowercase
			lastDash = false
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			result = append(result, byte(c))
			lastDash = false
		case c == ' ', c == '-', c == '_':
			if !lastDash {
				result = append(result, '-')
				lastDash = true
			}
		}
	}
	// Trim trailing dash
	if len(result) > 0 && result[len(result)-1] == '-' {
		result = result[:len(result)-1]
	}
	return string(result)
}

// getUserRole extracts the user role from the request context.
// Returns empty string if no user is present.
func getUserRole(r *http.Request) string {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		return ""
	}
	return user.Role
}
