package discovery

import (
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
)

// TestBuildDesired_ProxyApp: a container with the proxy label set and
// no gateway domain must produce a proxied App and no GatewaySite,
// with tracking stamped and a clean detach baseline.
func TestBuildDesired_ProxyApp(t *testing.T) {
	yes := true
	sug := Suggestion{
		Key: "label:sonarr", Name: "Sonarr", URL: "http://10.0.0.5:8989",
		Icon: "sonarr", Group: "Media", HealthURL: "http://10.0.0.5:8989/api/v3/health",
		EffectiveStrategy: config.StrategyContainerIP,
		Color:             "#3498db", Order: 10, OpenMode: "iframe", Proxy: &yes,
		MinRole: "user", AllowedGroups: []string{"family", "admins"},
	}
	d := BuildDesired(sug, "unix:///var/run/docker.sock")

	if d.Site != nil {
		t.Fatalf("no gateway domain -> no site, got %+v", d.Site)
	}
	if !d.App.Proxy {
		t.Error("Proxy label true -> App.Proxy true")
	}
	if d.App.DockerKey != "label:sonarr" || !d.App.DockerAutoImported {
		t.Errorf("tracking not set: key=%q auto=%v", d.App.DockerKey, d.App.DockerAutoImported)
	}
	if d.App.DockerEndpoint != "unix:///var/run/docker.sock" {
		t.Errorf("endpoint not carried: %q", d.App.DockerEndpoint)
	}
	if d.App.DockerStrategy != string(config.StrategyContainerIP) {
		t.Errorf("strategy not carried: %q", d.App.DockerStrategy)
	}
	if d.App.DockerManagedURL != d.App.URL {
		t.Errorf("managed URL must equal URL for clean detach detection: %q != %q", d.App.DockerManagedURL, d.App.URL)
	}
	if d.App.URL != "http://10.0.0.5:8989" {
		t.Errorf("URL = %q", d.App.URL)
	}
	if d.App.HealthURL != "http://10.0.0.5:8989/api/v3/health" {
		t.Errorf("health url not carried: %q", d.App.HealthURL)
	}
	if d.App.Icon.Type != "dashboard" || d.App.Icon.Name != "sonarr" {
		t.Errorf("icon = %+v", d.App.Icon)
	}
	if !d.App.Enabled {
		t.Error("app should be enabled")
	}
	if d.App.Name != "Sonarr" || d.App.Group != "Media" || d.App.MinRole != "user" {
		t.Error("label-derived fields not carried through")
	}
	if len(d.App.AllowedGroups) != 2 {
		t.Errorf("allowed groups = %v", d.App.AllowedGroups)
	}
}

// TestBuildDesired_DirectApp: no proxy label and no gateway domain ->
// a direct App (Proxy false), tracked, no Site.
func TestBuildDesired_DirectApp(t *testing.T) {
	sug := Suggestion{
		Key: "name:grafana", Name: "Grafana", URL: "http://10.0.0.9:3000",
		EffectiveStrategy: config.StrategyContainerIP,
	}
	d := BuildDesired(sug, "tcp://docker:2375")

	if d.Site != nil {
		t.Fatalf("no gateway domain -> no site, got %+v", d.Site)
	}
	if d.App.Proxy {
		t.Error("no proxy label -> App.Proxy false")
	}
	if d.App.DockerKey != "name:grafana" || !d.App.DockerAutoImported {
		t.Errorf("tracking not set: key=%q auto=%v", d.App.DockerKey, d.App.DockerAutoImported)
	}
	if d.App.DockerManagedURL != d.App.URL {
		t.Errorf("managed URL must equal URL: %q != %q", d.App.DockerManagedURL, d.App.URL)
	}
}

// TestBuildDesired_NilProxyIsDirect: a Suggestion with Proxy nil (no
// proxy label at all) must default App.Proxy to false.
func TestBuildDesired_NilProxyIsDirect(t *testing.T) {
	sug := Suggestion{Key: "id:abc", Name: "App", URL: "http://h:1"}
	d := BuildDesired(sug, "e")
	if d.App.Proxy {
		t.Error("nil Proxy -> App.Proxy must be false")
	}
	if d.App.ProxySkipTLSVerify != nil {
		t.Error("nil ProxySkipTLSVerify must stay nil")
	}
}

