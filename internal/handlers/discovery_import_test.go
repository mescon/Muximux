package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/discovery"
)

// newTestImportHandler builds a DiscoveryHandler with an in-memory
// config + temp configPath that can persist app/gateway mutations.
func newTestImportHandler(t *testing.T, seedApps []config.AppConfig) (*DiscoveryHandler, *config.Config) {
	t.Helper()
	cfg := &config.Config{}
	cfg.Apps = seedApps
	cfg.Discovery.Docker.Endpoint = "unix:///var/run/docker.sock"
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	svc := discovery.NewService(&cfg.Discovery.Docker)
	return NewDiscoveryHandler(svc, cfg, configPath, &sync.RWMutex{}, nil), cfg
}

func postImport(t *testing.T, h *DiscoveryHandler, body any) (*ImportResult, int) {
	t.Helper()
	b, _ := json.Marshal(body)
	req := adminCtxRequest(http.MethodPost, "/api/discovery/docker/import")
	req.Body = httpBody(b)
	w := httptest.NewRecorder()
	h.ImportDocker(w, req)
	var got ImportResult
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return &got, w.Code
}

func TestImportDocker_RejectsNonPost(t *testing.T) {
	h, _ := newTestImportHandler(t, nil)
	req := adminCtxRequest(http.MethodGet, "/api/discovery/docker/import")
	w := httptest.NewRecorder()
	h.ImportDocker(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestImportDocker_EmptyItemsIsNoOp(t *testing.T) {
	h, _ := newTestImportHandler(t, nil)
	res, code := postImport(t, h, ImportRequest{})
	if code != http.StatusOK {
		t.Errorf("code = %d", code)
	}
	if !res.Success || len(res.Items) != 0 {
		t.Errorf("got %+v", res)
	}
}

func TestImportDocker_CreatesAppOnly(t *testing.T) {
	h, cfg := newTestImportHandler(t, nil)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key:      "name:sonarr",
			Strategy: "container_ip",
			App: &ClientAppConfig{
				Name:    "Sonarr",
				URL:     "http://10.0.0.5:8989",
				Enabled: true,
			},
		}},
	})
	if !res.Success {
		t.Fatalf("Success = false: %+v", res)
	}
	if len(cfg.Apps) != 1 {
		t.Fatalf("apps = %d, want 1", len(cfg.Apps))
	}
	app := cfg.Apps[0]
	if app.Name != "Sonarr" || app.URL != "http://10.0.0.5:8989" {
		t.Errorf("app shape: %+v", app)
	}
	if app.DockerKey != "name:sonarr" {
		t.Errorf("DockerKey = %q, want name:sonarr", app.DockerKey)
	}
	if app.DockerEndpoint != "unix:///var/run/docker.sock" {
		t.Errorf("DockerEndpoint = %q", app.DockerEndpoint)
	}
	if app.DockerStrategy != "container_ip" {
		t.Errorf("DockerStrategy = %q", app.DockerStrategy)
	}
}

func TestImportDocker_CreatesAppAndGatewayPaired(t *testing.T) {
	h, cfg := newTestImportHandler(t, nil)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key:      "name:sonarr",
			Strategy: "container_ip",
			App: &ClientAppConfig{
				Name:    "Sonarr",
				URL:     "http://10.0.0.5:8989",
				Enabled: true,
			},
			Gateway: &config.GatewaySite{
				Domain:     "sonarr.example.com",
				BackendURL: "http://10.0.0.5:8989",
			},
		}},
	})
	if !res.Success {
		t.Fatalf("Success = false: %+v", res)
	}
	if len(cfg.Apps) != 1 || len(cfg.Server.GatewaySites) != 1 {
		t.Fatalf("counts wrong: apps=%d sites=%d", len(cfg.Apps), len(cfg.Server.GatewaySites))
	}
	site := cfg.Server.GatewaySites[0]
	if site.AppName != "Sonarr" {
		t.Errorf("site.AppName = %q, want Sonarr (auto-linked)", site.AppName)
	}
	if site.DockerKey != "name:sonarr" {
		t.Errorf("site.DockerKey = %q", site.DockerKey)
	}
}

