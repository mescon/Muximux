package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mescon/muximux/v3/internal/health"
	"github.com/mescon/muximux/v3/internal/logging"
)

// HealthHandler handles health-related API requests
type HealthHandler struct {
	monitor *health.Monitor
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(monitor *health.Monitor) *HealthHandler {
	return &HealthHandler{monitor: monitor}
}

// GetAllHealth returns health status for all apps
func (h *HealthHandler) GetAllHealth(w http.ResponseWriter, r *http.Request) {
	allHealth := h.monitor.GetAllHealth()

	// Convert to a slice for consistent JSON output
	result := make([]*health.AppHealth, 0, len(allHealth))
	for _, appHealth := range allHealth {
		result = append(result, appHealth)
	}

	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(result)
}

// GetAppHealth returns health status for a specific app
func (h *HealthHandler) GetAppHealth(w http.ResponseWriter, r *http.Request) {
	// Extract app name from path: /api/apps/{name}/health
	path := strings.TrimPrefix(r.URL.Path, "/api/apps/")
	path = strings.TrimSuffix(path, "/health")
	appName := path

	if appName == "" {
		http.Error(w, "App name required", http.StatusBadRequest)
		return
	}

	appHealth := h.monitor.GetHealth(appName)
	if appHealth == nil {
		http.Error(w, errAppNotFound, http.StatusNotFound)
		return
	}

	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(appHealth)
}

// CheckAppHealth triggers an immediate health check for an app
func (h *HealthHandler) CheckAppHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	// Extract app name from path
	path := strings.TrimPrefix(r.URL.Path, "/api/apps/")
	path = strings.TrimSuffix(path, "/health/check")
	appName := path

	if appName == "" {
		http.Error(w, "App name required", http.StatusBadRequest)
		return
	}

	logging.Debug("Manual health check triggered", "source", "health", "app", appName)
	appHealth := h.monitor.CheckNow(appName)
	if appHealth == nil {
		http.Error(w, errAppNotFound, http.StatusNotFound)
		return
	}

	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(appHealth)
}
