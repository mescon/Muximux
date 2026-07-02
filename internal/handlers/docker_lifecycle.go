package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
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
	BroadcastDockerStateChanged(appName string, state *websocket.DockerStatePayload, restricted bool)
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

	// Run the shared user-level gate ladder (the same one behind the
	// can_use_docker_lifecycle flag). Then map the failing rung to its
	// HTTP status. Keeping the ladder in evaluateLifecycleGate means the
	// enforcement here and the UI's advisory flag can't drift apart.
	socketWritable := h.dockerService != nil && h.dockerService.SocketWritable()
	switch reason := evaluateLifecycleGate(user, &cfg, socketWritable); reason {
	case denyNone:
		// allowed; fall through to the per-app checks below.
	case denyLifecycleDisabled:
		logging.Audit("Docker lifecycle action denied",
			"app", name, "action", action, "caller", caller, "reason", string(reason))
		respondError(w, r, http.StatusServiceUnavailable, errLifecycleDisabled)
		return
	case denySocketReadonly:
		logging.Audit("Docker lifecycle action denied",
			"app", name, "action", action, "caller", caller, "reason", string(reason))
		respondError(w, r, http.StatusServiceUnavailable, errSocketReadOnly)
		return
	default: // denyMinRole, denyNotInGroup
		logging.Audit("Docker lifecycle action denied",
			"app", name, "action", action, "caller", caller, "reason", string(reason))
		respondError(w, r, http.StatusForbidden, errAccessDenied)
		return
	}

	h.mu.RLock()
	var app config.AppConfig
	var found bool
	for i := range h.config.Apps {
		if h.config.Apps[i].Name == name {
			app = h.config.Apps[i]
			found = true
			break
		}
	}
	h.mu.RUnlock()
	if !found {
		respondError(w, r, http.StatusNotFound, errAppNotFound)
		return
	}
	// Per-app access gate: a user who clears the lifecycle gate above but
	// is not allowed to SEE this app (its own min_role / allowed_groups)
	// must not be able to control its container. The lifecycle gate alone
	// governs the capability; this governs which apps it applies to.
	// Mirrors http_action's appAccessible; admins bypass by role.
	if !appAccessible(user, &app) {
		logging.Audit("Docker lifecycle action denied",
			"app", name, "action", action, "caller", caller, "reason", "app_access_denied")
		respondError(w, r, http.StatusForbidden, errAccessDenied)
		return
	}
	appName := app.Name
	dockerKey := app.DockerKey
	if dockerKey == "" {
		respondError(w, r, http.StatusBadRequest, errAppNotDockerTracked)
		return
	}
	id, ok := h.dockerService.ResolveContainerID(r.Context(), dockerKey)
	if !ok {
		respondError(w, r, http.StatusNotFound, errContainerNotFound)
		return
	}

	// Detach the mutating op (and the post-action inspect) from the
	// request context. A lifecycle action must run to completion and
	// audit-log its true outcome even if the client navigates away
	// mid-request: otherwise a disconnect cancels the in-flight call,
	// the action may still take effect on the daemon, yet we'd log it
	// as "failed (context canceled)". WithoutCancel keeps context
	// values (logging) while dropping cancellation; the timeout
	// comfortably covers stop's 10s SIGTERM grace and restart's
	// stop-then-start plus daemon overhead.
	opCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 45*time.Second)
	defer cancel()

	start := time.Now()
	err := op(opCtx, id)
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

	// Refresh this single app's state so the response body and the
	// WebSocket broadcast both carry the post-action state without
	// waiting for the next poll tick. If the inspect fails (the op may
	// have consumed most of the 45s budget, or a stop removed the
	// container), the op itself still succeeded: record that, but do NOT
	// cache/broadcast/return a fabricated zero-value state -- that would
	// blank every client's status pill until the next poll. The poller
	// supplies the real state on its next tick.
	newState, inspectErr := h.dockerService.InspectContainerState(opCtx, id)
	if inspectErr != nil {
		logging.Audit("Docker lifecycle action succeeded; post-action inspect failed",
			"app", appName, "action", action,
			"container_id", id, "caller", caller,
			"inspect_error", inspectErr.Error(), "latency_ms", latency.Milliseconds())
		sendJSON(w, http.StatusOK, dockerActionResult{LatencyMS: latency.Milliseconds()})
		return
	}

	h.dockerService.SetDockerStateForApp(appName, &newState)
	if h.dockerHub != nil {
		// A restricted app (min_role / allowed_groups) broadcasts its
		// realtime state to admins only, matching the docker-state GET
		// filter, so a non-admin can't learn a hidden app's state.
		restricted := app.MinRole != "" || len(app.AllowedGroups) > 0
		h.dockerHub.BroadcastDockerStateChanged(appName, DockerStatePayloadFromState(&newState), restricted)
	}

	logging.Audit("Docker lifecycle action succeeded",
		"app", appName, "action", action,
		"container_id", id, "caller", caller,
		"new_status", string(newState.Status), "latency_ms", latency.Milliseconds())

	sendJSON(w, http.StatusOK, dockerActionResult{
		Status:    string(newState.Status),
		LatencyMS: latency.Milliseconds(),
	})
}

// DockerStatePayloadFromState converts a discovery.DockerState into the
// websocket wire payload. The two structs are deliberately separate (to
// avoid a websocket -> discovery import cycle); centralizing the field
// copy here means there is exactly one place to update when a field is
// added, and both the action handler and the poller's broadcast closure
// (server.go) go through it.
func DockerStatePayloadFromState(st *discovery.DockerState) *websocket.DockerStatePayload {
	return &websocket.DockerStatePayload{
		Status:       string(st.Status),
		Health:       string(st.Health),
		StartedAt:    st.StartedAt,
		FinishedAt:   st.FinishedAt,
		ExitCode:     st.ExitCode,
		RestartCount: st.RestartCount,
		Image:        st.Image,
	}
}

// DockerStart handles POST /api/app-docker/{name}/start.
func (h *APIHandler) DockerStart(w http.ResponseWriter, r *http.Request, name string) {
	h.dockerAction(w, r, name, "start", h.dockerStartOp)
}

// DockerStop handles POST /api/app-docker/{name}/stop. The underlying
// daemon call gets a 10s graceful-shutdown grace window; if the
// container ignores SIGTERM the daemon escalates to SIGKILL at that
// boundary. Default chosen to match docker-compose's default and the
// spec's Q6 decision.
func (h *APIHandler) DockerStop(w http.ResponseWriter, r *http.Request, name string) {
	h.dockerAction(w, r, name, "stop", h.dockerStopOp)
}

// DockerRestart handles POST /api/app-docker/{name}/restart.
func (h *APIHandler) DockerRestart(w http.ResponseWriter, r *http.Request, name string) {
	h.dockerAction(w, r, name, "restart", h.dockerRestartOp)
}
