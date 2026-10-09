package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
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
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

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
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

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

	// Admin GET /api/config returns every configured app, including
	// disabled, so the Settings UI can edit them and a save round-trip
	// preserves them. The nav bar filters disabled apps client-side.
	if len(response.Apps) != 3 {
		t.Errorf("expected 3 apps for admin (incl. disabled), got %d", len(response.Apps))
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

// Regression: the client config response must carry the Docker badge
// placement under "discovery.docker". Without it the frontend's
// config.discovery is undefined, so the navbar status badge
// (overview_and_nav) silently never renders.
func TestGetConfig_IncludesDiscoveryPlacement(t *testing.T) {
	cfg := createTestConfig()
	cfg.Discovery.Docker.HealthBadgePlacement = "overview_and_nav"
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()
	handler.GetConfig(w, req)

	var response struct {
		Discovery *struct {
			Docker struct {
				HealthBadgePlacement string `json:"health_badge_placement"`
			} `json:"docker"`
		} `json:"discovery"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.Discovery == nil {
		t.Fatal("config response missing discovery section")
	}
	if response.Discovery.Docker.HealthBadgePlacement != "overview_and_nav" {
		t.Errorf("health_badge_placement = %q, want overview_and_nav", response.Discovery.Docker.HealthBadgePlacement)
	}
}

func TestGetApps(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

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

	// Admin (no session = admin in tests) sees every app, including
	// disabled, so the Settings UI can manage them. Filtering happens
	// client-side in the nav bar.
	if len(apps) != 3 {
		t.Errorf("expected 3 apps for admin (incl. disabled), got %d", len(apps))
	}
}

func TestGetGroups(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

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
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

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
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

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

		handler := NewAPIHandler(cfg, tmpFile.Name(), &sync.RWMutex{})

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
		handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

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
		handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

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

		handler := NewAPIHandler(cfg, tmpFile.Name(), &sync.RWMutex{})
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
		handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

		req := httptest.NewRequest(http.MethodDelete, "/api/apps/NonExistent", nil)
		w := httptest.NewRecorder()

		handler.DeleteApp(w, req, "NonExistent")

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
}

func TestCreateAppInvalidJSON(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

	req := httptest.NewRequest(http.MethodPost, "/api/apps", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()

	handler.CreateApp(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateGroup(t *testing.T) {
	t.Run("valid group", func(t *testing.T) {
		cfg := createTestConfig()
		tmpFile, err := os.CreateTemp("", "config-*.yaml")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpFile.Name())

		handler := NewAPIHandler(cfg, tmpFile.Name(), &sync.RWMutex{})

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
		handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

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

func TestCreateGroupInvalidJSON(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

	req := httptest.NewRequest(http.MethodPost, "/api/groups", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()

	handler.CreateGroup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateGroupMissingName(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

	group := config.GroupConfig{Color: "#ff0000"}
	body, _ := json.Marshal(group)

	req := httptest.NewRequest(http.MethodPost, "/api/groups", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.CreateGroup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestDeleteGroup(t *testing.T) {
	t.Run("existing group", func(t *testing.T) {
		cfg := createTestConfig()
		tmpFile, err := os.CreateTemp("", "config-*.yaml")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpFile.Name())

		handler := NewAPIHandler(cfg, tmpFile.Name(), &sync.RWMutex{})

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
		handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

		req := httptest.NewRequest(http.MethodDelete, "/api/groups/NonExistent", nil)
		w := httptest.NewRecorder()

		handler.DeleteGroup(w, req, "NonExistent")

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
}

func TestDeleteGroup_LeavesLifecycleGroupsAlone(t *testing.T) {
	cfg := createTestConfig()
	cfg.Discovery.Docker.LifecycleEnabled = true
	cfg.Discovery.Docker.LifecycleAllowedGroups = []string{"Media", "Tools"}
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	handler := NewAPIHandler(cfg, tmpFile.Name(), &sync.RWMutex{})

	req := httptest.NewRequest(http.MethodDelete, "/api/groups/Media", nil)
	w := httptest.NewRecorder()
	handler.DeleteGroup(w, req, "Media")

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", w.Code, w.Body.String())
	}
	got := cfg.Discovery.Docker.LifecycleAllowedGroups
	if len(got) != 2 || got[0] != "Media" || got[1] != "Tools" {
		t.Fatalf("lifecycle allowlist after delete = %v, want [Media Tools] unchanged (user/IdP groups)", got)
	}
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
		{"Multiple   Spaces", "multiple-spaces"},
		{"123Numbers", "123numbers"},
		{"Special!@#Chars", "specialchars"},
		{"qBittorrent - WebUI", "qbittorrent-webui"},
		{"  Leading Spaces", "leading-spaces"},
		{"Trailing Spaces  ", "trailing-spaces"},
		{" - Dashes - ", "dashes"},
		{"---triple---dashes---", "triple-dashes"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := Slugify(tt.input)
			if result != tt.expected {
				t.Errorf("Slugify(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSaveConfigMethodNotAllowed(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()

	handler.SaveConfig(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestSaveConfigInvalidJSON(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	handler.SaveConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestUpdateApp(t *testing.T) {
	t.Run("existing app", func(t *testing.T) {
		cfg := createTestConfig()
		tmpFile, err := os.CreateTemp("", "config-*.yaml")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpFile.Name())

		handler := NewAPIHandler(cfg, tmpFile.Name(), &sync.RWMutex{})

		updated := ClientAppConfig{
			Name:    "App1",
			URL:     "http://localhost:9999",
			Color:   "#0000ff",
			Group:   "Tools",
			Order:   5,
			Enabled: true,
		}
		body, _ := json.Marshal(updated)

		req := httptest.NewRequest(http.MethodPut, "/api/apps/App1", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.UpdateApp(w, req, "App1")

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp ClientAppConfig
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.URL != "http://localhost:9999" {
			t.Errorf("expected URL 'http://localhost:9999', got %q", resp.URL)
		}
		if resp.Color != "#0000ff" {
			t.Errorf("expected color '#0000ff', got %q", resp.Color)
		}
	})

	t.Run("non-existing app", func(t *testing.T) {
		cfg := createTestConfig()
		handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

		updated := ClientAppConfig{
			Name: "NonExistent",
			URL:  "http://localhost:9999",
		}
		body, _ := json.Marshal(updated)

		req := httptest.NewRequest(http.MethodPut, "/api/apps/NonExistent", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.UpdateApp(w, req, "NonExistent")

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		cfg := createTestConfig()
		handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

		req := httptest.NewRequest(http.MethodPut, "/api/apps/App1", bytes.NewReader([]byte("not json")))
		w := httptest.NewRecorder()

		handler.UpdateApp(w, req, "App1")

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("preserves auth bypass", func(t *testing.T) {
		cfg := createTestConfig()
		// Add auth bypass to existing app
		cfg.Apps[0].AuthBypass = []config.AuthBypassRule{
			{Path: "/api/*", Methods: []string{"GET"}},
		}
		cfg.Apps[0].Access = config.AppAccessConfig{
			Roles: []string{"admin"},
		}

		tmpFile, err := os.CreateTemp("", "config-*.yaml")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpFile.Name())

		handler := NewAPIHandler(cfg, tmpFile.Name(), &sync.RWMutex{})

		updated := ClientAppConfig{
			Name:    "App1",
			URL:     "http://localhost:9999",
			Enabled: true,
		}
		body, _ := json.Marshal(updated)

		req := httptest.NewRequest(http.MethodPut, "/api/apps/App1", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.UpdateApp(w, req, "App1")

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		// Verify auth bypass was preserved
		if len(cfg.Apps[0].AuthBypass) != 1 {
			t.Errorf("expected 1 auth bypass rule, got %d", len(cfg.Apps[0].AuthBypass))
		}
		if len(cfg.Apps[0].Access.Roles) != 1 {
			t.Errorf("expected 1 access role, got %d", len(cfg.Apps[0].Access.Roles))
		}
	})

	t.Run("proxied app URL update", func(t *testing.T) {
		cfg := createTestConfig()
		tmpFile, err := os.CreateTemp("", "config-*.yaml")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpFile.Name())

		handler := NewAPIHandler(cfg, tmpFile.Name(), &sync.RWMutex{})

		updated := ClientAppConfig{
			Name:    "App2",
			URL:     "http://new-server:9090", // Frontend sends real URL
			Proxy:   true,
			Enabled: true,
		}
		body, _ := json.Marshal(updated)

		req := httptest.NewRequest(http.MethodPut, "/api/apps/App2", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.UpdateApp(w, req, "App2")

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		// URL should be updated, not preserved from old config
		if cfg.Apps[1].URL != "http://new-server:9090" {
			t.Errorf("expected URL 'http://new-server:9090', got %q", cfg.Apps[1].URL)
		}
	})
}

func TestUpdateGroup(t *testing.T) {
	t.Run("existing group", func(t *testing.T) {
		cfg := createTestConfig()
		tmpFile, err := os.CreateTemp("", "config-*.yaml")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpFile.Name())

		handler := NewAPIHandler(cfg, tmpFile.Name(), &sync.RWMutex{})

		updated := config.GroupConfig{
			Name:     "Media",
			Color:    "#0000ff",
			Order:    10,
			Expanded: true,
		}
		body, _ := json.Marshal(updated)

		req := httptest.NewRequest(http.MethodPut, "/api/groups/Media", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.UpdateGroup(w, req, "Media")

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp config.GroupConfig
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Color != "#0000ff" {
			t.Errorf("expected color '#0000ff', got %q", resp.Color)
		}
		if !resp.Expanded {
			t.Error("expected expanded=true")
		}
	})

	t.Run("non-existing group", func(t *testing.T) {
		cfg := createTestConfig()
		handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

		updated := config.GroupConfig{Name: "NonExistent"}
		body, _ := json.Marshal(updated)

		req := httptest.NewRequest(http.MethodPut, "/api/groups/NonExistent", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.UpdateGroup(w, req, "NonExistent")

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		cfg := createTestConfig()
		handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

		req := httptest.NewRequest(http.MethodPut, "/api/groups/Media", bytes.NewReader([]byte("not json")))
		w := httptest.NewRecorder()

		handler.UpdateGroup(w, req, "Media")

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

func TestMergeConfigUpdate(t *testing.T) {
	t.Run("basic merge", func(t *testing.T) {
		cfg := createTestConfig()

		update := &ClientConfigUpdate{
			Title: "New Title",
			Navigation: config.NavigationConfig{
				Position:   "top",
				ShowLabels: false,
			},
			Theme: config.ThemeConfig{
				Family:  "nord",
				Variant: "dark",
			},
			Groups: []config.GroupConfig{
				{Name: "NewGroup", Color: "#ff00ff"},
			},
			Apps: []ClientAppConfig{
				{
					Name:    "App1",
					URL:     "http://localhost:9090",
					Enabled: true,
				},
			},
		}

		mergeConfigUpdate(cfg, update, nil)

		if cfg.Server.Title != "New Title" {
			t.Errorf("expected title 'New Title', got %q", cfg.Server.Title)
		}
		if cfg.Navigation.Position != "top" {
			t.Errorf("expected position 'top', got %q", cfg.Navigation.Position)
		}
		if cfg.Theme.Family != "nord" {
			t.Errorf("expected theme family 'nord', got %q", cfg.Theme.Family)
		}
		if len(cfg.Groups) != 1 {
			t.Errorf("expected 1 group, got %d", len(cfg.Groups))
		}
		if len(cfg.Apps) != 1 {
			t.Errorf("expected 1 app, got %d", len(cfg.Apps))
		}
	})

	t.Run("proxied app URL update", func(t *testing.T) {
		cfg := createTestConfig()

		update := &ClientConfigUpdate{
			Title: "Test",
			Apps: []ClientAppConfig{
				{
					Name:    "App2",
					URL:     "http://new-server:8081", // Client sends the real URL
					Proxy:   true,
					Enabled: true,
				},
			},
		}

		mergeConfigUpdate(cfg, update, nil)

		// Frontend sends real URL (not proxy path), so it should be saved
		if len(cfg.Apps) != 1 {
			t.Fatalf("expected 1 app, got %d", len(cfg.Apps))
		}
		if cfg.Apps[0].URL != "http://new-server:8081" {
			t.Errorf("expected updated URL %q, got %q", "http://new-server:8081", cfg.Apps[0].URL)
		}
	})

	t.Run("rename and reorder does not mix up auth rules", func(t *testing.T) {
		cfg := createTestConfig()
		// Give App1 auth bypass rules
		cfg.Apps[0].AuthBypass = []config.AuthBypassRule{{Path: "/api/*"}}

		// Frontend sends: reordered [App2, App1_renamed] — App2 moved to pos 0, App1 renamed
		update := &ClientConfigUpdate{
			Title: "Test",
			Apps: []ClientAppConfig{
				{Name: "App2", URL: "http://localhost:8081", Proxy: true, Enabled: true},
				{Name: "App1Renamed", URL: "http://localhost:8080", Enabled: true},
			},
		}

		mergeConfigUpdate(cfg, update, nil)

		if len(cfg.Apps) != 2 {
			t.Fatalf("expected 2 apps, got %d", len(cfg.Apps))
		}
		// App2 should NOT have App1's auth bypass rules
		if len(cfg.Apps[0].AuthBypass) != 0 {
			t.Errorf("App2 should have no auth bypass rules, got %d", len(cfg.Apps[0].AuthBypass))
		}
		// Renamed app is new — no auth bypass rules inherited from wrong position
		if len(cfg.Apps[1].AuthBypass) != 0 {
			t.Errorf("App1Renamed should have no auth bypass rules (new name), got %d", len(cfg.Apps[1].AuthBypass))
		}
	})

	t.Run("with keybindings", func(t *testing.T) {
		cfg := createTestConfig()

		kb := &config.KeybindingsConfig{
			Bindings: map[string][]config.KeyCombo{
				"search": {{Key: "k", Ctrl: true}},
			},
		}

		update := &ClientConfigUpdate{
			Title:       "Test",
			Keybindings: kb,
			Apps:        []ClientAppConfig{},
		}

		mergeConfigUpdate(cfg, update, nil)

		if cfg.Keybindings.Bindings == nil {
			t.Fatal("expected keybindings to be set")
		}
		if len(cfg.Keybindings.Bindings["search"]) != 1 {
			t.Errorf("expected 1 search keybinding, got %d", len(cfg.Keybindings.Bindings["search"]))
		}
	})
}

func TestMergeClientApp(t *testing.T) {
	t.Run("new app", func(t *testing.T) {
		var existing *config.AppConfig
		clientApp := ClientAppConfig{
			Name:    "NewApp",
			URL:     "http://localhost:9000",
			Enabled: true,
			Color:   "#ff0000",
		}

		result, _ := mergeClientApp(&clientApp, existing)

		if result.Name != "NewApp" {
			t.Errorf("expected name 'NewApp', got %q", result.Name)
		}
		if result.URL != "http://localhost:9000" {
			t.Errorf("expected URL 'http://localhost:9000', got %q", result.URL)
		}
	})

	t.Run("existing proxied app updates URL", func(t *testing.T) {
		existing := &config.AppConfig{
			Name:    "ProxiedApp",
			URL:     "http://internal:8080",
			Proxy:   true,
			Enabled: true,
			AuthBypass: []config.AuthBypassRule{
				{Path: "/api/*"},
			},
		}
		clientApp := ClientAppConfig{
			Name:    "ProxiedApp",
			URL:     "http://new-internal:9090", // Frontend sends real URL, not proxy path
			Proxy:   true,
			Enabled: true,
		}

		result, _ := mergeClientApp(&clientApp, existing)

		// URL should be updated to the new value
		if result.URL != "http://new-internal:9090" {
			t.Errorf("expected updated URL 'http://new-internal:9090', got %q", result.URL)
		}
		// AuthBypass should still be preserved
		if len(result.AuthBypass) != 1 {
			t.Errorf("expected 1 auth bypass rule, got %d", len(result.AuthBypass))
		}
	})

	t.Run("existing non-proxied app updates URL", func(t *testing.T) {
		existing := &config.AppConfig{
			Name:    "App",
			URL:     "http://old:8080",
			Proxy:   false,
			Enabled: true,
		}
		clientApp := ClientAppConfig{
			Name:    "App",
			URL:     "http://new:9090",
			Proxy:   false,
			Enabled: true,
		}

		result, _ := mergeClientApp(&clientApp, existing)

		if result.URL != "http://new:9090" {
			t.Errorf("expected new URL 'http://new:9090', got %q", result.URL)
		}
	})
}

func TestBuildClientConfigResponse(t *testing.T) {
	cfg := createTestConfig()

	resp := buildClientConfigResponse(cfg, "admin", nil)

	if resp.Title != "Test Dashboard" {
		t.Errorf("expected title 'Test Dashboard', got %q", resp.Title)
	}
	if resp.Navigation.Position != "left" {
		t.Errorf("expected navigation position 'left', got %q", resp.Navigation.Position)
	}
	if len(resp.Groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(resp.Groups))
	}
	// Admin response carries every configured app, including
	// disabled, so the Settings UI can edit them and a save
	// round-trip preserves them.
	if len(resp.Apps) != 3 {
		t.Errorf("expected 3 apps for admin (incl. disabled), got %d", len(resp.Apps))
	}
	// Keybindings should be nil when empty
	if resp.Keybindings != nil {
		t.Error("expected keybindings to be nil for empty bindings")
	}

	// Test with keybindings
	cfg.Keybindings = config.KeybindingsConfig{
		Bindings: map[string][]config.KeyCombo{
			"search": {{Key: "k", Ctrl: true}},
		},
	}
	resp = buildClientConfigResponse(cfg, "admin", nil)
	if resp.Keybindings == nil {
		t.Error("expected keybindings to be set")
	}
}

// session_cookie_domain is surfaced to the frontend so the Gateway
// tab can pre-warn when an operator ticks require_auth on a site
// while the cookie domain is unset. Empty must remain empty (omitted
// from the JSON payload via the omitempty tag); a configured value
// must round-trip verbatim.
func TestBuildClientConfigResponse_SessionCookieDomain(t *testing.T) {
	cfg := createTestConfig()

	// Default: unset.
	resp := buildClientConfigResponse(cfg, "admin", nil)
	if resp.SessionCookieDomain != "" {
		t.Errorf("expected empty SessionCookieDomain by default, got %q", resp.SessionCookieDomain)
	}

	// Explicit value flows through.
	cfg.Server.SessionCookieDomain = ".example.com"
	resp = buildClientConfigResponse(cfg, "admin", nil)
	if resp.SessionCookieDomain != ".example.com" {
		t.Errorf("expected SessionCookieDomain '.example.com', got %q", resp.SessionCookieDomain)
	}
}

// TestSaveConfig_RoundTripPreservesDisabledAppsForAdmin pins the
// contract that GET-edit-PUT round-trips through /api/config (the
// pattern used by the GatewayTab cookie-scope editor, the
// onboarding wizard, and any future "load config, edit one field,
// save the whole thing" frontend code) don't silently destroy
// apps that were disabled.
//
// Background: sanitizeApps used to filter ALL disabled apps from
// the response regardless of caller. An admin loading the config,
// editing an unrelated field, and saving back would persist the
// filtered (smaller) apps slice - effectively deleting every
// disabled app on disk. The fix: admins receive disabled apps in
// the response so they round-trip cleanly; non-admins still see
// only enabled apps in their responses.
func TestSaveConfig_RoundTripPreservesDisabledAppsForAdmin(t *testing.T) {
	cfg := createTestConfig()
	// Find an already-disabled app in the fixture; if none exist,
	// disable one explicitly so the test exercises the path.
	var hadDisabled bool
	for i := range cfg.Apps {
		if !cfg.Apps[i].Enabled {
			hadDisabled = true
			break
		}
	}
	if !hadDisabled {
		cfg.Apps[0].Enabled = false
	}
	disabledNames := make(map[string]bool)
	for i := range cfg.Apps {
		if !cfg.Apps[i].Enabled {
			disabledNames[cfg.Apps[i].Name] = true
		}
	}
	if len(disabledNames) == 0 {
		t.Fatal("test setup error: expected at least one disabled app")
	}

	// Admin response must include disabled apps so the round-trip
	// can preserve them.
	resp := buildClientConfigResponse(cfg, auth.RoleAdmin, nil)
	gotDisabled := 0
	for _, a := range resp.Apps {
		if disabledNames[a.Name] {
			gotDisabled++
		}
	}
	if gotDisabled != len(disabledNames) {
		t.Fatalf("admin GET /api/config returned %d/%d disabled apps - round-trip would destroy the missing %d",
			gotDisabled, len(disabledNames), len(disabledNames)-gotDisabled)
	}

	// Symmetry check: non-admin responses still filter disabled
	// apps. The "disabled = hidden from nav" semantic for non-admins
	// is preserved.
	respUser := buildClientConfigResponse(cfg, auth.RoleUser, nil)
	for _, a := range respUser.Apps {
		if disabledNames[a.Name] {
			t.Errorf("non-admin response leaked disabled app %q", a.Name)
		}
	}
}

// TestSaveConfig_PreservesSessionCookieDomainThroughRoundTrip pins
// the contract that the standard "load config, edit a field, save
// the whole thing" frontend path doesn't accidentally clobber
// session_cookie_domain. The whole 3.1.0 auth-gate UX depends on
// this: an operator sets `.example.com` via the inline editor on
// the Gateway tab, later edits something unrelated (a theme, an
// app), and the cookie scope must survive that save.
//
// The risk model: mergeConfigUpdate writes the field
// unconditionally. If a frontend somehow sends a PUT body without
// the field, JSON unmarshal leaves it as "" (Go zero value) and we
// overwrite the configured value. So the practical guarantee is
// "any PUT that round-trips a buildClientConfigResponse must
// preserve the field." This test pins exactly that round-trip.
func TestSaveConfig_PreservesSessionCookieDomainThroughRoundTrip(t *testing.T) {
	cfg := createTestConfig()
	cfg.Server.SessionCookieDomain = ".example.com"

	// 1. Build the response shape the frontend GETs.
	resp := buildClientConfigResponse(cfg, "admin", nil)

	// 2. Marshal to JSON, then unmarshal into ClientConfigUpdate.
	// This is what happens on the wire: server emits JSON, frontend
	// edits the object, frontend PUTs the JSON back.
	wire, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal client config: %v", err)
	}
	var update ClientConfigUpdate
	if err := json.Unmarshal(wire, &update); err != nil {
		t.Fatalf("unmarshal client config update: %v", err)
	}
	if update.SessionCookieDomain != ".example.com" {
		t.Errorf("after JSON round-trip, update.SessionCookieDomain = %q, want %q",
			update.SessionCookieDomain, ".example.com")
	}

	// 3. Apply the update via the merge function the SaveConfig
	// handler uses. The original cfg's value must survive.
	mergeConfigUpdate(cfg, &update, nil)
	if cfg.Server.SessionCookieDomain != ".example.com" {
		t.Errorf("after mergeConfigUpdate, cfg.Server.SessionCookieDomain = %q, want %q",
			cfg.Server.SessionCookieDomain, ".example.com")
	}
}

// TestSaveConfig_RejectsInvalidSessionCookieDomain pins the contract
// that SaveConfig refuses to persist a config the next Load would
// reject. The risk model: a frontend (or a hand-crafted PUT) sets
// session_cookie_domain to a value that doesn't cover a configured
// gated gateway site. Without the API-side Validate call, the file
// would persist and the next process start would fail to load,
// leaving the operator with a half-running deployment that needs
// hand-editing of config.yaml to recover. With validation in place
// the request returns 400 and the in-memory state is rolled back.
func TestSaveConfig_RejectsInvalidSessionCookieDomain(t *testing.T) {
	cfg := createTestConfig()
	cfg.Server.SessionCookieDomain = ".example.com"
	cfg.Server.GatewaySites = []config.GatewaySite{
		{Domain: "sonarr.example.com", BackendURL: "http://10.0.0.1:8989", RequireAuth: true},
	}
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	handler := NewAPIHandler(cfg, tmpFile.Name(), &sync.RWMutex{})

	// Build a valid-looking response, then mutate the cookie domain
	// to something that no longer covers the gated site.
	resp := buildClientConfigResponse(cfg, "admin", nil)
	wire, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var update ClientConfigUpdate
	if err := json.Unmarshal(wire, &update); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	update.SessionCookieDomain = ".other-tld.com"
	update.Title = "Should-Not-Persist"
	body, _ := json.Marshal(update)

	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.SaveConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from validation, got %d: %s", w.Code, w.Body.String())
	}

	// In-memory state must be rolled back: title untouched and the
	// session_cookie_domain still the pre-request value.
	if cfg.Server.Title != "Test Dashboard" {
		t.Errorf("title not rolled back: got %q", cfg.Server.Title)
	}
	if cfg.Server.SessionCookieDomain != ".example.com" {
		t.Errorf("session_cookie_domain not rolled back: got %q", cfg.Server.SessionCookieDomain)
	}

	// The temp file must remain empty (no write happened).
	info, err := os.Stat(tmpFile.Name())
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("config file written despite validation failure (size=%d)", info.Size())
	}
}

func TestSaveConfigSuccess(t *testing.T) {
	cfg := createTestConfig()
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	handler := NewAPIHandler(cfg, tmpFile.Name(), &sync.RWMutex{})

	update := ClientConfigUpdate{
		Title: "Updated Title",
		Navigation: config.NavigationConfig{
			Position: "top",
		},
		Groups: cfg.Groups,
		Apps: []ClientAppConfig{
			{
				Name:    "App1",
				URL:     "http://localhost:8080",
				Enabled: true,
			},
		},
	}
	body, _ := json.Marshal(update)

	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.SaveConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp clientConfigResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Title != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got %q", resp.Title)
	}
}

func TestSanitizeApp(t *testing.T) {
	t.Run("non-proxied app", func(t *testing.T) {
		app := config.AppConfig{
			Name:    "Test",
			URL:     "http://localhost:8080",
			Enabled: true,
			Proxy:   false,
		}

		result := sanitizeApp(&app)

		if result.ProxyURL != "" {
			t.Errorf("expected empty proxyUrl for non-proxied app, got %q", result.ProxyURL)
		}
		if result.URL != "http://localhost:8080" {
			t.Errorf("expected URL 'http://localhost:8080', got %q", result.URL)
		}
	})

	t.Run("proxied app", func(t *testing.T) {
		app := config.AppConfig{
			Name:    "Test App",
			URL:     "http://internal:9090",
			Enabled: true,
			Proxy:   true,
		}

		result := sanitizeApp(&app)

		if result.ProxyURL != "/proxy/test-app/" {
			t.Errorf("expected proxyUrl '/proxy/test-app/', got %q", result.ProxyURL)
		}
	})

	// Regression: the http_action fields must survive the AppConfig ->
	// ClientAppConfig projection, or the frontend never learns an app
	// needs a confirmation prompt / silent fire / non-default method.
	// (The fields were originally added to AppConfig and the TS type but
	// not to the DTO bridge, so the whole frontend integration silently
	// no-op'd while every unit test still passed.)
	t.Run("http_action fields project to the client DTO", func(t *testing.T) {
		showToast := false
		app := config.AppConfig{
			Name:                "Webhook",
			URL:                 "https://n8n.local/hook",
			Enabled:             true,
			OpenMode:            "http_action",
			HTTPActionMethod:    "PUT",
			HTTPActionConfirm:   true,
			HTTPActionShowToast: &showToast,
			HTTPActionHeaders:   map[string]string{"Authorization": "Bearer secret"},
		}
		// sanitizeApp is the admin projection.
		result := sanitizeApp(&app)
		if result.OpenMode != "http_action" {
			t.Errorf("OpenMode = %q, want http_action", result.OpenMode)
		}
		if result.HTTPActionMethod != "PUT" {
			t.Errorf("HTTPActionMethod = %q, want PUT", result.HTTPActionMethod)
		}
		if !result.HTTPActionConfirm {
			t.Error("HTTPActionConfirm lost in projection")
		}
		if result.HTTPActionShowToast == nil || *result.HTTPActionShowToast != false {
			t.Errorf("HTTPActionShowToast lost: %v", result.HTTPActionShowToast)
		}
		if result.HTTPActionHeaders["Authorization"] != "Bearer secret" {
			t.Error("admin projection should include http_action_headers")
		}
	})

	// Security: http_action_headers can carry bearer tokens, so the
	// non-admin projection must omit them (mirrors proxy_headers). The
	// non-sensitive method/confirm/show_toast fields still pass through
	// so a non-admin user gets the correct confirm prompt + toast.
	t.Run("http_action_headers hidden from non-admins", func(t *testing.T) {
		app := config.AppConfig{
			Name:              "Webhook",
			URL:               "https://n8n.local/hook",
			Enabled:           true,
			OpenMode:          "http_action",
			HTTPActionMethod:  "POST",
			HTTPActionConfirm: true,
			HTTPActionHeaders: map[string]string{"Authorization": "Bearer secret"},
		}
		result := sanitizeAppForRole(&app, false)
		if len(result.HTTPActionHeaders) != 0 {
			t.Errorf("non-admin projection leaked http_action_headers: %v", result.HTTPActionHeaders)
		}
		if !result.HTTPActionConfirm {
			t.Error("non-admin should still see http_action_confirm (drives the prompt)")
		}
		if result.HTTPActionMethod != "POST" {
			t.Errorf("non-admin should still see method, got %q", result.HTTPActionMethod)
		}
	})

	// Regression: the write path (client DTO -> AppConfig) must also
	// carry the fields, or saving an http_action app through the UI
	// would drop them on the floor.
	t.Run("http_action fields survive clientAppToConfig", func(t *testing.T) {
		showToast := false
		client := &ClientAppConfig{
			Name:                "Webhook",
			URL:                 "https://n8n.local/hook",
			OpenMode:            "http_action",
			HTTPActionMethod:    "DELETE",
			HTTPActionConfirm:   true,
			HTTPActionShowToast: &showToast,
			HTTPActionHeaders:   map[string]string{"X-Token": "abc"},
		}
		out := clientAppToConfig(client)
		if out.HTTPActionMethod != "DELETE" || !out.HTTPActionConfirm ||
			out.HTTPActionShowToast == nil || *out.HTTPActionShowToast != false ||
			out.HTTPActionHeaders["X-Token"] != "abc" {
			t.Errorf("clientAppToConfig dropped http_action fields: %+v", out)
		}
	})
}

// TestPinned_SurvivesBothProjections guards the pinned flag through the read
// projection (config -> client DTO, for admins and non-admins alike, since a
// pinned app is a navigation fact rather than a privileged one) and the write
// path (client DTO -> config). Either side dropping it would make the "Pin to
// bar" checkbox appear to do nothing.
func TestPinned_SurvivesBothProjections(t *testing.T) {
	app := config.AppConfig{Name: "Plex", URL: "http://plex.local", Enabled: true, Pinned: true}
	for _, admin := range []bool{true, false} {
		if got := sanitizeAppForRole(&app, admin); !got.Pinned {
			t.Errorf("admin=%v: sanitizeAppForRole dropped pinned", admin)
		}
	}
	unpinned := config.AppConfig{Name: "Radarr", URL: "http://radarr.local", Enabled: true}
	if got := sanitizeAppForRole(&unpinned, true); got.Pinned {
		t.Error("sanitizeAppForRole invented pinned on an unpinned app")
	}
	client := &ClientAppConfig{Name: "Plex", URL: "http://plex.local", Pinned: true}
	if out := clientAppToConfig(client); !out.Pinned {
		t.Errorf("clientAppToConfig dropped pinned: %+v", out)
	}
}

// Tests for save-failure error paths using an invalid configPath.
// /dev/null/impossible is guaranteed to fail because /dev/null is a file, not a directory.
// TestCreateApp_RejectsInvalidHTTPAction guards that the per-app create
// path validates an http_action app before persisting. Without this,
// clientAppToConfig + Save writes a structurally invalid action to disk
// (Save does not validate), and the next boot's Load validation rejects
// the whole config -- turning a bad single-app edit into a startup
// failure. The handler must reject it up front with a 400.
func TestCreateApp_RejectsInvalidHTTPAction(t *testing.T) {
	cfg := createTestConfig()
	tmpFile, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewAPIHandler(cfg, tmpFile.Name(), &sync.RWMutex{})
	before := len(cfg.Apps)

	body, _ := json.Marshal(ClientAppConfig{
		Name:             "BadWebhook",
		URL:              "https://hook.local/fire",
		OpenMode:         "http_action",
		HTTPActionMethod: "TRACE", // not in the allowed verb set
		Enabled:          true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/apps", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.CreateApp(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid http_action, got %d: %s", w.Code, w.Body.String())
	}
	if len(cfg.Apps) != before {
		t.Errorf("invalid app must not be added: apps went from %d to %d", before, len(cfg.Apps))
	}
}

// TestUpdateApp_RejectsInvalidHTTPAction is the update-path counterpart:
// an edit that turns an app into an invalid http_action must be rejected
// and must not mutate the stored app.
func TestUpdateApp_RejectsInvalidHTTPAction(t *testing.T) {
	cfg := createTestConfig()
	tmpFile, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewAPIHandler(cfg, tmpFile.Name(), &sync.RWMutex{})
	origURL := cfg.Apps[0].URL

	body, _ := json.Marshal(ClientAppConfig{
		Name:             cfg.Apps[0].Name,
		URL:              "", // http_action requires a url
		OpenMode:         "http_action",
		HTTPActionMethod: "POST",
		Enabled:          true,
	})
	req := httptest.NewRequest(http.MethodPut, "/api/apps/"+cfg.Apps[0].Name, bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.UpdateApp(w, req, cfg.Apps[0].Name)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid http_action, got %d: %s", w.Code, w.Body.String())
	}
	if cfg.Apps[0].URL != origURL {
		t.Errorf("stored app must be unchanged on rejected update: URL is now %q", cfg.Apps[0].URL)
	}
}

// TestCreateApp_RejectsSlugCollision guards that two enabled apps cannot map
// to the same /proxy/<slug>/ route: "Radarr" and "radarr" both slugify to
// "radarr", which would silently overwrite one route (last-write-wins).
func TestCreateApp_RejectsSlugCollision(t *testing.T) {
	cfg := &config.Config{Apps: []config.AppConfig{{Name: "Radarr", URL: "http://radarr:7878", Enabled: true, Proxy: true}}}
	tmpFile, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewAPIHandler(cfg, tmpFile.Name(), &sync.RWMutex{})
	before := len(cfg.Apps)

	body, _ := json.Marshal(ClientAppConfig{
		Name:    "radarr", // distinct name, same slug as "Radarr"
		URL:     "http://other:1",
		Enabled: true,
		Proxy:   true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/apps", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.CreateApp(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for a slug collision, got %d: %s", w.Code, w.Body.String())
	}
	if len(cfg.Apps) != before {
		t.Errorf("colliding app must not be added: apps went from %d to %d", before, len(cfg.Apps))
	}
}

func TestCreateApp_DropsDockerTracking(t *testing.T) {
	cfg := &config.Config{}
	tmpFile, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewAPIHandler(cfg, tmpFile.Name(), &sync.RWMutex{})
	body := `{"name":"Forged","url":"http://x:1","enabled":true,"docker_key":"name:victim",` +
		`"docker_endpoint":"unix:///var/run/docker.sock","docker_strategy":"host_port","docker_managed_url":"http://x:1"}`
	w := httptest.NewRecorder()
	handler.CreateApp(w, httptest.NewRequest(http.MethodPost, "/api/apps", strings.NewReader(body)))
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	a := cfg.Apps[0]
	if a.DockerKey != "" || a.DockerEndpoint != "" || a.DockerStrategy != "" || a.DockerManagedURL != "" {
		t.Errorf("tracking accepted from the payload: %+v", a)
	}
}

func TestCreateAppSaveFails(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "/dev/null/impossible/config.yaml", &sync.RWMutex{})

	body, _ := json.Marshal(ClientAppConfig{
		Name:    "NewApp",
		URL:     "http://localhost:9999",
		Enabled: true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/apps", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.CreateApp(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestUpdateAppSaveFails(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "/dev/null/impossible/config.yaml", &sync.RWMutex{})

	body, _ := json.Marshal(ClientAppConfig{
		Name:    "App1",
		URL:     "http://localhost:9999",
		Enabled: true,
	})
	req := httptest.NewRequest(http.MethodPut, "/api/apps/App1", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.UpdateApp(w, req, "App1")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestDeleteAppSaveFails(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "/dev/null/impossible/config.yaml", &sync.RWMutex{})

	req := httptest.NewRequest(http.MethodDelete, "/api/apps/App1", nil)
	w := httptest.NewRecorder()
	handler.DeleteApp(w, req, "App1")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestCreateGroupSaveFails(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "/dev/null/impossible/config.yaml", &sync.RWMutex{})

	body, _ := json.Marshal(config.GroupConfig{Name: "NewGroup", Color: "#aabbcc"})
	req := httptest.NewRequest(http.MethodPost, "/api/groups", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.CreateGroup(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestUpdateGroupSaveFails(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "/dev/null/impossible/config.yaml", &sync.RWMutex{})

	body, _ := json.Marshal(config.GroupConfig{Name: "Media", Color: "#112233"})
	req := httptest.NewRequest(http.MethodPut, "/api/groups/Media", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.UpdateGroup(w, req, "Media")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestDeleteGroupSaveFails(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "/dev/null/impossible/config.yaml", &sync.RWMutex{})

	req := httptest.NewRequest(http.MethodDelete, "/api/groups/Media", nil)
	w := httptest.NewRecorder()
	handler.DeleteGroup(w, req, "Media")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestSaveConfigSaveFails(t *testing.T) {
	cfg := createTestConfig()
	priorTitle := cfg.Server.Title
	priorAppCount := len(cfg.Apps)
	handler := NewAPIHandler(cfg, "/dev/null/impossible/config.yaml", &sync.RWMutex{})

	update := ClientConfigUpdate{
		Title:      "Updated",
		Navigation: cfg.Navigation,
		Groups:     cfg.Groups,
		Apps:       []ClientAppConfig{{Name: "App1", URL: "http://localhost:8080", Enabled: true}},
	}
	body, _ := json.Marshal(update)
	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.SaveConfig(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
	// Regression for codebase review C1-shf: in-memory state must be
	// rolled back when Save fails, otherwise next GET returns the new
	// shape while disk still holds the old one.
	if cfg.Server.Title != priorTitle {
		t.Errorf("Server.Title not rolled back: got %q, want %q", cfg.Server.Title, priorTitle)
	}
	if len(cfg.Apps) != priorAppCount {
		t.Errorf("Apps length not rolled back: got %d, want %d", len(cfg.Apps), priorAppCount)
	}
}

// TestSaveConfig_PreservesDockerTrackingFields covers the codebase
// review fix #1 from the docker-discovery plan: when the frontend
// sends a SaveConfig payload that omits the docker_key field, we
// must NOT clear the existing tracking. Otherwise a stale frontend
// (or a scripted PUT) silently detaches the app from auto-management.
func TestSaveConfig_PreservesDockerTrackingFields(t *testing.T) {
	cfg := createTestConfig()
	// Mark App1 as docker-tracked.
	cfg.Apps[0].DockerKey = "name:sonarr"
	cfg.Apps[0].DockerEndpoint = "unix:///var/run/docker.sock"
	cfg.Apps[0].DockerStrategy = "container_ip"

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := cfg.Save(configPath); err != nil {
		t.Fatal(err)
	}
	handler := NewAPIHandler(cfg, configPath, &sync.RWMutex{})

	// Send a SaveConfig payload that's missing docker_key entirely on App1.
	// The bulk-merge code path must restore it from the existing entry.
	update := ClientConfigUpdate{
		Title:      "Same",
		Navigation: cfg.Navigation,
		Groups:     cfg.Groups,
		Apps: []ClientAppConfig{
			{Name: "App1", URL: "http://localhost:8080", Group: "Media", Enabled: true},
			{Name: "App2", URL: "http://localhost:8081", Group: "Tools", Enabled: true, Proxy: true},
			{Name: "DisabledApp", URL: "http://localhost:8082", Group: "Media"},
		},
	}
	body, _ := json.Marshal(update)
	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.SaveConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}

	// App1's docker tracking must survive the bulk save.
	if cfg.Apps[0].DockerKey != "name:sonarr" {
		t.Errorf("DockerKey was cleared: %+v", cfg.Apps[0])
	}
	if cfg.Apps[0].DockerEndpoint != "unix:///var/run/docker.sock" {
		t.Errorf("DockerEndpoint was cleared: %+v", cfg.Apps[0])
	}
	if cfg.Apps[0].DockerStrategy != "container_ip" {
		t.Errorf("DockerStrategy was cleared: %+v", cfg.Apps[0])
	}
}

// TestUpdateApp_PreservesDockerTrackingFieldsOnEmptyPayload covers
// the per-app PUT path - the previously-untested twin of
// TestSaveConfig_PreservesDockerTrackingFields. Without this guard,
// the AppForm in the SPA (which has no docker_key input on its
// edit form) would send a PUT that wipes tracking on every cosmetic
// edit (color, icon, group). The fix in applyDockerTrackingPreserva-
// tion checks updated.DockerKey == "" and copies the existing
// tracking back; this test pins both that the helper runs AND that
// it runs on the per-app PUT route specifically.
func TestUpdateApp_PreservesDockerTrackingFieldsOnEmptyPayload(t *testing.T) {
	cfg := createTestConfig()
	cfg.Apps[0].DockerKey = "name:sonarr"
	cfg.Apps[0].DockerEndpoint = "unix:///var/run/docker.sock"
	cfg.Apps[0].DockerStrategy = "container_ip"

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := cfg.Save(configPath); err != nil {
		t.Fatal(err)
	}
	handler := NewAPIHandler(cfg, configPath, &sync.RWMutex{})

	// Payload omits the three docker_* fields entirely - exactly
	// what AppForm sends today when the operator edits the icon or
	// color but not the URL.
	body, _ := json.Marshal(ClientAppConfig{
		Name:    "App1",
		URL:     cfg.Apps[0].URL, // same URL -> no auto-detach
		Color:   "#new-color",
		Enabled: true,
	})
	req := httptest.NewRequest(http.MethodPut, "/api/app/App1", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.UpdateApp(w, req, "App1")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}
	if cfg.Apps[0].DockerKey != "name:sonarr" {
		t.Errorf("DockerKey was wiped by per-app PUT: %+v", cfg.Apps[0])
	}
	if cfg.Apps[0].DockerEndpoint != "unix:///var/run/docker.sock" {
		t.Errorf("DockerEndpoint was wiped: %+v", cfg.Apps[0])
	}
	if cfg.Apps[0].DockerStrategy != "container_ip" {
		t.Errorf("DockerStrategy was wiped: %+v", cfg.Apps[0])
	}
	if cfg.Apps[0].Color != "#new-color" {
		t.Errorf("Color edit was not applied: %+v", cfg.Apps[0])
	}
}

// TestSaveConfig_AutoDetachesOnURLChange covers the plan v4
// "Manual URL edit on a docker-tracked app via SaveConfig is rejected
// or auto-detaches (pick: auto-detach, document)" line. When the
// payload echoes back the docker_key but with a NEW URL, the merge
// must clear all three tracking fields so the next refresh tick
// doesn't clobber the operator's manual edit.
func TestSaveConfig_AutoDetachesOnURLChange(t *testing.T) {
	cfg := createTestConfig()
	cfg.Apps[0].URL = "http://10.0.0.1:80"
	cfg.Apps[0].DockerKey = "label:sonarr-stable"
	cfg.Apps[0].DockerEndpoint = "unix:///var/run/docker.sock"
	cfg.Apps[0].DockerStrategy = "container_ip"

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := cfg.Save(configPath); err != nil {
		t.Fatal(err)
	}
	handler := NewAPIHandler(cfg, configPath, &sync.RWMutex{})

	update := ClientConfigUpdate{
		Title:      "Same",
		Navigation: cfg.Navigation,
		Groups:     cfg.Groups,
		Apps: []ClientAppConfig{
			{Name: "App1", URL: "http://manual:9999", Group: "Media", Enabled: true,
				DockerKey: "label:sonarr-stable", DockerEndpoint: "unix:///var/run/docker.sock", DockerStrategy: "container_ip"},
			{Name: "App2", URL: "http://localhost:8081", Group: "Tools", Enabled: true, Proxy: true},
			{Name: "DisabledApp", URL: "http://localhost:8082", Group: "Media"},
		},
	}
	body, _ := json.Marshal(update)
	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.SaveConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}

	if cfg.Apps[0].URL != "http://manual:9999" {
		t.Errorf("expected URL to be updated to manual value, got %q", cfg.Apps[0].URL)
	}
	if cfg.Apps[0].DockerKey != "" || cfg.Apps[0].DockerEndpoint != "" || cfg.Apps[0].DockerStrategy != "" {
		t.Errorf("expected auto-detach to clear tracking fields, got %+v", cfg.Apps[0])
	}
}

// TestUpdateApp_AutoDetachesOnURLChange covers the same auto-detach
// rule for the per-app PUT path. UpdateApp doesn't go through
// mergeClientApp; both paths must apply the same rule via
// applyDockerTrackingPreservation so the operator gets consistent
// behavior regardless of which endpoint the frontend uses.
func TestUpdateApp_AutoDetachesOnURLChange(t *testing.T) {
	cfg := createTestConfig()
	cfg.Apps[0].URL = "http://10.0.0.1:80"
	cfg.Apps[0].DockerKey = "label:sonarr-stable"
	cfg.Apps[0].DockerEndpoint = "unix:///var/run/docker.sock"
	cfg.Apps[0].DockerStrategy = "container_ip"

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := cfg.Save(configPath); err != nil {
		t.Fatal(err)
	}
	handler := NewAPIHandler(cfg, configPath, &sync.RWMutex{})

	body, _ := json.Marshal(ClientAppConfig{
		Name: "App1", URL: "http://manual:9999",
		DockerKey: "label:sonarr-stable", DockerEndpoint: "unix:///var/run/docker.sock", DockerStrategy: "container_ip",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/app/App1", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.UpdateApp(w, req, "App1")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}
	if cfg.Apps[0].URL != "http://manual:9999" {
		t.Errorf("URL not updated: %q", cfg.Apps[0].URL)
	}
	if cfg.Apps[0].DockerKey != "" {
		t.Errorf("expected DockerKey cleared by auto-detach; got %q", cfg.Apps[0].DockerKey)
	}
}

// TestSaveConfig_AutoDetachPersistsToDisk verifies the auto-detach
// outcome survives a config round-trip through disk. The existing
// in-memory tests would pass if a regression mutated cfg.Apps in
// memory but wrote the pre-mutation slice to YAML; this test fails
// that scenario because we re-Load from configPath after the request.
func TestSaveConfig_AutoDetachPersistsToDisk(t *testing.T) {
	cfg := createTestConfig()
	cfg.Apps[0].URL = "http://10.0.0.1:80"
	cfg.Apps[0].DockerKey = "label:sonarr-stable"
	cfg.Apps[0].DockerEndpoint = "unix:///var/run/docker.sock"
	cfg.Apps[0].DockerStrategy = "container_ip"

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := cfg.Save(configPath); err != nil {
		t.Fatal(err)
	}
	handler := NewAPIHandler(cfg, configPath, &sync.RWMutex{})

	update := ClientConfigUpdate{
		Title:      "Same",
		Navigation: cfg.Navigation,
		Groups:     cfg.Groups,
		Apps: []ClientAppConfig{
			{Name: "App1", URL: "http://manual:9999", Group: "Media", Enabled: true,
				DockerKey: "label:sonarr-stable", DockerEndpoint: "unix:///var/run/docker.sock", DockerStrategy: "container_ip"},
			{Name: "App2", URL: "http://localhost:8081", Group: "Tools", Enabled: true, Proxy: true},
			{Name: "DisabledApp", URL: "http://localhost:8082", Group: "Media"},
		},
	}
	body, _ := json.Marshal(update)
	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.SaveConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}

	// Re-load the YAML. Without this step a regression that mutates
	// in-memory but writes priorApps to disk would silently pass the
	// existing tests.
	reloaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Apps[0].URL != "http://manual:9999" {
		t.Errorf("URL not persisted; got %q", reloaded.Apps[0].URL)
	}
	if reloaded.Apps[0].DockerKey != "" {
		t.Errorf("auto-detach did not persist; on-disk DockerKey = %q", reloaded.Apps[0].DockerKey)
	}
}

// TestApplyDockerTrackingPreservation_RetainsTrackingOnNonURLEdits
// pins the rule that purely cosmetic edits (rename, icon swap) on
// a tracked app must NOT trigger auto-detach. Only a URL change
// is treated as "operator took manual control".
func TestApplyDockerTrackingPreservation_RetainsTrackingOnNonURLEdits(t *testing.T) {
	existing := config.AppConfig{
		Name: "App1", URL: "http://10.0.0.1:80",
		DockerKey: "label:foo", DockerEndpoint: "unix:///x.sock", DockerStrategy: "container_ip",
	}
	updated := config.AppConfig{
		Name: "App1", URL: "http://10.0.0.1:80", // same URL
		DockerKey: "label:foo", DockerEndpoint: "unix:///x.sock", DockerStrategy: "container_ip",
		Color: "#abcdef", // cosmetic-only edit
	}
	reason := applyDockerTrackingPreservation(&updated, &existing)
	if reason != "" {
		t.Errorf("non-URL edit unexpectedly triggered detach with reason %q", reason)
	}
	if updated.DockerKey != "label:foo" {
		t.Errorf("DockerKey was cleared by non-URL edit: %+v", updated)
	}
}

// TestApplyDockerTrackingPreservation_DockerAutoImportedMarker pins the
// provenance marker's lifecycle across a config save. The marker must be
// preserved alongside the other tracking fields on an empty-payload PUT
// (DockerKey == ""), and cleared in lockstep with the tracking fields when
// a URL change auto-detaches the app. This keeps the save path consistent
// with the detach Load() already performs.
func TestApplyDockerTrackingPreservation_DockerAutoImportedMarker(t *testing.T) {
	t.Run("preserved on empty-payload PUT", func(t *testing.T) {
		existing := config.AppConfig{
			Name: "App1", URL: "http://10.0.0.1:80",
			DockerKey: "label:foo", DockerEndpoint: "unix:///x.sock", DockerStrategy: "container_ip",
			DockerAutoImported: true,
		}
		updated := config.AppConfig{
			Name: "App1", URL: "http://10.0.0.1:80",
			// empty DockerKey signals a payload without tracking fields
		}
		reason := applyDockerTrackingPreservation(&updated, &existing)
		if reason != "" {
			t.Errorf("empty-payload PUT unexpectedly detached with reason %q", reason)
		}
		if !updated.DockerAutoImported {
			t.Errorf("DockerAutoImported not preserved from existing: %+v", updated)
		}
	})

	t.Run("cleared on URL-change auto-detach", func(t *testing.T) {
		existing := config.AppConfig{
			Name: "App1", URL: "http://10.0.0.1:80",
			DockerKey: "label:foo", DockerEndpoint: "unix:///x.sock", DockerStrategy: "container_ip",
			DockerAutoImported: true,
		}
		updated := config.AppConfig{
			Name: "App1", URL: "http://manual:9999", // URL changed -> manual control
			DockerKey: "label:foo", DockerEndpoint: "unix:///x.sock", DockerStrategy: "container_ip",
			DockerAutoImported: true,
		}
		reason := applyDockerTrackingPreservation(&updated, &existing)
		if reason == "" {
			t.Fatal("URL change on a tracked app should have detached")
		}
		if updated.DockerAutoImported {
			t.Errorf("DockerAutoImported not cleared by auto-detach: %+v", updated)
		}
	})

	t.Run("preserved on non-URL UI save", func(t *testing.T) {
		existing := config.AppConfig{
			Name: "App1", URL: "http://10.0.0.1:80",
			DockerKey: "label:foo", DockerEndpoint: "unix:///x.sock", DockerStrategy: "container_ip",
			DockerAutoImported: true,
		}
		updated := config.AppConfig{
			Name: "App1", URL: "http://10.0.0.1:80", // same URL
			DockerKey: "label:foo", DockerEndpoint: "unix:///x.sock", DockerStrategy: "container_ip",
			// ClientAppConfig has no docker_auto field, so the wire form
			// arrives with DockerAutoImported == false on a normal UI save.
			DockerAutoImported: false,
		}
		reason := applyDockerTrackingPreservation(&updated, &existing)
		if reason != "" {
			t.Errorf("non-URL save unexpectedly detached with reason %q", reason)
		}
		if !updated.DockerAutoImported {
			t.Errorf("DockerAutoImported not preserved on non-URL save: %+v", updated)
		}
	})
}

func TestCreateAppMissingName(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

	body, _ := json.Marshal(ClientAppConfig{URL: "http://localhost:9999"})
	req := httptest.NewRequest(http.MethodPost, "/api/apps", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.CreateApp(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateAppDuplicate(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

	body, _ := json.Marshal(ClientAppConfig{Name: "App1", URL: "http://localhost:9999"})
	req := httptest.NewRequest(http.MethodPost, "/api/apps", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.CreateApp(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}
}

func TestUpdateAppNotFound(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

	body, _ := json.Marshal(ClientAppConfig{Name: "Ghost", URL: "http://localhost:9999"})
	req := httptest.NewRequest(http.MethodPut, "/api/apps/Ghost", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.UpdateApp(w, req, "Ghost")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestUpdateAppInvalidJSON(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

	req := httptest.NewRequest(http.MethodPut, "/api/apps/App1", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	handler.UpdateApp(w, req, "App1")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestDeleteAppNotFound(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

	req := httptest.NewRequest(http.MethodDelete, "/api/apps/Ghost", nil)
	w := httptest.NewRecorder()
	handler.DeleteApp(w, req, "Ghost")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetAppNotFound(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

	req := httptest.NewRequest(http.MethodGet, "/api/apps/Ghost", nil)
	w := httptest.NewRecorder()
	handler.GetApp(w, req, "Ghost")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetGroupNotFound(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

	req := httptest.NewRequest(http.MethodGet, "/api/groups/Ghost", nil)
	w := httptest.NewRecorder()
	handler.GetGroup(w, req, "Ghost")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestDeleteGroupNotFound(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

	req := httptest.NewRequest(http.MethodDelete, "/api/groups/Ghost", nil)
	w := httptest.NewRecorder()
	handler.DeleteGroup(w, req, "Ghost")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestUpdateGroupInvalidJSON(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

	req := httptest.NewRequest(http.MethodPut, "/api/groups/Media", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	handler.UpdateGroup(w, req, "Media")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestUpdateGroupNotFound(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

	body, _ := json.Marshal(config.GroupConfig{Name: "Ghost"})
	req := httptest.NewRequest(http.MethodPut, "/api/groups/Ghost", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.UpdateGroup(w, req, "Ghost")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestCreateGroupDuplicate(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

	body, _ := json.Marshal(config.GroupConfig{Name: "Media", Color: "#ff0000"})
	req := httptest.NewRequest(http.MethodPost, "/api/groups", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.CreateGroup(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}
}

// TestStripURLCredentials covers the URL sanitizer used by
// sanitizeAppForRole (findings.md H12).
func TestStripURLCredentials(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"http://example.com/", "http://example.com/"},
		{"https://user@example.com/", "https://example.com/"},
		{"https://user:token@example.com/path?x=1", "https://example.com/path?x=1"},
		{"", ""},
		{"not a url", "not a url"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.in, func(t *testing.T) {
			if got := stripURLCredentials(c.in); got != c.want {
				t.Errorf("stripURLCredentials(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestSanitizeAppsHidesCredentialsForNonAdmin covers findings.md H12:
// a non-admin reading /api/config must never see embedded URL
// credentials or per-app ProxyHeaders.
func TestSanitizeAppsHidesCredentialsForNonAdmin(t *testing.T) {
	apps := []config.AppConfig{
		{ //nolint:gosec // fixture URL, not a real credential
			Name:    "Secret",
			URL:     "https://adminuser:s3cret@example.com/path",
			Enabled: true,
			ProxyHeaders: map[string]string{
				"Authorization": "Bearer supersecret",
			},
		},
	}

	t.Run("admin keeps secrets", func(t *testing.T) {
		result := sanitizeApps(apps, "admin", nil, nil)
		if len(result) != 1 || !strings.Contains(result[0].URL, "adminuser:s3cret@") {
			t.Errorf("admin URL = %q, want credentials preserved", result[0].URL)
		}
		if result[0].ProxyHeaders["Authorization"] != "Bearer supersecret" {
			t.Error("admin should still receive ProxyHeaders")
		}
	})

	t.Run("non-admin loses credentials", func(t *testing.T) {
		result := sanitizeApps(apps, "user", nil, nil)
		if len(result) != 1 {
			t.Fatalf("expected 1 app, got %d", len(result))
		}
		if strings.Contains(result[0].URL, "adminuser") || strings.Contains(result[0].URL, "s3cret") {
			t.Errorf("non-admin URL leaked credentials: %q", result[0].URL)
		}
		if result[0].ProxyHeaders != nil {
			t.Errorf("non-admin should NOT receive ProxyHeaders: %v", result[0].ProxyHeaders)
		}
	})
}

func TestSanitizeApps(t *testing.T) {
	apps := []config.AppConfig{
		{Name: "Enabled1", URL: "http://a:8080", Enabled: true},
		{Name: "Disabled", URL: "http://b:8080", Enabled: false},
		{Name: "Enabled2", URL: "http://c:8080", Enabled: true, Proxy: true},
	}

	// Admin sees every configured app, including disabled, so the
	// Settings UI can manage them and a save round-trip preserves
	// them. The nav-bar itself filters disabled apps on the client.
	t.Run("admin sees disabled apps", func(t *testing.T) {
		result := sanitizeApps(apps, "admin", nil, nil)
		if len(result) != 3 {
			t.Fatalf("expected 3 apps for admin (incl. disabled), got %d", len(result))
		}
		found := false
		for _, app := range result {
			if app.Name == "Disabled" {
				found = true
				if app.Enabled {
					t.Error("disabled app should retain enabled=false")
				}
			}
		}
		if !found {
			t.Error("admin response missing disabled app - round-trip save would destroy it")
		}
	})

	// Non-admin roles still get the "disabled = hidden" semantic.
	t.Run("non-admin filters disabled apps", func(t *testing.T) {
		result := sanitizeApps(apps, "user", nil, nil)
		if len(result) != 2 {
			t.Errorf("expected 2 enabled apps for user, got %d", len(result))
		}
		for _, app := range result {
			if app.Name == "Disabled" {
				t.Error("non-admin response leaked disabled app")
			}
		}
	})
}

func TestSanitizeAppsRoleFiltering(t *testing.T) {
	apps := []config.AppConfig{
		{Name: "Public", URL: "http://a:8080", Enabled: true, MinRole: ""},
		{Name: "PowerOnly", URL: "http://b:8080", Enabled: true, MinRole: "power-user"},
		{Name: "AdminOnly", URL: "http://c:8080", Enabled: true, MinRole: "admin"},
	}

	t.Run("admin sees all", func(t *testing.T) {
		result := sanitizeApps(apps, "admin", nil, nil)
		if len(result) != 3 {
			t.Errorf("admin should see 3 apps, got %d", len(result))
		}
	})

	t.Run("power-user sees public and power-user", func(t *testing.T) {
		result := sanitizeApps(apps, "power-user", nil, nil)
		if len(result) != 2 {
			t.Errorf("power-user should see 2 apps, got %d", len(result))
		}
		for _, app := range result {
			if app.Name == "AdminOnly" {
				t.Error("power-user should not see admin-only app")
			}
		}
	})

	t.Run("user sees only public", func(t *testing.T) {
		result := sanitizeApps(apps, "user", nil, nil)
		if len(result) != 1 {
			t.Errorf("user should see 1 app, got %d", len(result))
		}
		if result[0].Name != "Public" {
			t.Errorf("expected 'Public', got %q", result[0].Name)
		}
	})

	t.Run("empty role disables filtering", func(t *testing.T) {
		result := sanitizeApps(apps, "", nil, nil)
		if len(result) != 3 {
			t.Errorf("empty role should see all 3 apps, got %d", len(result))
		}
	})
}

func TestSanitizeAppsGroupFiltering(t *testing.T) {
	apps := []config.AppConfig{
		{Name: "Open", URL: "http://a:8080", Enabled: true},
		{Name: "DevsOnly", URL: "http://b:8080", Enabled: true, AllowedGroups: []string{"developers"}},
		{Name: "OnCall", URL: "http://c:8080", Enabled: true, AllowedGroups: []string{"sre", "on-call"}},
		{Name: "AdminAndDevs", URL: "http://d:8080", Enabled: true, MinRole: "admin", AllowedGroups: []string{"developers"}},
	}

	t.Run("user with no groups only sees open apps", func(t *testing.T) {
		result := sanitizeApps(apps, "user", nil, nil)
		if len(result) != 1 {
			t.Errorf("user with no groups should see 1 app (Open), got %d: %+v", len(result), names(result))
		}
		if result[0].Name != "Open" {
			t.Errorf("expected 'Open', got %q", result[0].Name)
		}
	})

	t.Run("user in developers group sees Open and DevsOnly", func(t *testing.T) {
		result := sanitizeApps(apps, "user", []string{"developers"}, nil)
		if len(result) != 2 {
			t.Errorf("expected 2 apps, got %d: %+v", len(result), names(result))
		}
	})

	t.Run("group matching is case-insensitive", func(t *testing.T) {
		// User group is uppercase, app's allowed_groups is lowercase.
		// Mirroring the OIDC admin-group case-insensitive comparison
		// keeps operators from being bitten by IdP casing variations.
		result := sanitizeApps(apps, "user", []string{"DEVELOPERS"}, nil)
		if len(result) != 2 {
			t.Errorf("case-insensitive match failed: got %d, want 2", len(result))
		}
	})

	t.Run("user in any of multiple allowed groups passes the gate", func(t *testing.T) {
		// OnCall app allows ["sre", "on-call"]; user in just "on-call".
		result := sanitizeApps(apps, "user", []string{"on-call"}, nil)
		if len(result) != 2 {
			t.Errorf("expected Open + OnCall, got %+v", names(result))
		}
		var sawOnCall bool
		for _, a := range result {
			if a.Name == "OnCall" {
				sawOnCall = true
			}
		}
		if !sawOnCall {
			t.Error("OnCall app should be visible to user in 'on-call' group")
		}
	})

	t.Run("admin bypasses group gate", func(t *testing.T) {
		// Even with no group memberships, admin sees every app the
		// role allows — including ones with allowed_groups set.
		result := sanitizeApps(apps, "admin", nil, nil)
		if len(result) != 4 {
			t.Errorf("admin should see all 4 apps, got %d: %+v", len(result), names(result))
		}
	})

	t.Run("non-admin in matching group still gated by min_role", func(t *testing.T) {
		// AdminAndDevs requires admin role AND developers group. A
		// user in developers but not admin should not see it.
		result := sanitizeApps(apps, "user", []string{"developers"}, nil)
		for _, a := range result {
			if a.Name == "AdminAndDevs" {
				t.Error("user role should not pass min_role=admin gate even with matching group")
			}
		}
	})

	t.Run("empty role disables both gates", func(t *testing.T) {
		// Mirrors the existing role-only behaviour: unauth setup
		// previews see every app regardless of allowed_groups.
		result := sanitizeApps(apps, "", nil, nil)
		if len(result) != 4 {
			t.Errorf("expected all 4 apps for empty role, got %d", len(result))
		}
	})
}

func TestSanitizeApps_GatewayPairing(t *testing.T) {
	apps := []config.AppConfig{
		{Name: "Sonarr", URL: "http://sonarr:8989", Enabled: true},
		{Name: "Radarr", URL: "http://radarr:7878", Enabled: true},
		{Name: "Plex", URL: "http://plex:32400", Enabled: true},
	}

	t.Run("no gateway sites means no GatewayDomain on any app", func(t *testing.T) {
		got := sanitizeApps(apps, "admin", nil, nil)
		for _, c := range got {
			if c.GatewayDomain != "" {
				t.Errorf("app %q got unexpected GatewayDomain %q", c.Name, c.GatewayDomain)
			}
		}
	})

	t.Run("matching app_name surfaces GatewayDomain", func(t *testing.T) {
		sites := []config.GatewaySite{
			{Domain: "sonarr.example.com", BackendURL: "http://sonarr:8989", AppName: "Sonarr"},
			{Domain: "plex.example.com", BackendURL: "http://plex:32400", AppName: "Plex", Streaming: true},
		}
		got := sanitizeApps(apps, "admin", nil, sites)

		byName := map[string]string{}
		for _, c := range got {
			byName[c.Name] = c.GatewayDomain
		}
		if byName["Sonarr"] != "sonarr.example.com" {
			t.Errorf("Sonarr.GatewayDomain = %q, want sonarr.example.com", byName["Sonarr"])
		}
		if byName["Plex"] != "plex.example.com" {
			t.Errorf("Plex.GatewayDomain = %q, want plex.example.com", byName["Plex"])
		}
		// Radarr has no matching gateway site.
		if byName["Radarr"] != "" {
			t.Errorf("Radarr.GatewayDomain = %q, want empty", byName["Radarr"])
		}
	})

	t.Run("gateway site without app_name does not pair", func(t *testing.T) {
		sites := []config.GatewaySite{
			{Domain: "standalone.example.com", BackendURL: "http://app:80"}, // no AppName
		}
		got := sanitizeApps(apps, "admin", nil, sites)
		for _, c := range got {
			if c.GatewayDomain != "" {
				t.Errorf("app %q got unexpected GatewayDomain %q", c.Name, c.GatewayDomain)
			}
		}
	})

	t.Run("app_name pointing at a non-existent app is silently ignored", func(t *testing.T) {
		// gatewayDomainsByAppName populates the map regardless of
		// existence; it just never matches an app, so every app gets
		// empty GatewayDomain. This intentionally keeps the gateway
		// side authoritative without bidirectional validation.
		sites := []config.GatewaySite{
			{Domain: "ghost.example.com", BackendURL: "http://x:80", AppName: "DoesNotExist"},
		}
		got := sanitizeApps(apps, "admin", nil, sites)
		for _, c := range got {
			if c.GatewayDomain != "" {
				t.Errorf("app %q should not be paired, got %q", c.Name, c.GatewayDomain)
			}
		}
	})
}

func TestGatewayDomainsByAppName(t *testing.T) {
	t.Run("nil input returns nil map", func(t *testing.T) {
		if got := gatewayDomainsByAppName(nil); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("sites without AppName are skipped", func(t *testing.T) {
		got := gatewayDomainsByAppName([]config.GatewaySite{
			{Domain: "a.example.com"},
			{Domain: "b.example.com", AppName: "BApp"},
		})
		if len(got) != 1 || got["BApp"] != "b.example.com" {
			t.Errorf("got %v, want {BApp: b.example.com}", got)
		}
	})

	t.Run("duplicate AppName: last entry wins", func(t *testing.T) {
		// Operator misconfiguration; sanitizeApps still produces a
		// usable result (one of the two sites).
		got := gatewayDomainsByAppName([]config.GatewaySite{
			{Domain: "first.example.com", AppName: "Same"},
			{Domain: "second.example.com", AppName: "Same"},
		})
		if got["Same"] != "second.example.com" {
			t.Errorf("got %v, want last entry to win", got)
		}
	})
}

// names extracts app names from a sanitized result for nicer test
// output when length-only assertions fail. Indexes rather than ranging
// so gocritic doesn't flag the per-iteration struct copy.
func names(apps []ClientAppConfig) []string {
	out := make([]string, len(apps))
	for i := range apps {
		out[i] = apps[i].Name
	}
	return out
}

func TestUserInAnyAllowedGroup(t *testing.T) {
	cases := []struct {
		name       string
		userGroups []string
		allowed    []string
		want       bool
	}{
		{"empty user groups", nil, []string{"a"}, false},
		{"empty allowed list", []string{"a"}, nil, false},
		{"both empty", nil, nil, false},
		{"single match", []string{"developers"}, []string{"developers"}, true},
		{"case insensitive", []string{"Developers"}, []string{"DEVELOPERS"}, true},
		{"whitespace tolerant", []string{"  developers  "}, []string{"developers"}, true},
		{"no overlap", []string{"a", "b"}, []string{"c", "d"}, false},
		{"partial overlap", []string{"a", "b", "c"}, []string{"x", "b", "y"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := userInAnyAllowedGroup(c.userGroups, c.allowed); got != c.want {
				t.Errorf("userInAnyAllowedGroup(%v, %v) = %v, want %v", c.userGroups, c.allowed, got, c.want)
			}
		})
	}
}

func TestSetOnConfigSave(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

	if handler.onConfigSave != nil {
		t.Fatal("expected onConfigSave to be nil initially")
	}

	called := false
	handler.SetOnConfigSave(func() { called = true })

	if handler.onConfigSave == nil {
		t.Fatal("expected onConfigSave to be set after SetOnConfigSave")
	}

	handler.onConfigSave()
	if !called {
		t.Error("expected callback to be invoked")
	}
}

func TestNotifyConfigSaved(t *testing.T) {
	t.Run("with callback", func(t *testing.T) {
		cfg := createTestConfig()
		handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

		called := false
		handler.SetOnConfigSave(func() { called = true })
		handler.notifyConfigSaved()

		if !called {
			t.Error("expected notifyConfigSaved to invoke callback")
		}
	})

	t.Run("without callback", func(t *testing.T) {
		cfg := createTestConfig()
		handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

		// Should not panic when no callback is set
		handler.notifyConfigSaved()
	})
}

func TestExportConfig(t *testing.T) {
	cfg := createTestConfig()
	cfg.Auth.Method = "builtin"
	cfg.Auth.APIKeyHash = "$2a$10$somehashedvalue"
	cfg.Auth.OIDC.ClientSecret = "secret-oidc"
	cfg.Auth.Users = []config.UserConfig{
		{Username: "admin", PasswordHash: "$2a$10$hashvalue", Role: "admin"},
	}

	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

	req := httptest.NewRequest(http.MethodGet, "/api/config/export", nil)
	w := httptest.NewRecorder()

	handler.ExportConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check Content-Type and Content-Disposition headers
	ct := w.Header().Get("Content-Type")
	if ct != "application/x-yaml" {
		t.Errorf("expected Content-Type 'application/x-yaml', got %q", ct)
	}
	cd := w.Header().Get("Content-Disposition")
	if cd == "" {
		t.Error("expected Content-Disposition header to be set")
	}
	if !bytes.Contains([]byte(cd), []byte("muximux-config-")) {
		t.Errorf("expected Content-Disposition to contain 'muximux-config-', got %q", cd)
	}

	// Parse the exported YAML
	body := w.Body.Bytes()
	var exported config.Config
	if err := yaml.Unmarshal(body, &exported); err != nil {
		t.Fatalf("exported YAML is invalid: %v", err)
	}

	// Verify sensitive fields are stripped
	if exported.Auth.APIKeyHash != "" {
		t.Errorf("expected APIKeyHash to be stripped, got %q", exported.Auth.APIKeyHash)
	}
	if exported.Auth.OIDC.ClientSecret != "" {
		t.Errorf("expected OIDC ClientSecret to be stripped, got %q", exported.Auth.OIDC.ClientSecret)
	}
	for _, u := range exported.Auth.Users {
		if u.PasswordHash != "" {
			t.Errorf("expected PasswordHash to be stripped for user %q, got %q", u.Username, u.PasswordHash)
		}
	}

	// Verify non-sensitive fields are preserved
	if exported.Auth.Method != "builtin" {
		t.Errorf("expected auth method 'builtin', got %q", exported.Auth.Method)
	}
	if len(exported.Auth.Users) != 1 {
		t.Errorf("expected 1 user, got %d", len(exported.Auth.Users))
	}
	if exported.Auth.Users[0].Username != "admin" {
		t.Errorf("expected username 'admin', got %q", exported.Auth.Users[0].Username)
	}

	// Verify that the original config is NOT mutated
	if cfg.Auth.APIKeyHash != "$2a$10$somehashedvalue" {
		t.Error("ExportConfig mutated the original config's APIKeyHash")
	}
	if cfg.Auth.OIDC.ClientSecret != "secret-oidc" {
		t.Error("ExportConfig mutated the original config's OIDC ClientSecret")
	}
	if cfg.Auth.Users[0].PasswordHash != "$2a$10$hashvalue" {
		t.Error("ExportConfig mutated the original config's PasswordHash")
	}
}

func TestParseImportedConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := createTestConfig()
		handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

		yamlBody := []byte(`
apps:
  - name: Sonarr
    url: http://localhost:8989
    enabled: true
  - name: Radarr
    url: http://localhost:7878
    enabled: true
`)
		req := httptest.NewRequest(http.MethodPost, "/api/config/import", bytes.NewReader(yamlBody))
		w := httptest.NewRecorder()

		handler.ParseImportedConfig(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp clientConfigResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(resp.Apps) != 2 {
			t.Errorf("expected 2 apps in response, got %d", len(resp.Apps))
		}
	})

	// findings.md M20: the import must reject structurally invalid
	// inputs (bad URLs, unknown open_mode, unknown roles, bad
	// durations) up front rather than leaving a later runtime to
	// stumble over them.
	t.Run("rejects invalid app URL", func(t *testing.T) {
		cfg := createTestConfig()
		handler := NewAPIHandler(cfg, "", &sync.RWMutex{})
		yamlBody := []byte(`
apps:
  - name: Bad
    url: "javascript:alert(1)"
    enabled: true
`)
		req := httptest.NewRequest(http.MethodPost, "/api/config/import", bytes.NewReader(yamlBody))
		w := httptest.NewRecorder()
		handler.ParseImportedConfig(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for non-http(s) URL, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("rejects unknown open_mode", func(t *testing.T) {
		cfg := createTestConfig()
		handler := NewAPIHandler(cfg, "", &sync.RWMutex{})
		yamlBody := []byte(`
apps:
  - name: X
    url: http://example.com
    enabled: true
    open_mode: telepathy
`)
		req := httptest.NewRequest(http.MethodPost, "/api/config/import", bytes.NewReader(yamlBody))
		w := httptest.NewRecorder()
		handler.ParseImportedConfig(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for unknown open_mode, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("rejects unparseable duration", func(t *testing.T) {
		cfg := createTestConfig()
		handler := NewAPIHandler(cfg, "", &sync.RWMutex{})
		yamlBody := []byte(`
server:
  proxy_timeout: "five minutes"
apps:
  - name: X
    url: http://example.com
    enabled: true
`)
		req := httptest.NewRequest(http.MethodPost, "/api/config/import", bytes.NewReader(yamlBody))
		w := httptest.NewRecorder()
		handler.ParseImportedConfig(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for bad duration, got %d: %s", w.Code, w.Body.String())
		}
	})

	// The runtime parser accepts a "d" (day) suffix on durations and the
	// docs advertise session_max_age values like "7d". The import validator
	// must accept the same values the runtime does, or an exported config
	// using the documented value fails its own re-import round-trip.
	t.Run("accepts day-suffixed durations", func(t *testing.T) {
		cfg := createTestConfig()
		handler := NewAPIHandler(cfg, "", &sync.RWMutex{})
		yamlBody := []byte(`
auth:
  session_max_age: "7d"
server:
  proxy_timeout: "1d"
apps:
  - name: X
    url: http://example.com
    enabled: true
`)
		req := httptest.NewRequest(http.MethodPost, "/api/config/import", bytes.NewReader(yamlBody))
		w := httptest.NewRecorder()
		handler.ParseImportedConfig(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 for documented day-suffixed durations, got %d: %s", w.Code, w.Body.String())
		}
	})

	// findings.md M7: unknown fields in the imported YAML must be
	// rejected, not silently dropped. This guards against future
	// mass-assignment if the Config struct gains a field whose name
	// happens to match something carried in a stale backup.
	t.Run("rejects unknown fields", func(t *testing.T) {
		cfg := createTestConfig()
		handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

		yamlBody := []byte(`
apps:
  - name: Sonarr
    url: http://localhost:8989
    enabled: true
unknown_future_field: "this should not be accepted"
`)
		req := httptest.NewRequest(http.MethodPost, "/api/config/import", bytes.NewReader(yamlBody))
		w := httptest.NewRecorder()

		handler.ParseImportedConfig(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for unknown field, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		cfg := createTestConfig()
		handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

		req := httptest.NewRequest(http.MethodGet, "/api/config/import", nil)
		w := httptest.NewRecorder()

		handler.ParseImportedConfig(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})

	t.Run("invalid YAML", func(t *testing.T) {
		cfg := createTestConfig()
		handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

		req := httptest.NewRequest(http.MethodPost, "/api/config/import", bytes.NewReader([]byte("not: [valid: yaml")))
		w := httptest.NewRecorder()

		handler.ParseImportedConfig(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("no apps", func(t *testing.T) {
		cfg := createTestConfig()
		handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

		yamlBody := []byte(`
server:
  title: "Empty"
`)
		req := httptest.NewRequest(http.MethodPost, "/api/config/import", bytes.NewReader(yamlBody))
		w := httptest.NewRecorder()

		handler.ParseImportedConfig(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
		if !bytes.Contains(w.Body.Bytes(), []byte("at least one app")) {
			t.Errorf("expected error about apps, got %q", w.Body.String())
		}
	})

	t.Run("app missing name", func(t *testing.T) {
		cfg := createTestConfig()
		handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

		yamlBody := []byte(`
apps:
  - url: http://localhost:8989
    enabled: true
`)
		req := httptest.NewRequest(http.MethodPost, "/api/config/import", bytes.NewReader(yamlBody))
		w := httptest.NewRecorder()

		handler.ParseImportedConfig(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
		if !bytes.Contains(w.Body.Bytes(), []byte("must have a name")) {
			t.Errorf("expected error about name, got %q", w.Body.String())
		}
	})

	t.Run("app missing URL", func(t *testing.T) {
		cfg := createTestConfig()
		handler := NewAPIHandler(cfg, "", &sync.RWMutex{})

		yamlBody := []byte(`
apps:
  - name: Sonarr
    enabled: true
`)
		req := httptest.NewRequest(http.MethodPost, "/api/config/import", bytes.NewReader(yamlBody))
		w := httptest.NewRecorder()

		handler.ParseImportedConfig(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
		if !bytes.Contains(w.Body.Bytes(), []byte("must have a URL")) {
			t.Errorf("expected error about URL, got %q", w.Body.String())
		}
	})
}

func TestBuildClientConfigResponseWithAuth(t *testing.T) {
	t.Run("with auth method only", func(t *testing.T) {
		cfg := createTestConfig()
		cfg.Auth.Method = "builtin"

		resp := buildClientConfigResponse(cfg, "admin", nil)

		if resp.Auth == nil {
			t.Fatal("expected auth to be set")
		}
		if resp.Auth.Method != "builtin" {
			t.Errorf("expected auth method 'builtin', got %q", resp.Auth.Method)
		}
		if resp.Auth.TrustedProxies != nil {
			t.Error("expected TrustedProxies to be nil when empty")
		}
		if resp.Auth.Headers != nil {
			t.Error("expected Headers to be nil when empty")
		}
	})

	t.Run("with trusted proxies and headers", func(t *testing.T) {
		cfg := createTestConfig()
		cfg.Auth.Method = "forward_auth"
		cfg.Auth.TrustedProxies = []string{"10.0.0.0/8"}
		cfg.Auth.Headers = map[string]string{"X-User": "username"}

		resp := buildClientConfigResponse(cfg, "admin", nil)

		if resp.Auth == nil {
			t.Fatal("expected auth to be set")
		}
		if len(resp.Auth.TrustedProxies) != 1 {
			t.Errorf("expected 1 trusted proxy, got %d", len(resp.Auth.TrustedProxies))
		}
		if resp.Auth.TrustedProxies[0] != "10.0.0.0/8" {
			t.Errorf("expected trusted proxy '10.0.0.0/8', got %q", resp.Auth.TrustedProxies[0])
		}
		if len(resp.Auth.Headers) != 1 {
			t.Errorf("expected 1 header, got %d", len(resp.Auth.Headers))
		}
	})

	t.Run("no auth method", func(t *testing.T) {
		cfg := createTestConfig()
		cfg.Auth.Method = ""

		resp := buildClientConfigResponse(cfg, "admin", nil)

		if resp.Auth != nil {
			t.Error("expected auth to be nil when method is empty")
		}
	})

	t.Run("health config included", func(t *testing.T) {
		cfg := createTestConfig()
		cfg.Health = config.HealthConfig{Enabled: true, Interval: "30s", Timeout: "5s"}

		resp := buildClientConfigResponse(cfg, "admin", nil)

		if resp.Health == nil {
			t.Fatal("expected health config to be included")
		}
		if !resp.Health.Enabled {
			t.Error("expected health.enabled to be true")
		}
	})

	t.Run("proxy timeout preserved", func(t *testing.T) {
		cfg := createTestConfig()
		cfg.Server.ProxyTimeout = "60s"

		resp := buildClientConfigResponse(cfg, "admin", nil)

		if resp.ProxyTimeout != "60s" {
			t.Errorf("expected proxy_timeout '60s', got %q", resp.ProxyTimeout)
		}
	})

	t.Run("log level preserved", func(t *testing.T) {
		cfg := createTestConfig()
		cfg.Server.LogLevel = "debug"

		resp := buildClientConfigResponse(cfg, "admin", nil)

		if resp.LogLevel != "debug" {
			t.Errorf("expected log_level 'debug', got %q", resp.LogLevel)
		}
	})
}

// TestExportConfig_NoRaceWithConcurrentAppMutation guards the config
// export against a data race: ExportConfig takes a shallow copy of the
// config and marshals it after releasing the lock, so the marshal read of
// cfg.Apps[i] must not alias a slice the poller/UpdateApp mutate in place.
// Fails under -race unless Apps/Groups/GatewaySites are copied under the
// lock (like Users already is).
func TestExportConfig_NoRaceWithConcurrentAppMutation(t *testing.T) {
	cfg := &config.Config{Apps: []config.AppConfig{{Name: "a", URL: "http://a"}}}
	var mu sync.RWMutex
	h := NewAPIHandler(cfg, "", &mu)

	urls := []string{"http://a-0", "http://a-1"}
	done := make(chan struct{})
	go func() {
		for i := 0; i < 2000; i++ {
			mu.Lock()
			cfg.Apps[0].URL = urls[i%2]
			mu.Unlock()
		}
		close(done)
	}()
	for i := 0; i < 2000; i++ {
		w := httptest.NewRecorder()
		h.ExportConfig(w, httptest.NewRequest(http.MethodGet, "/api/config/export", nil))
	}
	<-done
}

// TestGetApp_ProjectsForCallerRole is the regression test for
// GHSA-44h2-562r-3m67: the single-app endpoint used the admin projection for
// everyone and skipped the visibility rules, so a plain user could read proxy
// headers, action headers, URL credentials, and admin-only apps that the list
// endpoint correctly withheld.
func TestGetApp_ProjectsForCallerRole(t *testing.T) {
	cfg := &config.Config{Apps: []config.AppConfig{
		{Name: "Sonarr", URL: "http://svc:secretpass@sonarr.local:8989", Enabled: true, Proxy: true, ProxyHeaders: map[string]string{"X-Api-Key": "sonarr-key"}}, //nolint:gosec // fixture: the leak of exactly this credential is what the test detects
		{Name: "Webhook", URL: "http://hooks.local/fire", Enabled: true, OpenMode: "http_action", HTTPActionMethod: "POST", HTTPActionHeaders: map[string]string{"X-Webhook-Token": "whsec_abc"}},
		{Name: "AdminOnly", URL: "http://admin-only.local", Enabled: true, MinRole: "admin"},
		{Name: "OpsOnly", URL: "http://ops.local", Enabled: true, AllowedGroups: []string{"ops"}},
		{Name: "Disabled", URL: "http://off.local", Enabled: false},
	}}
	handler := NewAPIHandler(cfg, "/dev/null/impossible", &sync.RWMutex{})
	get := func(name string, user *auth.User) (int, ClientAppConfig) {
		req := httptest.NewRequest(http.MethodGet, "/api/app/"+name, nil)
		if user != nil {
			req = req.WithContext(auth.WithUserContext(req.Context(), user))
		}
		w := httptest.NewRecorder()
		handler.GetApp(w, req, name)
		var out ClientAppConfig
		if w.Code == http.StatusOK {
			if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
				t.Fatalf("decode: %v", err)
			}
		}
		return w.Code, out
	}
	viewer := &auth.User{Username: "viewer", Role: "user"}
	opsUser := &auth.User{Username: "ops", Role: "user", Groups: []string{"Ops"}}
	admin := &auth.User{Username: "admin", Role: auth.RoleAdmin}

	t.Run("user gets a redacted projection", func(t *testing.T) {
		code, app := get("Sonarr", viewer)
		if code != http.StatusOK {
			t.Fatalf("code = %d", code)
		}
		if app.URL != "http://sonarr.local:8989" {
			t.Errorf("url leaked credentials: %q", app.URL)
		}
		if len(app.ProxyHeaders) != 0 {
			t.Errorf("proxy_headers leaked: %v", app.ProxyHeaders)
		}
		code, app = get("Webhook", viewer)
		if code != http.StatusOK || len(app.HTTPActionHeaders) != 0 {
			t.Errorf("http_action_headers leaked (code %d): %v", code, app.HTTPActionHeaders)
		}
	})
	t.Run("user cannot see apps the list withholds", func(t *testing.T) {
		for _, name := range []string{"AdminOnly", "OpsOnly", "Disabled"} {
			if code, _ := get(name, viewer); code != http.StatusNotFound {
				t.Errorf("GET %s as user = %d, want 404", name, code)
			}
		}
	})
	t.Run("group membership opens a group-gated app", func(t *testing.T) {
		if code, _ := get("OpsOnly", opsUser); code != http.StatusOK {
			t.Errorf("GET OpsOnly as ops member = %d, want 200", code)
		}
	})
	t.Run("admin still sees everything", func(t *testing.T) {
		code, app := get("Sonarr", admin)
		if code != http.StatusOK || app.ProxyHeaders["X-Api-Key"] != "sonarr-key" || app.URL != "http://svc:secretpass@sonarr.local:8989" { //nolint:gosec // fixture credential, see above
			t.Errorf("admin projection lost fields (code %d): %+v", code, app)
		}
		for _, name := range []string{"AdminOnly", "OpsOnly", "Disabled"} {
			if code, _ := get(name, admin); code != http.StatusOK {
				t.Errorf("GET %s as admin = %d, want 200", name, code)
			}
		}
	})
	t.Run("unknown app is still 404", func(t *testing.T) {
		if code, _ := get("Nope", viewer); code != http.StatusNotFound {
			t.Errorf("code = %d", code)
		}
	})
}

// saveAppsForTest PUTs a full config with the given apps through SaveConfig.
func saveAppsForTest(t *testing.T, cfg *config.Config, apps []ClientAppConfig) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.Save(configPath); err != nil {
		t.Fatal(err)
	}
	handler := NewAPIHandler(cfg, configPath, &sync.RWMutex{})
	body, _ := json.Marshal(ClientConfigUpdate{Title: "Same", Navigation: cfg.Navigation, Groups: cfg.Groups, Apps: apps})
	w := httptest.NewRecorder()
	handler.SaveConfig(w, httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}
}

// TestSaveConfig_StaleTrackingDoesNotReattach reproduces #479: the operator
// detaches an app under Settings -> Discovery (a server-side write), then
// presses Save in the same Settings session. The dialog still holds the
// app with its old docker_key, and the save used to write that key back,
// so the app was tracked again and its URL stayed locked.
func TestSaveConfig_StaleTrackingDoesNotReattach(t *testing.T) {
	cfg := createTestConfig()
	cfg.Apps[0].URL = "http://10.0.0.1:8989" // detached: no tracking fields on the server

	saveAppsForTest(t, cfg, []ClientAppConfig{
		{Name: "App1", URL: "http://10.0.0.1:8989", Group: "Media", Enabled: true,
			DockerKey: "label:sonarr", DockerEndpoint: "unix:///var/run/docker.sock", DockerStrategy: "container_ip",
			DockerManagedURL: "http://10.0.0.1:8989"},
		{Name: "App2", URL: "http://localhost:8081", Group: "Tools", Enabled: true, Proxy: true},
		{Name: "DisabledApp", URL: "http://localhost:8082", Group: "Media"},
	})

	a := cfg.Apps[0]
	if a.DockerKey != "" || a.DockerEndpoint != "" || a.DockerStrategy != "" || a.DockerManagedURL != "" || a.DockerAutoImported {
		t.Errorf("a stale docker_key in the save payload re-attached the detached app: %+v", a)
	}
}

// TestSaveConfig_StaleKeyDoesNotOverrideRelink: tracking fields belong to
// the server. A re-link that happened while the dialog was open must not be
// undone by the dialog's older key.
func TestSaveConfig_StaleKeyDoesNotOverrideRelink(t *testing.T) {
	cfg := createTestConfig()
	cfg.Apps[0].URL = "http://10.0.0.1:8989"
	cfg.Apps[0].DockerKey = "label:sonarr-new"
	cfg.Apps[0].DockerEndpoint = "unix:///var/run/docker.sock"
	cfg.Apps[0].DockerStrategy = "container_ip"
	cfg.Apps[0].DockerManagedURL = "http://10.0.0.1:8989"

	saveAppsForTest(t, cfg, []ClientAppConfig{
		{Name: "App1", URL: "http://10.0.0.1:8989", Group: "Media", Enabled: true, Color: "#123456",
			DockerKey: "label:sonarr-old", DockerEndpoint: "tcp://old:2375", DockerStrategy: "host_port"},
		{Name: "App2", URL: "http://localhost:8081", Group: "Tools", Enabled: true, Proxy: true},
		{Name: "DisabledApp", URL: "http://localhost:8082", Group: "Media"},
	})

	a := cfg.Apps[0]
	if a.DockerKey != "label:sonarr-new" || a.DockerEndpoint != "unix:///var/run/docker.sock" || a.DockerStrategy != "container_ip" {
		t.Errorf("stale client tracking overwrote the server's: %+v", a)
	}
	if a.Color != "#123456" {
		t.Errorf("non-tracking edit was not applied: %+v", a)
	}
}

// TestUpdateApp_StaleTrackingDoesNotReattach covers the per-app PUT path.
func TestUpdateApp_StaleTrackingDoesNotReattach(t *testing.T) {
	cfg := createTestConfig()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.Save(configPath); err != nil {
		t.Fatal(err)
	}
	handler := NewAPIHandler(cfg, configPath, &sync.RWMutex{})
	body, _ := json.Marshal(ClientAppConfig{Name: "App1", URL: cfg.Apps[0].URL, Group: "Media", Enabled: true,
		DockerKey: "label:sonarr", DockerEndpoint: "unix:///var/run/docker.sock", DockerStrategy: "container_ip"})
	w := httptest.NewRecorder()
	handler.UpdateApp(w, httptest.NewRequest(http.MethodPut, "/api/app/App1", bytes.NewReader(body)), "App1")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}
	if cfg.Apps[0].DockerKey != "" {
		t.Errorf("per-app PUT attached an untracked app: %+v", cfg.Apps[0])
	}
}

// clientSnapshot is the client-shaped config an admin browser loads,
// round-tripped through JSON so it shares no memory with cfg.
func clientSnapshot(t *testing.T, cfg *config.Config) ClientConfigUpdate {
	t.Helper()
	resp := buildClientConfigResponse(cfg, auth.RoleAdmin, nil)
	raw, err := json.Marshal(updateFromResponse(&resp))
	if err != nil {
		t.Fatal(err)
	}
	var out ClientConfigUpdate
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// putConfigForTest saves cfg to a temp file, PUTs body through SaveConfig
// and returns the response, the handler and the path of the file.
func putConfigForTest(t *testing.T, cfg *config.Config, body any) (*httptest.ResponseRecorder, *APIHandler, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	h := NewAPIHandler(cfg, path, &sync.RWMutex{})
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.SaveConfig(w, httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(raw)))
	return w, h, path
}

// putConfigOK is putConfigForTest for a save that must succeed; it
// returns the config reloaded from disk.
func putConfigOK(t *testing.T, cfg *config.Config, body any) *config.Config {
	t.Helper()
	w, _, path := putConfigForTest(t, cfg, body)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return loaded
}

func findApp(cfg *config.Config, name string) *config.AppConfig {
	for i := range cfg.Apps {
		if cfg.Apps[i].Name == name {
			return &cfg.Apps[i]
		}
	}
	return nil
}

func dropClientApp(apps []ClientAppConfig, name string) []ClientAppConfig {
	out := make([]ClientAppConfig, 0, len(apps))
	for i := range apps {
		if apps[i].Name != name {
			out = append(out, apps[i])
		}
	}
	return out
}

func boolPtr(b bool) *bool { return &b }

// S-04: the poller refreshed a tracked app's URL while Settings was open.
// The dialog still holds the old URL; saving an unrelated edit must keep
// the server's URL and the tracking, not auto-detach the app.
func TestSaveConfig_ThreeWay_StaleURLKeptAndTracked(t *testing.T) {
	cfg := createTestConfig()
	base := clientSnapshot(t, cfg)
	base.Apps[0].URL = "http://172.17.0.2"
	base.Apps[0].DockerKey = "k"
	base.Apps[0].DockerManagedURL = "http://172.17.0.2"
	cfg.Apps[0].URL = "http://172.17.0.9"
	cfg.Apps[0].DockerKey = "k"
	cfg.Apps[0].DockerManagedURL = "http://172.17.0.9"

	mine := clientSnapshot(t, cfg)
	mine.Apps = append([]ClientAppConfig(nil), base.Apps...)
	mine.Title = "Renamed Dashboard"
	mine.Base = &base

	loaded := putConfigOK(t, cfg, &mine)
	a := findApp(loaded, "App1")
	if a == nil || a.URL != "http://172.17.0.9" || a.DockerKey != "k" || a.DockerManagedURL != "http://172.17.0.9" {
		t.Errorf("stale URL reverted or tracking lost: %+v", a)
	}
	if loaded.Server.Title != "Renamed Dashboard" {
		t.Errorf("title = %q", loaded.Server.Title)
	}
	// No auto-detach: the live config the PUT left behind is still tracked
	// against the server's URL (a detach clears every tracking field).
	if live := findApp(cfg, "App1"); live == nil || live.DockerKey != "k" || live.DockerManagedURL != "http://172.17.0.9" {
		t.Errorf("live app detached by a stale URL: %+v", live)
	}
}

// S-04, health_url half: the server's health_url changed after the client
// loaded; a payload echoing the stale base value keeps the server's.
func TestSaveConfig_ThreeWay_StaleHealthURLKept(t *testing.T) {
	cfg := createTestConfig()
	cfg.Apps[0].DockerKey = "k"
	cfg.Apps[0].DockerManagedURL = cfg.Apps[0].URL
	cfg.Apps[0].HealthURL = "http://172.17.0.2/health"
	base := clientSnapshot(t, cfg)
	cfg.Apps[0].HealthURL = "http://172.17.0.9/health"
	mine := base
	mine.Apps = append([]ClientAppConfig(nil), base.Apps...)
	mine.Base = &base

	loaded := putConfigOK(t, cfg, &mine)
	a := findApp(loaded, "App1")
	if a == nil || a.HealthURL != "http://172.17.0.9/health" {
		t.Errorf("stale health_url reverted the server's: %+v", a)
	}
	if a == nil || a.DockerKey != "k" || a.DockerManagedURL != a.URL {
		t.Errorf("tracking lost: %+v", a)
	}
}

// A manual URL edit (mine differs from base and theirs) on the three-way
// path still auto-detaches, as TestSaveConfig_AutoDetachesOnURLChange
// checks for the two-way path.
func TestSaveConfig_ThreeWay_ManualURLEditDetaches(t *testing.T) {
	cfg := createTestConfig()
	cfg.Apps[0].URL = "http://172.17.0.2"
	cfg.Apps[0].DockerKey = "label:sonarr-stable"
	cfg.Apps[0].DockerEndpoint = "unix:///var/run/docker.sock"
	cfg.Apps[0].DockerStrategy = "container_ip"
	cfg.Apps[0].DockerManagedURL = "http://172.17.0.2"
	base := clientSnapshot(t, cfg)
	cfg.Apps[0].URL = "http://172.17.0.9" // the poller refreshed it meanwhile
	cfg.Apps[0].DockerManagedURL = "http://172.17.0.9"
	mine := clientSnapshot(t, cfg)
	mine.Apps = append([]ClientAppConfig(nil), base.Apps...)
	mine.Apps[0].URL = "http://manual:9999"
	mine.Base = &base

	loaded := putConfigOK(t, cfg, &mine)
	for _, a := range []*config.AppConfig{findApp(cfg, "App1"), findApp(loaded, "App1")} {
		if a == nil || a.URL != "http://manual:9999" {
			t.Fatalf("manual URL not stored: %+v", a)
		}
		if a.DockerKey != "" || a.DockerEndpoint != "" || a.DockerStrategy != "" || a.DockerManagedURL != "" {
			t.Errorf("expected auto-detach to clear tracking fields, got %+v", a)
		}
	}
}

// S-05: the server removed an app the client still holds unchanged from
// base; the save must not resurrect it.
func TestSaveConfig_ThreeWay_ServerRemovedAppNotResurrected(t *testing.T) {
	cfg := createTestConfig()
	base := clientSnapshot(t, cfg)
	mine := clientSnapshot(t, cfg)
	mine.Base = &base
	cfg.Apps = cfg.Apps[1:] // the server deleted App1

	loaded := putConfigOK(t, cfg, &mine)
	if findApp(loaded, "App1") != nil {
		t.Errorf("server-removed app resurrected: %+v", loaded.Apps)
	}
	if findApp(loaded, "App2") == nil {
		t.Errorf("apps = %+v", loaded.Apps)
	}
}

// S-11: the server added an app and a gateway site pointing at it while
// the dialog was open; the save passes validation and keeps both.
func TestSaveConfig_ThreeWay_ServerAddedAppWithGatewaySite(t *testing.T) {
	cfg := createTestConfig()
	base := clientSnapshot(t, cfg)
	cfg.Apps = append(cfg.Apps, config.AppConfig{Name: "Added", URL: "http://added:80", Enabled: true})
	cfg.Server.GatewaySites = []config.GatewaySite{{Domain: "added.example.com", BackendURL: "http://added:80", AppName: "Added"}}
	mine := base
	mine.Title = "Edited"
	mine.Base = &base

	loaded := putConfigOK(t, cfg, &mine)
	if findApp(loaded, "Added") == nil {
		t.Errorf("server-added app dropped: %+v", loaded.Apps)
	}
	if len(loaded.Server.GatewaySites) != 1 || loaded.Server.GatewaySites[0].AppName != "Added" {
		t.Errorf("gateway sites = %+v", loaded.Server.GatewaySites)
	}
	if loaded.Server.Title != "Edited" {
		t.Errorf("title = %q", loaded.Server.Title)
	}
}

// S-05, S-11: an app the server added while the dialog was open (absent
// from base and payload) survives the save.
func TestSaveConfig_ThreeWay_ServerAddedAppSurvives(t *testing.T) {
	cfg := createTestConfig()
	base := clientSnapshot(t, cfg)
	base.Apps = dropClientApp(base.Apps, "App2")
	mine := clientSnapshot(t, cfg)
	mine.Apps = dropClientApp(mine.Apps, "App2")
	mine.Base = &base

	loaded := putConfigOK(t, cfg, &mine)
	if findApp(loaded, "App2") == nil || findApp(loaded, "App1") == nil {
		t.Errorf("apps = %+v", loaded.Apps)
	}
}

// S-10: the session cookie domain set on the server while the dialog was
// open is not cleared by a dialog that never touched it.
func TestSaveConfig_ThreeWay_CookieDomainKept(t *testing.T) {
	cfg := createTestConfig()
	base := clientSnapshot(t, cfg)
	cfg.Server.SessionCookieDomain = ".x.org"
	mine := base
	mine.Base = &base

	loaded := putConfigOK(t, cfg, &mine)
	if loaded.Server.SessionCookieDomain != ".x.org" {
		t.Errorf("session_cookie_domain = %q", loaded.Server.SessionCookieDomain)
	}
}

// S-12: a rename carries original_name, so the renamed app keeps every
// server-owned field and its gateway site follows the new name.
func TestSaveConfig_RenameKeepsServerOwnedFields(t *testing.T) {
	for _, threeWay := range []bool{false, true} {
		t.Run(fmt.Sprintf("threeWay=%v", threeWay), func(t *testing.T) {
			cfg := createTestConfig()
			old := &cfg.Apps[0]
			old.Name = "Old"
			old.AuthBypass = []config.AuthBypassRule{{Path: "/api/*"}}
			old.Access = config.AppAccessConfig{Roles: []string{"admin"}}
			old.ForwardedHeaders = boolPtr(false)
			old.DockerAutoImported = true
			old.DockerKey = "k"
			old.DockerManagedURL = old.URL
			cfg.Server.GatewaySites = []config.GatewaySite{{Domain: "old.example.com", BackendURL: "http://backend:80", AppName: "Old"}}

			base := clientSnapshot(t, cfg)
			mine := clientSnapshot(t, cfg)
			mine.Apps[0].Name = "New"
			mine.Apps[0].OriginalName = "Old"
			if threeWay {
				mine.Base = &base
			}

			loaded := putConfigOK(t, cfg, &mine)
			if findApp(loaded, "Old") != nil {
				t.Fatalf("old name still present: %+v", loaded.Apps)
			}
			a := findApp(loaded, "New")
			if a == nil {
				t.Fatalf("renamed app missing: %+v", loaded.Apps)
			}
			if len(a.AuthBypass) != 1 || len(a.Access.Roles) != 1 || a.ForwardedHeaders == nil || *a.ForwardedHeaders {
				t.Errorf("server-owned fields lost: %+v", a)
			}
			if !a.DockerAutoImported || a.DockerKey != "k" {
				t.Errorf("tracking lost: %+v", a)
			}
			if got := loaded.Server.GatewaySites[0].AppName; got != "New" {
				t.Errorf("gateway app_name = %q, want New", got)
			}
		})
	}
}

// S-09: forwarded_headers is not in the client payload, so a two-way save
// of an unchanged app must keep the stored value.
func TestSaveConfig_ForwardedHeadersPreservedByName(t *testing.T) {
	cfg := createTestConfig()
	cfg.Apps[0].ForwardedHeaders = boolPtr(false)
	w, _, path := putConfigForTest(t, cfg, clientSnapshot(t, cfg))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "forwarded_headers: false") {
		t.Errorf("forwarded_headers lost from file:\n%s", raw)
	}
}

func TestSaveConfig_DeletedAppClearsGatewayAppName(t *testing.T) {
	cfg := createTestConfig()
	cfg.Server.GatewaySites = []config.GatewaySite{{Domain: "app2.example.com", BackendURL: "http://backend:80", AppName: "App2"}}
	update := clientSnapshot(t, cfg)
	update.Apps = dropClientApp(update.Apps, "App2")

	loaded := putConfigOK(t, cfg, &update)
	if got := loaded.Server.GatewaySites[0].AppName; got != "" {
		t.Errorf("gateway app_name = %q, want cleared", got)
	}
}

func TestSaveConfig_GroupRenameRepointsApps(t *testing.T) {
	cfg := createTestConfig()
	update := clientSnapshot(t, cfg)
	update.Groups[0].Name = "Video"
	update.Groups[0].OriginalName = "Media"

	w, h, path := putConfigForTest(t, cfg, &update)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if a := findApp(loaded, "App1"); a == nil || a.Group != "Video" {
		t.Errorf("App1 = %+v", a)
	}
	for i := range h.config.Groups {
		if h.config.Groups[i].OriginalName != "" {
			t.Errorf("live group kept original_name: %+v", h.config.Groups[i])
		}
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "original_name") {
		t.Errorf("original_name written to file:\n%s", raw)
	}
}

func TestSaveConfig_GroupRenameDoesNotTouchLifecycleGroups(t *testing.T) {
	cfg := createTestConfig()
	cfg.Discovery.Docker.LifecycleAllowedGroups = []string{"admins", "Media"}
	update := clientSnapshot(t, cfg)
	update.Groups[0].Name = "Video"
	update.Groups[0].OriginalName = "Media"

	w, h, _ := putConfigForTest(t, cfg, &update)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}
	got := h.config.Discovery.Docker.LifecycleAllowedGroups
	if len(got) != 2 || got[0] != "admins" || got[1] != "Media" {
		t.Errorf("lifecycle_allowed_groups = %v, want [admins Media]", got)
	}
}

func TestSaveConfig_ThreeWay_GroupRenameRepointsServerAddedApp(t *testing.T) {
	cfg := createTestConfig()
	base := clientSnapshot(t, cfg)
	cfg.Apps = append(cfg.Apps, config.AppConfig{Name: "B", URL: "http://b:1", Group: "Media", Enabled: true})
	mine := clientSnapshot(t, cfg)
	mine.Apps = dropClientApp(mine.Apps, "B")
	mine.Groups[0].Name = "Video"
	mine.Groups[0].OriginalName = "Media"
	mine.Apps[0].Group = "Video" // the Apps tab re-points its own apps
	mine.Base = &base

	loaded := putConfigOK(t, cfg, &mine)
	for _, name := range []string{"App1", "B"} {
		if a := findApp(loaded, name); a == nil || a.Group != "Video" {
			t.Errorf("%s = %+v", name, a)
		}
	}
}

// Ruling: the payload renames Media to Video and adds a new group named
// Media. Apps that were in the old Media (base or server-added) move to
// Video; an app the user put in the new Media stays there.
func TestSaveConfig_GroupRenamePlusNewGroupOfOldName(t *testing.T) {
	for _, threeWay := range []bool{false, true} {
		t.Run(fmt.Sprintf("threeWay=%v", threeWay), func(t *testing.T) {
			cfg := createTestConfig()
			base := clientSnapshot(t, cfg)
			if threeWay {
				cfg.Apps = append(cfg.Apps, config.AppConfig{Name: "B", URL: "http://b:1", Group: "Media", Enabled: true})
			}
			mine := clientSnapshot(t, cfg)
			mine.Apps = dropClientApp(mine.Apps, "B")
			mine.Groups[0].Name = "Video"
			mine.Groups[0].OriginalName = "Media"
			mine.Groups = append(mine.Groups, config.GroupConfig{Name: "Media", Color: "#abcdef", Order: 2})
			mine.Apps = append(mine.Apps, ClientAppConfig{Name: "Fresh", URL: "http://fresh:1", Group: "Media", Enabled: true})
			if threeWay {
				mine.Base = &base
			}

			loaded := putConfigOK(t, cfg, &mine)
			want := map[string]string{"App1": "Video", "Fresh": "Media"}
			if threeWay {
				want["B"] = "Video"
			}
			for name, group := range want {
				if a := findApp(loaded, name); a == nil || a.Group != group {
					t.Errorf("%s = %+v, want group %q", name, a, group)
				}
			}
		})
	}
}

// Review Focus 5: a PUT without base replaces the app list as in 3.5.0.
func TestSaveConfig_WithoutBaseIsTwoWay(t *testing.T) {
	cfg := createTestConfig()
	cfg.Apps[0].AuthBypass = []config.AuthBypassRule{{Path: "/api/*"}}
	update := clientSnapshot(t, cfg)
	update.Apps = dropClientApp(update.Apps, "App2")

	loaded := putConfigOK(t, cfg, &update)
	if findApp(loaded, "App2") != nil {
		t.Errorf("two-way save kept an app the payload omitted: %+v", loaded.Apps)
	}
	if a := findApp(loaded, "App1"); a == nil || len(a.AuthBypass) != 1 {
		t.Errorf("App1 = %+v", a)
	}
}

func TestSaveConfig_ThreeWay_NameClashIs409(t *testing.T) {
	cfg := createTestConfig()
	base := clientSnapshot(t, cfg)
	cfg.Apps = append(cfg.Apps, config.AppConfig{Name: "X", URL: "http://server:1", Enabled: true})
	mine := base
	mine.Apps = append(append([]ClientAppConfig(nil), base.Apps...), ClientAppConfig{Name: "X", URL: "http://mine:1", Enabled: true})
	mine.Base = &base

	w, h, _ := putConfigForTest(t, cfg, &mine)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `an app named "X" was added on the server`) {
		t.Errorf("body = %q", w.Body.String())
	}
	if a := findApp(h.config, "X"); a == nil || a.URL != "http://server:1" {
		t.Errorf("config changed on conflict: %+v", a)
	}
}

func TestMergeErrorStatus(t *testing.T) {
	if got := mergeErrorStatus(&MergeConflictError{Kind: "app", Name: "X"}); got != http.StatusConflict {
		t.Errorf("conflict = %d", got)
	}
	if got := mergeErrorStatus(fmt.Errorf("wrapped: %w", &MergeConflictError{Kind: "group", Name: "G"})); got != http.StatusConflict {
		t.Errorf("wrapped conflict = %d", got)
	}
	if got := mergeErrorStatus(errors.New("other")); got != http.StatusBadRequest {
		t.Errorf("other = %d", got)
	}
}

// A save rejected by validation rolls back the gateway-site cascade too.
func TestSaveConfig_RollbackRestoresGatewaySites(t *testing.T) {
	cfg := createTestConfig()
	cfg.Server.GatewaySites = []config.GatewaySite{{Domain: "app2.example.com", BackendURL: "http://backend:80", AppName: "App2"}}
	update := clientSnapshot(t, cfg)
	update.Apps = dropClientApp(update.Apps, "App2")
	// Two apps sharing a slug fail Validate after the cascade ran.
	update.Apps = append(update.Apps, ClientAppConfig{Name: "Dup App", URL: "http://a:1", Enabled: true}, ClientAppConfig{Name: "dup-app", URL: "http://b:1", Enabled: true})

	w, h, _ := putConfigForTest(t, cfg, &update)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}
	if got := h.config.Server.GatewaySites[0].AppName; got != "App2" {
		t.Errorf("gateway app_name after rollback = %q, want App2", got)
	}
	if findApp(h.config, "App2") == nil {
		t.Errorf("apps not rolled back: %+v", h.config.Apps)
	}
}

// S-09 for PUT /api/app/{name}.
func TestUpdateApp_PreservesForwardedHeaders(t *testing.T) {
	cfg := createTestConfig()
	cfg.Apps[0].ForwardedHeaders = boolPtr(false)
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	h := NewAPIHandler(cfg, path, &sync.RWMutex{})
	body, _ := json.Marshal(ClientAppConfig{Name: "App1", URL: "http://localhost:8080", Group: "Media", Enabled: true, Color: "#abcdef"})
	w := httptest.NewRecorder()
	h.UpdateApp(w, httptest.NewRequest(http.MethodPut, "/api/app/App1", bytes.NewReader(body)), "App1")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	a := findApp(loaded, "App1")
	if a == nil || a.ForwardedHeaders == nil || *a.ForwardedHeaders || a.Color != "#abcdef" {
		t.Errorf("App1 = %+v", a)
	}
}

func newGroupTestHandler(t *testing.T, cfg *config.Config) *APIHandler {
	t.Helper()
	return NewAPIHandler(cfg, filepath.Join(t.TempDir(), "config.yaml"), &sync.RWMutex{})
}

func TestSaveConfig_ProxyTimeoutCanBeCleared(t *testing.T) {
	cfg := createTestConfig()
	cfg.Server.ProxyTimeout = "30s"
	handler := newGroupTestHandler(t, cfg)
	update := ClientConfigUpdate{Title: "T", Groups: cfg.Groups, ProxyTimeout: ""}
	body, _ := json.Marshal(update)
	w := httptest.NewRecorder()
	handler.SaveConfig(w, httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if cfg.Server.ProxyTimeout != "" {
		t.Errorf("ProxyTimeout = %q, want cleared", cfg.Server.ProxyTimeout)
	}
	data, err := os.ReadFile(handler.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "proxy_timeout: 30s") {
		t.Errorf("file still has the old timeout:\n%s", data)
	}
	// The proxy falls back to its 30s default for an empty value.
	if got := NewReverseProxyHandler(nil, "").timeout; got != 30*time.Second {
		t.Errorf("proxy timeout for an empty value = %v, want 30s", got)
	}
}

func TestUpdateGroup_RejectsEmptyName(t *testing.T) {
	cfg := createTestConfig()
	handler := newGroupTestHandler(t, cfg)
	w := httptest.NewRecorder()
	handler.UpdateGroup(w, httptest.NewRequest(http.MethodPut, "/api/group/Media", strings.NewReader(`{"name":"","color":"#fff"}`)), "Media")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400: %s", w.Code, w.Body.String())
	}
	if cfg.Groups[0].Name != "Media" || cfg.Apps[0].Group != "Media" {
		t.Errorf("group or app changed: %+v %+v", cfg.Groups[0], cfg.Apps[0])
	}
}

func TestUpdateGroup_RenameRepointsAppsAndLeavesLifecycleAllowedGroups(t *testing.T) {
	cfg := createTestConfig()
	cfg.Discovery.Docker.LifecycleAllowedGroups = []string{"Media"}
	handler := newGroupTestHandler(t, cfg)
	body := `{"name":"Video","color":"#ff0000"}`
	w := httptest.NewRecorder()
	handler.UpdateGroup(w, httptest.NewRequest(http.MethodPut, "/api/group/Media", strings.NewReader(body)), "Media")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if cfg.Apps[0].Group != "Video" {
		t.Errorf("App1 group = %q, want Video", cfg.Apps[0].Group)
	}
	if cfg.Apps[1].Group != "Tools" {
		t.Errorf("App2 group = %q, want Tools", cfg.Apps[1].Group)
	}
	if got := cfg.Discovery.Docker.LifecycleAllowedGroups; len(got) != 1 || got[0] != "Media" {
		t.Errorf("lifecycle_allowed_groups = %v, want unchanged [Media] (user/IdP groups, not dashboard groups)", got)
	}
}

func TestUpdateGroup_RenameCollisionRejected(t *testing.T) {
	cfg := createTestConfig()
	handler := newGroupTestHandler(t, cfg)
	w := httptest.NewRecorder()
	handler.UpdateGroup(w, httptest.NewRequest(http.MethodPut, "/api/group/Media", strings.NewReader(`{"name":"Tools"}`)), "Media")
	if w.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409", w.Code)
	}
	if cfg.Groups[0].Name != "Media" || cfg.Apps[0].Group != "Media" {
		t.Error("state changed on rejected rename")
	}
}

func TestUpdateGroup_RenameRollsBackAppsOnSaveFailure(t *testing.T) {
	cfg := createTestConfig()
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := NewAPIHandler(cfg, filepath.Join(blocker, "config.yaml"), &sync.RWMutex{})
	w := httptest.NewRecorder()
	handler.UpdateGroup(w, httptest.NewRequest(http.MethodPut, "/api/group/Media", strings.NewReader(`{"name":"Video"}`)), "Media")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500: %s", w.Code, w.Body.String())
	}
	if cfg.Groups[0].Name != "Media" || cfg.Apps[0].Group != "Media" {
		t.Error("rename not rolled back")
	}
}

func TestCreateGroup_IgnoresOriginalName(t *testing.T) {
	cfg := createTestConfig()
	handler := newGroupTestHandler(t, cfg)
	w := httptest.NewRecorder()
	handler.CreateGroup(w, httptest.NewRequest(http.MethodPost, "/api/groups", strings.NewReader(`{"name":"New","original_name":"x"}`)))
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	assertNoGroupOriginalName(t, handler, cfg, "New")
}

func TestUpdateGroup_IgnoresOriginalName(t *testing.T) {
	cfg := createTestConfig()
	handler := newGroupTestHandler(t, cfg)
	w := httptest.NewRecorder()
	handler.UpdateGroup(w, httptest.NewRequest(http.MethodPut, "/api/group/Media", strings.NewReader(`{"name":"Media","original_name":"x"}`)), "Media")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	assertNoGroupOriginalName(t, handler, cfg, "Media")
}

func assertNoGroupOriginalName(t *testing.T, handler *APIHandler, cfg *config.Config, name string) {
	t.Helper()
	for i := range cfg.Groups {
		if cfg.Groups[i].Name == name && cfg.Groups[i].OriginalName != "" {
			t.Errorf("stored group %s has OriginalName %q", name, cfg.Groups[i].OriginalName)
		}
	}
	w := httptest.NewRecorder()
	handler.GetConfig(w, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if strings.Contains(w.Body.String(), "original_name") {
		t.Errorf("GET /api/config exposes original_name: %s", w.Body.String())
	}
}

func getConfigAs(t *testing.T, cfg *config.Config, role string) map[string]json.RawMessage {
	t.Helper()
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req = req.WithContext(auth.WithUserContext(req.Context(), &auth.User{Username: "u", Role: role}))
	w := httptest.NewRecorder()
	handler.GetConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]json.RawMessage
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestGetConfig_ExposesEnvOverrides(t *testing.T) {
	cfg := createTestConfig()
	cfg.Server.LogLevel = "info"
	cfg.ApplyOverride(config.OverrideLogLevel, "MUXIMUX_LOG_LEVEL", "debug")

	resp := getConfigAs(t, cfg, auth.RoleAdmin)
	var overrides map[string]string
	if err := json.Unmarshal(resp["env_overrides"], &overrides); err != nil {
		t.Fatalf("env_overrides missing or malformed: %v (%s)", err, resp["env_overrides"])
	}
	if overrides["log_level"] != "MUXIMUX_LOG_LEVEL" {
		t.Errorf("env_overrides.log_level = %q, want MUXIMUX_LOG_LEVEL", overrides["log_level"])
	}
}

func TestGetConfig_EnvOverridesAdminOnly(t *testing.T) {
	cfg := createTestConfig()
	cfg.ApplyOverride(config.OverrideLogLevel, "MUXIMUX_LOG_LEVEL", "debug")
	cfg.ApplyOverride(config.OverrideListen, "--listen", ":9999")

	resp := getConfigAs(t, cfg, auth.RoleUser)
	if raw, ok := resp["env_overrides"]; ok {
		t.Errorf("non-admin response carries env_overrides: %s", raw)
	}
}

func TestSaveConfig_IgnoresOverriddenLogLevel(t *testing.T) {
	cfg := createTestConfig()
	cfg.Server.LogLevel = "info"
	cfg.ApplyOverride(config.OverrideLogLevel, "MUXIMUX_LOG_LEVEL", "debug")
	path := filepath.Join(t.TempDir(), "config.yaml")
	handler := NewAPIHandler(cfg, path, &sync.RWMutex{})

	update := ClientConfigUpdate{
		Title:      cfg.Server.Title,
		LogLevel:   "warn",
		Navigation: cfg.Navigation,
		Groups:     cfg.Groups,
		Apps:       []ClientAppConfig{{Name: "App1", URL: "http://localhost:8080", Enabled: true}},
	}
	body, _ := json.Marshal(update)
	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.SaveConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}

	if cfg.Server.LogLevel != "debug" {
		t.Errorf("live log level = %q, want the override debug", cfg.Server.LogLevel)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var onDisk config.Config
	if err := yaml.Unmarshal(data, &onDisk); err != nil {
		t.Fatal(err)
	}
	if onDisk.Server.LogLevel != "info" {
		t.Errorf("file log level = %q, want info", onDisk.Server.LogLevel)
	}
}

func TestGetConfig_ExposesForwardAuthAdminGroups(t *testing.T) {
	cfg := createTestConfig()
	cfg.Auth.Method = "forward_auth"
	cfg.Auth.ForwardAuthAdminGroups = []string{"dashboard-admins"}

	resp := getConfigAs(t, cfg, auth.RoleAdmin)
	var authCfg struct {
		Groups []string `json:"forward_auth_admin_groups"`
	}
	if err := json.Unmarshal(resp["auth"], &authCfg); err != nil {
		t.Fatalf("auth missing or malformed: %v", err)
	}
	if len(authCfg.Groups) != 1 || authCfg.Groups[0] != "dashboard-admins" {
		t.Errorf("forward_auth_admin_groups = %v, want [dashboard-admins]", authCfg.Groups)
	}
}

func TestGetConfig_HidesForwardAuthAdminGroupsFromNonAdmins(t *testing.T) {
	cfg := createTestConfig()
	cfg.Auth.Method = "forward_auth"
	cfg.Auth.ForwardAuthAdminGroups = []string{"dashboard-admins"}

	for _, role := range []string{auth.RoleUser, auth.RolePowerUser} {
		resp := getConfigAs(t, cfg, role)
		if strings.Contains(string(resp["auth"]), "forward_auth_admin_groups") {
			t.Errorf("%s sees forward_auth_admin_groups: %s", role, resp["auth"])
		}
	}
}
