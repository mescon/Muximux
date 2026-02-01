package handlers

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/mescon/muximux3/internal/icons"
)

// IconHandler handles icon-related requests
type IconHandler struct {
	dashboardClient *icons.DashboardIconsClient
	customIconsDir  string
}

// NewIconHandler creates a new icon handler
func NewIconHandler(dashboardClient *icons.DashboardIconsClient, customIconsDir string) *IconHandler {
	return &IconHandler{
		dashboardClient: dashboardClient,
		customIconsDir:  customIconsDir,
	}
}

// GetDashboardIcon serves a dashboard icon
func (h *IconHandler) GetDashboardIcon(w http.ResponseWriter, r *http.Request) {
	// Extract icon name from path: /api/icons/dashboard/{name}
	path := strings.TrimPrefix(r.URL.Path, "/api/icons/dashboard/")
	if path == "" {
		http.Error(w, "Icon name required", http.StatusBadRequest)
		return
	}

	// Parse name and variant
	name := path
	variant := r.URL.Query().Get("variant")
	if variant == "" {
		variant = "svg"
	}

	// Get the icon
	data, contentType, err := h.dashboardClient.GetIcon(name, variant)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400") // Cache for 24 hours
	w.Write(data)
}

// ListDashboardIcons returns a list of available dashboard icons
func (h *IconHandler) ListDashboardIcons(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	var iconList []icons.IconInfo
	var err error

	if query != "" {
		iconList, err = h.dashboardClient.SearchIcons(query)
	} else {
		iconList, err = h.dashboardClient.ListIcons()
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(iconList)
}

// ServeIcon serves an icon based on type (dashboard, custom, or builtin)
func (h *IconHandler) ServeIcon(w http.ResponseWriter, r *http.Request) {
	// Path format: /icons/{type}/{name}
	path := strings.TrimPrefix(r.URL.Path, "/icons/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		http.Error(w, "Invalid icon path", http.StatusBadRequest)
		return
	}

	iconType := parts[0]
	iconName := parts[1]

	switch iconType {
	case "dashboard":
		variant := r.URL.Query().Get("variant")
		if variant == "" {
			// Try to determine from extension
			ext := filepath.Ext(iconName)
			if ext != "" {
				variant = strings.TrimPrefix(ext, ".")
				iconName = strings.TrimSuffix(iconName, ext)
			} else {
				variant = "svg"
			}
		}

		data, contentType, err := h.dashboardClient.GetIcon(iconName, variant)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Write(data)

	case "custom":
		// Serve from custom icons directory
		// TODO: Implement custom icon serving
		http.Error(w, "Custom icons not yet implemented", http.StatusNotImplemented)

	case "builtin":
		// Serve from embedded assets
		// TODO: Implement builtin icon serving
		http.Error(w, "Builtin icons not yet implemented", http.StatusNotImplemented)

	default:
		http.Error(w, "Unknown icon type", http.StatusBadRequest)
	}
}