func TestImportDocker_CreatesGatewayOnly(t *testing.T) {
	h, cfg := newTestImportHandler(t, nil)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key:      "name:sonarr",
			Strategy: "container_ip",
			Gateway: &config.GatewaySite{
				Domain:     "sonarr.example.com",
				BackendURL: "http://10.0.0.5:8989",
			},
		}},
	})
	if !res.Success {
		t.Fatalf("Success = false: %+v", res)
	}
	if len(cfg.Apps) != 0 {
		t.Errorf("expected zero apps, got %d", len(cfg.Apps))
	}
	if len(cfg.Server.GatewaySites) != 1 {
		t.Fatalf("sites = %d, want 1", len(cfg.Server.GatewaySites))
	}
	if cfg.Server.GatewaySites[0].AppName != "" {
		t.Errorf("standalone site should have empty AppName, got %q", cfg.Server.GatewaySites[0].AppName)
	}
}

func TestImportDocker_SkipIfExistsByDockerKey(t *testing.T) {
	// Re-importing a container that's already tracked should be a no-op.
	h, cfg := newTestImportHandler(t, []config.AppConfig{
		{Name: "Sonarr", URL: "http://existing", DockerKey: "name:sonarr", DockerEndpoint: "unix:///var/run/docker.sock"},
	})
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key:      "name:sonarr",
			Strategy: "container_ip",
			App:      &ClientAppConfig{Name: "Sonarr-renamed", URL: "http://new"},
		}},
	})
	if !res.Success || res.Items[0].Status != "skipped_exists" {
		t.Errorf("got %+v", res)
	}
	// Existing app untouched.
	if cfg.Apps[0].URL != "http://existing" {
		t.Errorf("existing app got mutated: %+v", cfg.Apps[0])
	}
}

func TestImportDocker_NameCollisionInBatch(t *testing.T) {
	h, _ := newTestImportHandler(t, nil)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{
			{
				Key:      "name:a",
				Strategy: "container_ip",
				App:      &ClientAppConfig{Name: "Sonarr", URL: "http://a"},
			},
			{
				Key:      "name:b",
				Strategy: "container_ip",
				App:      &ClientAppConfig{Name: "Sonarr", URL: "http://b"}, // collides
			},
		},
	})
	if res.Success {
		t.Errorf("Success should be false on name collision")
	}
	if res.Items[1].Status != "name_collision_in_batch" {
		t.Errorf("item[1].Status = %q, want name_collision_in_batch", res.Items[1].Status)
	}
	if res.Items[0].Status != "aborted_by_batch_failure" {
		t.Errorf("item[0].Status = %q, want aborted_by_batch_failure", res.Items[0].Status)
	}
}

func TestImportDocker_ItemFailureRollsBackPredecessors(t *testing.T) {
	h, cfg := newTestImportHandler(t, nil)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{
			{
				Key:      "name:a",
				Strategy: "container_ip",
				App:      &ClientAppConfig{Name: "AppA", URL: "http://a"},
			},
			{
				Key:      "name:b",
				Strategy: "container_ip",
				App:      &ClientAppConfig{Name: "", URL: "http://b"}, // missing name -> validation_failed
			},
			{
				Key:      "name:c",
				Strategy: "container_ip",
				App:      &ClientAppConfig{Name: "AppC", URL: "http://c"},
			},
		},
	})
	if res.Success {
		t.Errorf("Success should be false")
	}
	if res.Items[0].Status != "aborted_by_batch_failure" {
		t.Errorf("item[0].Status = %q", res.Items[0].Status)
	}
	if res.Items[1].Status != "validation_failed" {
		t.Errorf("item[1].Status = %q", res.Items[1].Status)
	}
	if res.Items[2].Status != "aborted_by_batch_failure" {
		t.Errorf("item[2].Status = %q", res.Items[2].Status)
	}
	// Nothing committed.
	if len(cfg.Apps) != 0 {
		t.Errorf("expected zero apps, got %d", len(cfg.Apps))
	}
}

