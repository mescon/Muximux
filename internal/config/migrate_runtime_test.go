package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAutoMigrateGateway_HappyPath covers the canonical upgrade
// path: a 3.0.x config.yaml that points server.gateway: at a
// Caddyfile with a couple of clean reverse_proxy blocks. Load()
// converts in memory, backs both files up to .pre-3.1.0.bak, and
// rewrites config.yaml with gateway_sites: + no gateway: key.
func TestAutoMigrateGateway_HappyPath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	caddyfilePath := filepath.Join(dir, "sites.Caddyfile")

	if err := os.WriteFile(caddyfilePath, []byte(`plex.example.com {
    reverse_proxy http://plex:32400
}
sonarr.example.com {
    reverse_proxy http://sonarr:8989
}
`), 0o600); err != nil {
		t.Fatalf("write caddyfile: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`server:
  listen: ":8080"
  title: "test"
  gateway: `+caddyfilePath+`
auth:
  method: none
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// In-memory state: gateway: cleared, gateway_sites: populated.
	if cfg.Server.Gateway != "" {
		t.Errorf("server.gateway should be cleared after migration, got %q", cfg.Server.Gateway)
	}
	if len(cfg.Server.GatewaySites) != 2 {
		t.Fatalf("expected 2 migrated sites, got %d", len(cfg.Server.GatewaySites))
	}
	gotDomains := map[string]bool{}
	for _, s := range cfg.Server.GatewaySites {
		gotDomains[s.Domain] = true
	}
	for _, want := range []string{"plex.example.com", "sonarr.example.com"} {
		if !gotDomains[want] {
			t.Errorf("missing migrated site: %s", want)
		}
	}

	// Backups exist with the original content.
	configBak, err := os.ReadFile(configPath + backupSuffix)
	if err != nil {
		t.Fatalf("read config backup: %v", err)
	}
	if !strings.Contains(string(configBak), "gateway: "+caddyfilePath) {
		t.Errorf("config backup should preserve legacy gateway: line, got:\n%s", configBak)
	}
	caddyfileBak, err := os.ReadFile(caddyfilePath + backupSuffix)
	if err != nil {
		t.Fatalf("read caddyfile backup: %v", err)
	}
	if !strings.Contains(string(caddyfileBak), "reverse_proxy http://plex:32400") {
		t.Errorf("caddyfile backup should preserve original directives, got:\n%s", caddyfileBak)
	}

	// Rewritten config.yaml: gateway: gone, gateway_sites: present.
	rewritten, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read rewritten config: %v", err)
	}
	body := string(rewritten)
	if strings.Contains(body, "gateway: "+caddyfilePath) {
		t.Errorf("rewritten config still contains legacy gateway: line:\n%s", body)
	}
	if !strings.Contains(body, "gateway_sites:") {
		t.Errorf("rewritten config missing gateway_sites: section:\n%s", body)
	}
	if !strings.Contains(body, "plex.example.com") || !strings.Contains(body, "sonarr.example.com") {
		t.Errorf("rewritten config missing migrated domains:\n%s", body)
	}

	// And Load() on the rewritten file should succeed without
	// triggering another migration (idempotent on already-migrated
	// configs).
	cfg2, err := Load(configPath)
	if err != nil {
		t.Fatalf("re-Load rewritten config: %v", err)
	}
	if cfg2.Server.Gateway != "" {
		t.Errorf("re-Load: gateway: should still be empty, got %q", cfg2.Server.Gateway)
	}
	if len(cfg2.Server.GatewaySites) != 2 {
		t.Errorf("re-Load: expected 2 sites, got %d", len(cfg2.Server.GatewaySites))
	}
}

// TestAutoMigrateGateway_RejectsLossyConversion pins the contract
// that when the converter emits any warning we refuse to silently
// rewrite the operator's config.yaml. A directive like
// `php_fastcgi` has no structured-form equivalent so the migrator
// would emit a warning and we must hard fail.
func TestAutoMigrateGateway_RejectsLossyConversion(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	caddyfilePath := filepath.Join(dir, "sites.Caddyfile")

	// php_fastcgi has no gateway_sites equivalent -> warning -> hard fail.
	if err := os.WriteFile(caddyfilePath, []byte(`example.com {
    reverse_proxy http://app:8000
    php_fastcgi unix//run/php/php-fpm.sock
}
`), 0o600); err != nil {
		t.Fatalf("write caddyfile: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`server:
  listen: ":8080"
  gateway: `+caddyfilePath+`
auth:
  method: none
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected Load to reject lossy migration, got nil error")
	}
	if !strings.Contains(err.Error(), "migrate-gateway") {
		t.Errorf("error should point at the migrate-gateway CLI; got %v", err)
	}

	// The original config.yaml must be untouched on failure.
	body, _ := os.ReadFile(configPath)
	if !strings.Contains(string(body), "gateway: "+caddyfilePath) {
		t.Errorf("original config.yaml should be untouched on failure; got:\n%s", body)
	}
	// And no backup files should exist (we abort before writing).
	if _, err := os.Stat(configPath + backupSuffix); err == nil {
		t.Error("config backup should not exist when migration is rejected")
	}
}

// TestAutoMigrateGateway_RejectsBothFormsSet covers the corner case
// where the operator has both legacy gateway: and a hand-written
// gateway_sites: already. We can't safely combine, so refuse.
func TestAutoMigrateGateway_RejectsBothFormsSet(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	caddyfilePath := filepath.Join(dir, "sites.Caddyfile")
	_ = os.WriteFile(caddyfilePath, []byte(`x.example.com { reverse_proxy http://x:1 }`), 0o600)
	if err := os.WriteFile(configPath, []byte(`server:
  listen: ":8080"
  gateway: `+caddyfilePath+`
  gateway_sites:
    - domain: y.example.com
      backend_url: http://y:2
auth:
  method: none
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected Load to refuse mixed gateway / gateway_sites configs")
	}
	if !strings.Contains(err.Error(), "both set") {
		t.Errorf("error should explain both forms are set; got %v", err)
	}
}

// TestAutoMigrateGateway_MissingCaddyfile pins the behaviour when
// the gateway: pointer is set but the file is gone (operator moved
// or deleted it but left the pointer dangling). We log + clear the
// field for this boot rather than failing closed.
func TestAutoMigrateGateway_MissingCaddyfile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`server:
  listen: ":8080"
  gateway: `+filepath.Join(dir, "does-not-exist.Caddyfile")+`
auth:
  method: none
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load with missing caddyfile should not fail; got %v", err)
	}
	if cfg.Server.Gateway != "" {
		t.Errorf("gateway field should be cleared when target file is missing, got %q", cfg.Server.Gateway)
	}
}
