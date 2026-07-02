package handlers

import (
	"net/http"
	"strings"
	"sync"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/health"
	"github.com/mescon/muximux/v3/internal/logging"
)

// HealthHandler handles health-related API requests
type HealthHandler struct {
	monitor  *health.Monitor
	config   *config.Config
	configMu *sync.RWMutex
}

// NewHealthHandler creates a new health handler. config/configMu are used to
// filter health results by the caller's per-app visibility so a non-admin
// cannot learn the existence or status of an app hidden from them by
// min_role / allowed_groups (the same gate the app list and docker-state
// endpoint apply).
func NewHealthHandler(monitor *health.Monitor, cfg *config.Config, configMu *sync.RWMutex) *HealthHandler {
	return &HealthHandler{monitor: monitor, config: cfg, configMu: configMu}
}

// appVisibleTo reports whether the given user may see the named app. An app
// absent from config (an orphaned health entry) is treated as not visible.
func (h *HealthHandler) appVisibleTo(user *auth.User, appName string) bool {
	if user == nil {
		return false
	}
	h.configMu.RLock()
	defer h.configMu.RUnlock()
	for i := range h.config.Apps {
		if h.config.Apps[i].Name == appName {
			return appAccessible(user, &h.config.Apps[i])
		}
	}
	return false
}

// GetAllHealth returns health status for the apps the caller can see.
func (h *HealthHandler) GetAllHealth(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	all := h.monitor.GetAllHealthSlice()
	visible := make([]*health.AppHealth, 0, len(all))
	for _, ah := range all {
		if h.appVisibleTo(user, ah.Name) {
			visible = append(visible, ah)
		}
	}
	sendJSON(w, http.StatusOK, visible)
}

// GetAppHealth returns health status for a specific app.
func (h *HealthHandler) GetAppHealth(w http.ResponseWriter, r *http.Request) {
	appName := extractAppName(r.URL.Path, "/health")
	if appName == "" {
		respondError(w, r, http.StatusBadRequest, "App name required")
		return
	}

	// Gate before touching the monitor so a hidden app is indistinguishable
	// from a nonexistent one (same 404, no existence oracle).
	if !h.appVisibleTo(auth.GetUserFromContext(r.Context()), appName) {
		respondError(w, r, http.StatusNotFound, errAppNotFound)
		return
	}

	appHealth := h.monitor.GetHealth(appName)
	if appHealth == nil {
		respondError(w, r, http.StatusNotFound, errAppNotFound)
		return
	}

	sendJSON(w, http.StatusOK, appHealth)
}

// CheckAppHealth triggers an immediate health check for an app
func (h *HealthHandler) CheckAppHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	appName := extractAppName(r.URL.Path, "/health/check")
	if appName == "" {
		respondError(w, r, http.StatusBadRequest, "App name required")
		return
	}

	if !h.appVisibleTo(auth.GetUserFromContext(r.Context()), appName) {
		respondError(w, r, http.StatusNotFound, errAppNotFound)
		return
	}

	logging.From(r.Context()).Debug("Manual health check triggered", "source", "health", "app", appName)
	appHealth := h.monitor.CheckNow(appName)
	if appHealth == nil {
		respondError(w, r, http.StatusNotFound, errAppNotFound)
		return
	}

	sendJSON(w, http.StatusOK, appHealth)
}

// extractAppName extracts the app name from paths like /api/apps/{name}/{suffix}
func extractAppName(path, suffix string) string {
	return strings.TrimSuffix(strings.TrimPrefix(path, "/api/apps/"), suffix)
}
