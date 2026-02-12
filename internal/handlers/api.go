package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/mescon/muximux/v3/internal/config"
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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// GetConfig returns the current configuration (sanitized)
func (h *APIHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Return config without sensitive fields
	clientConfig := struct {
		Title       string                    `json:"title"`
		Navigation  config.NavigationConfig   `json:"navigation"`
		Theme       config.ThemeConfig        `json:"theme"`
		Keybindings *config.KeybindingsConfig `json:"keybindings,omitempty"`
		Groups      []config.GroupConfig      `json:"groups"`
		Apps        []ClientAppConfig         `json:"apps"`
	}{
		Title:      h.config.Server.Title,
		Navigation: h.config.Navigation,
		Theme:      h.config.Theme,
		Groups:     h.config.Groups,
		Apps:       sanitizeApps(h.config.Apps),
	}
	// Only include keybindings if there are custom bindings
	if len(h.config.Keybindings.Bindings) > 0 {
		clientConfig.Keybindings = &h.config.Keybindings
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clientConfig)
}

// ClientConfigUpdate represents the configuration update from the frontend
type ClientConfigUpdate struct {
	Title       string                    `json:"title"`
	Navigation  config.NavigationConfig   `json:"navigation"`
	Theme       config.ThemeConfig        `json:"theme"`
	Keybindings *config.KeybindingsConfig `json:"keybindings,omitempty"`
	Groups      []config.GroupConfig      `json:"groups"`
	Apps        []ClientAppConfig         `json:"apps"`
}

// SaveConfig updates and saves the configuration
func (h *APIHandler) SaveConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var update ClientConfigUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Update the config
	h.config.Server.Title = update.Title
	h.config.Navigation = update.Navigation
	h.config.Theme = update.Theme
	h.config.Groups = update.Groups
	if update.Keybindings != nil {
		h.config.Keybindings = *update.Keybindings
	}

	// Convert client apps back to full app configs
	// Preserve existing sensitive data (auth_bypass, access) for apps that still exist
	existingApps := make(map[string]config.AppConfig)
	for _, app := range h.config.Apps {
		existingApps[app.Name] = app
	}

	newApps := make([]config.AppConfig, 0, len(update.Apps))
	for _, clientApp := range update.Apps {
		// Get original URL if this was a proxied app
		url := clientApp.URL
		if clientApp.Proxy {
			if existing, ok := existingApps[clientApp.Name]; ok {
				url = existing.URL // Preserve original URL for proxied apps
			}
		}

		app := config.AppConfig{
			Name:                     clientApp.Name,
			URL:                      url,
			Icon:                     clientApp.Icon,
			Color:                    clientApp.Color,
			Group:                    clientApp.Group,
			Order:                    clientApp.Order,
			Enabled:                  clientApp.Enabled,
			Default:                  clientApp.Default,
			OpenMode:                 clientApp.OpenMode,
			Proxy:                    clientApp.Proxy,
			Scale:                    clientApp.Scale,
			DisableKeyboardShortcuts: clientApp.DisableKeyboardShortcuts,
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

		newApps = append(newApps, app)
	}
	h.config.Apps = newApps

	// Save to file
	if err := h.config.Save(h.configPath); err != nil {
		log.Printf("Failed to save config: %v", err)
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	log.Printf("Configuration saved successfully")

	// Return the updated config
	clientConfig := struct {
		Title       string                    `json:"title"`
		Navigation  config.NavigationConfig   `json:"navigation"`
		Theme       config.ThemeConfig        `json:"theme"`
		Keybindings *config.KeybindingsConfig `json:"keybindings,omitempty"`
		Groups      []config.GroupConfig      `json:"groups"`
		Apps        []ClientAppConfig         `json:"apps"`
	}{
		Title:      h.config.Server.Title,
		Navigation: h.config.Navigation,
		Theme:      h.config.Theme,
		Groups:     h.config.Groups,
		Apps:       sanitizeApps(h.config.Apps),
	}
	if len(h.config.Keybindings.Bindings) > 0 {
		clientConfig.Keybindings = &h.config.Keybindings
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clientConfig)
}

// GetApps returns the list of apps
func (h *APIHandler) GetApps(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sanitizeApps(h.config.Apps))
}

// GetGroups returns the list of groups
func (h *APIHandler) GetGroups(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.config.Groups)
}

