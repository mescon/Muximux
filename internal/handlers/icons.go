package handlers

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/mescon/muximux/v3/internal/icons"
	"github.com/mescon/muximux/v3/internal/logging"
)

// validateHostSSRF resolves a hostname and rejects private/internal IPs.
// Defined as a variable so tests can override it for localhost test servers.
var validateHostSSRF = func(hostname string) error {
	ips, err := net.LookupHost(hostname)
	if err != nil {
		return err
	}
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return &net.AddrError{Err: "address is private or internal", Addr: ipStr}
		}
	}
	return nil
}

// IconHandler handles icon-related requests
type IconHandler struct {
	dashboardClient *icons.DashboardIconsClient
	lucideClient    *icons.LucideClient
	customManager   *icons.CustomIconsManager
}

// NewIconHandler creates a new icon handler
func NewIconHandler(dashboardClient *icons.DashboardIconsClient, lucideClient *icons.LucideClient, customIconsDir string) *IconHandler {
	return &IconHandler{
		dashboardClient: dashboardClient,
		lucideClient:    lucideClient,
		customManager:   icons.NewCustomIconsManager(customIconsDir),
	}
}

// GetDashboardIcon serves a dashboard icon
func (h *IconHandler) GetDashboardIcon(w http.ResponseWriter, r *http.Request) {
	// Extract icon name from path: /api/icons/dashboard/{name}.{ext}
	path := strings.TrimPrefix(r.URL.Path, "/api/icons/dashboard/")
	if path == "" {
		respondError(w, r, http.StatusBadRequest, errIconNameRequired)
		return
	}

	// Parse name and variant from extension or query param
	name := path
	variant := r.URL.Query().Get("variant")
	if variant == "" {
		ext := filepath.Ext(name)
		if ext != "" {
			variant = strings.TrimPrefix(ext, ".")
			name = strings.TrimSuffix(name, ext)
		} else {
			variant = "svg"
		}
	}

	// Get the icon (falls back through svg → webp → png)
	data, contentType, err := h.dashboardClient.GetIcon(name, variant)
	if err != nil {
		respondError(w, r, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set(headerContentType, contentType)
	w.Header().Set(headerCacheControl, cachePublic24h)
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
		respondError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	sendJSON(w, http.StatusOK, iconList)
}

// ListLucideIcons returns a list of available Lucide icons with optional search
func (h *IconHandler) ListLucideIcons(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	var iconList []icons.LucideIconInfo
	var err error

	if query != "" {
		iconList, err = h.lucideClient.SearchIcons(query)
	} else {
		iconList, err = h.lucideClient.ListIcons()
	}

	if err != nil {
		respondError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	sendJSON(w, http.StatusOK, iconList)
}

// GetLucideIcon serves a single Lucide icon by name
func (h *IconHandler) GetLucideIcon(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/icons/lucide/")
	if path == "" {
		respondError(w, r, http.StatusBadRequest, errIconNameRequired)
		return
	}

	data, contentType, err := h.lucideClient.GetIcon(path)
	if err != nil {
		respondError(w, r, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set(headerContentType, contentType)
	w.Header().Set(headerCacheControl, cachePublic24h)
	w.Write(data)
}

// ServeIcon serves an icon based on type (dashboard, lucide, or custom)
func (h *IconHandler) ServeIcon(w http.ResponseWriter, r *http.Request) {
	// Path format: /icons/{type}/{name}
	path := strings.TrimPrefix(r.URL.Path, "/icons/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		respondError(w, r, http.StatusBadRequest, "Invalid icon path")
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
			respondError(w, r, http.StatusNotFound, err.Error())
			return
		}

		w.Header().Set(headerContentType, contentType)
		w.Header().Set(headerCacheControl, cachePublic24h)
		w.Write(data)

	case "custom":
		// Serve from custom icons directory
		data, contentType, err := h.customManager.GetIcon(iconName)
		if err != nil {
			respondError(w, r, http.StatusNotFound, err.Error())
			return
		}

		w.Header().Set(headerContentType, contentType)
		w.Header().Set(headerCacheControl, cachePublic24h)
		w.Write(data)

	case "lucide":
		// Serve from Lucide CDN (cached locally)
		name := strings.TrimSuffix(iconName, filepath.Ext(iconName))
		data, contentType, err := h.lucideClient.GetIcon(name)
		if err != nil {
			respondError(w, r, http.StatusNotFound, err.Error())
			return
		}

		w.Header().Set(headerContentType, contentType)
		w.Header().Set(headerCacheControl, cachePublic24h)
		w.Write(data)

	default:
		respondError(w, r, http.StatusBadRequest, "Unknown icon type")
	}
}

// ListCustomIcons returns a list of custom uploaded icons
func (h *IconHandler) ListCustomIcons(w http.ResponseWriter, r *http.Request) {
	iconList, err := h.customManager.ListIcons()
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	sendJSON(w, http.StatusOK, iconList)
}

// UploadCustomIcon handles custom icon file uploads
func (h *IconHandler) UploadCustomIcon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, icons.MaxIconSize+1024) // Extra for form overhead

	// Parse multipart form
	if err := r.ParseMultipartForm(icons.MaxIconSize); err != nil {
		respondError(w, r, http.StatusBadRequest, "File too large or invalid form")
		return
	}

	// Get the file
	file, header, err := r.FormFile("icon")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "No icon file provided")
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
		respondError(w, r, http.StatusInternalServerError, "Failed to read file")
		return
	}

	// Determine content type
	contentType := header.Header.Get(headerContentType)
	if contentType == "" || contentType == "application/octet-stream" {
		// Detect from file content
		contentType = http.DetectContentType(data)
	}

	// Save the icon
	if err := h.customManager.SaveIcon(name, data, contentType); err != nil {
		respondError(w, r, http.StatusBadRequest, err.Error(), "source", "icons", "name", name, "error", err)
		return
	}

	logging.From(r.Context()).Info("Custom icon uploaded", "source", "icons", "name", name, "size", len(data))
	sendJSON(w, http.StatusOK, map[string]string{
		"name":   name,
		"status": "uploaded",
	})
}