// TestBuildDesired_GatewaySite: a SuggestedDomain plus a gateway label
// set must produce a GatewaySite mirroring ImportDocker: BackendURL is
// the container URL, App.URL becomes the public domain, App.Proxy is
// false, both App and Site are tracked, and the Site's clean-detach
// baseline equals its BackendURL.
func TestBuildDesired_GatewaySite(t *testing.T) {
	yes := true
	no := false
	sug := Suggestion{
		Key: "label:sonarr", Name: "Sonarr", URL: "http://10.0.0.5:8989",
		EffectiveStrategy: config.StrategyContainerIP,
		SuggestedDomain:   "sonarr.example.com",
		SuggestedGateway: &SuggestedGatewayConfig{
			TLS:                "auto",
			Streaming:          &yes,
			StripFrameBlockers: &yes,
			ForwardedHeaders:   &no,
			RequireAuth:        &yes,
			MinRole:            "power-user",
			AllowedGroups:      []string{"staff"},
		},
	}
	d := BuildDesired(sug, "unix:///x")
	if d.Site == nil {
		t.Fatal("gateway domain -> gateway site expected")
	}
	if d.Site.Domain != "sonarr.example.com" {
		t.Errorf("site domain = %q", d.Site.Domain)
	}
	if d.Site.BackendURL != "http://10.0.0.5:8989" {
		t.Errorf("site backend must be the container URL, got %q", d.Site.BackendURL)
	}
	if d.Site.TLS != config.TLSModeAuto {
		t.Errorf("site tls = %q", d.Site.TLS)
	}
	if d.Site.Streaming != true || d.Site.StripFrameBlockers != true {
		t.Errorf("streaming=%v stripframe=%v", d.Site.Streaming, d.Site.StripFrameBlockers)
	}
	if d.Site.ForwardedHeaders == nil || *d.Site.ForwardedHeaders != false {
		t.Errorf("forwarded headers = %v", d.Site.ForwardedHeaders)
	}
	if !d.Site.RequireAuth || d.Site.MinRole != "power-user" || len(d.Site.AllowedGroups) != 1 {
		t.Errorf("auth gate not mapped: %+v", d.Site)
	}
	if d.Site.DockerKey != "label:sonarr" {
		t.Errorf("site must carry tracking key, got %q", d.Site.DockerKey)
	}
	if d.Site.DockerEndpoint != "unix:///x" || d.Site.DockerStrategy != string(config.StrategyContainerIP) {
		t.Errorf("site tracking incomplete: %+v", d.Site)
	}
	if d.Site.DockerManagedURL != d.Site.BackendURL {
		t.Errorf("site managed URL must equal backend URL: %q != %q", d.Site.DockerManagedURL, d.Site.BackendURL)
	}
	if d.Site.AppName != "Sonarr" {
		t.Errorf("site must link to the app: %q", d.Site.AppName)
	}

	// App side: URL becomes the public https domain, proxy off.
	if d.App.URL != "https://sonarr.example.com" {
		t.Errorf("gateway app URL = %q, want https://sonarr.example.com", d.App.URL)
	}
	if d.App.Proxy {
		t.Error("gateway app must not be proxy-routed")
	}
	if d.App.DockerKey != "label:sonarr" || !d.App.DockerAutoImported {
		t.Errorf("gateway app tracking not set: %+v", d.App)
	}
	if d.App.DockerManagedURL != d.App.URL {
		t.Errorf("gateway app managed URL must equal URL: %q != %q", d.App.DockerManagedURL, d.App.URL)
	}
}

// TestBuildDesired_GatewayTLSNone: TLS=none means the public app URL is
// plain http (mirrors ImportDocker's scheme selection).
func TestBuildDesired_GatewayTLSNone(t *testing.T) {
	sug := Suggestion{
		Key: "label:x", Name: "X", URL: "http://h:1",
		SuggestedDomain:  "x.example.com",
		SuggestedGateway: &SuggestedGatewayConfig{TLS: "none"},
	}
	d := BuildDesired(sug, "e")
	if d.Site == nil {
		t.Fatal("expected site")
	}
	if d.Site.TLS != config.TLSModeNone {
		t.Errorf("tls = %q", d.Site.TLS)
	}
	if d.App.URL != "http://x.example.com" {
		t.Errorf("tls=none -> http app URL, got %q", d.App.URL)
	}
}

