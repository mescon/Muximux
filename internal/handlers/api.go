package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/logging"
)

// APIHandler handles API requests
type APIHandler struct {
	config     *config.Config
	configPath string
	mu         sync.RWMutex
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(cfg *config.Config, configPath string) *APIHandler {
	return &APIHandler{
		config:     cfg,
		configPath: configPath,
	}
}

// GetConfigRef returns a reference to the current config (thread-safe)
func (h *APIHandler) GetConfigRef() *config.Config {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.config
}

// Health returns server health status
func (h *APIHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// GetConfig returns the current configuration (sanitized)
func (h *APIHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	userRole := getUserRole(r)
	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(buildClientConfigResponse(h.config, userRole))
}

// ExportConfig returns the full configuration as a downloadable YAML file,
// with sensitive auth fields (password hashes, secrets, API keys) stripped.
func (h *APIHandler) ExportConfig(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	cfg := *h.config
	h.mu.RUnlock()

	// Strip sensitive auth data
	for i := range cfg.Auth.Users {
		cfg.Auth.Users[i].PasswordHash = ""
	}
	cfg.Auth.APIKey = ""
	cfg.Auth.OIDC.ClientSecret = ""

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		http.Error(w, "Failed to marshal config", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("muximux-config-%s.yaml", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Write(data)
}

// ParseImportedConfig accepts a YAML config file via POST, validates it, and
// returns the parsed config as JSON so the frontend can preview before applying.
func (h *APIHandler) ParseImportedConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB limit
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var cfg config.Config
	if err := yaml.Unmarshal(body, &cfg); err != nil {
		http.Error(w, fmt.Sprintf("Invalid YAML: %s", err.Error()), http.StatusBadRequest)
		return
	}

	if len(cfg.Apps) == 0 {
		http.Error(w, "Config must contain at least one app", http.StatusBadRequest)
		return
	}

	for i := range cfg.Apps {
		if cfg.Apps[i].Name == "" {
			http.Error(w, "Each app must have a name", http.StatusBadRequest)
			return
		}
		if cfg.Apps[i].URL == "" {
			http.Error(w, fmt.Sprintf("App %q must have a URL", cfg.Apps[i].Name), http.StatusBadRequest)
			return
		}
	}

	// Return as the same sanitized JSON format the frontend expects
	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(buildClientConfigResponse(&cfg, ""))
}

// clientConfigResponse is the sanitized config structure sent to the frontend.
type clientConfigResponse struct {
	Title        string                    `json:"title"`
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
}

// buildClientConfigResponse creates a sanitized config response from the server config.
// userRole filters apps by minimum role; empty string means no filtering (e.g. import preview).
func buildClientConfigResponse(cfg *config.Config, userRole string) clientConfigResponse {
	resp := clientConfigResponse{
		Title:        cfg.Server.Title,
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
		resp.Auth = authCfg
	}
	return resp
}

// ClientConfigUpdate represents the configuration update from the frontend
type ClientConfigUpdate struct {
	Title        string                    `json:"title"`
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
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	var update ClientConfigUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	mergeConfigUpdate(h.config, &update)

	// Save to file
	if err := h.config.Save(h.configPath); err != nil {
		logging.Error("Failed to save config", "source", "config", "error", err)
		http.Error(w, errFailedSaveConfig, http.StatusInternalServerError)
		return
	}

	logging.Info("Configuration saved successfully", "source", "config")

	// Apply log level change at runtime
	if h.config.Server.LogLevel != "" {
		logging.SetLevel(logging.Level(h.config.Server.LogLevel))
		logging.Info("Log level changed", "source", "config", "level", h.config.Server.LogLevel)
	}

	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(buildClientConfigResponse(h.config, auth.RoleAdmin))
}

// mergeConfigUpdate applies a client config update to the server config,
// preserving sensitive fields (auth bypass, access rules, original proxy URLs).
func mergeConfigUpdate(cfg *config.Config, update *ClientConfigUpdate) {
	cfg.Server.Title = update.Title
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

	// Build lookup of existing apps to preserve sensitive data
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

// mergeClientApp converts a client app config back to a full app config,
// preserving sensitive fields from the existing app if it was previously configured.
func mergeClientApp(clientApp *ClientAppConfig, existingApps map[string]config.AppConfig) config.AppConfig {
	// Get original URL if this was a proxied app
	appURL := clientApp.URL
	if clientApp.Proxy {
		if existing, ok := existingApps[clientApp.Name]; ok {
			appURL = existing.URL // Preserve original URL for proxied apps
		}
	}

	app := config.AppConfig{
		Name:                     clientApp.Name,
		URL:                      appURL,
		Icon:                     clientApp.Icon,
		Color:                    clientApp.Color,
		Group:                    clientApp.Group,
		Order:                    clientApp.Order,
		Enabled:                  clientApp.Enabled,
		Default:                  clientApp.Default,
		OpenMode:                 clientApp.OpenMode,
		Proxy:                    clientApp.Proxy,
		HealthCheck:              clientApp.HealthCheck,
		ProxySkipTLSVerify:       clientApp.ProxySkipTLSVerify,
		ProxyHeaders:             clientApp.ProxyHeaders,
		Scale:                    clientApp.Scale,
		Shortcut:                 clientApp.Shortcut,
		DisableKeyboardShortcuts: clientApp.DisableKeyboardShortcuts,
		MinRole:                  clientApp.MinRole,
		ForceIconBackground:      clientApp.ForceIconBackground,
	}

	// Preserve auth bypass and access rules if app existed before
	if existing, ok := existingApps[clientApp.Name]; ok {
		app.AuthBypass = existing.AuthBypass
		app.Access = existing.Access
		// If URL wasn't proxied, use the new one
		if !clientApp.Proxy {
			app.URL = clientApp.URL
		}
	}

	return app
}

// GetApps returns the list of apps
func (h *APIHandler) GetApps(w http.ResponseWriter, r *http.Request) {
	userRole := getUserRole(r)
	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(sanitizeApps(h.config.Apps, userRole))
}

// GetGroups returns the list of groups
func (h *APIHandler) GetGroups(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(h.config.Groups)
}

// GetApp returns a single app by name
func (h *APIHandler) GetApp(w http.ResponseWriter, r *http.Request, name string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for i := range h.config.Apps {
		if h.config.Apps[i].Name == name {
			w.Header().Set(headerContentType, contentTypeJSON)
			json.NewEncoder(w).Encode(sanitizeApp(&h.config.Apps[i]))
			return
		}
	}

	http.Error(w, errAppNotFound, http.StatusNotFound)
}

// CreateApp creates a new app
func (h *APIHandler) CreateApp(w http.ResponseWriter, r *http.Request) {
	var clientApp ClientAppConfig
	if err := json.NewDecoder(r.Body).Decode(&clientApp); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if clientApp.Name == "" {
		http.Error(w, "App name is required", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if app already exists
	for i := range h.config.Apps {
		if h.config.Apps[i].Name == clientApp.Name {
			http.Error(w, "App already exists", http.StatusConflict)
			return
		}
	}

	// Create new app config
	newApp := config.AppConfig{
		Name:                     clientApp.Name,
		URL:                      clientApp.URL,
		Icon:                     clientApp.Icon,
		Color:                    clientApp.Color,
		Group:                    clientApp.Group,
		Order:                    len(h.config.Apps), // Add at end
		Enabled:                  clientApp.Enabled,
		Default:                  clientApp.Default,
		OpenMode:                 clientApp.OpenMode,
		Proxy:                    clientApp.Proxy,
		HealthCheck:              clientApp.HealthCheck,
		ProxySkipTLSVerify:       clientApp.ProxySkipTLSVerify,
		ProxyHeaders:             clientApp.ProxyHeaders,
		Scale:                    clientApp.Scale,
		Shortcut:                 clientApp.Shortcut,
		DisableKeyboardShortcuts: clientApp.DisableKeyboardShortcuts,
		MinRole:                  clientApp.MinRole,
		ForceIconBackground:      clientApp.ForceIconBackground,
	}

	h.config.Apps = append(h.config.Apps, newApp)

	// Save config
	if err := h.config.Save(h.configPath); err != nil {
		http.Error(w, errFailedSaveConfig, http.StatusInternalServerError)
		return
	}

	logging.Info("App created", "source", "config", "app", newApp.Name)
	w.Header().Set(headerContentType, contentTypeJSON)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sanitizeApp(&newApp))
}

// UpdateApp updates an existing app
func (h *APIHandler) UpdateApp(w http.ResponseWriter, r *http.Request, name string) {
	var clientApp ClientAppConfig
	if err := json.NewDecoder(r.Body).Decode(&clientApp); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
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
		http.Error(w, errAppNotFound, http.StatusNotFound)
		return
	}

	// Preserve sensitive fields
	existing := h.config.Apps[idx]

	// Update app config
	h.config.Apps[idx] = config.AppConfig{
		Name:                     clientApp.Name,
		URL:                      clientApp.URL,
		Icon:                     clientApp.Icon,
		Color:                    clientApp.Color,
		Group:                    clientApp.Group,
		Order:                    clientApp.Order,
		Enabled:                  clientApp.Enabled,
		Default:                  clientApp.Default,
		OpenMode:                 clientApp.OpenMode,
		Proxy:                    clientApp.Proxy,
		HealthCheck:              clientApp.HealthCheck,
		ProxySkipTLSVerify:       clientApp.ProxySkipTLSVerify,
		ProxyHeaders:             clientApp.ProxyHeaders,
		Scale:                    clientApp.Scale,
		Shortcut:                 clientApp.Shortcut,
		DisableKeyboardShortcuts: clientApp.DisableKeyboardShortcuts,
		AuthBypass:               existing.AuthBypass,
		Access:                   existing.Access,
	}

	// Save config
	if err := h.config.Save(h.configPath); err != nil {
		http.Error(w, errFailedSaveConfig, http.StatusInternalServerError)
		return
	}

	logging.Info("App updated", "source", "config", "app", clientApp.Name)
	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(sanitizeApp(&h.config.Apps[idx]))
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
		http.Error(w, errAppNotFound, http.StatusNotFound)
		return
	}

	h.config.Apps = append(h.config.Apps[:idx], h.config.Apps[idx+1:]...)

	// Save config
	if err := h.config.Save(h.configPath); err != nil {
		http.Error(w, errFailedSaveConfig, http.StatusInternalServerError)
		return
	}

	logging.Info("App deleted", "source", "config", "app", name)
	w.WriteHeader(http.StatusNoContent)
}

// GetGroup returns a single group by name
func (h *APIHandler) GetGroup(w http.ResponseWriter, r *http.Request, name string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for i := range h.config.Groups {
		if h.config.Groups[i].Name == name {
			w.Header().Set(headerContentType, contentTypeJSON)
			json.NewEncoder(w).Encode(h.config.Groups[i])
			return
		}
	}

	http.Error(w, errGroupNotFound, http.StatusNotFound)
}

// CreateGroup creates a new group
func (h *APIHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var group config.GroupConfig
	if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if group.Name == "" {
		http.Error(w, "Group name is required", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if group already exists
	for i := range h.config.Groups {
		if h.config.Groups[i].Name == group.Name {
			http.Error(w, "Group already exists", http.StatusConflict)
			return
		}
	}

	group.Order = len(h.config.Groups)
	h.config.Groups = append(h.config.Groups, group)

	// Save config
	if err := h.config.Save(h.configPath); err != nil {
		http.Error(w, errFailedSaveConfig, http.StatusInternalServerError)
		return
	}

	logging.Info("Group created", "source", "config", "group", group.Name)
	w.Header().Set(headerContentType, contentTypeJSON)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(group)
}

// UpdateGroup updates an existing group
func (h *APIHandler) UpdateGroup(w http.ResponseWriter, r *http.Request, name string) {
	var group config.GroupConfig
	if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
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
		http.Error(w, errGroupNotFound, http.StatusNotFound)
		return
	}

	h.config.Groups[idx] = group

	// Save config
	if err := h.config.Save(h.configPath); err != nil {
		http.Error(w, errFailedSaveConfig, http.StatusInternalServerError)
		return
	}

	logging.Info("Group updated", "source", "config", "group", group.Name)
	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(group)
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
		http.Error(w, errGroupNotFound, http.StatusNotFound)
		return
	}

	h.config.Groups = append(h.config.Groups[:idx], h.config.Groups[idx+1:]...)

	// Optionally: Move apps in this group to "ungrouped"
	deletedName := name
	for i := range h.config.Apps {
		if h.config.Apps[i].Group == deletedName {
			h.config.Apps[i].Group = ""
		}
	}

	// Save config
	if err := h.config.Save(h.configPath); err != nil {
		http.Error(w, errFailedSaveConfig, http.StatusInternalServerError)
		return
	}

	logging.Info("Group deleted", "source", "config", "group", name)
	w.WriteHeader(http.StatusNoContent)
}

// sanitizeApp converts a single app config to client format
func sanitizeApp(app *config.AppConfig) ClientAppConfig {
	var proxyURL string
	if app.Proxy {
		proxyURL = proxyPathPrefix + slugify(app.Name) + "/"
	}
	return ClientAppConfig{
		Name:                     app.Name,
		URL:                      app.URL,
		ProxyURL:                 proxyURL,
		Icon:                     app.Icon,
		Color:                    app.Color,
		Group:                    app.Group,
		Order:                    app.Order,
		Enabled:                  app.Enabled,
		Default:                  app.Default,
		OpenMode:                 app.OpenMode,
		Proxy:                    app.Proxy,
		HealthCheck:              app.HealthCheck,
		ProxySkipTLSVerify:       app.ProxySkipTLSVerify,
		ProxyHeaders:             app.ProxyHeaders,
		Scale:                    app.Scale,
		Shortcut:                 app.Shortcut,
		DisableKeyboardShortcuts: app.DisableKeyboardShortcuts,
		MinRole:                  app.MinRole,
		ForceIconBackground:      app.ForceIconBackground,
	}
}

// ClientAppConfig is the app config sent to the frontend (no sensitive data)
type ClientAppConfig struct {
	Name                     string               `json:"name"`
	URL                      string               `json:"url"`                // Original target URL (for editing/config)
	ProxyURL                 string               `json:"proxyUrl,omitempty"` // Proxy path for iframe loading (when proxy enabled)
	Icon                     config.AppIconConfig `json:"icon"`
	Color                    string               `json:"color"`
	Group                    string               `json:"group"`
	Order                    int                  `json:"order"`
	Enabled                  bool                 `json:"enabled"`
	Default                  bool                 `json:"default"`
	OpenMode                 string               `json:"open_mode"`
	Proxy                    bool                 `json:"proxy"`
	HealthCheck              *bool                `json:"health_check,omitempty"`          // nil/true = enabled, false = disabled
	ProxySkipTLSVerify       *bool                `json:"proxy_skip_tls_verify,omitempty"` // nil = true (default)
	ProxyHeaders             map[string]string    `json:"proxy_headers,omitempty"`
	Scale                    float64              `json:"scale"`
	Shortcut                 *int                 `json:"shortcut,omitempty"`
	DisableKeyboardShortcuts bool                 `json:"disable_keyboard_shortcuts"`
	MinRole                  string               `json:"min_role,omitempty"`
	ForceIconBackground      bool                 `json:"force_icon_background,omitempty"`
}

// sanitizeApps removes sensitive fields and filters by role.
// userRole is the requesting user's role; empty string disables filtering.
func sanitizeApps(apps []config.AppConfig, userRole string) []ClientAppConfig {
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
		result = append(result, sanitizeApp(&apps[i]))
	}
	return result
}

// slugify converts a name to a URL-safe slug
func slugify(name string) string {
	// Simple slugify - replace spaces with dashes, lowercase
	result := make([]byte, 0, len(name))
	for _, c := range name {
		switch {
		case c >= 'A' && c <= 'Z':
			result = append(result, byte(c+32)) // lowercase
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			result = append(result, byte(c))
		case c == ' ', c == '-', c == '_':
			result = append(result, '-')
		}
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
