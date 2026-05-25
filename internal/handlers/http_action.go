package handlers

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/logging"
)

type fireActionResult struct {
	Status    int    `json:"status,omitempty"`
	Error     string `json:"error,omitempty"`
	LatencyMS int64  `json:"latency_ms"`
	URLHost   string `json:"url_host,omitempty"`
	Method    string `json:"method,omitempty"`
}

// FireAppAction relays an http_action click from the dashboard to the
// configured target URL and returns a small JSON result. Every fire
// audit-logs the host (never the full URL) because webhook URLs often
// embed secrets in query strings.
func (h *APIHandler) FireAppAction(w http.ResponseWriter, r *http.Request, name string) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		respondError(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	h.mu.RLock()
	app, ok := findAppByName(h.config.Apps, name)
	h.mu.RUnlock()
	if !ok {
		respondError(w, r, http.StatusNotFound, errAppNotFound)
		return
	}

	if app.OpenMode != "http_action" {
		respondError(w, r, http.StatusBadRequest, "App is not an http_action")
		return
	}

	if !appAccessible(user, &app) {
		logging.Audit("HTTP action denied",
			"app", app.Name,
			"caller", user.Username,
			"reason", "access_denied")
		respondError(w, r, http.StatusForbidden, "Access denied")
		return
	}

	method := app.HTTPActionMethod
	if method == "" {
		method = http.MethodPost
	}
	urlHost := safeURLHost(app.URL)

	req, err := http.NewRequestWithContext(r.Context(), method, app.URL, nil)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, fmt.Sprintf("invalid url: %s", err))
		return
	}
	for k, v := range app.HTTPActionHeaders {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := h.actionClient.Do(req)
	latency := time.Since(start)

	if err != nil {
		logging.Audit("HTTP action failed",
			"app", app.Name,
			"method", method,
			"url_host", urlHost,
			"caller", user.Username,
			"error", err.Error(),
			"latency_ms", latency.Milliseconds())
		sendJSON(w, http.StatusBadGateway, fireActionResult{
			Error: err.Error(), LatencyMS: latency.Milliseconds(), URLHost: urlHost, Method: method,
		})
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	logging.Audit("HTTP action fired",
		"app", app.Name,
		"method", method,
		"url_host", urlHost,
		"caller", user.Username,
		"status", resp.StatusCode,
		"latency_ms", latency.Milliseconds(),
		"success", success)

	sendJSON(w, http.StatusOK, fireActionResult{
		Status: resp.StatusCode, LatencyMS: latency.Milliseconds(), URLHost: urlHost, Method: method,
	})
}

func findAppByName(apps []config.AppConfig, name string) (config.AppConfig, bool) {
	for i := range apps {
		if apps[i].Name == name {
			return apps[i], true
		}
	}
	return config.AppConfig{}, false
}

func appAccessible(user *auth.User, app *config.AppConfig) bool {
	if user.Role == auth.RoleAdmin {
		return true
	}
	if app.MinRole != "" && !auth.HasMinRole(user.Role, app.MinRole) {
		return false
	}
	if len(app.AllowedGroups) > 0 && !userInAnyAllowedGroup(user.Groups, app.AllowedGroups) {
		return false
	}
	return true
}

func safeURLHost(rawURL string) string {
	if strings.TrimSpace(rawURL) == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Host
}
