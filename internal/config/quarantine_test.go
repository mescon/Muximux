package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func writeTempConfig(t *testing.T, yml string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(yml), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const autoNoURLYAML = `
apps:
  - name: Manual
    url: http://manual:80
    enabled: true
  - name: Vaultwarden
    url: ""
    enabled: true
    docker_key: "name:authentik_server.1.abc"
    docker_endpoint: unix:///var/run/docker.sock
    docker_strategy: container_dns
    docker_auto: true
`

func TestLoad_QuarantinesInvalidAutoImportedApp(t *testing.T) { // F-01, repro TestIssue499_UnlabeledNoPortImportedWithEmptyURL part 2
	path := writeTempConfig(t, autoNoURLYAML)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	if len(cfg.Apps) != 1 || cfg.Apps[0].Name != "Manual" {
		t.Fatalf("live apps = %+v", cfg.Apps)
	}
	q := cfg.Quarantined()
	if len(q) != 1 || q[0].Key != "name:authentik_server.1.abc" || q[0].Kind != "app" || !strings.Contains(q[0].Reason, "url is required") {
		t.Fatalf("quarantined = %+v", q)
	}
	// Save keeps the entry in the file.
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "name:authentik_server.1.abc") {
		t.Fatalf("quarantined app dropped from file:\n%s", raw)
	}
	again, err := Load(path)
	if err != nil || len(again.Quarantined()) != 1 || len(again.Apps) != 1 {
		t.Fatalf("second load: err=%v apps=%d quarantined=%d", err, len(again.Apps), len(again.Quarantined()))
	}
}

func TestLoad_QuarantinesAutoAppOnSlugCollision(t *testing.T) { // F-10 at load
	path := writeTempConfig(t, `
apps:
  - name: Home Assistant
    url: http://ha:8123
    enabled: true
  - name: Home-Assistant
    url: http://ha2:8123
    enabled: true
    docker_key: "label:ha"
    docker_auto: true
`)
	cfg, err := Load(path)
	if err != nil || len(cfg.Apps) != 1 || len(cfg.Quarantined()) != 1 || !strings.Contains(cfg.Quarantined()[0].Reason, "slug") {
		t.Fatalf("err=%v apps=%+v q=%+v", err, cfg.Apps, cfg.Quarantined())
	}
}