// TestBuildDesired_GatewayNilConfig: a domain with no muximux.gateway.*
// labels (SuggestedGateway nil) still builds a Site with defaults and
// an https app URL, without panicking.
func TestBuildDesired_GatewayNilConfig(t *testing.T) {
	sug := Suggestion{
		Key: "label:y", Name: "Y", URL: "http://h:2",
		SuggestedDomain: "y.example.com",
	}
	d := BuildDesired(sug, "e")
	if d.Site == nil {
		t.Fatal("domain present -> site expected even without gateway labels")
	}
	if d.Site.Domain != "y.example.com" || d.Site.BackendURL != "http://h:2" {
		t.Errorf("site = %+v", d.Site)
	}
	if d.Site.TLS != config.TLSModeDefault {
		t.Errorf("default tls expected, got %q", d.Site.TLS)
	}
	if d.Site.RequireAuth || d.Site.Streaming || d.Site.StripFrameBlockers {
		t.Errorf("defaults should be off: %+v", d.Site)
	}
	if d.Site.ForwardedHeaders != nil {
		t.Errorf("forwarded headers should stay nil when unset, got %v", d.Site.ForwardedHeaders)
	}
	if d.App.URL != "https://y.example.com" {
		t.Errorf("default scheme https, got %q", d.App.URL)
	}
}

// TestBuildDesired_CarriesAllLabelFields asserts every label-derived
// Suggestion field with a value lands on the AppConfig. Guards against
// future divergence from manual import.
func TestBuildDesired_CarriesAllLabelFields(t *testing.T) {
	no := false
	yes := true
	shortcut := 3
	sug := Suggestion{
		Key: "label:x", Name: "X", URL: "http://h:1", OpenMode: "redirect",
		Color: "#fff", Order: 7, MinRole: "admin",
		AllowedGroups: []string{"g"}, Permissions: []string{"clipboard-read"},
		Proxy: &no, ProxySkipTLSVerify: &no,
		AllowNotifications:  &yes,
		Default:             &yes,
		Shortcut:            shortcut,
		HTTPActionMethod:    "POST",
		HTTPActionHeaders:   map[string]string{"X-Api": "k"},
		HTTPActionConfirm:   &yes,
		HTTPActionShowToast: &no,
	}
	d := BuildDesired(sug, "e")
	a := d.App
	if a.OpenMode != "redirect" || a.Color != "#fff" || a.Order != 7 ||
		a.MinRole != "admin" || len(a.AllowedGroups) != 1 ||
		len(a.Permissions) != 1 || a.Proxy {
		t.Errorf("label field dropped: %+v", a)
	}
	if a.ProxySkipTLSVerify == nil || *a.ProxySkipTLSVerify != false {
		t.Errorf("proxy skip tls verify = %v", a.ProxySkipTLSVerify)
	}
	if !a.AllowNotifications {
		t.Error("allow notifications dropped")
	}
	if !a.Default {
		t.Error("default dropped")
	}
	if a.Shortcut == nil || *a.Shortcut != 3 {
		t.Errorf("shortcut = %v", a.Shortcut)
	}
	if a.HTTPActionMethod != "POST" || len(a.HTTPActionHeaders) != 1 {
		t.Errorf("http action method/headers dropped: %+v", a)
	}
	if !a.HTTPActionConfirm {
		t.Error("http action confirm dropped")
	}
	if a.HTTPActionShowToast == nil || *a.HTTPActionShowToast != false {
		t.Errorf("http action show toast = %v", a.HTTPActionShowToast)
	}
}

// TestBuildDesired_OptionalPointersUnset: when the optional pointer and
// scalar fields are unset, the App keeps Go zero values (no spurious
// non-nil pointers, no shortcut).
func TestBuildDesired_OptionalPointersUnset(t *testing.T) {
	sug := Suggestion{Key: "k", Name: "N", URL: "http://h:1"}
	d := BuildDesired(sug, "e")
	a := d.App
	if a.AllowNotifications || a.Default || a.HTTPActionConfirm {
		t.Error("unset bool pointers must yield false")
	}
	if a.Shortcut != nil {
		t.Errorf("unset shortcut must stay nil, got %v", a.Shortcut)
	}
	if a.HTTPActionShowToast != nil {
		t.Errorf("unset show toast must stay nil, got %v", a.HTTPActionShowToast)
	}
}
