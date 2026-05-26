package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/discovery"
	"github.com/mescon/muximux/v3/internal/logging"
	"github.com/mescon/muximux/v3/internal/websocket"
)

// mapDockerError maps the verbose Docker daemon error strings to
// short, operator-readable messages suitable for a toast. The full
// daemon error continues to land in the audit log so an operator
// can grep the actual cause; this function only shapes the UI text.
func mapDockerError(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "port is already allocated"):
		return "Port already in use"
	case strings.Contains(s, "no such image"):
		return "Image not found"
	case strings.Contains(s, "No such container"):
		return "Container not found"
	case strings.Contains(s, "permission denied"):
		return "Permission denied (socket access)"
	case strings.Contains(s, "is already started"):
		return "Already running"
	case strings.Contains(s, "is not running"):
		return "Already stopped"
	case strings.Contains(s, "context deadline exceeded"):
		return "Docker daemon timeout"
	default:
		return "Action failed (see audit log)"
	}
}

// DockerServiceAPI is the narrow surface the lifecycle handlers need
// from discovery.Service. An interface (rather than a *discovery.Service
// pointer) keeps the handler unit-testable without spinning up a real
// daemon.
type DockerServiceAPI interface {
	SocketWritable() bool
	ResolveContainerID(ctx context.Context, key string) (string, bool)
	InspectContainerState(ctx context.Context, id string) (discovery.DockerState, error)
	SetDockerStateForApp(name string, st *discovery.DockerState)
}

// DockerHubBroadcaster is the narrow surface the lifecycle handlers
// need from websocket.Hub. Same rationale as DockerServiceAPI.
type DockerHubBroadcaster interface {
	BroadcastDockerStateChanged(appName string, state *websocket.DockerStatePayload)
}

// dockerOp is the signature of the underlying daemon call. Start has
// no timeout (uses the default); Stop / Restart wrap StopContainer /
// RestartContainer in a 10s-timeout closure (see Task 17).
type dockerOp func(ctx context.Context, id string) error

// dockerActionResult is the JSON body returned by every successful or
// failed lifecycle handler.
type dockerActionResult struct {
	Status    string `json:"status,omitempty"`
	Error     string `json:"error,omitempty"`
	LatencyMS int64  `json:"latency_ms"`
}

// dockerAction is the shared body behind DockerStart / DockerStop /
// DockerRestart. It runs the gate ladder (lifecycle_enabled, socket
// writable, role floor, group allowlist, app exists, app is
// Docker-tracked, container resolves), fires the op, refreshes state,
// broadcasts, and audit-logs every outcome (denied, failed, succeeded).
//
// The gate ladder is deliberately identical to
// ComputeCanUseDockerLifecycle in auth.go so the server-side
// enforcement here never diverges from the can_use_docker_lifecycle
// flag the frontend uses to show/hide the controls.
func (h *APIHandler) dockerAction(w http.ResponseWriter, r *http.Request, name, action string, op dockerOp) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		respondError(w, r, http.StatusUnauthorized, "Unauthorized")
		return
	}
	caller := user.Username

	h.mu.RLock()
	cfg := h.config.Discovery.Docker
	h.mu.RUnlock()

	if !cfg.LifecycleEnabled {
		logging.Audit("Docker lifecycle action denied",
			"app", name, "action", action,
			"caller", caller, "reason", "lifecycle_disabled")
		respondError(w, r, http.StatusServiceUnavailable, errLifecycleDisabled)
		return
	}
	if h.dockerService == nil || !h.dockerService.SocketWritable() {
		logging.Audit("Docker lifecycle action denied",
			"app", name, "action", action,
			"caller", caller, "reason", "socket_readonly")
		respondError(w, r, http.StatusServiceUnavailable, errSocketReadOnly)
		return
	}

	minRole := cfg.LifecycleMinRole
	if minRole == "" {
		minRole = auth.RoleAdmin
	}
	if !auth.HasMinRole(user.Role, minRole) {
		logging.Audit("Docker lifecycle action denied",
			"app", name, "action", action,
			"caller", caller, "reason", "min_role_not_met")
		respondError(w, r, http.StatusForbidden, errAccessDenied)
		return
	}
	if len(cfg.LifecycleAllowedGroups) > 0 && !auth.InAnyGroup(user, cfg.LifecycleAllowedGroups) {
		logging.Audit("Docker lifecycle action denied",
			"app", name, "action", action,
			"caller", caller, "reason", "not_in_allowed_groups")
		respondError(w, r, http.StatusForbidden, errAccessDenied)
		return
	}

	h.mu.RLock()
	var appName, dockerKey string
	var found bool
	for i := range h.config.Apps {
		if h.config.Apps[i].Name == name {
			appName = h.config.Apps[i].Name
			dockerKey = h.config.Apps[i].DockerKey
			found = true
			break
		}
	}
	h.mu.RUnlock()
	if !found {
		respondError(w, r, http.StatusNotFound, errAppNotFound)
		return
	}
	if dockerKey == "" {
		respondError(w, r, http.StatusBadRequest, errAppNotDockerTracked)
		return
	}
	id, ok := h.dockerService.ResolveContainerID(r.Context(), dockerKey)
	if !ok {
		respondError(w, r, http.StatusNotFound, errContainerNotFound)
		return
	}

	start := time.Now()
	err := op(r.Context(), id)
	latency := time.Since(start)

	if err != nil {
		short := mapDockerError(err)
		logging.Audit("Docker lifecycle action failed",
			"app", appName, "action", action,
			"container_id", id, "caller", caller,
			"error", err.Error(), "latency_ms", latency.Milliseconds())
		sendJSON(w, http.StatusBadGateway, dockerActionResult{
			Error:     short,
			LatencyMS: latency.Milliseconds(),
		})
		return
	}

	// Refresh this single app's state immediately so the response body
	// and the WebSocket broadcast both carry the post-action state
	// without waiting for the next poll tick.
	newState, inspectErr := h.dockerService.InspectContainerState(r.Context(), id)
	if inspectErr == nil {
		h.dockerService.SetDockerStateForApp(appName, &newState)
	}
	if h.dockerHub != nil {
		h.dockerHub.BroadcastDockerStateChanged(appName, &websocket.DockerStatePayload{
			Status:       newState.Status,
			Health:       newState.Health,
			StartedAt:    newState.StartedAt,
			FinishedAt:   newState.FinishedAt,
			ExitCode:     newState.ExitCode,
			RestartCount: newState.RestartCount,
			Image:        newState.Image,
		})
	}

	logging.Audit("Docker lifecycle action succeeded",
		"app", appName, "action", action,
		"container_id", id, "caller", caller,
		"new_status", newState.Status, "latency_ms", latency.Milliseconds())

	sendJSON(w, http.StatusOK, dockerActionResult{
		Status:    newState.Status,
		LatencyMS: latency.Milliseconds(),
	})
}

// DockerStart handles POST /api/app-docker/{name}/start.
func (h *APIHandler) DockerStart(w http.ResponseWriter, r *http.Request, name string) {
	h.dockerAction(w, r, name, "start", h.dockerStartOp)
}
