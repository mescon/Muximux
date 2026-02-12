package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mescon/muximux/v3/internal/health"
)

func setupHealthTest(t *testing.T) (*HealthHandler, *health.Monitor) {
	t.Helper()

	monitor := health.NewMonitor(30*time.Second, 5*time.Second)

	// Set up some apps
	monitor.SetApps([]health.AppConfig{
		{Name: "sonarr", URL: "http://localhost:8989", Enabled: true},
		{Name: "radarr", URL: "http://localhost:7878", Enabled: true},
	})

	handler := NewHealthHandler(monitor)
	return handler, monitor
}

func TestGetAllHealth(t *testing.T) {
	handler, _ := setupHealthTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
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

func TestGetAppHealth(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		handler, _ := setupHealthTest(t)

		req := httptest.NewRequest(http.MethodGet, "/api/apps/sonarr/health", nil)
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
		handler, _ := setupHealthTest(t)

		req := httptest.NewRequest(http.MethodGet, "/api/apps/nonexistent/health", nil)
		w := httptest.NewRecorder()

		handler.GetAppHealth(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		handler, _ := setupHealthTest(t)

		req := httptest.NewRequest(http.MethodGet, "/api/apps//health", nil)
		w := httptest.NewRecorder()

		handler.GetAppHealth(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

func TestCheckAppHealth(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		handler, _ := setupHealthTest(t)

		req := httptest.NewRequest(http.MethodPost, "/api/apps/nonexistent/health/check", nil)
		w := httptest.NewRecorder()

		handler.CheckAppHealth(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		handler, _ := setupHealthTest(t)

		req := httptest.NewRequest(http.MethodGet, "/api/apps/sonarr/health/check", nil)
		w := httptest.NewRecorder()

		handler.CheckAppHealth(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		handler, _ := setupHealthTest(t)

		req := httptest.NewRequest(http.MethodPost, "/api/apps//health/check", nil)
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

		handler := NewHealthHandler(monitor)

		req := httptest.NewRequest(http.MethodPost, "/api/apps/testapp/health/check", nil)
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
