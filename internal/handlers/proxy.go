package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/mescon/muximux3/internal/proxy"
)

// ProxyHandler handles proxy-related API requests
type ProxyHandler struct {
	proxy *proxy.Proxy
}

// NewProxyHandler creates a new proxy handler
func NewProxyHandler(p *proxy.Proxy) *ProxyHandler {
	return &ProxyHandler{proxy: p}
}

// ProxyStatusResponse represents the proxy status
type ProxyStatusResponse struct {
	Enabled bool   `json:"enabled"`
	Running bool   `json:"running"`
	Listen  string `json:"listen,omitempty"`
}

// GetStatus returns the proxy status
func (h *ProxyHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := ProxyStatusResponse{
		Enabled: h.proxy != nil,
		Running: h.proxy != nil && h.proxy.IsRunning(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// AppProxyInfo represents proxy info for an app
type AppProxyInfo struct {
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	ProxyURL string `json:"proxy_url"`
	Enabled  bool   `json:"enabled"`
}

// GetAppProxyURL returns the proxy URL for a specific app
func (h *ProxyHandler) GetAppProxyURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract app slug from query parameter
	slug := r.URL.Query().Get("slug")
	if slug == "" {
		http.Error(w, "Missing slug parameter", http.StatusBadRequest)
		return
	}

	proxyURL := ""
	if h.proxy != nil {
		proxyURL = h.proxy.GetProxyURL(slug)
	}

	info := AppProxyInfo{
		Slug:     slug,
		ProxyURL: proxyURL,
		Enabled:  proxyURL != "",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}
