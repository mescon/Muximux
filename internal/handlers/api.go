package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/mescon/muximux3/internal/config"
)

// APIHandler handles API requests
type APIHandler struct {
	config *config.Config
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(cfg *config.Config) *APIHandler {
	return &APIHandler{config: cfg}
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