// A docker-owned app listed before the operator's own app with the same
// slug is the one set aside, not the operator's app.
func TestLoad_QuarantinesAutoAppListedBeforeCollidingManualApp(t *testing.T) {
	path := writeTempConfig(t, `
apps:
  - name: Home-Assistant
    url: http://ha2:8123
    enabled: true
    docker_key: "label:ha"
    docker_auto: true
  - name: Home Assistant
    url: http://ha:8123
    enabled: true
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if len(cfg.Apps) != 1 || cfg.Apps[0].Name != "Home Assistant" {
		t.Fatalf("live apps = %+v", cfg.Apps)
	}
	if q := cfg.Quarantined(); len(q) != 1 || q[0].Key != "label:ha" {
		t.Fatalf("quarantined = %+v", q)
	}
}

func TestLoad_QuarantinesSiteOfQuarantinedApp(t *testing.T) {
	path := writeTempConfig(t, `
server:
  gateway_sites:
    - domain: vw.example.com
      backend_url: ""
      app_name: Vaultwarden
      docker_key: "label:vw"
apps:
  - name: Vaultwarden
    url: ""
    enabled: true
    docker_key: "label:vw"
    docker_auto: true
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	q := cfg.Quarantined()
	if len(q) != 2 || q[1].Kind != "gateway" || q[1].Name != "vw.example.com" {
		t.Fatalf("quarantined = %+v", q)
	}
	if len(cfg.Server.GatewaySites) != 0 {
		t.Fatalf("site still live: %+v", cfg.Server.GatewaySites)
	}
	// Save writes both the app and the site back.
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	again, err := Load(path)
	if err != nil || len(again.Quarantined()) != 2 {
		t.Fatalf("second load: err=%v q=%+v", err, again.Quarantined())
	}
}

// Docker-owned sites of live apps are quarantined when they fail validation
// or duplicate a domain already in use; the valid ones stay live.
func TestLoad_QuarantinesInvalidSitesOfLiveAutoApps(t *testing.T) {
	path := writeTempConfig(t, `
server:
  gateway_sites:
    - domain: dup.example.com
      backend_url: http://a:80
      docker_key: "label:a"
    - domain: ok.example.com
      backend_url: http://a:80
      app_name: A
      docker_key: "label:a"
    - domain: bad.example.com
      backend_url: "ftp://b"
      docker_key: "label:b"
    - domain: DUP.example.com
      backend_url: http://manual:80
apps:
  - name: A
    url: http://a:80
    enabled: true
    docker_key: "label:a"
    docker_auto: true
  - name: B
    url: http://b:80
    enabled: true
    docker_key: "label:b"
    docker_auto: true
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if len(cfg.Apps) != 2 {
		t.Fatalf("live apps = %+v", cfg.Apps)
	}
	var live []string
	for i := range cfg.Server.GatewaySites {
		live = append(live, cfg.Server.GatewaySites[i].Domain)
	}
	if strings.Join(live, ",") != "ok.example.com,DUP.example.com" {
		t.Fatalf("live sites = %v", live)
	}
	q := cfg.Quarantined()
	if len(q) != 2 || q[0].Name != "dup.example.com" || !strings.Contains(q[0].Reason, "duplicate domain") ||
		q[1].Name != "bad.example.com" || !strings.Contains(q[1].Reason, "http or https") {
		t.Fatalf("quarantined = %+v", q)
	}
}

// Ruling 10: a manual site (no docker_key) that names a quarantined app
// loses its app_name instead of failing Load.
func TestLoad_ManualSiteNamingQuarantinedAppIsUnlinked(t *testing.T) {
	path := writeTempConfig(t, `
server:
  gateway_sites:
    - domain: vw.example.com
      backend_url: http://vw:80
      app_name: Vaultwarden
apps:
  - name: Manual
    url: http://manual:80
    enabled: true
  - name: Vaultwarden
    url: ""
    enabled: true
    docker_key: "label:vw"
    docker_auto: true
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if len(cfg.Server.GatewaySites) != 1 || cfg.Server.GatewaySites[0].AppName != "" {
		t.Fatalf("sites = %+v", cfg.Server.GatewaySites)
	}
	if q := cfg.Quarantined(); len(q) != 1 || q[0].Kind != "app" {
		t.Fatalf("quarantined = %+v (the manual site itself must stay live)", q)
	}
}

func TestLoad_ManualInvalidAppsStillFail(t *testing.T) { // Review Focus 3
	cases := map[string]string{
		"manual app": `
apps:
  - name: Broken
    url: ""
    enabled: true
`,
		"manually imported tracked app": `
apps:
  - name: Broken
    url: "javascript:alert(1)"
    enabled: true
    docker_key: "label:x"
    docker_managed_url: "javascript:alert(1)"
`,
		"auto app with hand-edited url": `
apps:
  - name: Broken
    url: "ftp://nope"
    enabled: true
    docker_key: "label:x"
    docker_managed_url: "http://old:80"
    docker_auto: true
`,
	}
	for name, yml := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeTempConfig(t, yml)); err == nil {
				t.Fatalf("Load() = nil, want validation error")
			}
		})
	}
}

func TestQuarantine_DropAndHas(t *testing.T) {
	cfg := defaultConfig()
	cfg.QuarantineApp(&AppConfig{Name: "A", DockerKey: "label:a", DockerAutoImported: true}, "url is required")
	cfg.QuarantineSite(&GatewaySite{Domain: "a.x", DockerKey: "label:a"}, "its app is quarantined")
	if !cfg.HasQuarantined("label:a") || cfg.HasQuarantined("label:b") {
		t.Fatal("HasQuarantined wrong")
	}
	if n := cfg.DropQuarantined("label:a"); n != 2 || len(cfg.Quarantined()) != 0 {
		t.Fatalf("DropQuarantined = %d, left %d", n, len(cfg.Quarantined()))
	}
	snap := cfg.QuarantineSnapshot()
	cfg.QuarantineApp(&AppConfig{Name: "B", DockerKey: "label:b"}, "x")
	cfg.RestoreQuarantine(snap)
	if len(cfg.Quarantined()) != 0 {
		t.Fatal("RestoreQuarantine did not restore")
	}
}