// GetApp returns a single app by name
func (h *APIHandler) GetApp(w http.ResponseWriter, r *http.Request, name string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, app := range h.config.Apps {
		if app.Name == name {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(sanitizeApp(app))
			return
		}
	}

	http.Error(w, "App not found", http.StatusNotFound)
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
	for _, app := range h.config.Apps {
		if app.Name == clientApp.Name {
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
		Scale:                    clientApp.Scale,
		DisableKeyboardShortcuts: clientApp.DisableKeyboardShortcuts,
	}

	h.config.Apps = append(h.config.Apps, newApp)

	// Save config
	if err := h.config.Save(h.configPath); err != nil {
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sanitizeApp(newApp))
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
	for i, app := range h.config.Apps {
		if app.Name == name {
			idx = i
			break
		}
	}

	if idx == -1 {
		http.Error(w, "App not found", http.StatusNotFound)
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
		Scale:                    clientApp.Scale,
		DisableKeyboardShortcuts: clientApp.DisableKeyboardShortcuts,
		AuthBypass:               existing.AuthBypass,
		Access:                   existing.Access,
	}

	// Save config
	if err := h.config.Save(h.configPath); err != nil {
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sanitizeApp(h.config.Apps[idx]))
}

// DeleteApp removes an app
func (h *APIHandler) DeleteApp(w http.ResponseWriter, r *http.Request, name string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Find and remove the app
	idx := -1
	for i, app := range h.config.Apps {
		if app.Name == name {
			idx = i
			break
		}
	}

	if idx == -1 {
		http.Error(w, "App not found", http.StatusNotFound)
		return
	}

	h.config.Apps = append(h.config.Apps[:idx], h.config.Apps[idx+1:]...)

	// Save config
	if err := h.config.Save(h.configPath); err != nil {
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetGroup returns a single group by name
func (h *APIHandler) GetGroup(w http.ResponseWriter, r *http.Request, name string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, group := range h.config.Groups {
		if group.Name == name {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(group)
			return
		}
	}

	http.Error(w, "Group not found", http.StatusNotFound)
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
	for _, g := range h.config.Groups {
		if g.Name == group.Name {
			http.Error(w, "Group already exists", http.StatusConflict)
			return
		}
	}

	group.Order = len(h.config.Groups)
	h.config.Groups = append(h.config.Groups, group)

	// Save config
	if err := h.config.Save(h.configPath); err != nil {
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
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
	for i, g := range h.config.Groups {
		if g.Name == name {
			idx = i
			break
		}
	}

	if idx == -1 {
		http.Error(w, "Group not found", http.StatusNotFound)
		return
	}

	h.config.Groups[idx] = group

	// Save config
	if err := h.config.Save(h.configPath); err != nil {
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(group)
}

// DeleteGroup removes a group
func (h *APIHandler) DeleteGroup(w http.ResponseWriter, r *http.Request, name string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Find and remove the group
	idx := -1
	for i, g := range h.config.Groups {
		if g.Name == name {
			idx = i
			break
		}
	}

	if idx == -1 {
		http.Error(w, "Group not found", http.StatusNotFound)
		return
	}

	h.config.Groups = append(h.config.Groups[:idx], h.config.Groups[idx+1:]...)

	// Optionally: Move apps in this group to "ungrouped"
	deletedName := name
	for i, app := range h.config.Apps {
		if app.Group == deletedName {
			h.config.Apps[i].Group = ""
		}
	}

	// Save config
	if err := h.config.Save(h.configPath); err != nil {
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// sanitizeApp converts a single app config to client format
func sanitizeApp(app config.AppConfig) ClientAppConfig {
	var proxyURL string
	if app.Proxy {
		proxyURL = "/proxy/" + slugify(app.Name) + "/"
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
		Scale:                    app.Scale,
		DisableKeyboardShortcuts: app.DisableKeyboardShortcuts,
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
	Scale                    float64              `json:"scale"`
	DisableKeyboardShortcuts bool                 `json:"disable_keyboard_shortcuts"`
}

// sanitizeApps removes sensitive fields from app configs
func sanitizeApps(apps []config.AppConfig) []ClientAppConfig {
	result := make([]ClientAppConfig, 0, len(apps))
	for _, app := range apps {
		if !app.Enabled {
			continue
		}

		// URL is always the original target URL
		// ProxyURL is set when proxy is enabled (for iframe loading)
		var proxyURL string
		if app.Proxy {
			proxyURL = "/proxy/" + slugify(app.Name) + "/"
		}

		result = append(result, ClientAppConfig{
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
			Scale:                    app.Scale,
			DisableKeyboardShortcuts: app.DisableKeyboardShortcuts,
		})
	}
	return result
}

// slugify converts a name to a URL-safe slug
func slugify(name string) string {
	// Simple slugify - replace spaces with dashes, lowercase
	result := make([]byte, 0, len(name))
	for _, c := range name {
		if c >= 'A' && c <= 'Z' {
			result = append(result, byte(c+32)) // lowercase
		} else if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			result = append(result, byte(c))
		} else if c == ' ' || c == '-' || c == '_' {
			result = append(result, '-')
		}
	}
	return string(result)
}
