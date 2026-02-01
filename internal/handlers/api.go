package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/mescon/muximux3/internal/config"
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
		Title      string                  `json:"title"`
		Navigation config.NavigationConfig `json:"navigation"`
		Groups     []config.GroupConfig    `json:"groups"`
		Apps       []ClientAppConfig       `json:"apps"`
	}{
		Title:      h.config.Server.Title,
		Navigation: h.config.Navigation,
		Groups:     h.config.Groups,
		Apps:       sanitizeApps(h.config.Apps),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clientConfig)
}

// ClientConfigUpdate represents the configuration update from the frontend
type ClientConfigUpdate struct {
	Title      string                  `json:"title"`
	Navigation config.NavigationConfig `json:"navigation"`
	Groups     []config.GroupConfig    `json:"groups"`
	Apps       []ClientAppConfig       `json:"apps"`
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
	h.config.Groups = update.Groups

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
			Name:     clientApp.Name,
			URL:      url,
			Icon:     clientApp.Icon,
			Color:    clientApp.Color,
			Group:    clientApp.Group,
			Order:    clientApp.Order,
			Enabled:  clientApp.Enabled,
			Default:  clientApp.Default,
			OpenMode: clientApp.OpenMode,
			Proxy:    clientApp.Proxy,
			Scale:    clientApp.Scale,
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
		Title      string                  `json:"title"`
		Navigation config.NavigationConfig `json:"navigation"`
		Groups     []config.GroupConfig    `json:"groups"`
		Apps       []ClientAppConfig       `json:"apps"`
	}{
		Title:      h.config.Server.Title,
		Navigation: h.config.Navigation,
		Groups:     h.config.Groups,
		Apps:       sanitizeApps(h.config.Apps),
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

// ClientAppConfig is the app config sent to the frontend (no sensitive data)
type ClientAppConfig struct {
	Name     string              `json:"name"`
	URL      string              `json:"url"`
	Icon     config.AppIconConfig `json:"icon"`
	Color    string              `json:"color"`
	Group    string              `json:"group"`
	Order    int                 `json:"order"`
	Enabled  bool                `json:"enabled"`
	Default  bool                `json:"default"`
	OpenMode string              `json:"open_mode"`
	Proxy    bool                `json:"proxy"`
	Scale    float64             `json:"scale"`
}

// sanitizeApps removes sensitive fields from app configs
func sanitizeApps(apps []config.AppConfig) []ClientAppConfig {
	result := make([]ClientAppConfig, 0, len(apps))
	for _, app := range apps {
		if !app.Enabled {
			continue
		}

		// Determine the URL to send to client
		url := app.URL
		if app.Proxy {
			// If proxied, use the proxy path instead of direct URL
			url = "/proxy/" + slugify(app.Name) + "/"
		}

		result = append(result, ClientAppConfig{
			Name:     app.Name,
			URL:      url,
			Icon:     app.Icon,
			Color:    app.Color,
			Group:    app.Group,
			Order:    app.Order,
			Enabled:  app.Enabled,
			Default:  app.Default,
			OpenMode: app.OpenMode,
			Proxy:    app.Proxy,
			Scale:    app.Scale,
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