func TestQuarantine_DropKeepsOtherKeys(t *testing.T) {
	cfg := defaultConfig()
	cfg.QuarantineApp(&AppConfig{Name: "A", DockerKey: "label:a"}, "r")
	cfg.QuarantineApp(&AppConfig{Name: "B", DockerKey: "label:b"}, "r")
	snap := cfg.QuarantineSnapshot()
	if n := cfg.DropQuarantined("label:a"); n != 1 {
		t.Fatalf("DropQuarantined = %d", n)
	}
	if q := cfg.Quarantined(); len(q) != 1 || q[0].Key != "label:b" {
		t.Fatalf("left = %+v", q)
	}
	// The snapshot is unaffected by the drop.
	cfg.RestoreQuarantine(snap)
	if len(cfg.Quarantined()) != 2 {
		t.Fatalf("restored = %+v", cfg.Quarantined())
	}
}

func TestFileView_DoesNotAliasLiveApps(t *testing.T) {
	cfg := defaultConfig()
	cfg.Apps = make([]AppConfig, 1, 4) // spare capacity: append must not write into it
	cfg.Apps[0] = AppConfig{Name: "Live", URL: "http://x", Enabled: true}
	cfg.QuarantineApp(&AppConfig{Name: "Q", DockerKey: "label:q", DockerAutoImported: true}, "r")
	v := cfg.fileView()
	if len(v.Apps) != 2 || len(cfg.Apps) != 1 || cfg.Apps[:2][1].Name == "Q" {
		t.Fatalf("fileView aliased the live slice: live=%+v view=%+v", cfg.Apps[:2], v.Apps)
	}
}

func TestQuarantineReason(t *testing.T) {
	slugs := map[string]string{"home-assistant": "Home Assistant"}
	if r := quarantineReason(&AppConfig{Name: "X", URL: ""}, slugs); !strings.Contains(r, "url is required") {
		t.Fatalf("reason = %q", r)
	}
	if r := quarantineReason(&AppConfig{Name: "Home-Assistant", URL: "http://h:1", Enabled: true}, slugs); !strings.Contains(r, "slug") {
		t.Fatalf("reason = %q", r)
	}
	if r := quarantineReason(&AppConfig{Name: "Home-Assistant", URL: "http://h:1"}, slugs); r != "" {
		t.Fatalf("a disabled app does not collide: %q", r)
	}
	rememberSlug(slugs, &AppConfig{Name: "Sonarr", Enabled: true})
	rememberSlug(slugs, &AppConfig{Name: "sonarr", Enabled: true})
	if slugs["sonarr"] != "Sonarr" {
		t.Fatalf("first kept name must win: %v", slugs)
	}
}

func TestLogQuarantined(t *testing.T) {
	cfg := defaultConfig()
	cfg.QuarantineApp(&AppConfig{Name: "A", DockerKey: "label:a", DockerAutoImported: true}, "r")
	cfg.QuarantineSite(&GatewaySite{Domain: "a.x", DockerKey: "label:a"}, "r")
	if n := LogQuarantined(cfg.Quarantined()); n != 2 {
		t.Fatalf("LogQuarantined = %d, want 2", n)
	}
	if n := LogQuarantined(nil); n != 0 {
		t.Fatalf("LogQuarantined(nil) = %d", n)
	}
}