func TestImportDocker_RejectsAppWithoutNameOrURL(t *testing.T) {
	h, _ := newTestImportHandler(t, nil)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key:      "name:x",
			Strategy: "container_ip",
			App:      &ClientAppConfig{Name: "X"}, // no URL
		}},
	})
	if res.Items[0].Status != "validation_failed" {
		t.Errorf("status = %q", res.Items[0].Status)
	}
}

func TestImportDocker_RejectsItemWithNoAppOrGateway(t *testing.T) {
	h, _ := newTestImportHandler(t, nil)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{Key: "name:x", Strategy: "container_ip"}},
	})
	if res.Items[0].Status != "validation_failed" {
		t.Errorf("status = %q", res.Items[0].Status)
	}
}

// Routing-mode tests cover the per-row "Direct / Proxy / Gateway"
// radio added in Phase G. Each mode shapes App.URL, App.Proxy, and
// the App's tracking fields differently. The default ("" or
// "direct") matches the pre-Phase-G behavior so existing imports
// stay backward compatible.

func TestImportDocker_Routing_DirectIsDefault(t *testing.T) {
	h, cfg := newTestImportHandler(t, nil)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key:      "name:sonarr",
			Strategy: "container_ip",
			App: &ClientAppConfig{
				Name:    "Sonarr",
				URL:     "http://10.0.0.5:8989",
				Enabled: true,
			},
			// Routing intentionally omitted -> defaults to "direct".
		}},
	})
	if !res.Success {
		t.Fatalf("Success=false: %+v", res)
	}
	app := cfg.Apps[0]
	if app.URL != "http://10.0.0.5:8989" {
		t.Errorf("default routing should leave URL untouched; got %q", app.URL)
	}
	if app.Proxy {
		t.Errorf("default routing should not enable proxy")
	}
	if app.DockerKey != "name:sonarr" {
		t.Errorf("default routing should keep tracking; got DockerKey=%q", app.DockerKey)
	}
}

func TestImportDocker_Routing_ProxySetsProxyFlag(t *testing.T) {
	h, cfg := newTestImportHandler(t, nil)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key:      "name:sonarr",
			Strategy: "container_ip",
			Routing:  "proxy",
			App: &ClientAppConfig{
				Name:    "Sonarr",
				URL:     "http://10.0.0.5:8989",
				Enabled: true,
			},
		}},
	})
	if !res.Success {
		t.Fatalf("Success=false: %+v", res)
	}
	app := cfg.Apps[0]
	if !app.Proxy {
		t.Errorf("routing=proxy should set App.Proxy=true; got %+v", app)
	}
	if app.URL != "http://10.0.0.5:8989" {
		t.Errorf("routing=proxy should keep URL as upstream; got %q", app.URL)
	}
	if app.DockerKey != "name:sonarr" {
		t.Errorf("routing=proxy still tracks the app; got DockerKey=%q", app.DockerKey)
	}
}

func TestImportDocker_Routing_GatewayPointsAppAtDomain(t *testing.T) {
	h, cfg := newTestImportHandler(t, nil)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key:      "name:sonarr",
			Strategy: "container_ip",
			Routing:  "gateway",
			App: &ClientAppConfig{
				Name:    "Sonarr",
				URL:     "http://10.0.0.5:8989",
				Enabled: true,
			},
			Gateway: &config.GatewaySite{
				Domain:     "sonarr.example.com",
				BackendURL: "http://10.0.0.5:8989",
				TLS:        "auto",
			},
		}},
	})
	if !res.Success {
		t.Fatalf("Success=false: %+v", res)
	}
	app := cfg.Apps[0]
	if app.URL != "https://sonarr.example.com" {
		t.Errorf("routing=gateway should rewrite URL to gateway domain; got %q", app.URL)
	}
	if app.Proxy {
		t.Errorf("routing=gateway should not set Proxy")
	}
	// App is NOT tracked - the gateway is the docker-managed entry.
	if app.DockerKey != "" || app.DockerEndpoint != "" || app.DockerStrategy != "" {
		t.Errorf("routing=gateway should leave app un-tracked; got %+v", app)
	}
	site := cfg.Server.GatewaySites[0]
	if site.DockerKey != "name:sonarr" {
		t.Errorf("gateway should be tracked instead; got DockerKey=%q", site.DockerKey)
	}
	if site.AppName != "Sonarr" {
		t.Errorf("gateway -> app linkage should auto-fill AppName; got %q", site.AppName)
	}
}

