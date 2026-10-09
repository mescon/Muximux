package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/logging"
	"github.com/mescon/muximux/v3/internal/proxy"
)

// setupGatewayHandler builds a handler against an in-memory config
// rooted at a temp directory. proxyServer is intentionally nil so the
// applyAndPersist path takes the restart-required branch and we don't
// need to spin Caddy up just to drive the persist logic.
func setupGatewayHandler(t *testing.T) (*GatewayHandler, *config.Config, string) {
	t.Helper()
	cfg := &config.Config{
		Server: config.ServerConfig{Listen: ":8080"},
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	h := NewGatewayHandler(cfg, configPath, &sync.RWMutex{}, nil)
	return h, cfg, configPath
}

// loadConfigForGatewayTest reads the persisted config from disk so
// tests can verify what actually got written, independent of the
// in-memory state the handler maintains.
func loadConfigForGatewayTest(t *testing.T, path string) *config.Config {
	t.Helper()
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

func TestGateway_ListSites_Empty(t *testing.T) {
	h, _, _ := setupGatewayHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/gateway/sites", nil)
	w := httptest.NewRecorder()
	h.ListSites(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var got []config.GatewaySite
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty list, got %v", got)
	}
}

func TestGateway_CreateSite_Success(t *testing.T) {
	h, cfg, configPath := setupGatewayHandler(t)

	body, _ := json.Marshal(config.GatewaySite{
		Domain:     "plex.example.com",
		BackendURL: "http://plex:32400",
		Streaming:  true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/gateway/sites", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateSite(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp gatewayMutationResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if !resp.RestartRequired {
		t.Error("expected restart_required=true (no proxy in this test)")
	}
	if resp.Site == nil || resp.Site.Domain != "plex.example.com" {
		t.Errorf("response site missing or wrong: %+v", resp.Site)
	}

	// In-memory state updated.
	if len(cfg.Server.GatewaySites) != 1 {
		t.Errorf("in-memory sites: got %d", len(cfg.Server.GatewaySites))
	}

	// On-disk state persisted.
	on := loadConfigForGatewayTest(t, configPath)
	if len(on.Server.GatewaySites) != 1 || on.Server.GatewaySites[0].Domain != "plex.example.com" {
		t.Errorf("on-disk sites: %+v", on.Server.GatewaySites)
	}
}

func TestGateway_CreateSite_ValidationError_DoesNotMutate(t *testing.T) {
	h, cfg, configPath := setupGatewayHandler(t)

	// Backend URL with an invalid scheme triggers ValidateGatewaySites.
	body, _ := json.Marshal(config.GatewaySite{
		Domain:     "x.example.com",
		BackendURL: "ftp://bad",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/gateway/sites", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateSite(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
	if len(cfg.Server.GatewaySites) != 0 {
		t.Errorf("in-memory state was mutated despite validation failure: %+v", cfg.Server.GatewaySites)
	}

	on := loadConfigForGatewayTest(t, configPath)
	if len(on.Server.GatewaySites) != 0 {
		t.Errorf("on-disk state was mutated despite validation failure: %+v", on.Server.GatewaySites)
	}
}

func TestGateway_CreateSite_DuplicateDomain_409(t *testing.T) {
	h, _, _ := setupGatewayHandler(t)

	for i, status := range []int{http.StatusOK, http.StatusConflict} {
		body, _ := json.Marshal(config.GatewaySite{
			Domain:     "x.example.com",
			BackendURL: "http://app:80",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/gateway/sites", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.CreateSite(w, req)
		if w.Code != status {
			t.Errorf("attempt %d: status = %d, want %d", i, w.Code, status)
		}
	}
}

func TestGateway_UpdateSite_NotFound_404(t *testing.T) {
	h, _, _ := setupGatewayHandler(t)

	body, _ := json.Marshal(config.GatewaySite{
		Domain:     "ghost.example.com",
		BackendURL: "http://app:80",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/gateway/sites/ghost.example.com", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.UpdateSite(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestGateway_UpdateSite_Rename_Success(t *testing.T) {
	h, cfg, _ := setupGatewayHandler(t)

	// Seed an existing site directly so we can update it.
	cfg.Server.GatewaySites = []config.GatewaySite{
		{Domain: "old.example.com", BackendURL: "http://app:80"},
	}

	body, _ := json.Marshal(config.GatewaySite{
		Domain:     "new.example.com",
		BackendURL: "http://app:80",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/gateway/sites/old.example.com", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.UpdateSite(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if len(cfg.Server.GatewaySites) != 1 || cfg.Server.GatewaySites[0].Domain != "new.example.com" {
		t.Errorf("rename did not take effect: %+v", cfg.Server.GatewaySites)
	}
}

func TestGateway_UpdateSite_RenameCollides_409(t *testing.T) {
	h, cfg, _ := setupGatewayHandler(t)

	cfg.Server.GatewaySites = []config.GatewaySite{
		{Domain: "a.example.com", BackendURL: "http://app:80"},
		{Domain: "b.example.com", BackendURL: "http://app:80"},
	}

	// Try to rename "a" to "b" — should 409 because b already exists.
	body, _ := json.Marshal(config.GatewaySite{
		Domain:     "b.example.com",
		BackendURL: "http://app:80",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/gateway/sites/a.example.com", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.UpdateSite(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
	// State unchanged.
	if cfg.Server.GatewaySites[0].Domain != "a.example.com" {
		t.Errorf("state changed despite 409: %+v", cfg.Server.GatewaySites)
	}
}

func TestGateway_DeleteSite_Success(t *testing.T) {
	h, cfg, _ := setupGatewayHandler(t)

	cfg.Server.GatewaySites = []config.GatewaySite{
		{Domain: "x.example.com", BackendURL: "http://app:80"},
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/gateway/sites/x.example.com", nil)
	w := httptest.NewRecorder()
	h.DeleteSite(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if len(cfg.Server.GatewaySites) != 0 {
		t.Errorf("expected empty list after delete, got %+v", cfg.Server.GatewaySites)
	}
}

func TestGateway_DeleteSite_Idempotent(t *testing.T) {
	// Deleting a non-existent domain returns 200 so the UI doesn't
	// have to special-case retry-after-flaky-network paths.
	h, _, _ := setupGatewayHandler(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/gateway/sites/nothing.example.com", nil)
	w := httptest.NewRecorder()
	h.DeleteSite(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestGateway_ValidateSite_AlwaysReturns200(t *testing.T) {
	h, _, _ := setupGatewayHandler(t)

	t.Run("valid candidate", func(t *testing.T) {
		body, _ := json.Marshal(config.GatewaySite{
			Domain:     "ok.example.com",
			BackendURL: "http://app:80",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/gateway/validate", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.ValidateSite(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
		var got map[string]interface{}
		_ = json.NewDecoder(w.Body).Decode(&got)
		if got["valid"] != true {
			t.Errorf("expected valid=true, got %+v", got)
		}
	})

	t.Run("invalid candidate still returns 200 with error text", func(t *testing.T) {
		body, _ := json.Marshal(config.GatewaySite{
			Domain:     "*.wildcard.example.com",
			BackendURL: "http://app:80",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/gateway/validate", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.ValidateSite(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
		var got map[string]interface{}
		_ = json.NewDecoder(w.Body).Decode(&got)
		if got["valid"] != false {
			t.Errorf("expected valid=false, got %+v", got)
		}
		if got["error"] == nil || got["error"] == "" {
			t.Errorf("expected error message, got %+v", got)
		}
	})
}

func TestGateway_MethodNotAllowed(t *testing.T) {
	h, _, _ := setupGatewayHandler(t)
	cases := []struct {
		fn     func(http.ResponseWriter, *http.Request)
		method string
	}{
		{h.ListSites, http.MethodPost},
		{h.CreateSite, http.MethodGet},
		{h.UpdateSite, http.MethodGet},
		{h.DeleteSite, http.MethodGet},
		{h.ValidateSite, http.MethodGet},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		c.fn(w, httptest.NewRequest(c.method, "/api/gateway/sites", nil))
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: status = %d, want 405", c.method, w.Code)
		}
	}
}

func TestGateway_CreateSite_PersistFailure_RollsBack(t *testing.T) {
	h, cfg, _ := setupGatewayHandler(t)

	// Point configPath at a directory so config.Save's atomic rename
	// fails (the temp file can be created but cannot be renamed over a
	// directory). Mirrors the API-Key handler's rollback test.
	h.configPath = t.TempDir()

	body, _ := json.Marshal(config.GatewaySite{
		Domain:     "x.example.com",
		BackendURL: "http://app:80",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/gateway/sites", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateSite(w, req)

	// applyAndPersist now returns 500 for runtime save failures (vs
	// 400 for validation). The key invariant we still verify here is
	// that the in-memory state matches what is on disk, regardless of
	// status code.
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 (save failure), got %d", w.Code)
	}
	if len(cfg.Server.GatewaySites) != 0 {
		t.Errorf("in-memory state was not rolled back: %+v", cfg.Server.GatewaySites)
	}
}

// fakeProxy is a controllable gatewayProxy used by the rollback /
// reload tests. Each method's behaviour can be tweaked from the test
// without touching real Caddy.
type fakeProxy struct {
	running         bool
	previewResult   string // returned by CaddyfilePreview
	previewCalls    int
	reloadCalls     int
	reloadErr       error // returned by Reload on the Nth call (see reloadErrAt)
	reloadErrAt     int   // 0-indexed call to fail; -1 means never; defaults to 0
	rollbackErrOnce error // returned only by the second Reload call (the rollback)
	sites           []proxy.GatewaySite
}

func newFakeProxy(running bool) *fakeProxy {
	return &fakeProxy{running: running, reloadErrAt: -1}
}

func (f *fakeProxy) IsRunning() bool { return f.running }

func (f *fakeProxy) SetGatewaySites(sites []proxy.GatewaySite) {
	f.sites = append([]proxy.GatewaySite(nil), sites...)
}

func (f *fakeProxy) CaddyfilePreview(sites []proxy.GatewaySite) string {
	f.previewCalls++
	// Return a Caddyfile that always parses cleanly so proxy.Validate
	// passes; tests that want to fail the parse stage instead set
	// previewResult to a known-bad string.
	if f.previewResult != "" {
		return f.previewResult
	}
	// A trivially valid Caddyfile: an HTTP-only site with no upstream.
	return ":18099 {\n\trespond \"ok\"\n}\n"
}

func (f *fakeProxy) Reload() error {
	idx := f.reloadCalls
	f.reloadCalls++
	if f.rollbackErrOnce != nil && idx == 1 {
		// The second Reload call is the rollback; this error simulates
		// the catastrophic divergence case.
		return f.rollbackErrOnce
	}
	if f.reloadErrAt >= 0 && idx == f.reloadErrAt {
		return f.reloadErr
	}
	return nil
}

func TestGateway_CreateSite_ReloadFailure_RollsBackInMemory(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Listen: ":8080"}}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	fp := newFakeProxy(true)
	fp.reloadErrAt = 0
	fp.reloadErr = errors.New("port already in use")

	h := newGatewayHandlerWithProxy(cfg, configPath, &sync.RWMutex{}, fp)

	body, _ := json.Marshal(config.GatewaySite{
		Domain:     "x.example.com",
		BackendURL: "http://app:80",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/gateway/sites", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateSite(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 from reload failure, got %d", w.Code)
	}
	if len(cfg.Server.GatewaySites) != 0 {
		t.Errorf("in-memory state was not rolled back after Reload failure: %+v", cfg.Server.GatewaySites)
	}
	// The proxy snapshot must also be rolled back so a follow-up
	// Reload-from-current-state would produce the prior config.
	if len(fp.sites) != 0 {
		t.Errorf("proxy snapshot not rolled back: %+v", fp.sites)
	}
	// On-disk file must remain pristine — we never reached the Save
	// step.
	on, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if len(on.Server.GatewaySites) != 0 {
		t.Errorf("on-disk state was mutated despite reload failure: %+v", on.Server.GatewaySites)
	}
}

func TestGateway_CreateSite_ReloadAndReassertBothFail_ReturnsDivergence(t *testing.T) {
	// The Caddy "transactional reload" assumption isn't ironclad for
	// post-parse failures. If the candidate Reload fails AND the
	// re-assert Reload (with the prior site list) also fails, we
	// can't guarantee Caddy is serving the prior config — return the
	// divergence response so the operator restarts to recover.
	cfg := &config.Config{Server: config.ServerConfig{Listen: ":8080"}}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	fp := newFakeProxy(true)
	// Both Reload calls (candidate + re-assert) fail.
	fp.reloadErrAt = 0
	fp.reloadErr = errors.New("port already in use")
	fp.rollbackErrOnce = errors.New("caddy crashed during re-assert")

	h := newGatewayHandlerWithProxy(cfg, configPath, &sync.RWMutex{}, fp)

	body, _ := json.Marshal(config.GatewaySite{
		Domain:     "x.example.com",
		BackendURL: "http://app:80",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/gateway/sites", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateSite(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 (divergence), got %d", w.Code)
	}
	var resp gatewayMutationResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Mismatch {
		t.Errorf("expected mismatch=true on reload-and-reassert-failed response, got %+v", resp)
	}
	if fp.reloadCalls != 2 {
		t.Errorf("expected 2 Reload calls (apply + re-assert), got %d", fp.reloadCalls)
	}
}

func TestGateway_CreateSite_DivergenceFlagsMismatch(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Listen: ":8080"}}

	// Point configPath at a directory so Save fails after Reload
	// succeeded — the catastrophic divergence path.
	configPath := t.TempDir()

	fp := newFakeProxy(true)
	fp.rollbackErrOnce = errors.New("caddy crashed during rollback")

	h := newGatewayHandlerWithProxy(cfg, configPath, &sync.RWMutex{}, fp)

	body, _ := json.Marshal(config.GatewaySite{
		Domain:     "x.example.com",
		BackendURL: "http://app:80",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/gateway/sites", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateSite(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 (divergence), got %d", w.Code)
	}
	var resp gatewayMutationResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Mismatch {
		t.Errorf("expected mismatch=true on divergence response, got %+v", resp)
	}
	if resp.Success {
		t.Errorf("success must be false on divergence")
	}
	// Reload was called twice: once for the initial apply, once for
	// the rollback (which itself failed).
	if fp.reloadCalls != 2 {
		t.Errorf("expected 2 Reload calls (apply + rollback), got %d", fp.reloadCalls)
	}
}

func TestGateway_CreateSite_ProxyNotRunningTreatsAsRestartRequired(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Listen: ":8080"}}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	// Simulate a proxy whose Start failed — non-nil but not running.
	fp := newFakeProxy(false)
	h := newGatewayHandlerWithProxy(cfg, configPath, &sync.RWMutex{}, fp)

	body, _ := json.Marshal(config.GatewaySite{
		Domain:     "x.example.com",
		BackendURL: "http://app:80",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/gateway/sites", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateSite(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (save succeeded, restart required), got %d", w.Code)
	}
	if fp.reloadCalls != 0 {
		t.Errorf("Reload should be skipped when the proxy is not running; got %d calls", fp.reloadCalls)
	}
	var resp gatewayMutationResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.RestartRequired {
		t.Error("expected restart_required=true when proxy is not running")
	}
}

func TestGateway_CreateSite_CaddyfileValidationCatchesParseErrors(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Listen: ":8080"}}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	fp := newFakeProxy(true)
	// Force the Caddyfile preview to be syntactically invalid so
	// proxy.Validate rejects it. This stages the second-stage
	// validation — which catches a class of errors the structural
	// validator doesn't.
	fp.previewResult = "this is not a valid caddyfile {"

	h := newGatewayHandlerWithProxy(cfg, configPath, &sync.RWMutex{}, fp)

	body, _ := json.Marshal(config.GatewaySite{
		Domain:     "x.example.com",
		BackendURL: "http://app:80",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/gateway/sites", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateSite(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (Caddyfile parse failure), got %d", w.Code)
	}
	if len(cfg.Server.GatewaySites) != 0 {
		t.Errorf("in-memory state mutated despite parse failure: %+v", cfg.Server.GatewaySites)
	}
	if fp.reloadCalls != 0 {
		t.Errorf("Reload should not be called when parse fails; got %d", fp.reloadCalls)
	}
}

// putGatewaySite drives UpdateSite with a JSON body for the given
// path domain and returns the recorder.
func putGatewaySite(t *testing.T, h *GatewayHandler, pathDomain string, site *config.GatewaySite) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(site)
	req := httptest.NewRequest(http.MethodPut, "/api/gateway/sites/"+pathDomain, bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.UpdateSite(w, req)
	return w
}

// readFileForGatewayTest returns the raw bytes of the persisted
// config so a test can assert a refused save left it untouched.
func readFileForGatewayTest(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	return b
}

// S-15: a site save is validated like Load, so a gated site without a
// session cookie domain is refused instead of breaking the next boot.
func TestCreateSite_RequireAuthWithoutCookieDomainIs400(t *testing.T) {
	h, cfg, configPath := setupGatewayHandler(t)
	before := readFileForGatewayTest(t, configPath)

	body, _ := json.Marshal(config.GatewaySite{
		Domain:      "sonarr.example.com",
		BackendURL:  "http://sonarr:8989",
		RequireAuth: true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/gateway/sites", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateSite(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "session_cookie_domain is required") {
		t.Errorf("body should name the cookie-domain rule, got %s", w.Body.String())
	}
	if len(cfg.Server.GatewaySites) != 0 {
		t.Errorf("in-memory sites mutated: %+v", cfg.Server.GatewaySites)
	}
	if after := readFileForGatewayTest(t, configPath); !bytes.Equal(before, after) {
		t.Errorf("config file changed despite the refused save")
	}
}

func TestUpdateSite_GatedSiteOutsideCookieDomainIs400(t *testing.T) {
	h, cfg, configPath := setupGatewayHandler(t)
	cfg.Server.SessionCookieDomain = ".a.org"
	cfg.Server.GatewaySites = []config.GatewaySite{
		{Domain: "x.b.org", BackendURL: "http://x:80"},
	}
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("seed: %v", err)
	}
	before := readFileForGatewayTest(t, configPath)

	w := putGatewaySite(t, h, "x.b.org", &config.GatewaySite{
		Domain:      "x.b.org",
		BackendURL:  "http://x:80",
		RequireAuth: true,
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "is not under server.session_cookie_domain") {
		t.Errorf("body should name the cookie-domain rule, got %s", w.Body.String())
	}
	if cfg.Server.GatewaySites[0].RequireAuth {
		t.Errorf("in-memory site mutated: %+v", cfg.Server.GatewaySites[0])
	}
	if after := readFileForGatewayTest(t, configPath); !bytes.Equal(before, after) {
		t.Errorf("config file changed despite the refused save")
	}
}

// S-28: UpdateSite keeps server-owned Docker tracking from the stored
// site and detaches only on a real backend_url edit.
func TestUpdateSite_PreservesTrackingAndDetachesOnBackendEdit(t *testing.T) {
	if logging.Buffer() == nil {
		if err := logging.Init(logging.Config{Level: logging.LevelInfo, Format: "text", Output: "stdout"}); err != nil {
			t.Fatalf("init logging: %v", err)
		}
	}
	h, cfg, configPath := setupGatewayHandler(t)
	cfg.Server.GatewaySites = []config.GatewaySite{{
		Domain:           "app.example.com",
		BackendURL:       "http://a",
		DockerKey:        "k",
		DockerEndpoint:   "unix:///var/run/docker.sock",
		DockerStrategy:   "ip",
		DockerManagedURL: "http://a",
	}}

	// Same backend, no docker fields in the body: tracking kept.
	w := putGatewaySite(t, h, "app.example.com", &config.GatewaySite{
		Domain:     "app.example.com",
		BackendURL: "http://a",
		Streaming:  true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	got := cfg.Server.GatewaySites[0]
	if got.DockerKey != "k" || got.DockerEndpoint != "unix:///var/run/docker.sock" ||
		got.DockerStrategy != "ip" || got.DockerManagedURL != "http://a" {
		t.Errorf("tracking not preserved: %+v", got)
	}
	if !got.Streaming {
		t.Errorf("edit did not apply: %+v", got)
	}
	if on := loadConfigForGatewayTest(t, configPath); on.Server.GatewaySites[0].DockerKey != "k" {
		t.Errorf("tracking not persisted: %+v", on.Server.GatewaySites[0])
	}

	// Backend edited: tracking cleared and an audit line emitted.
	w = putGatewaySite(t, h, "app.example.com", &config.GatewaySite{
		Domain:     "app.example.com",
		BackendURL: "http://b",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	got = cfg.Server.GatewaySites[0]
	if got.DockerKey != "" || got.DockerEndpoint != "" || got.DockerStrategy != "" || got.DockerManagedURL != "" {
		t.Errorf("tracking not detached: %+v", got)
	}
	if got.BackendURL != "http://b" {
		t.Errorf("backend = %q, want http://b", got.BackendURL)
	}
	found := false
	for _, e := range logging.Buffer().Recent(1000) {
		if e.Message == "Docker tracking auto-detached on backend_url change" && e.Attrs["domain"] == "app.example.com" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected audit line 'Docker tracking auto-detached on backend_url change'")
	}
}

func TestApplyGatewayTrackingPreservation(t *testing.T) {
	t.Run("untracked site stays untracked", func(t *testing.T) {
		updated := config.GatewaySite{BackendURL: "http://b", DockerKey: "spoofed"}
		existing := config.GatewaySite{BackendURL: "http://a"}
		if key := applyGatewayTrackingPreservation(&updated, &existing); key != "" {
			t.Errorf("key = %q, want empty", key)
		}
		if updated.DockerKey != "" {
			t.Errorf("client-sent docker_key should be dropped, got %q", updated.DockerKey)
		}
	})
	t.Run("empty managed url falls back to stored backend", func(t *testing.T) {
		updated := config.GatewaySite{BackendURL: "http://a"}
		existing := config.GatewaySite{BackendURL: "http://a", DockerKey: "k"}
		if key := applyGatewayTrackingPreservation(&updated, &existing); key != "" {
			t.Errorf("key = %q, want empty", key)
		}
		if updated.DockerKey != "k" || updated.DockerManagedURL != "http://a" {
			t.Errorf("tracking not kept: %+v", updated)
		}
	})
	t.Run("empty managed url and edited backend detaches", func(t *testing.T) {
		updated := config.GatewaySite{BackendURL: "http://b"}
		existing := config.GatewaySite{BackendURL: "http://a", DockerKey: "k"}
		if key := applyGatewayTrackingPreservation(&updated, &existing); key != "k" {
			t.Errorf("key = %q, want k", key)
		}
		if updated.DockerKey != "" {
			t.Errorf("tracking not cleared: %+v", updated)
		}
	})
}

// putGatewaySiteRaw drives UpdateSite with an arbitrary JSON body so a
// test can include fields outside config.GatewaySite (base_backend_url).
func putGatewaySiteRaw(t *testing.T, h *GatewayHandler, pathDomain string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/api/gateway/sites/"+pathDomain, bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.UpdateSite(w, req)
	return w
}

// seedTrackedGatewaySite installs a Docker-tracked site whose backend
// the poller has refreshed to http://new since the client loaded the
// form at http://old.
func seedTrackedGatewaySite(t *testing.T) (*GatewayHandler, *config.Config, string) {
	t.Helper()
	h, cfg, configPath := setupGatewayHandler(t)
	cfg.Server.GatewaySites = []config.GatewaySite{{
		Domain:           "app.example.com",
		BackendURL:       "http://new",
		DockerKey:        "k",
		DockerEndpoint:   "unix:///var/run/docker.sock",
		DockerStrategy:   "ip",
		DockerManagedURL: "http://new",
	}}
	return h, cfg, configPath
}

func TestUpdateSite_StaleUneditedBackendKeepsCurrentAndTracking(t *testing.T) {
	h, cfg, configPath := seedTrackedGatewaySite(t)

	w := putGatewaySiteRaw(t, h, "app.example.com", map[string]any{
		"domain":           "app.example.com",
		"backend_url":      "http://old",
		"base_backend_url": "http://old",
		"streaming":        true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	got := cfg.Server.GatewaySites[0]
	if got.BackendURL != "http://new" {
		t.Errorf("backend = %q, want the poller's http://new", got.BackendURL)
	}
	if got.DockerKey != "k" || got.DockerManagedURL != "http://new" {
		t.Errorf("tracking lost: %+v", got)
	}
	if !got.Streaming {
		t.Errorf("other edits not applied: %+v", got)
	}
	on := loadConfigForGatewayTest(t, configPath)
	if on.Server.GatewaySites[0].BackendURL != "http://new" || on.Server.GatewaySites[0].DockerKey != "k" {
		t.Errorf("persisted site wrong: %+v", on.Server.GatewaySites[0])
	}
}

func TestUpdateSite_EditedBackendWithBaseDetaches(t *testing.T) {
	h, cfg, _ := seedTrackedGatewaySite(t)

	w := putGatewaySiteRaw(t, h, "app.example.com", map[string]any{
		"domain":           "app.example.com",
		"backend_url":      "http://manual",
		"base_backend_url": "http://old",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	got := cfg.Server.GatewaySites[0]
	if got.BackendURL != "http://manual" {
		t.Errorf("backend = %q, want http://manual", got.BackendURL)
	}
	if got.DockerKey != "" || got.DockerManagedURL != "" {
		t.Errorf("tracking not detached: %+v", got)
	}
}

func TestUpdateSite_NoBaseKeepsSubmittedBackend(t *testing.T) {
	h, cfg, _ := seedTrackedGatewaySite(t)

	// Without base_backend_url a differing backend is taken as an edit,
	// as before: stored and detached.
	w := putGatewaySiteRaw(t, h, "app.example.com", map[string]any{
		"domain":      "app.example.com",
		"backend_url": "http://old",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	got := cfg.Server.GatewaySites[0]
	if got.BackendURL != "http://old" || got.DockerKey != "" {
		t.Errorf("want submitted backend and detach, got %+v", got)
	}
}