// A quarantined app that shares its name with a live app never unlinks a
// site from the live app, and Save drops the superseded entry so the file
// holds one "Sonarr".
func TestLoad_SameNameLiveAppKeepsSiteLink(t *testing.T) {
	path := writeTempConfig(t, `
server:
  gateway_sites:
    - domain: sonarr.example.com
      backend_url: http://sonarr:8989
      app_name: Sonarr
apps:
  - name: Sonarr
    url: http://sonarr:8989
    enabled: true
  - name: Sonarr
    url: http://sonarr2:8989
    enabled: true
    docker_key: "label:sonarr"
    docker_auto: true
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if len(cfg.Server.GatewaySites) != 1 || cfg.Server.GatewaySites[0].AppName != "Sonarr" {
		t.Fatalf("site lost its link: %+v", cfg.Server.GatewaySites)
	}
	if q := cfg.Quarantined(); len(q) != 1 || q[0].Key != "label:sonarr" {
		t.Fatalf("quarantined = %+v", q)
	}
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if n := len(regexp.MustCompile(`(?m)^\s*- name: Sonarr\s*$`).FindAll(raw, -1)); n != 1 {
		t.Fatalf("file holds %d Sonarr apps:\n%s", n, raw)
	}
	if len(cfg.Quarantined()) != 0 {
		t.Fatalf("superseded entry still in memory: %+v", cfg.Quarantined())
	}
	again, err := Load(path)
	if err != nil || again.Server.GatewaySites[0].AppName != "Sonarr" {
		t.Fatalf("second load: err=%v sites=%+v", err, again.Server.GatewaySites)
	}
}

func TestPruneSupersededQuarantine(t *testing.T) {
	cfg := defaultConfig()
	cfg.QuarantineApp(&AppConfig{Name: "Vaultwarden", DockerKey: "label:vw"}, "r")
	cfg.QuarantineSite(&GatewaySite{Domain: "vw.example.com", DockerKey: "label:vw"}, "r")
	cfg.QuarantineApp(&AppConfig{Name: "Radarr", DockerKey: "label:r"}, "r")
	cfg.QuarantineSite(&GatewaySite{Domain: "Taken.example.com", DockerKey: "label:r"}, "r")
	cfg.QuarantineSite(&GatewaySite{Domain: "free.example.com", DockerKey: "label:r"}, "r")
	if n := cfg.pruneSupersededQuarantine(); n != 0 {
		t.Fatalf("nothing live yet, pruned %d", n)
	}
	// A live app with the quarantined app's slug supersedes it and its
	// site; a live site with a quarantined site's domain supersedes it.
	cfg.Apps = []AppConfig{{Name: "vaultwarden", URL: "http://vw:80", Enabled: true}}
	cfg.Server.GatewaySites = []GatewaySite{{Domain: "taken.example.com", BackendURL: "http://t:80"}}
	if n := cfg.pruneSupersededQuarantine(); n != 3 {
		t.Fatalf("pruned %d, want 3", n)
	}
	q := cfg.Quarantined()
	if len(q) != 2 || q[0].Name != "Radarr" || q[1].Name != "free.example.com" {
		t.Fatalf("left = %+v", q)
	}
	empty := defaultConfig()
	if n := empty.pruneSupersededQuarantine(); n != 0 {
		t.Fatalf("empty pruned %d", n)
	}
}

// Spec section 2: a docker-owned site with require_auth and no usable
// session_cookie_domain is quarantined; a manual one still fails Load.
func TestLoad_QuarantinesGatedDockerSite(t *testing.T) {
	const apps = `
apps:
  - name: A
    url: http://a:80
    enabled: true
    docker_key: "label:a"
    docker_auto: true
`
	t.Run("no cookie domain", func(t *testing.T) {
		cfg, err := Load(writeTempConfig(t, `
server:
  gateway_sites:
    - domain: a.example.com
      backend_url: http://a:80
      require_auth: true
      docker_key: "label:a"
`+apps))
		if err != nil {
			t.Fatalf("Load() = %v", err)
		}
		if q := cfg.Quarantined(); len(q) != 1 || q[0].Kind != "gateway" || !strings.Contains(q[0].Reason, "session_cookie_domain") {
			t.Fatalf("quarantined = %+v", q)
		}
	})
	t.Run("domain outside cookie domain", func(t *testing.T) {
		cfg, err := Load(writeTempConfig(t, `
server:
  session_cookie_domain: .example.com
  gateway_sites:
    - domain: a.other.org
      backend_url: http://a:80
      require_auth: true
      docker_key: "label:a"
    - domain: b.example.com
      backend_url: http://a:80
      require_auth: true
      docker_key: "label:a"
`+apps))
		if err != nil {
			t.Fatalf("Load() = %v", err)
		}
		q := cfg.Quarantined()
		if len(q) != 1 || q[0].Name != "a.other.org" || !strings.Contains(q[0].Reason, "not under") {
			t.Fatalf("quarantined = %+v", q)
		}
		if len(cfg.Server.GatewaySites) != 1 || cfg.Server.GatewaySites[0].Domain != "b.example.com" {
			t.Fatalf("live sites = %+v", cfg.Server.GatewaySites)
		}
	})
	t.Run("manual site still fails", func(t *testing.T) {
		_, err := Load(writeTempConfig(t, `
server:
  gateway_sites:
    - domain: a.example.com
      backend_url: http://a:80
      require_auth: true
`+apps))
		if err == nil || !strings.Contains(err.Error(), "session_cookie_domain") {
			t.Fatalf("Load() = %v, want session_cookie_domain error", err)
		}
	})
}

func TestDockerSiteReason(t *testing.T) {
	ok := GatewaySite{Domain: "a.example.com", BackendURL: "http://a:80", DockerKey: "label:a"}
	gated := ok
	gated.RequireAuth = true
	noBackend := ok
	noBackend.BackendURL = ""
	srv := &ServerConfig{}
	cookie := &ServerConfig{SessionCookieDomain: "example.com"}
	cases := []struct {
		name string
		s    *GatewaySite
		srv  *ServerConfig
		want string // substring; "" means valid
	}{
		{"valid", &ok, srv, ""},
		{"valid nil server", &ok, nil, ""},
		{"gated nil server skips the cookie rule", &gated, nil, ""},
		{"per-site rule", &noBackend, srv, "backend_url is required"},
		{"gated without cookie domain", &gated, srv, "session_cookie_domain is empty"},
		{"gated with cookie domain", &gated, cookie, ""},
	}
	for _, tc := range cases {
		got := DockerSiteReason(tc.s, tc.srv)
		if (tc.want == "") != (got == "") || !strings.Contains(got, tc.want) {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestQuarantineUnchangedInvalidDockerApps(t *testing.T) {
	prior := []AppConfig{
		{Name: "Broken", URL: "", DockerKey: "label:b", DockerAutoImported: true, Enabled: true},
		{Name: "Edited", URL: "", DockerKey: "label:e", DockerAutoImported: true, Enabled: true},
		{Name: "Fine", URL: "http://f", DockerKey: "label:f", DockerAutoImported: true, Enabled: true},
	}
	cfg := defaultConfig()
	cfg.Apps = append([]AppConfig(nil), prior...)
	cfg.Apps[1].Color = "#fff" // the payload changed this one
	cfg.Server.GatewaySites = []GatewaySite{
		{Domain: "b.example.com", BackendURL: "http://b:80", DockerKey: "label:b"},
		{Domain: "manual.example.com", BackendURL: "http://m:80", AppName: "Broken"},
	}
	if n := cfg.QuarantineUnchangedInvalidDockerApps(prior); n != 1 || len(cfg.Apps) != 2 || cfg.Apps[0].Name != "Edited" {
		t.Fatalf("n=%d apps=%+v", n, cfg.Apps)
	}
	// The shared site rule ran: the docker site of Broken is quarantined,
	// the manual site naming Broken lost its app_name.
	if len(cfg.Server.GatewaySites) != 1 || cfg.Server.GatewaySites[0].AppName != "" || !cfg.HasQuarantined("label:b") {
		t.Fatalf("sites=%+v quarantined=%+v", cfg.Server.GatewaySites, cfg.Quarantined())
	}
	if len(cfg.Quarantined()) != 2 {
		t.Fatalf("quarantined = %+v", cfg.Quarantined())
	}
}

func TestQuarantineUnchangedInvalidDockerApps_SlugCollision(t *testing.T) {
	prior := []AppConfig{
		{Name: "Home Assistant", URL: "http://ha", Enabled: true},
		{Name: "Home-Assistant", URL: "http://ha2", DockerKey: "label:ha", DockerAutoImported: true, Enabled: true},
	}
	cfg := defaultConfig()
	cfg.Apps = append([]AppConfig(nil), prior...)
	if n := cfg.QuarantineUnchangedInvalidDockerApps(prior); n != 1 || len(cfg.Apps) != 1 {
		t.Fatalf("n=%d apps=%+v", n, cfg.Apps)
	}
}

func TestSameSavedApp(t *testing.T) {
	p := AppConfig{Name: "A", URL: "", DockerKey: "k", DockerAutoImported: true,
		DockerManagedURL: "http://old", ProxyHeaders: map[string]string{}, HTTPActionHeaders: map[string]string{},
		AllowedGroups: []string{}, Permissions: []string{}}
	a := AppConfig{Name: "A", URL: "", DockerKey: "k", DockerAutoImported: true}
	if !sameSavedApp(&a, &p) {
		t.Fatal("merge-made differences must be ignored")
	}
	a.AllowedGroups = []string{"admins"}
	if sameSavedApp(&a, &p) {
		t.Fatal("a real edit must count as a change")
	}
}

func TestQuarantineUnchangedInvalidDockerApps_NothingToDo(t *testing.T) {
	prior := []AppConfig{{Name: "Manual", URL: "", Enabled: true}}
	cfg := defaultConfig()
	cfg.Apps = append([]AppConfig(nil), prior...)
	cfg.Server.GatewaySites = []GatewaySite{{Domain: "m.example.com", BackendURL: "http://m:80", AppName: "Manual"}}
	if n := cfg.QuarantineUnchangedInvalidDockerApps(prior); n != 0 || len(cfg.Apps) != 1 || cfg.Server.GatewaySites[0].AppName != "Manual" {
		t.Fatalf("n=%d apps=%+v sites=%+v", n, cfg.Apps, cfg.Server.GatewaySites)
	}
}
