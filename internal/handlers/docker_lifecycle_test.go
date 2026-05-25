package handlers

import (
	"errors"
	"testing"
)

func TestMapDockerError_KnownErrors(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"port_allocated", "Error response from daemon: port is already allocated", "Port already in use"},
		{"no_such_image", "Error response from daemon: no such image: foo:latest", "Image not found"},
		{"no_such_container", "Error response from daemon: No such container: abc123", "Container not found"},
		{"permission_denied", "Got permission denied while trying to connect to the Docker daemon socket", "Permission denied (socket access)"},
		{"already_started", "Container is already started", "Already running"},
		{"not_running", "Container abc is not running", "Already stopped"},
		{"deadline", "context deadline exceeded", "Docker daemon timeout"},
		{"unknown", "the moon ate my container", "Action failed (see audit log)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapDockerError(errors.New(tc.in))
			if got != tc.want {
				t.Fatalf("mapDockerError(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestMapDockerError_NilReturnsEmpty(t *testing.T) {
	if got := mapDockerError(nil); got != "" {
		t.Fatalf("expected empty for nil err, got %q", got)
	}
}
