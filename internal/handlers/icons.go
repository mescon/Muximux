package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/mescon/muximux3/internal/icons"
)

// IconHandler handles icon-related requests
type IconHandler struct {
	dashboardClient *icons.DashboardIconsClient
	customManager   *icons.CustomIconsManager
}

// NewIconHandler creates a new icon handler
func NewIconHandler(dashboardClient *icons.DashboardIconsClient, customIconsDir string) *IconHandler {
	return &IconHandler{
		dashboardClient: dashboardClient,
		customManager:   icons.NewCustomIconsManager(customIconsDir),
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

// ListBuiltinIcons returns a list of available builtin icons
func (h *IconHandler) ListBuiltinIcons(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	var iconList []icons.BuiltinIconInfo
	var err error

	if query != "" {
		iconList, err = icons.SearchBuiltinIcons(query)
	} else {
		iconList, err = icons.ListBuiltinIcons()
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
		data, contentType, err := h.customManager.GetIcon(iconName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Write(data)

	case "builtin":
		// Serve from embedded assets
		data, contentType, err := icons.GetBuiltinIcon(iconName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Write(data)

	default:
		http.Error(w, "Unknown icon type", http.StatusBadRequest)
	}
}

// ListCustomIcons returns a list of custom uploaded icons
func (h *IconHandler) ListCustomIcons(w http.ResponseWriter, r *http.Request) {
	iconList, err := h.customManager.ListIcons()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(iconList)
}

// UploadCustomIcon handles custom icon file uploads
func (h *IconHandler) UploadCustomIcon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, icons.MaxIconSize+1024) // Extra for form overhead

	// Parse multipart form
	if err := r.ParseMultipartForm(icons.MaxIconSize); err != nil {
		http.Error(w, "File too large or invalid form", http.StatusBadRequest)
		return
	}

	// Get the file
	file, header, err := r.FormFile("icon")
	if err != nil {
		http.Error(w, "No icon file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Get icon name (from form or filename)
	name := r.FormValue("name")
	if name == "" {
		name = strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	}

	// Read file content
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// Determine content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/octet-stream" {
		// Detect from file content
		contentType = http.DetectContentType(data)
	}

	// Save the icon
	if err := h.customManager.SaveIcon(name, data, contentType); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"name":   name,
		"status": "uploaded",
	})
}

// DeleteCustomIcon handles custom icon deletion
func (h *IconHandler) DeleteCustomIcon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract icon name from path: /api/icons/custom/{name}
	path := strings.TrimPrefix(r.URL.Path, "/api/icons/custom/")
	if path == "" {
		http.Error(w, "Icon name required", http.StatusBadRequest)
		return
	}

	if err := h.customManager.DeleteIcon(path); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "deleted",
	})
}
