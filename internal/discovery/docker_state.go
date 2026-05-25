package discovery

import "time"

// DockerState is the parsed subset of /containers/{id}/json we surface
// in the dashboard. Kept narrow so the wire format and the cache map
// don't bloat with fields no one reads. Mirrored on the frontend in
// web/src/lib/types.ts as type `DockerState`.
type DockerState struct {
	Status       string    `json:"status"` // running / exited / paused / restarting / created / dead / missing
	Health       string    `json:"health"` // healthy / unhealthy / starting / none
	StartedAt    time.Time `json:"started_at,omitempty"`
	FinishedAt   time.Time `json:"finished_at,omitempty"`
	ExitCode     int       `json:"exit_code,omitempty"`
	RestartCount int       `json:"restart_count"`
	Image        string    `json:"image"`
}

// dockerStateCache is the per-app cache the poller writes and the
// /api/discovery/docker-state handler reads. Keyed by app name so
// frontend code resolves directly off App.Name without consulting the
// app -> container-id table.
type dockerStateCache map[string]DockerState
