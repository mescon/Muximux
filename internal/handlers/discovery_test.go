package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/discovery"
)

func TestGetDockerStatus_NilService(t *testing.T) {
	// On first boot before discovery is wired, service is nil. The
	// handler must not panic and must return Configured=false so the
	// frontend's CTA-mode kicks in.
	h := NewDiscoveryHandler(nil)
	req := adminCtxRequest(http.MethodGet, "/api/discovery/docker/status")
	w := httptest.NewRecorder()
	h.GetDockerStatus(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got discovery.StatusResult
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Configured {
		t.Errorf("Configured = true, want false for nil service")
	}
}

func TestGetDockerStatus_DisabledConfig(t *testing.T) {
	svc := discovery.NewService(&config.DiscoveryDockerConfig{Enabled: false})
	h := NewDiscoveryHandler(svc)
	req := adminCtxRequest(http.MethodGet, "/api/discovery/docker/status")
	w := httptest.NewRecorder()
	h.GetDockerStatus(w, req)

	var got discovery.StatusResult
	_ = json.NewDecoder(w.Body).Decode(&got)
	if got.Configured {
		t.Errorf("Configured = true, want false")
	}
}

func TestGetDockerStatus_RejectsNonGet(t *testing.T) {
	h := NewDiscoveryHandler(nil)
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := adminCtxRequest(method, "/api/discovery/docker/status")
		w := httptest.NewRecorder()
		h.GetDockerStatus(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s -> status %d, want 405", method, w.Code)
		}
	}
}

func TestGetDockerStatus_BadEndpointSurfacesLastError(t *testing.T) {
	svc := discovery.NewService(&config.DiscoveryDockerConfig{
		Enabled:  true,
		Endpoint: "ssh://nope",
	})
	h := NewDiscoveryHandler(svc)
	req := adminCtxRequest(http.MethodGet, "/api/discovery/docker/status")
	w := httptest.NewRecorder()
	h.GetDockerStatus(w, req)

	var got discovery.StatusResult
	_ = json.NewDecoder(w.Body).Decode(&got)
	if !got.Configured {
		t.Errorf("Configured should be true (operator opted in), got false")
	}
	if got.LastError == "" {
		t.Errorf("LastError empty; want client-construction error")
	}
}

// adminCtxRequest is defined in system_test.go in this package and
// reused here. It seeds an admin user into the request context so the
// handler sees a privileged caller, matching the requireAdmin wrap
// at registration time.
