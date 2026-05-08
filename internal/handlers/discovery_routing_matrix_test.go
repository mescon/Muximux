package handlers

import (
	"sync"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/discovery"
)

// Routing-matrix tests cover every meaningful combination an
// operator can select in the Discover modal. Each test names the
// scenario in a way the matrix in dev/docker-discovery-plan.md and
// the Wiki page can cross-reference.
//
// Combinations:
//
//	| # | Add to menu | Add gateway | Routing  | Expected outcome           |
//	|---|-------------|-------------|----------|----------------------------|
//	| 1 | off         | off         | -        | rejected (must set one)    |
//	| 2 | off         | on          | -        | gateway only, tracked      |
//	| 3 | on          | off         | direct   | app, internal URL, tracked |
//	| 4 | on          | off         | proxy    | app, proxy=true, tracked   |
//	| 5 | on          | off         | gateway  | rejected (gateway required)|
//	| 6 | on          | on          | direct   | app+gateway, both tracked  |
//	| 7 | on          | on          | proxy    | app(proxy=true)+gateway    |
//	| 8 | on          | on          | gateway  | app(URL=domain)+gateway,   |
//	|   |             |             |          | app NOT tracked            |
//
// Plus edge cases:
//   - Empty routing string normalises to "direct"
//   - TLS=none on gateway -> http:// scheme on app URL when routing=gateway

func newRoutingMatrixHandler(t *testing.T) (*DiscoveryHandler, *config.Config) {
	t.Helper()
	cfg := &config.Config{}
	cfg.Discovery.Docker.Endpoint = "unix:///var/run/docker.sock"
	configPath := t.TempDir() + "/config.yaml"
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	svc := discovery.NewService(&cfg.Discovery.Docker)
	return NewDiscoveryHandler(svc, cfg, configPath, &sync.RWMutex{}, nil), cfg
}

func TestRoutingMatrix_1_NoAppNoGateway_Rejected(t *testing.T) {
	h, _ := newRoutingMatrixHandler(t)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key:      "name:c",
			Strategy: "container_ip",
			// no App, no Gateway
		}},
	})
	if res.Success {
		t.Fatalf("expected failure")
	}
	if res.Items[0].Status != "validation_failed" {
		t.Errorf("status = %q", res.Items[0].Status)
	}
}

func TestRoutingMatrix_2_GatewayOnly_TrackedOnGateway(t *testing.T) {
	h, cfg := newRoutingMatrixHandler(t)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key:      "name:c",
			Strategy: "container_ip",
			Gateway:  &config.GatewaySite{Domain: "c.example.com", BackendURL: "http://10.0.0.5:80", TLS: "auto"},
		}},
	})
	if !res.Success {
		t.Fatalf("Success=false: %+v", res)
	}
	if len(cfg.Apps) != 0 {
		t.Errorf("apps = %d, want 0", len(cfg.Apps))
	}
	if len(cfg.Server.GatewaySites) != 1 {
		t.Fatalf("sites = %d, want 1", len(cfg.Server.GatewaySites))
	}
	if cfg.Server.GatewaySites[0].DockerKey != "name:c" {
		t.Errorf("gateway not tracked: %+v", cfg.Server.GatewaySites[0])
	}
	if cfg.Server.GatewaySites[0].AppName != "" {
		t.Errorf("gateway-only should leave AppName empty; got %q", cfg.Server.GatewaySites[0].AppName)
	}
}

func TestRoutingMatrix_3_AppOnly_Direct(t *testing.T) {
	h, cfg := newRoutingMatrixHandler(t)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key: "name:c", Strategy: "container_ip",
			Routing: "direct",
			App:     &ClientAppConfig{Name: "C", URL: "http://10.0.0.5:80", Enabled: true},
		}},
	})
	if !res.Success {
		t.Fatalf("Success=false: %+v", res)
	}
	app := cfg.Apps[0]
	if app.URL != "http://10.0.0.5:80" || app.Proxy {
		t.Errorf("app shape wrong: %+v", app)
	}
	if app.DockerKey != "name:c" {
		t.Errorf("app should be tracked: %+v", app)
	}
}

func TestRoutingMatrix_4_AppOnly_Proxy(t *testing.T) {
	h, cfg := newRoutingMatrixHandler(t)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key: "name:c", Strategy: "container_ip",
			Routing: "proxy",
			App:     &ClientAppConfig{Name: "C", URL: "http://10.0.0.5:80", Enabled: true},
		}},
	})
	if !res.Success {
		t.Fatalf("Success=false: %+v", res)
	}
	app := cfg.Apps[0]
	if !app.Proxy {
		t.Errorf("Proxy=false; expected true")
	}
	if app.URL != "http://10.0.0.5:80" {
		t.Errorf("URL should remain internal upstream: %q", app.URL)
	}
	if app.DockerKey != "name:c" {
		t.Errorf("app should still be tracked")
	}
}

func TestRoutingMatrix_5_GatewayRoutingNoSite_Rejected(t *testing.T) {
	h, _ := newRoutingMatrixHandler(t)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key: "name:c", Strategy: "container_ip",
			Routing: "gateway",
			App:     &ClientAppConfig{Name: "C", URL: "http://10.0.0.5:80", Enabled: true},
			// no Gateway -> routing=gateway is unsatisfiable
		}},
	})
	if res.Success {
		t.Fatalf("expected failure")
	}
	if res.Items[0].Status != "validation_failed" {
		t.Errorf("status = %q", res.Items[0].Status)
	}
}

