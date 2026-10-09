package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
)

// A PUT that creates a live app with a quarantined app's name supersedes
// the quarantined entry: the file holds exactly one app with that name.
func TestSaveConfig_LiveAppSupersedesQuarantinedApp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(`
apps:
  - name: Manual
    url: http://manual:80
    enabled: true
  - name: Vaultwarden
    url: ""
    enabled: true
    docker_key: "label:vw"
    docker_auto: true
`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if len(cfg.Quarantined()) != 1 {
		t.Fatalf("quarantined = %+v", cfg.Quarantined())
	}
	handler := NewAPIHandler(cfg, path, &sync.RWMutex{})

	resp := buildClientConfigResponse(cfg, auth.RoleAdmin, nil)
	wire, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var update ClientConfigUpdate
	if err := json.Unmarshal(wire, &update); err != nil {
		t.Fatal(err)
	}
	update.Apps = append(update.Apps, ClientAppConfig{Name: "Vaultwarden", URL: "http://vw:80", Enabled: true})
	body, _ := json.Marshal(update)

	w := httptest.NewRecorder()
	handler.SaveConfig(w, httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("SaveConfig = %d: %s", w.Code, w.Body.String())
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(regexp.MustCompile(`(?m)^\s*- name: Vaultwarden\s*$`).FindAll(raw, -1)); n != 1 {
		t.Fatalf("file holds %d Vaultwarden apps:\n%s", n, raw)
	}
	if len(cfg.Quarantined()) != 0 {
		t.Fatalf("superseded entry still quarantined: %+v", cfg.Quarantined())
	}
	again, err := config.Load(path)
	if err != nil || len(again.Quarantined()) != 0 || len(again.Apps) != 2 {
		t.Fatalf("reload: err=%v apps=%+v q=%+v", err, again.Apps, again.Quarantined())
	}
}
