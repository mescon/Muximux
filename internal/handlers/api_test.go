package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"

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

	// Should only include enabled apps
	if len(apps) != 2 {
		t.Errorf("expected 2 apps, got %d", len(apps))
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

		mergeConfigUpdate(cfg, update)

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

	t.Run("preserves proxy URL", func(t *testing.T) {
		cfg := createTestConfig()
		originalURL := cfg.Apps[1].URL // App2 is proxied

		update := &ClientConfigUpdate{
			Title: "Test",
			Apps: []ClientAppConfig{
				{
					Name:    "App2",
					URL:     "/proxy/app2/", // Client sends proxy URL
					Proxy:   true,
					Enabled: true,
				},
			},
		}

		mergeConfigUpdate(cfg, update)

		// Should preserve original URL for proxied apps
		if len(cfg.Apps) != 1 {
			t.Fatalf("expected 1 app, got %d", len(cfg.Apps))
		}
		if cfg.Apps[0].URL != originalURL {
			t.Errorf("expected original URL %q to be preserved, got %q", originalURL, cfg.Apps[0].URL)
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

		mergeConfigUpdate(cfg, update)

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
		existing := map[string]config.AppConfig{}
		clientApp := ClientAppConfig{
			Name:    "NewApp",
			URL:     "http://localhost:9000",
			Enabled: true,
			Color:   "#ff0000",
		}

		result := mergeClientApp(&clientApp, existing)

		if result.Name != "NewApp" {
			t.Errorf("expected name 'NewApp', got %q", result.Name)
		}
		if result.URL != "http://localhost:9000" {
			t.Errorf("expected URL 'http://localhost:9000', got %q", result.URL)
		}
	})

	t.Run("existing proxied app preserves URL", func(t *testing.T) {
		existing := map[string]config.AppConfig{
			"ProxiedApp": {
				Name:    "ProxiedApp",
				URL:     "http://internal:8080",
				Proxy:   true,
				Enabled: true,
				AuthBypass: []config.AuthBypassRule{
					{Path: "/api/*"},
				},
			},
		}
		clientApp := ClientAppConfig{
			Name:    "ProxiedApp",
			URL:     "/proxy/proxiedapp/", // The proxy URL sent by the client
			Proxy:   true,
			Enabled: true,
		}

		result := mergeClientApp(&clientApp, existing)

		if result.URL != "http://internal:8080" {
			t.Errorf("expected preserved URL 'http://internal:8080', got %q", result.URL)
		}
		if len(result.AuthBypass) != 1 {
			t.Errorf("expected 1 auth bypass rule, got %d", len(result.AuthBypass))
		}
	})

	t.Run("existing non-proxied app updates URL", func(t *testing.T) {
		existing := map[string]config.AppConfig{
			"App": {
				Name:    "App",
				URL:     "http://old:8080",
				Proxy:   false,
				Enabled: true,
			},
		}
		clientApp := ClientAppConfig{
			Name:    "App",
			URL:     "http://new:9090",
			Proxy:   false,
			Enabled: true,
		}

		result := mergeClientApp(&clientApp, existing)

		if result.URL != "http://new:9090" {
			t.Errorf("expected new URL 'http://new:9090', got %q", result.URL)
		}
	})
}

func TestBuildClientConfigResponse(t *testing.T) {
	cfg := createTestConfig()

	resp := buildClientConfigResponse(cfg, "admin")

	if resp.Title != "Test Dashboard" {
		t.Errorf("expected title 'Test Dashboard', got %q", resp.Title)
	}
	if resp.Navigation.Position != "left" {
		t.Errorf("expected navigation position 'left', got %q", resp.Navigation.Position)
	}
	if len(resp.Groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(resp.Groups))
	}
	// Only enabled apps should be in the response
	if len(resp.Apps) != 2 {
		t.Errorf("expected 2 enabled apps, got %d", len(resp.Apps))
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
	resp = buildClientConfigResponse(cfg, "admin")
	if resp.Keybindings == nil {
		t.Error("expected keybindings to be set")
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
}

// Tests for save-failure error paths using an invalid configPath.
// /dev/null/impossible is guaranteed to fail because /dev/null is a file, not a directory.
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

func TestSanitizeApps(t *testing.T) {
	apps := []config.AppConfig{
		{Name: "Enabled1", URL: "http://a:8080", Enabled: true},
		{Name: "Disabled", URL: "http://b:8080", Enabled: false},
		{Name: "Enabled2", URL: "http://c:8080", Enabled: true, Proxy: true},
	}

	result := sanitizeApps(apps, "admin")

	if len(result) != 2 {
		t.Errorf("expected 2 enabled apps, got %d", len(result))
	}

	// Verify disabled apps are excluded
	for _, app := range result {
		if app.Name == "Disabled" {
			t.Error("disabled app should not be in sanitized list")
		}
	}
}

func TestSanitizeAppsRoleFiltering(t *testing.T) {
	apps := []config.AppConfig{
		{Name: "Public", URL: "http://a:8080", Enabled: true, MinRole: ""},
		{Name: "PowerOnly", URL: "http://b:8080", Enabled: true, MinRole: "power-user"},
		{Name: "AdminOnly", URL: "http://c:8080", Enabled: true, MinRole: "admin"},
	}

	t.Run("admin sees all", func(t *testing.T) {
		result := sanitizeApps(apps, "admin")
		if len(result) != 3 {
			t.Errorf("admin should see 3 apps, got %d", len(result))
		}
	})

	t.Run("power-user sees public and power-user", func(t *testing.T) {
		result := sanitizeApps(apps, "power-user")
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
		result := sanitizeApps(apps, "user")
		if len(result) != 1 {
			t.Errorf("user should see 1 app, got %d", len(result))
		}
		if result[0].Name != "Public" {
			t.Errorf("expected 'Public', got %q", result[0].Name)
		}
	})

	t.Run("empty role disables filtering", func(t *testing.T) {
		result := sanitizeApps(apps, "")
		if len(result) != 3 {
			t.Errorf("empty role should see all 3 apps, got %d", len(result))
		}
	})
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
	cfg.Auth.APIKey = "secret-api-key"
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
	if exported.Auth.APIKey != "" {
		t.Errorf("expected APIKey to be stripped, got %q", exported.Auth.APIKey)
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
	if cfg.Auth.APIKey != "secret-api-key" {
		t.Error("ExportConfig mutated the original config's APIKey")
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

		resp := buildClientConfigResponse(cfg, "admin")

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

		resp := buildClientConfigResponse(cfg, "admin")

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

		resp := buildClientConfigResponse(cfg, "admin")

		if resp.Auth != nil {
			t.Error("expected auth to be nil when method is empty")
		}
	})

	t.Run("health config included", func(t *testing.T) {
		cfg := createTestConfig()
		cfg.Health = config.HealthConfig{Enabled: true, Interval: "30s", Timeout: "5s"}

		resp := buildClientConfigResponse(cfg, "admin")

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

		resp := buildClientConfigResponse(cfg, "admin")

		if resp.ProxyTimeout != "60s" {
			t.Errorf("expected proxy_timeout '60s', got %q", resp.ProxyTimeout)
		}
	})

	t.Run("log level preserved", func(t *testing.T) {
		cfg := createTestConfig()
		cfg.Server.LogLevel = "debug"

		resp := buildClientConfigResponse(cfg, "admin")

		if resp.LogLevel != "debug" {
			t.Errorf("expected log_level 'debug', got %q", resp.LogLevel)
		}
	})
}
