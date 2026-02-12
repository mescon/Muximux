package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/mescon/muximux/internal/config"
)

func createTestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Title:  "Test Dashboard",
			Listen: ":3000",
		},
		Navigation: config.NavigationConfig{
			Position:   "left",
			ShowLabels: true,
		},
		Groups: []config.GroupConfig{
			{Name: "Media", Color: "#ff0000", Order: 0},
			{Name: "Tools", Color: "#00ff00", Order: 1},
		},
		Apps: []config.AppConfig{
			{
				Name:    "App1",
				URL:     "http://localhost:8080",
				Color:   "#ff0000",
				Group:   "Media",
				Order:   0,
				Enabled: true,
			},
			{
				Name:    "App2",
				URL:     "http://localhost:8081",
				Color:   "#00ff00",
				Group:   "Tools",
				Order:   1,
				Enabled: true,
				Proxy:   true,
			},
			{
				Name:    "DisabledApp",
				URL:     "http://localhost:8082",
				Enabled: false,
			},
		},
	}
}

func TestHealth(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "")

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	handler.Health(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%s'", response["status"])
	}
}

func TestGetConfig(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()

	handler.GetConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response struct {
		Title      string                  `json:"title"`
		Navigation config.NavigationConfig `json:"navigation"`
		Groups     []config.GroupConfig    `json:"groups"`
		Apps       []ClientAppConfig       `json:"apps"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Title != "Test Dashboard" {
		t.Errorf("expected title 'Test Dashboard', got '%s'", response.Title)
	}

	if len(response.Groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(response.Groups))
	}

	// Should only return enabled apps
	if len(response.Apps) != 2 {
		t.Errorf("expected 2 enabled apps, got %d", len(response.Apps))
	}

	// Proxied app should have proxyUrl field set
	for _, app := range response.Apps {
		if app.Name == "App2" && app.Proxy {
			if app.ProxyURL != "/proxy/app2/" {
				t.Errorf("expected proxyUrl '/proxy/app2/', got '%s'", app.ProxyURL)
			}
		}
	}
}

func TestGetApps(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "")

	req := httptest.NewRequest(http.MethodGet, "/api/apps", nil)
	w := httptest.NewRecorder()

	handler.GetApps(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var apps []ClientAppConfig
	if err := json.NewDecoder(w.Body).Decode(&apps); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should only include enabled apps
	if len(apps) != 2 {
		t.Errorf("expected 2 apps, got %d", len(apps))
	}
}

func TestGetGroups(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "")

	req := httptest.NewRequest(http.MethodGet, "/api/groups", nil)
	w := httptest.NewRecorder()

	handler.GetGroups(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var groups []config.GroupConfig
	if err := json.NewDecoder(w.Body).Decode(&groups); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}
}

func TestGetApp(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "")

	t.Run("existing app", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/apps/App1", nil)
		w := httptest.NewRecorder()

		handler.GetApp(w, req, "App1")

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var app ClientAppConfig
		if err := json.NewDecoder(w.Body).Decode(&app); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if app.Name != "App1" {
			t.Errorf("expected app name 'App1', got '%s'", app.Name)
		}
	})

	t.Run("non-existing app", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/apps/NonExistent", nil)
		w := httptest.NewRecorder()

		handler.GetApp(w, req, "NonExistent")

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
}

func TestGetGroup(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "")

	t.Run("existing group", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/groups/Media", nil)
		w := httptest.NewRecorder()

		handler.GetGroup(w, req, "Media")

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var group config.GroupConfig
		if err := json.NewDecoder(w.Body).Decode(&group); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if group.Name != "Media" {
			t.Errorf("expected group name 'Media', got '%s'", group.Name)
		}
	})

	t.Run("non-existing group", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/groups/NonExistent", nil)
		w := httptest.NewRecorder()

		handler.GetGroup(w, req, "NonExistent")

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
}

func TestCreateApp(t *testing.T) {
	t.Run("valid app", func(t *testing.T) {
		cfg := createTestConfig()
		tmpFile, err := os.CreateTemp("", "config-*.yaml")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpFile.Name())

		handler := NewAPIHandler(cfg, tmpFile.Name())

		newApp := ClientAppConfig{
			Name:    "NewApp",
			URL:     "http://localhost:9000",
			Color:   "#0000ff",
			Enabled: true,
		}
		body, _ := json.Marshal(newApp)

		req := httptest.NewRequest(http.MethodPost, "/api/apps", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.CreateApp(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
		}

		// Verify app was added
		if len(cfg.Apps) != 4 {
			t.Errorf("expected 4 apps, got %d", len(cfg.Apps))
		}
	})

	t.Run("duplicate app", func(t *testing.T) {
		cfg := createTestConfig()
		handler := NewAPIHandler(cfg, "")

		newApp := ClientAppConfig{
			Name: "App1", // Already exists
			URL:  "http://localhost:9000",
		}
		body, _ := json.Marshal(newApp)

		req := httptest.NewRequest(http.MethodPost, "/api/apps", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.CreateApp(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("expected status 409, got %d", w.Code)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		cfg := createTestConfig()
		handler := NewAPIHandler(cfg, "")

		newApp := ClientAppConfig{
			URL: "http://localhost:9000",
		}
		body, _ := json.Marshal(newApp)

		req := httptest.NewRequest(http.MethodPost, "/api/apps", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.CreateApp(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

func TestDeleteApp(t *testing.T) {
	t.Run("existing app", func(t *testing.T) {
		cfg := createTestConfig()
		tmpFile, err := os.CreateTemp("", "config-*.yaml")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpFile.Name())

		handler := NewAPIHandler(cfg, tmpFile.Name())
		initialCount := len(cfg.Apps)

		req := httptest.NewRequest(http.MethodDelete, "/api/apps/App1", nil)
		w := httptest.NewRecorder()

		handler.DeleteApp(w, req, "App1")

		if w.Code != http.StatusNoContent {
			t.Errorf("expected status 204, got %d: %s", w.Code, w.Body.String())
		}

		if len(cfg.Apps) != initialCount-1 {
			t.Errorf("expected %d apps, got %d", initialCount-1, len(cfg.Apps))
		}
	})

	t.Run("non-existing app", func(t *testing.T) {
		cfg := createTestConfig()
		handler := NewAPIHandler(cfg, "")

		req := httptest.NewRequest(http.MethodDelete, "/api/apps/NonExistent", nil)
		w := httptest.NewRecorder()

		handler.DeleteApp(w, req, "NonExistent")

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
}

func TestCreateGroup(t *testing.T) {
	t.Run("valid group", func(t *testing.T) {
		cfg := createTestConfig()
		tmpFile, err := os.CreateTemp("", "config-*.yaml")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpFile.Name())

		handler := NewAPIHandler(cfg, tmpFile.Name())

		newGroup := config.GroupConfig{
			Name:  "NewGroup",
			Color: "#0000ff",
		}
		body, _ := json.Marshal(newGroup)

		req := httptest.NewRequest(http.MethodPost, "/api/groups", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.CreateGroup(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
		}

		if len(cfg.Groups) != 3 {
			t.Errorf("expected 3 groups, got %d", len(cfg.Groups))
		}
	})

	t.Run("duplicate group", func(t *testing.T) {
		cfg := createTestConfig()
		handler := NewAPIHandler(cfg, "")

		newGroup := config.GroupConfig{
			Name: "Media", // Already exists
		}
		body, _ := json.Marshal(newGroup)

		req := httptest.NewRequest(http.MethodPost, "/api/groups", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.CreateGroup(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("expected status 409, got %d", w.Code)
		}
	})
}

func TestDeleteGroup(t *testing.T) {
	t.Run("existing group", func(t *testing.T) {
		cfg := createTestConfig()
		tmpFile, err := os.CreateTemp("", "config-*.yaml")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpFile.Name())

		handler := NewAPIHandler(cfg, tmpFile.Name())

		req := httptest.NewRequest(http.MethodDelete, "/api/groups/Media", nil)
		w := httptest.NewRecorder()

		handler.DeleteGroup(w, req, "Media")

		if w.Code != http.StatusNoContent {
			t.Errorf("expected status 204, got %d: %s", w.Code, w.Body.String())
		}

		// Verify apps in the group are now ungrouped
		for _, app := range cfg.Apps {
			if app.Name == "App1" && app.Group != "" {
				t.Errorf("expected app to be ungrouped, got group '%s'", app.Group)
			}
		}
	})

	t.Run("non-existing group", func(t *testing.T) {
		cfg := createTestConfig()
		handler := NewAPIHandler(cfg, "")

		req := httptest.NewRequest(http.MethodDelete, "/api/groups/NonExistent", nil)
		w := httptest.NewRecorder()

		handler.DeleteGroup(w, req, "NonExistent")

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Simple", "simple"},
		{"Two Words", "two-words"},
		{"CamelCase", "camelcase"},
		{"with_underscores", "with-underscores"},
		{"Already-slug", "already-slug"},
		{"Multiple   Spaces", "multiple---spaces"},
		{"123Numbers", "123numbers"},
		{"Special!@#Chars", "specialchars"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := slugify(tt.input)
			if result != tt.expected {
				t.Errorf("slugify(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSaveConfigMethodNotAllowed(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()

	handler.SaveConfig(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestSaveConfigInvalidJSON(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "")

	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	handler.SaveConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestGetConfigRef(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "")

	ref := handler.GetConfigRef()

	if ref != cfg {
		t.Error("GetConfigRef should return the same config reference")
	}

	if ref.Server.Title != "Test Dashboard" {
		t.Errorf("expected title 'Test Dashboard', got '%s'", ref.Server.Title)
	}
}
