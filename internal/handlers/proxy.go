package handlers

import (
	"net/http"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/proxy"
)

// ProxyHandler handles proxy-related API requests
type ProxyHandler struct {
	proxy        *proxy.Proxy
	serverConfig config.ServerConfig
}

// NewProxyHandler creates a new proxy handler
func NewProxyHandler(p *proxy.Proxy, serverCfg *config.ServerConfig) *ProxyHandler {
	return &ProxyHandler{proxy: p, serverConfig: *serverCfg}
}

// ProxyStatusResponse represents the proxy status
type ProxyStatusResponse struct {
	Enabled bool   `json:"enabled"`
	Running bool   `json:"running"`
	TLS     bool   `json:"tls"`
	Domain  string `json:"domain,omitempty"`
	Gateway string `json:"gateway,omitempty"`
}

// GetStatus returns the proxy status
func (h *ProxyHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	status := ProxyStatusResponse{
		Enabled: h.proxy != nil,
		Running: h.proxy != nil && h.proxy.IsRunning(),
		TLS:     h.serverConfig.TLS.Domain != "" || h.serverConfig.TLS.Cert != "",
		Domain:  h.serverConfig.TLS.Domain,
		Gateway: h.serverConfig.Gateway,
	}

	sendJSON(w, http.StatusOK, status)
}