// fetchIconRequest is the JSON body for POST /api/icons/custom/fetch
type fetchIconRequest struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

// FetchCustomIcon downloads an icon from a URL and saves it as a custom icon
func (h *IconHandler) FetchCustomIcon(w http.ResponseWriter, r *http.Request) {
	var req fetchIconRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidBody)
		return
	}

	if req.URL == "" {
		respondError(w, r, http.StatusBadRequest, "URL is required")
		return
	}

	// Parse and validate URL scheme
	parsed, err := url.Parse(req.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		respondError(w, r, http.StatusBadRequest, "Invalid URL: must be http or https")
		return
	}

	// SSRF protection: resolve hostname and reject private/internal IPs
	if err := validateHostSSRF(parsed.Hostname()); err != nil {
		respondError(w, r, http.StatusBadRequest, "URL must not point to a private or internal address")
		return
	}

	// Download with timeout and size limit (use parsed URL, not raw input).
	// Re-check scheme on the reconstructed URL as a defense-in-depth measure
	// (also satisfies static analysis SSRF checks).
	sanitizedURL := parsed.String()
	if !strings.HasPrefix(sanitizedURL, "http://") && !strings.HasPrefix(sanitizedURL, "https://") {
		respondError(w, r, http.StatusBadRequest, "Invalid URL scheme")
		return
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(sanitizedURL) //nolint:gosec // SSRF mitigated by validateHostSSRF above
	if err != nil {
		respondError(w, r, http.StatusBadGateway, "Failed to download icon", "source", "icons", "url", sanitizedURL, "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respondError(w, r, http.StatusBadGateway, "Remote server returned "+resp.Status)
		return
	}

	// Read with size limit
	data, err := io.ReadAll(io.LimitReader(resp.Body, icons.MaxIconSize+1))
	if err != nil {
		respondError(w, r, http.StatusBadGateway, "Failed to read response")
		return
	}
	if len(data) > icons.MaxIconSize {
		respondError(w, r, http.StatusBadRequest, "File too large: max size is 2MB")
		return
	}

	// Detect content type from response header, falling back to content sniffing
	contentType := resp.Header.Get(headerContentType)
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(data)
	}
	// Strip parameters (e.g. "image/png; charset=utf-8" -> "image/png")
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	// Validate MIME type
	if _, ok := icons.AllowedMimeTypes[contentType]; !ok {
		respondError(w, r, http.StatusBadRequest, "Unsupported file type: "+contentType)
		return
	}

	// Derive name from URL filename if not provided
	name := req.Name
	if name == "" {
		name = filepath.Base(parsed.Path)
		name = strings.TrimSuffix(name, filepath.Ext(name))
		if name == "" || name == "." {
			name = "fetched-icon"
		}
	}

	// Save the icon (reuses same validation as file upload)
	if err := h.customManager.SaveIcon(name, data, contentType); err != nil {
		respondError(w, r, http.StatusBadRequest, err.Error(), "source", "icons", "name", name, "error", err)
		return
	}

	logging.From(r.Context()).Info("Custom icon fetched from URL", "source", "icons", "name", name, "url", req.URL, "size", len(data))
	sendJSON(w, http.StatusOK, map[string]string{
		"name":   name,
		"status": "uploaded",
	})
}

// DeleteCustomIcon handles custom icon deletion
func (h *IconHandler) DeleteCustomIcon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	// Extract icon name from path: /api/icons/custom/{name}
	path := strings.TrimPrefix(r.URL.Path, "/api/icons/custom/")
	if path == "" {
		respondError(w, r, http.StatusBadRequest, errIconNameRequired)
		return
	}

	if err := h.customManager.DeleteIcon(path); err != nil {
		respondError(w, r, http.StatusNotFound, err.Error())
		return
	}

	logging.From(r.Context()).Info("Custom icon deleted", "source", "icons", "name", path)
	sendJSON(w, http.StatusOK, map[string]string{
		"status": "deleted",
	})
}
