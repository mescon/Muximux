package discovery

import "time"

// ContainerStatus is a container's lifecycle state. The values mirror
// Docker's State.Status (the daemon's own closed set), plus "missing" --
// a Muximux sentinel for a tracked container the daemon no longer lists
// (the daemon itself never returns "missing").
type ContainerStatus string

const (
	StatusCreated    ContainerStatus = "created"
	StatusRunning    ContainerStatus = "running"
	StatusPaused     ContainerStatus = "paused"
	StatusRestarting ContainerStatus = "restarting"
	StatusRemoving   ContainerStatus = "removing"
	StatusExited     ContainerStatus = "exited"
	StatusDead       ContainerStatus = "dead"
	StatusMissing    ContainerStatus = "missing" // Muximux sentinel: tracked but not listed
)

// ContainerHealth is a container's healthcheck state. "none" means the
// container declares no healthcheck.
type ContainerHealth string

const (
	HealthHealthy   ContainerHealth = "healthy"
	HealthUnhealthy ContainerHealth = "unhealthy"
	HealthStarting  ContainerHealth = "starting"
	HealthNone      ContainerHealth = "none"
)

// DockerState is the parsed subset of /containers/{id}/json we surface
// in the dashboard. Kept narrow so the wire format and the cache map
// don't bloat with fields no one reads. Mirrored on the frontend in
// web/src/lib/types.ts as type `DockerState`.
type DockerState struct {
	Status       ContainerStatus `json:"status"`
	Health       ContainerHealth `json:"health"`
	StartedAt    time.Time       `json:"started_at,omitempty"`
	FinishedAt   time.Time       `json:"finished_at,omitempty"`
	ExitCode     int             `json:"exit_code,omitempty"`
	RestartCount int             `json:"restart_count"`
	Image        string          `json:"image"`
}

// dockerStateCache is the per-app cache the poller writes and the
// /api/discovery/docker-state handler reads. Keyed by app name so
// frontend code resolves directly off App.Name without consulting the
// app -> container-id table.
type dockerStateCache map[string]DockerState
