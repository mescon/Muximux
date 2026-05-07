package handlers

import (
	"net/http"

	"github.com/mescon/muximux/v3/internal/discovery"
)

// DiscoveryHandler exposes discovery-related HTTP endpoints. v1 ships
// only the capability-status endpoint; scan / import / detach / refresh
// land in subsequent phases per dev/docker-discovery-plan.md.
type DiscoveryHandler struct {
	service *discovery.Service
}

// NewDiscoveryHandler binds the handler to a discovery.Service. The
// service is allowed to be nil (and will be on first boot before
// discovery config is wired); a nil service surfaces a Configured=false
// status rather than panicking.
func NewDiscoveryHandler(svc *discovery.Service) *DiscoveryHandler {
	return &DiscoveryHandler{service: svc}
}

// GetDockerStatus handles GET /api/discovery/docker/status. Admin-only
// at registration. The body is the StatusResult struct directly so
// the frontend gets the four-state UI gating ladder (Configured,
// Reachable, StrategyOK, plus error/warning text).
func (h *DiscoveryHandler) GetDockerStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	if h.service == nil {
		// Service is nil only when discovery wasn't configured at
		// startup. Treat as "discovery is off"; the operator can
		// enable it via Settings later (which rebuilds the service).
		sendJSON(w, http.StatusOK, discovery.StatusResult{Configured: false})
		return
	}
	sendJSON(w, http.StatusOK, h.service.Status(r.Context()))
}
