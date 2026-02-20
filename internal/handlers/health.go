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
	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(h.monitor.GetAllHealthSlice())
}

// GetAppHealth returns health status for a specific app
func (h *HealthHandler) GetAppHealth(w http.ResponseWriter, r *http.Request) {
	appName := extractAppName(r.URL.Path, "/health")
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

	appName := extractAppName(r.URL.Path, "/health/check")
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

// extractAppName extracts the app name from paths like /api/apps/{name}/{suffix}
func extractAppName(path, suffix string) string {
	return strings.TrimSuffix(strings.TrimPrefix(path, "/api/apps/"), suffix)
}