func TestRoutingMatrix_6_AppPlusGateway_Direct(t *testing.T) {
	h, cfg := newRoutingMatrixHandler(t)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key: "name:c", Strategy: "container_ip",
			Routing: "direct",
			App:     &ClientAppConfig{Name: "C", URL: "http://10.0.0.5:80", Enabled: true},
			Gateway: &config.GatewaySite{Domain: "c.example.com", BackendURL: "http://10.0.0.5:80", TLS: "auto"},
		}},
	})
	if !res.Success {
		t.Fatalf("Success=false: %+v", res)
	}
	app := cfg.Apps[0]
	site := cfg.Server.GatewaySites[0]
	// Direct mode: app URL stays internal; both app + gateway tracked.
	if app.URL != "http://10.0.0.5:80" || app.Proxy {
		t.Errorf("app: %+v", app)
	}
	if app.DockerKey != "name:c" {
		t.Errorf("app should be tracked")
	}
	if site.DockerKey != "name:c" {
		t.Errorf("site should be tracked")
	}
	if site.AppName != "C" {
		t.Errorf("auto-link AppName = %q", site.AppName)
	}
}

func TestRoutingMatrix_7_AppPlusGateway_Proxy(t *testing.T) {
	h, cfg := newRoutingMatrixHandler(t)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key: "name:c", Strategy: "container_ip",
			Routing: "proxy",
			App:     &ClientAppConfig{Name: "C", URL: "http://10.0.0.5:80", Enabled: true},
			Gateway: &config.GatewaySite{Domain: "c.example.com", BackendURL: "http://10.0.0.5:80", TLS: "auto"},
		}},
	})
	if !res.Success {
		t.Fatalf("Success=false: %+v", res)
	}
	app := cfg.Apps[0]
	site := cfg.Server.GatewaySites[0]
	if !app.Proxy {
		t.Errorf("app.Proxy = false; want true")
	}
	if app.URL != "http://10.0.0.5:80" {
		t.Errorf("app URL = %q; want internal upstream", app.URL)
	}
	if app.DockerKey != "name:c" {
		t.Errorf("app should be tracked")
	}
	if site.DockerKey != "name:c" {
		t.Errorf("site should be tracked")
	}
}

func TestRoutingMatrix_8_AppPlusGateway_GatewayDomain(t *testing.T) {
	h, cfg := newRoutingMatrixHandler(t)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key: "name:c", Strategy: "container_ip",
			Routing: "gateway",
			App:     &ClientAppConfig{Name: "C", URL: "http://10.0.0.5:80", Enabled: true},
			Gateway: &config.GatewaySite{Domain: "c.example.com", BackendURL: "http://10.0.0.5:80", TLS: "auto"},
		}},
	})
	if !res.Success {
		t.Fatalf("Success=false: %+v", res)
	}
	app := cfg.Apps[0]
	site := cfg.Server.GatewaySites[0]
	// Gateway mode: app URL points at the public domain; app is
	// NOT tracked (gateway is the docker-managed entry).
	if app.URL != "https://c.example.com" {
		t.Errorf("app URL = %q; want gateway domain", app.URL)
	}
	if app.Proxy {
		t.Errorf("Proxy should be false in gateway mode")
	}
	if app.DockerKey != "" {
		t.Errorf("app should NOT be tracked in gateway mode; got %q", app.DockerKey)
	}
	if site.DockerKey != "name:c" {
		t.Errorf("gateway should be the docker-managed entry; got %q", site.DockerKey)
	}
	if site.AppName != "C" {
		t.Errorf("auto-link broken: AppName = %q", site.AppName)
	}
}

func TestRoutingMatrix_EmptyRoutingNormalisesToDirect(t *testing.T) {
	h, cfg := newRoutingMatrixHandler(t)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key: "name:c", Strategy: "container_ip",
			// Routing intentionally omitted
			App: &ClientAppConfig{Name: "C", URL: "http://10.0.0.5:80", Enabled: true},
		}},
	})
	if !res.Success {
		t.Fatalf("Success=false: %+v", res)
	}
	if cfg.Apps[0].Proxy {
		t.Errorf("empty routing should default to direct (proxy=false)")
	}
	if cfg.Apps[0].DockerKey != "name:c" {
		t.Errorf("default routing should track")
	}
}

func TestRoutingMatrix_GatewayWithTLSNoneUsesHTTP(t *testing.T) {
	h, cfg := newRoutingMatrixHandler(t)
	res, _ := postImport(t, h, ImportRequest{
		Items: []ImportItem{{
			Key: "name:c", Strategy: "container_ip",
			Routing: "gateway",
			App:     &ClientAppConfig{Name: "C", URL: "http://10.0.0.5:80", Enabled: true},
			Gateway: &config.GatewaySite{Domain: "c.local", BackendURL: "http://10.0.0.5:80", TLS: "none"},
		}},
	})
	if !res.Success {
		t.Fatalf("Success=false: %+v", res)
	}
	if cfg.Apps[0].URL != "http://c.local" {
		t.Errorf("TLS=none should produce http://; got %q", cfg.Apps[0].URL)
	}
}