func TestImportDocker_Routing_GatewayWithTLSNoneUsesHTTP(t *testing.T) {
	// TLS=none on the gateway means Caddy serves plain HTTP, so
	// the menu URL should also be http:// not https://.
	h, cfg := newTestImportHandler(t, nil)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key:      "name:foo",
			Strategy: "container_ip",
			Routing:  "gateway",
			App:      &ClientAppConfig{Name: "Foo", URL: "http://10.0.0.5:80", Enabled: true},
			Gateway:  &config.GatewaySite{Domain: "foo.local", BackendURL: "http://10.0.0.5:80", TLS: "none"},
		}},
	})
	if !res.Success {
		t.Fatalf("Success=false: %+v", res)
	}
	if cfg.Apps[0].URL != "http://foo.local" {
		t.Errorf("TLS=none should produce http://; got %q", cfg.Apps[0].URL)
	}
}

func TestImportDocker_Routing_GatewayWithoutGatewaySiteFails(t *testing.T) {
	h, _ := newTestImportHandler(t, nil)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key:      "name:foo",
			Strategy: "container_ip",
			Routing:  "gateway",
			App:      &ClientAppConfig{Name: "Foo", URL: "http://10.0.0.5:80", Enabled: true},
			// no Gateway field - the routing decision is contradictory
		}},
	})
	if res.Success {
		t.Fatalf("expected failure; got success")
	}
	if res.Items[0].Status != "validation_failed" {
		t.Errorf("status = %q, want validation_failed", res.Items[0].Status)
	}
}

func TestImportDocker_Routing_UnknownValueIsRejected(t *testing.T) {
	h, _ := newTestImportHandler(t, nil)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key:      "name:foo",
			Strategy: "container_ip",
			Routing:  "magic",
			App:      &ClientAppConfig{Name: "Foo", URL: "http://10.0.0.5:80", Enabled: true},
		}},
	})
	if res.Success {
		t.Fatalf("expected failure; got success")
	}
	if res.Items[0].Status != "validation_failed" {
		t.Errorf("status = %q", res.Items[0].Status)
	}
}

// TestImportDocker_FiresOnConfigSavedAfterSuccess pins the
// reverse-proxy route-rebuild hook the import flow exposes via
// SetOnConfigSave. Without this callback firing, a freshly-imported
// App.Proxy=true entry shows up in /api/apps but /proxy/<slug>/
// returns 404 until the next restart. The test counts callback
// invocations after a successful import + after a validation-failed
// import (should only fire on success).
func TestImportDocker_FiresOnConfigSavedAfterSuccess(t *testing.T) {
	h, _ := newTestImportHandler(t, nil)
	fired := 0
	h.SetOnConfigSave(func() { fired++ })

	// Success path -> callback should fire exactly once.
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key: "name:c", Strategy: "container_ip", Routing: "proxy",
			App: &ClientAppConfig{Name: "C", URL: "http://10.0.0.5:80", Enabled: true},
		}},
	})
	if !res.Success {
		t.Fatalf("import failed unexpectedly: %+v", res)
	}
	if fired != 1 {
		t.Errorf("expected callback to fire once after successful import; got %d", fired)
	}

	// Validation failure -> callback must NOT fire (no config was
	// saved). A regression that moves the call to before the Save
	// would trigger spurious rebuilds and surface as a false success.
	res2, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key:     "name:bad",
			Routing: "magic-not-a-real-mode",
			App:     &ClientAppConfig{Name: "Bad", URL: "http://10.0.0.6:80", Enabled: true},
		}},
	})
	if res2.Success {
		t.Fatalf("expected validation failure; got success")
	}
	if fired != 1 {
		t.Errorf("callback should not fire on validation failure; total fires=%d", fired)
	}
}

// httpBody is defined in discovery_test.go in this package; reused.
var _ = bytes.NewBuffer
