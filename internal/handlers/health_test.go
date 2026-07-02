package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/health"
)

func setupHealthTest(t *testing.T) *HealthHandler {
	t.Helper()

	monitor := health.NewMonitor(30*time.Second, 5*time.Second)

	// Set up some apps
	monitor.SetApps([]health.AppConfig{
		{Name: "sonarr", URL: "http://localhost:8989", Enabled: true},
		{Name: "radarr", URL: "http://localhost:7878", Enabled: true},
	})

	cfg := &config.Config{Apps: []config.AppConfig{{Name: "sonarr"}, {Name: "radarr"}}}
	handler := NewHealthHandler(monitor, cfg, &sync.RWMutex{})
	return handler
}

// adminHealthReq builds a request whose context carries an admin user, as the
// auth middleware does in production, so the handler's visibility filter has a
// user to evaluate.
func adminHealthReq(method, path string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	return req.WithContext(auth.WithUserContext(req.Context(), &auth.User{Username: "admin", Role: auth.RoleAdmin}))
}

func TestGetAllHealth(t *testing.T) {
	handler := setupHealthTest(t)

	req := adminHealthReq(http.MethodGet, "/api/health")
	w := httptest.NewRecorder()

	handler.GetAllHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var result []*health.AppHealth
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 health entries, got %d", len(result))
	}

	// Check that both apps are present
	names := make(map[string]bool)
	for _, h := range result {
		names[h.Name] = true
	}
	if !names["sonarr"] {
		t.Error("expected 'sonarr' in health results")
	}
	if !names["radarr"] {
		t.Error("expected 'radarr' in health results")
	}
}

// TestHealth_FiltersByAppVisibility guards that the health endpoints do not
// expose an app hidden from the caller by min_role / allowed_groups.
func TestHealth_FiltersByAppVisibility(t *testing.T) {
	monitor := health.NewMonitor(30*time.Second, 5*time.Second)
	monitor.SetApps([]health.AppConfig{
		{Name: "public", URL: "http://localhost:1", Enabled: true},
		{Name: "secret", URL: "http://localhost:2", Enabled: true},
	})
	cfg := &config.Config{Apps: []config.AppConfig{
		{Name: "public"},
		{Name: "secret", AllowedGroups: []string{"admins"}},
	}}
	h := NewHealthHandler(monitor, cfg, &sync.RWMutex{})

	nonAdmin := func(method, path string) *http.Request {
		req := httptest.NewRequest(method, path, nil)
		return req.WithContext(auth.WithUserContext(req.Context(), &auth.User{Username: "bob", Role: auth.RoleUser}))
	}

	// GetAllHealth: non-admin sees only the unrestricted app.
	w := httptest.NewRecorder()
	h.GetAllHealth(w, nonAdmin(http.MethodGet, "/api/apps/health"))
	var all []*health.AppHealth
	if err := json.NewDecoder(w.Body).Decode(&all); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, ah := range all {
		if ah.Name == "secret" {
			t.Fatal("non-admin must not see the restricted app's health")
		}
	}

	// GetAppHealth: the restricted app reads as 404, not an existence oracle.
	w = httptest.NewRecorder()
	h.GetAppHealth(w, nonAdmin(http.MethodGet, "/api/apps/secret/health"))
	if w.Code != http.StatusNotFound {
		t.Errorf("restricted app health should 404 for a non-admin, got %d", w.Code)
	}
}

func TestGetAppHealth(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		handler := setupHealthTest(t)

		req := adminHealthReq(http.MethodGet, "/api/apps/sonarr/health")
		w := httptest.NewRecorder()

		handler.GetAppHealth(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var result health.AppHealth
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if result.Name != "sonarr" {
			t.Errorf("expected name 'sonarr', got %q", result.Name)
		}
	})

	t.Run("not found", func(t *testing.T) {
		handler := setupHealthTest(t)

		req := adminHealthReq(http.MethodGet, "/api/apps/nonexistent/health")
		w := httptest.NewRecorder()

		handler.GetAppHealth(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		handler := setupHealthTest(t)

		req := adminHealthReq(http.MethodGet, "/api/apps//health")
		w := httptest.NewRecorder()

		handler.GetAppHealth(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

func TestCheckAppHealth(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		handler := setupHealthTest(t)

		req := adminHealthReq(http.MethodPost, "/api/apps/nonexistent/health/check")
		w := httptest.NewRecorder()

		handler.CheckAppHealth(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		handler := setupHealthTest(t)

		req := adminHealthReq(http.MethodGet, "/api/apps/sonarr/health/check")
		w := httptest.NewRecorder()

		handler.CheckAppHealth(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		handler := setupHealthTest(t)

		req := adminHealthReq(http.MethodPost, "/api/apps//health/check")
		w := httptest.NewRecorder()

		handler.CheckAppHealth(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("triggers check for existing app", func(t *testing.T) {
		// Create a backend that responds to health checks
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer backend.Close()

		monitor := health.NewMonitor(30*time.Second, 5*time.Second)
		monitor.SetApps([]health.AppConfig{
			{Name: "testapp", URL: backend.URL, Enabled: true},
		})

		cfg := &config.Config{Apps: []config.AppConfig{{Name: "testapp"}}}
		handler := NewHealthHandler(monitor, cfg, &sync.RWMutex{})

		req := adminHealthReq(http.MethodPost, "/api/apps/testapp/health/check")
		w := httptest.NewRecorder()

		handler.CheckAppHealth(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var result health.AppHealth
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if result.Name != "testapp" {
			t.Errorf("expected name 'testapp', got %q", result.Name)
		}
		if result.Status != health.StatusHealthy {
			t.Errorf("expected status 'healthy', got %q", result.Status)
		}
	})
}
