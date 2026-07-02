package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	// Create a temporary config file
	content := `
server:
  listen: ":9090"
  title: "Test Dashboard"

navigation:
  position: left
  show_labels: true

groups:
  - name: Test Group
    icon:
      type: dashboard
      name: server
    color: "#ff0000"
    order: 1

apps:
  - name: Test App
    url: http://localhost:8080
    icon:
      type: lucide
      name: server
    color: "#00ff00"
    group: Test Group
    order: 1
    enabled: true
    default: true
    open_mode: iframe
    proxy: false
    scale: 1.0
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load the config
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify values
	if cfg.Server.Listen != ":9090" {
		t.Errorf("Expected listen :9090, got %s", cfg.Server.Listen)
	}
	if cfg.Server.Title != "Test Dashboard" {
		t.Errorf("Expected title 'Test Dashboard', got %s", cfg.Server.Title)
	}
	if cfg.Navigation.Position != "left" {
		t.Errorf("Expected position left, got %s", cfg.Navigation.Position)
	}
	if len(cfg.Groups) != 1 {
		t.Errorf("Expected 1 group, got %d", len(cfg.Groups))
	}
	if len(cfg.Apps) != 1 {
		t.Errorf("Expected 1 app, got %d", len(cfg.Apps))
	}
	if cfg.Apps[0].Name != "Test App" {
		t.Errorf("Expected app name 'Test App', got %s", cfg.Apps[0].Name)
	}
	// TLS should be empty by default
	if cfg.Server.NeedsCaddy() {
		t.Error("Expected NeedsCaddy() false with no TLS/gateway config")
	}
}

func TestLoadMissingFile(t *testing.T) {
	// The current implementation returns default config if file doesn't exist
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should return default config
	if cfg.Server.Listen != ":8080" {
		t.Errorf("Expected default listen :8080, got %s", cfg.Server.Listen)
	}
	if cfg.Server.Title != "Muximux" {
		t.Errorf("Expected default title 'Muximux', got %s", cfg.Server.Title)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	content := `
server:
  listen: [invalid yaml
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()

	if cfg.Server.Listen != ":8080" {
		t.Errorf("Expected default listen :8080, got %s", cfg.Server.Listen)
	}
	if cfg.Navigation.Position != "top" {
		t.Errorf("Expected default position top, got %s", cfg.Navigation.Position)
	}
	if cfg.Auth.Method != "none" {
		t.Errorf("Expected default auth method none, got %s", cfg.Auth.Method)
	}
	if !cfg.Health.Enabled {
		t.Error("Expected health to be enabled by default")
	}
	if !cfg.Icons.DashboardIcons.Enabled {
		t.Error("Expected dashboard icons to be enabled by default")
	}
	if cfg.Server.NeedsCaddy() {
		t.Error("Default config should not need Caddy")
	}
}

func TestDefaultShowHomeButton(t *testing.T) {
	cfg := defaultConfig()
	if !cfg.Navigation.ShowHomeButton {
		t.Error("ShowHomeButton should default to true")
	}
	if cfg.Navigation.HomeIcon != nil {
		t.Error("HomeIcon should default to nil")
	}
}

func TestShowHomeButtonFalseDisablesSplash(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`
navigation:
  show_home_button: false
  show_splash_on_startup: true
`), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Navigation.ShowHomeButton {
		t.Error("ShowHomeButton should be false")
	}
	if cfg.Navigation.ShowSplashOnStart {
		t.Error("ShowSplashOnStart should be forced to false when ShowHomeButton is false")
	}
}

func TestShowHomeButtonMissingDefaultsTrue(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`
navigation:
  position: left
`), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Navigation.ShowHomeButton {
		t.Error("ShowHomeButton should default to true when not in config")
	}
}

func TestHomeIconRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := defaultConfig()
	cfg.Navigation.HomeIcon = &AppIconConfig{Type: "lucide", Name: "star"}
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Navigation.HomeIcon == nil {
		t.Fatal("HomeIcon should be preserved after save/load")
	}
	if loaded.Navigation.HomeIcon.Type != "lucide" || loaded.Navigation.HomeIcon.Name != "star" {
		t.Errorf("HomeIcon = %+v, want lucide/star", loaded.Navigation.HomeIcon)
	}
}

func TestSave(t *testing.T) {
	cfg := defaultConfig()
	cfg.Server.Title = "Saved Config"
	cfg.Groups = []GroupConfig{
		{Name: "Test", Icon: AppIconConfig{Type: "dashboard", Name: "test"}, Color: "#000000", Order: 1},
	}

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "saved.yaml")

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Reload and verify
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to reload config: %v", err)
	}

	if loaded.Server.Title != "Saved Config" {
		t.Errorf("Expected title 'Saved Config', got %s", loaded.Server.Title)
	}
	if len(loaded.Groups) != 1 {
		t.Errorf("Expected 1 group, got %d", len(loaded.Groups))
	}
}

func TestNormalizedBasePath(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		want     string
	}{
		{"empty", "", ""},
		{"slash only", "/", ""},
		{"simple path", "/muximux", "/muximux"},
		{"trailing slash", "/muximux/", "/muximux"},
		{"no leading slash", "muximux", "/muximux"},
		{"nested path", "/apps/muximux", "/apps/muximux"},
		{"nested trailing slash", "/apps/muximux/", "/apps/muximux"},
		{"multiple trailing slashes", "/muximux///", "/muximux"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &ServerConfig{BasePath: tt.basePath}
			got := sc.NormalizedBasePath()
			if got != tt.want {
				t.Errorf("NormalizedBasePath(%q) = %q, want %q", tt.basePath, got, tt.want)
			}
		})
	}
}

func TestNeedsCaddy(t *testing.T) {
	tests := []struct {
		name   string
		server ServerConfig
		want   bool
	}{
		{"empty", ServerConfig{Listen: ":8080"}, false},
		{"domain set", ServerConfig{Listen: ":8080", TLS: TLSConfig{Domain: "example.com", Email: "a@b.com"}}, true},
		{"cert set", ServerConfig{Listen: ":8080", TLS: TLSConfig{Cert: "/a.pem", Key: "/b.pem"}}, true},
		{"gateway set", ServerConfig{Listen: ":8080", Gateway: "/path/to/file"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.server.NeedsCaddy(); got != tt.want {
				t.Errorf("NeedsCaddy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a dummy gateway file for the valid-gateway test
	gatewayPath := filepath.Join(tmpDir, "sites.Caddyfile")
	if err := os.WriteFile(gatewayPath, []byte("# test"), 0600); err != nil {
		t.Fatal(err)
	}
	_ = gatewayPath // kept for documentation; the legacy-gateway test now points at lossy.Caddyfile below
	// A Caddyfile with a directive the structured form can't
	// represent. Triggers the auto-migration's lossy-refuse path so
	// the legacy-server.gateway-with-lossy-directives test case
	// expects the "run migrate-gateway" error rather than silent
	// rewrite.
	lossyCaddyfile := filepath.Join(tmpDir, "lossy.Caddyfile")
	if err := os.WriteFile(lossyCaddyfile, []byte(`example.com {
    reverse_proxy http://app:8000
    php_fastcgi unix//run/php/php-fpm.sock
}
`), 0600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "domain without email",
			yaml: `
server:
  listen: ":8080"
  tls:
    domain: example.com
`,
			wantErr: "tls.email is required",
		},
		{
			name: "email without domain is rejected",
			yaml: `
server:
  listen: ":8080"
  tls:
    email: ops@example.com
`,
			wantErr: "tls.email is set but tls.domain is empty",
		},
		{
			name: "cert without key",
			yaml: `
server:
  listen: ":8080"
  tls:
    cert: /a.pem
`,
			wantErr: "tls.cert and tls.key must both be set",
		},
		{
			name: "key without cert",
			yaml: `
server:
  listen: ":8080"
  tls:
    key: /b.pem
`,
			wantErr: "tls.cert and tls.key must both be set",
		},
		{
			name: "domain and cert both set",
			yaml: `
server:
  listen: ":8080"
  tls:
    domain: example.com
    email: a@b.com
    cert: /a.pem
    key: /b.pem
`,
			wantErr: "use tls.domain or tls.cert/tls.key, not both",
		},
		{
			// v3.1.0 auto-migrates `server.gateway:` Caddyfiles to
			// the new `server.gateway_sites:` form when the
			// conversion is clean. When the Caddyfile contains
			// directives the structured form can't represent the
			// hook refuses to silently rewrite the operator's
			// config and surfaces a clear "run migrate-gateway"
			// message instead. Pin the lossy-rejection path here.
			name: "legacy server.gateway with lossy directives is rejected",
			yaml: `
server:
  listen: ":8080"
  gateway: ` + filepath.Join(tmpDir, "lossy.Caddyfile") + `
`,
			wantErr: "migrate-gateway",
		},
		{
			name: "valid domain config",
			yaml: `
server:
  listen: ":8080"
  tls:
    domain: example.com
    email: admin@example.com
`,
			wantErr: "",
		},
		{
			name: "valid cert config",
			yaml: `
server:
  listen: ":8080"
  tls:
    cert: /a.pem
    key: /b.pem
`,
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(tmpDir, tt.name+".yaml")
			if err := os.WriteFile(configPath, []byte(tt.yaml), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(configPath)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got: %v", tt.wantErr, err)
				}
			}
		})
	}
}

func TestNewConfigFields(t *testing.T) {
	t.Run("bar_style defaults to grouped", func(t *testing.T) {
		cfg := defaultConfig()
		if cfg.Navigation.BarStyle != "grouped" {
			t.Errorf("expected default bar_style 'grouped', got %q", cfg.Navigation.BarStyle)
		}
	})

	t.Run("bar_style parsed from YAML", func(t *testing.T) {
		content := `
navigation:
  bar_style: flat
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")
		if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Navigation.BarStyle != "flat" {
			t.Errorf("expected bar_style 'flat', got %q", cfg.Navigation.BarStyle)
		}
	})

	t.Run("health_check nil by default", func(t *testing.T) {
		content := `
apps:
  - name: TestApp
    url: http://localhost:8080
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")
		if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if len(cfg.Apps) != 1 {
			t.Fatal("expected 1 app")
		}
		if cfg.Apps[0].HealthCheck != nil {
			t.Error("expected nil health_check when not specified")
		}
	})

	t.Run("health_check false parsed", func(t *testing.T) {
		content := `
apps:
  - name: TestApp
    url: http://localhost:8080
    health_check: false
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")
		if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Apps[0].HealthCheck == nil {
			t.Fatal("expected non-nil health_check")
		}
		if *cfg.Apps[0].HealthCheck != false {
			t.Error("expected health_check=false")
		}
	})

	t.Run("shortcut parsed from YAML", func(t *testing.T) {
		content := `
apps:
  - name: TestApp
    url: http://localhost:8080
    shortcut: 3
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")
		if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Apps[0].Shortcut == nil {
			t.Fatal("expected non-nil shortcut")
		}
		if *cfg.Apps[0].Shortcut != 3 {
			t.Errorf("expected shortcut=3, got %d", *cfg.Apps[0].Shortcut)
		}
	})

	t.Run("shortcut nil by default", func(t *testing.T) {
		content := `
apps:
  - name: TestApp
    url: http://localhost:8080
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")
		if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Apps[0].Shortcut != nil {
			t.Error("expected nil shortcut when not specified")
		}
	})

	t.Run("base_path parsed from YAML", func(t *testing.T) {
		content := `
server:
  base_path: /dashboard
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")
		if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Server.BasePath != "/dashboard" {
			t.Errorf("expected base_path '/dashboard', got %q", cfg.Server.BasePath)
		}
	})

	t.Run("base_path empty by default", func(t *testing.T) {
		cfg := defaultConfig()
		if cfg.Server.BasePath != "" {
			t.Errorf("expected empty default base_path, got %q", cfg.Server.BasePath)
		}
	})

	t.Run("permissions nil when absent", func(t *testing.T) {
		content := `
apps:
  - name: TestApp
    url: http://localhost:8080
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")
		if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Apps[0].Permissions != nil {
			t.Errorf("expected nil permissions, got %v", cfg.Apps[0].Permissions)
		}
		if cfg.Apps[0].AllowNotifications {
			t.Error("expected allow_notifications to default to false")
		}
	})

	t.Run("permissions and allow_notifications parsed from YAML", func(t *testing.T) {
		content := `
apps:
  - name: TestApp
    url: http://localhost:8080
    permissions:
      - camera
      - microphone
      - geolocation
    allow_notifications: true
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")
		if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatal(err)
		}
		got := cfg.Apps[0].Permissions
		want := []string{"camera", "microphone", "geolocation"}
		if len(got) != len(want) {
			t.Fatalf("permissions: got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("permissions[%d]: got %q, want %q", i, got[i], want[i])
			}
		}
		if !cfg.Apps[0].AllowNotifications {
			t.Error("expected allow_notifications=true")
		}
	})
}

func TestLoadPreservesBcryptHash(t *testing.T) {
	hash := "$2a$10$/ijadDMQO1SvqjoQgLdKOO62yB9x3Voi2OZ5LSp3uVYeOGrqjmpq."
	content := `
auth:
  method: builtin
  users:
    - username: admin
      password_hash: ` + hash + `
      role: admin
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if len(cfg.Auth.Users) != 1 {
		t.Fatalf("Expected 1 user, got %d", len(cfg.Auth.Users))
	}
	if cfg.Auth.Users[0].PasswordHash != hash {
		t.Errorf("Bcrypt hash corrupted:\n  want: %s\n  got:  %s", hash, cfg.Auth.Users[0].PasswordHash)
	}
}

func TestLoadExpandsBracedEnvVars(t *testing.T) {
	t.Setenv("MUXIMUX_TEST_TITLE", "FromEnv")
	content := `
server:
  title: ${MUXIMUX_TEST_TITLE}
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Server.Title != "FromEnv" {
		t.Errorf("Expected title 'FromEnv', got %q", cfg.Server.Title)
	}
}

func TestLoadIgnoresBareEnvVars(t *testing.T) {
	content := `
server:
  title: $NONEXISTENT_VAR_test
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Server.Title != "$NONEXISTENT_VAR_test" {
		t.Errorf("Bare $VAR should not be expanded, got %q", cfg.Server.Title)
	}
}

func TestLoadPreservesUnsetBracedEnvVars(t *testing.T) {
	// ${UNSET_VAR} should be left as-is if the env var doesn't exist,
	// protecting OIDC secrets, proxy headers, etc. from silent corruption.
	content := `
auth:
  oidc:
    client_secret: "abc${MUXIMUX_TEST_NONEXISTENT}xyz"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	want := "abc${MUXIMUX_TEST_NONEXISTENT}xyz"
	if cfg.Auth.OIDC.ClientSecret != want {
		t.Errorf("Unset ${VAR} should be preserved:\n  want: %s\n  got:  %s", want, cfg.Auth.OIDC.ClientSecret)
	}
}

func TestNeedsSetup(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want bool
	}{
		{
			name: "fresh install needs setup",
			cfg:  *defaultConfig(),
			want: true,
		},
		{
			name: "setup_complete flag set",
			cfg:  Config{Auth: AuthConfig{SetupComplete: true}},
			want: false,
		},
		{
			name: "existing apps",
			cfg: Config{
				Auth: AuthConfig{Method: "none"},
				Apps: []AppConfig{{Name: "test", URL: "http://localhost"}},
			},
			want: false,
		},
		{
			name: "non-default auth method",
			cfg:  Config{Auth: AuthConfig{Method: "builtin"}},
			want: false,
		},
		{
			name: "existing users",
			cfg: Config{
				Auth: AuthConfig{
					Method: "none",
					Users:  []UserConfig{{Username: "admin"}},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.NeedsSetup()
			if got != tt.want {
				t.Errorf("NeedsSetup() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSave_RoundTrip covers findings.md L10's happy path: Save writes
// the config atomically (temp-file + rename) and the result parses back.
// The fsync-parent-dir addition itself is best-effort and can't be
// portably observed from user space, but the round-trip at minimum
// confirms the save path still produces a readable file.
func TestSave_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")
	c := &Config{
		Server: ServerConfig{Listen: ":8080", Title: "Round Trip"},
		Apps: []AppConfig{
			{Name: "App", URL: "http://example.com", Enabled: true},
		},
	}
	if err := c.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// File must exist with mode 0600.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat saved config: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("saved config mode = %o, want 0600", info.Mode().Perm())
	}

	// No stray .config-*.yaml temp file left behind.
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected exactly one file in dir, got %d entries", len(entries))
	}

	// Parseable as our own Config type.
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load round-tripped config: %v", err)
	}
	if loaded.Server.Title != "Round Trip" {
		t.Errorf("Server.Title = %q, want Round Trip", loaded.Server.Title)
	}
	if len(loaded.Apps) != 1 || loaded.Apps[0].Name != "App" {
		t.Errorf("Apps mismatch: %+v", loaded.Apps)
	}
}

func TestValidateGatewaySite(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")
	if err := os.WriteFile(certPath, []byte("dummy"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte("dummy"), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		site    GatewaySite
		srv     *ServerConfig
		wantErr string
	}{
		{
			name:    "minimal valid auto-tls site",
			site:    GatewaySite{Domain: "sonarr.example.com", BackendURL: "http://sonarr:8989"},
			wantErr: "",
		},
		{
			name:    "missing domain",
			site:    GatewaySite{BackendURL: "http://app:80"},
			wantErr: "domain is required",
		},
		{
			name:    "wildcard domain rejected",
			site:    GatewaySite{Domain: "*.example.com", BackendURL: "http://app:80"},
			wantErr: "is not a valid hostname",
		},
		{
			name:    "domain collides with primary muximux domain",
			site:    GatewaySite{Domain: "muximux.example.com", BackendURL: "http://app:80"},
			srv:     &ServerConfig{TLS: TLSConfig{Domain: "muximux.example.com"}},
			wantErr: "collides with server.tls.domain",
		},
		{
			name:    "missing backend",
			site:    GatewaySite{Domain: "x.example.com"},
			wantErr: "backend_url is required",
		},
		{
			name:    "backend with non-http scheme",
			site:    GatewaySite{Domain: "x.example.com", BackendURL: "ftp://app:80"},
			wantErr: "must use http or https",
		},
		{
			name:    "backend with empty host",
			site:    GatewaySite{Domain: "x.example.com", BackendURL: "http://"},
			wantErr: "missing a host",
		},
		{
			name:    "backend pointing at 0.0.0.0 rejected",
			site:    GatewaySite{Domain: "x.example.com", BackendURL: "http://0.0.0.0:8080"},
			wantErr: "0.0.0.0",
		},
		{
			name:    "private IP backend allowed",
			site:    GatewaySite{Domain: "x.example.com", BackendURL: "http://192.168.1.42:8080"},
			wantErr: "",
		},
		{
			name:    "tls custom requires both cert and key",
			site:    GatewaySite{Domain: "x.example.com", BackendURL: "http://app:80", TLS: "custom", TLSCert: certPath},
			wantErr: "requires both tls_cert and tls_key",
		},
		{
			name:    "tls custom with valid files",
			site:    GatewaySite{Domain: "x.example.com", BackendURL: "http://app:80", TLS: "custom", TLSCert: certPath, TLSKey: keyPath},
			wantErr: "",
		},
		{
			name:    "tls custom with unreadable cert",
			site:    GatewaySite{Domain: "x.example.com", BackendURL: "http://app:80", TLS: "custom", TLSCert: "/no/such/cert.pem", TLSKey: keyPath},
			wantErr: "tls_cert",
		},
		{
			name:    "tls auto rejects cert path",
			site:    GatewaySite{Domain: "x.example.com", BackendURL: "http://app:80", TLSCert: certPath},
			wantErr: "only valid when tls is",
		},
		{
			name:    "invalid tls value",
			site:    GatewaySite{Domain: "x.example.com", BackendURL: "http://app:80", TLS: "magic"},
			wantErr: `tls="magic" is invalid`,
		},
		{
			name:    "tls none with cert is rejected",
			site:    GatewaySite{Domain: "x.example.com", BackendURL: "http://app:80", TLS: "none", TLSCert: certPath, TLSKey: keyPath},
			wantErr: "only valid when tls is",
		},
		{
			name:    "backend with path component is rejected",
			site:    GatewaySite{Domain: "x.example.com", BackendURL: "http://app:80/api"},
			wantErr: "must not include a path",
		},
		{
			name:    "backend with bare slash path is allowed",
			site:    GatewaySite{Domain: "x.example.com", BackendURL: "http://app:80/"},
			wantErr: "",
		},
		{
			name:    "backend with query string is rejected",
			site:    GatewaySite{Domain: "x.example.com", BackendURL: "http://app:80?token=abc"},
			wantErr: "query string",
		},
		{
			name:    "backend with fragment is rejected",
			site:    GatewaySite{Domain: "x.example.com", BackendURL: "http://app:80#frag"},
			wantErr: "fragment",
		},
		{
			name: "proxy_headers key with space is rejected",
			site: GatewaySite{
				Domain:       "x.example.com",
				BackendURL:   "http://app:80",
				ProxyHeaders: map[string]string{"X Bad": "value"},
			},
			wantErr: "valid HTTP header name",
		},
		{
			name: "proxy_headers key with newline is rejected",
			site: GatewaySite{
				Domain:       "x.example.com",
				BackendURL:   "http://app:80",
				ProxyHeaders: map[string]string{"X-Bad\n": "value"},
			},
			wantErr: "valid HTTP header name",
		},
		{
			name: "proxy_headers value with control char is rejected",
			site: GatewaySite{
				Domain:       "x.example.com",
				BackendURL:   "http://app:80",
				ProxyHeaders: map[string]string{"X-Api-Key": "good\rinjected"},
			},
			wantErr: "invalid character",
		},
		{
			name: "proxy_headers value with double quote is rejected",
			site: GatewaySite{
				Domain:       "x.example.com",
				BackendURL:   "http://app:80",
				ProxyHeaders: map[string]string{"X-Api-Key": `bad"quote`},
			},
			wantErr: "invalid character",
		},
		{
			name: "proxy_headers with valid token-grammar key and tab-containing value pass",
			site: GatewaySite{
				Domain:       "x.example.com",
				BackendURL:   "http://app:80",
				ProxyHeaders: map[string]string{"X-Api-Key!": "value\twith\ttab"},
			},
			wantErr: "",
		},
		{
			name:    "self-loop wildcard listen + localhost backend",
			site:    GatewaySite{Domain: "x.example.com", BackendURL: "http://localhost:8080"},
			srv:     &ServerConfig{Listen: ":8080"},
			wantErr: "loop the proxy back",
		},
		{
			name:    "self-loop wildcard listen + 127.0.0.1 backend",
			site:    GatewaySite{Domain: "x.example.com", BackendURL: "http://127.0.0.1:8080"},
			srv:     &ServerConfig{Listen: "0.0.0.0:8080"},
			wantErr: "loop the proxy back",
		},
		{
			name:    "self-loop exact host:port match",
			site:    GatewaySite{Domain: "x.example.com", BackendURL: "http://192.168.1.10:8080"},
			srv:     &ServerConfig{Listen: "192.168.1.10:8080"},
			wantErr: "loop the proxy back",
		},
		{
			name:    "self-loop port mismatch is allowed",
			site:    GatewaySite{Domain: "x.example.com", BackendURL: "http://localhost:9000"},
			srv:     &ServerConfig{Listen: ":8080"},
			wantErr: "",
		},
		{
			name:    "self-loop different host is allowed",
			site:    GatewaySite{Domain: "x.example.com", BackendURL: "http://10.0.0.5:8080"},
			srv:     &ServerConfig{Listen: "127.0.0.1:8080"},
			wantErr: "",
		},
		{
			name:    "self-loop default https port + listener",
			site:    GatewaySite{Domain: "x.example.com", BackendURL: "https://localhost"},
			srv:     &ServerConfig{Listen: ":443"},
			wantErr: "loop the proxy back",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateGatewaySite(&c.site, c.srv)
			if c.wantErr == "" {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", c.wantErr)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), c.wantErr)
			}
		})
	}
}

func TestValidateGatewaySites_DuplicateDomain(t *testing.T) {
	sites := []GatewaySite{
		{Domain: "Sonarr.Example.com", BackendURL: "http://sonarr:8989"},
		{Domain: "sonarr.example.com", BackendURL: "http://sonarr2:8989"},
	}
	err := validateGatewaySites(sites, nil)
	if err == nil {
		t.Fatal("expected duplicate-domain error (case-insensitive match)")
	}
	if !strings.Contains(err.Error(), "duplicate domain") {
		t.Errorf("error %q does not mention duplicate", err.Error())
	}
}

func TestValidateGatewaySites_AppNameMustExist(t *testing.T) {
	cfg := &Config{
		Apps: []AppConfig{{Name: "Sonarr", URL: "http://sonarr:8989", Enabled: true}},
	}
	t.Run("matching app passes", func(t *testing.T) {
		sites := []GatewaySite{
			{Domain: "sonarr.example.com", BackendURL: "http://sonarr:8989", AppName: "Sonarr"},
		}
		if err := validateGatewaySites(sites, cfg); err != nil {
			t.Errorf("expected nil error for matching app_name, got: %v", err)
		}
	})

	t.Run("dangling app_name is rejected", func(t *testing.T) {
		sites := []GatewaySite{
			{Domain: "ghost.example.com", BackendURL: "http://x:80", AppName: "DoesNotExist"},
		}
		err := validateGatewaySites(sites, cfg)
		if err == nil {
			t.Fatal("expected dangling app_name to be rejected")
		}
		if !strings.Contains(err.Error(), "does not match any apps") {
			t.Errorf("error %q does not mention dangling app_name", err.Error())
		}
	})

	t.Run("nil cfg disables the cross-reference check", func(t *testing.T) {
		// Backwards-compat for unit-test callers that don't have a
		// full Config; the loader and handler always pass a real cfg.
		sites := []GatewaySite{
			{Domain: "x.example.com", BackendURL: "http://x:80", AppName: "AnythingGoes"},
		}
		if err := validateGatewaySites(sites, nil); err != nil {
			t.Errorf("expected nil cfg to skip the check, got: %v", err)
		}
	})

	t.Run("empty apps list disables the check", func(t *testing.T) {
		// Edge case: the validator runs at import preview time when
		// the imported config might legitimately have apps defined
		// but none yet known to the validator. Skip when apps is
		// empty rather than failing.
		sites := []GatewaySite{
			{Domain: "x.example.com", BackendURL: "http://x:80", AppName: "AnyApp"},
		}
		if err := validateGatewaySites(sites, &Config{}); err != nil {
			t.Errorf("expected empty Apps to skip the check, got: %v", err)
		}
	})
}

func TestValidateGatewaySites_DuplicateAppName(t *testing.T) {
	cfg := &Config{
		Apps: []AppConfig{{Name: "Sonarr", URL: "http://sonarr:8989", Enabled: true}},
	}
	sites := []GatewaySite{
		{Domain: "first.example.com", BackendURL: "http://x:80", AppName: "Sonarr"},
		{Domain: "second.example.com", BackendURL: "http://x:80", AppName: "Sonarr"},
	}
	err := validateGatewaySites(sites, cfg)
	if err == nil {
		t.Fatal("expected duplicate app_name to be rejected")
	}
	if !strings.Contains(err.Error(), "already used") {
		t.Errorf("error %q does not mention duplicate use", err.Error())
	}
}

func TestIsValidGatewayDomain(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"sonarr.example.com", true},
		{"a.b", true},
		{"single-label", true},
		{"", false},
		{".", false},
		{"-leading-hyphen.example.com", false},
		{"trailing-hyphen-.example.com", false},
		{"underscores_not_allowed.example.com", false},
		{"*.example.com", false},
		{"with space.example.com", false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := isValidGatewayDomain(c.in); got != c.want {
				t.Errorf("isValidGatewayDomain(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestValidateGatewayListen(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"", false},               // empty = use default 80/443
		{":8443", false},          // port-only
		{"0.0.0.0:8443", false},   // explicit all-interfaces
		{"127.0.0.1:8443", false}, // localhost-only
		{"[::]:8443", false},      // IPv6 all-interfaces
		{":80", false},            // privileged but well-formed
		{"8443", true},            // missing port separator
		{":notaport", true},       // non-numeric port
		{":99999", true},          // out-of-range port
		{"host:", true},           // bare colon, no port
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			err := validateGatewayListen(c.in)
			if (err != nil) != c.wantErr {
				t.Errorf("validateGatewayListen(%q) err=%v wantErr=%v", c.in, err, c.wantErr)
			}
		})
	}
}

// TestValidateSessionCookieDomain covers the cross-subdomain cookie
// rules required by the gateway auth gate.
func TestValidateSessionCookieDomain(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "empty + no gated sites is fine",
			cfg:  Config{},
		},
		{
			name: "empty + gated site is rejected",
			cfg: Config{
				Server: ServerConfig{
					GatewaySites: []GatewaySite{{Domain: "x.example.com", RequireAuth: true}},
				},
			},
			wantErr: true,
		},
		{
			name: "leading-dot domain accepted",
			cfg: Config{
				Server: ServerConfig{
					SessionCookieDomain: ".example.com",
					TLS:                 TLSConfig{Domain: "muximux.example.com"},
					GatewaySites:        []GatewaySite{{Domain: "sonarr.example.com", RequireAuth: true}},
				},
			},
		},
		{
			name: "no-dot domain accepted (browser normalises)",
			cfg: Config{
				Server: ServerConfig{
					SessionCookieDomain: "example.com",
					GatewaySites:        []GatewaySite{{Domain: "sonarr.example.com", RequireAuth: true}},
				},
			},
		},
		{
			name: "site outside the cookie domain is rejected",
			cfg: Config{
				Server: ServerConfig{
					SessionCookieDomain: ".example.com",
					GatewaySites:        []GatewaySite{{Domain: "sonarr.other.org", RequireAuth: true}},
				},
			},
			wantErr: true,
		},
		{
			name: "tls.domain outside cookie domain is rejected",
			cfg: Config{
				Server: ServerConfig{
					SessionCookieDomain: ".example.com",
					TLS:                 TLSConfig{Domain: "muximux.elsewhere.com"},
				},
			},
			wantErr: true,
		},
		{
			name: "substring trap: evil-example.com must not satisfy parent=example.com",
			cfg: Config{
				Server: ServerConfig{
					SessionCookieDomain: ".example.com",
					GatewaySites:        []GatewaySite{{Domain: "evil-example.com", RequireAuth: true}},
				},
			},
			wantErr: true,
		},
		{
			name: "ungated sites are not subject to the subdomain check",
			cfg: Config{
				Server: ServerConfig{
					SessionCookieDomain: ".example.com",
					GatewaySites:        []GatewaySite{{Domain: "anywhere.org", RequireAuth: false}},
				},
			},
		},
		{
			name: "only-dot domain rejected",
			cfg: Config{
				Server: ServerConfig{
					SessionCookieDomain: ".",
				},
			},
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateSessionCookieDomain(&c.cfg)
			if (err != nil) != c.wantErr {
				t.Errorf("err=%v wantErr=%v", err, c.wantErr)
			}
		})
	}
}

func TestValidateGatewaySite_MinRoleWhenRequireAuth(t *testing.T) {
	cases := []struct {
		role    string
		wantErr bool
	}{
		{"", false},
		{"user", false},
		{"power-user", false},
		{"admin", false},
		{"superadmin", true}, // unknown role
		{"User", true},       // case-sensitive
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			s := &GatewaySite{
				Domain:      "x.example.com",
				BackendURL:  "http://10.0.0.5:80",
				TLS:         TLSModeAuto,
				RequireAuth: true,
				MinRole:     c.role,
			}
			err := validateGatewaySite(s, nil)
			if (err != nil) != c.wantErr {
				t.Errorf("MinRole=%q err=%v wantErr=%v", c.role, err, c.wantErr)
			}
		})
	}
}

func TestValidateGatewaySite_MinRoleIgnoredWhenRequireAuthFalse(t *testing.T) {
	// MinRole="garbage" is dead config when RequireAuth is false; do
	// not surface an error for the sake of operators experimenting
	// with the flag.
	s := &GatewaySite{
		Domain:      "x.example.com",
		BackendURL:  "http://10.0.0.5:80",
		TLS:         TLSModeAuto,
		RequireAuth: false,
		MinRole:     "garbage",
	}
	if err := validateGatewaySite(s, nil); err != nil {
		t.Errorf("MinRole should be ignored when RequireAuth=false; got err=%v", err)
	}
}

func TestValidate_HTTPAction(t *testing.T) {
	base := func() *Config {
		return &Config{
			Apps: []AppConfig{{
				Name:             "Webhook",
				URL:              "https://example.com/hook",
				Enabled:          true,
				OpenMode:         "http_action",
				HTTPActionMethod: "POST",
			}},
		}
	}
	cases := []struct {
		name    string
		mutate  func(*Config)
		wantErr string // substring; "" = expect success
	}{
		{"ok defaults", func(c *Config) {}, ""},
		{"ok all verbs GET", func(c *Config) { c.Apps[0].HTTPActionMethod = "GET" }, ""},
		{"ok all verbs PUT", func(c *Config) { c.Apps[0].HTTPActionMethod = "PUT" }, ""},
		{"ok all verbs DELETE", func(c *Config) { c.Apps[0].HTTPActionMethod = "DELETE" }, ""},
		{"ok all verbs PATCH", func(c *Config) { c.Apps[0].HTTPActionMethod = "PATCH" }, ""},
		{"ok empty method defaults to POST", func(c *Config) { c.Apps[0].HTTPActionMethod = "" }, ""},
		{"reject method", func(c *Config) { c.Apps[0].HTTPActionMethod = "TRACE" }, "http_action_method"},
		{"reject empty url", func(c *Config) { c.Apps[0].URL = "" }, "url"},
		{"reject file scheme", func(c *Config) { c.Apps[0].URL = "file:///etc/passwd" }, "http or https"},
		{"reject gopher scheme", func(c *Config) { c.Apps[0].URL = "gopher://example.com/" }, "http or https"},
		{"reject bare path", func(c *Config) { c.Apps[0].URL = "http:///path" }, "hostname"},
		{"ok valid header", func(c *Config) {
			c.Apps[0].HTTPActionHeaders = map[string]string{"Authorization": "Bearer abc"}
		}, ""},
		{"reject header with space in key", func(c *Config) {
			c.Apps[0].HTTPActionHeaders = map[string]string{"X Bad": "value"}
		}, "header key"},
		{"reject header with colon in key", func(c *Config) {
			c.Apps[0].HTTPActionHeaders = map[string]string{"X:Bad": "value"}
		}, "header key"},
		{"reject CR in header value", func(c *Config) {
			c.Apps[0].HTTPActionHeaders = map[string]string{"X-Header": "v\rx"}
		}, "header value"},
		{"reject LF in header value", func(c *Config) {
			c.Apps[0].HTTPActionHeaders = map[string]string{"X-Header": "v\nx"}
		}, "header value"},
		{"reject NUL in header value", func(c *Config) {
			c.Apps[0].HTTPActionHeaders = map[string]string{"X-Header": "v\x00x"}
		}, "header value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base()
			tc.mutate(cfg)
			err := cfg.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected success, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidate_HTTPActionFieldsIgnoredForOtherModes(t *testing.T) {
	cfg := &Config{
		Apps: []AppConfig{{
			Name:              "App",
			URL:               "http://example.com",
			Enabled:           true,
			OpenMode:          "iframe",
			HTTPActionMethod:  "POST",
			HTTPActionHeaders: map[string]string{"X-Token": "abc"},
		}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("non-http_action mode should accept http_action fields, got %v", err)
	}
}

func TestIsBracedEnvRef(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"${HOME}", true},
		{"prefix ${VAR} suffix", true},
		{"${A}${B}", true},
		{"$HOME", false},
		{"plain string", false},
		{"", false},
		{"$2a$10$abcdef", false}, // bcrypt-shaped strings must not match
		{"${}", false},           // empty braces are not a valid var ref
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := IsBracedEnvRef(c.in); got != c.want {
				t.Errorf("IsBracedEnvRef(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestValidateGatewaySites_PublicWrapper(t *testing.T) {
	// The exported wrapper should accept nil cfg and an empty slice.
	if err := ValidateGatewaySites(nil, nil); err != nil {
		t.Errorf("empty + nil cfg should validate, got %v", err)
	}
	if err := ValidateGatewaySites([]GatewaySite{}, nil); err != nil {
		t.Errorf("empty slice + nil cfg should validate, got %v", err)
	}
	// A clearly invalid site should produce an error through the wrapper too.
	if err := ValidateGatewaySites([]GatewaySite{{Domain: ""}}, nil); err == nil {
		t.Error("empty domain should fail validation")
	}
}

func TestDefaultDockerEndpoint(t *testing.T) {
	// Helper is OS-specific. Just confirm we get a non-empty endpoint
	// in either branch and that it uses an expected scheme.
	got := defaultDockerEndpoint()
	if got == "" {
		t.Fatal("defaultDockerEndpoint returned empty")
	}
	if !strings.HasPrefix(got, "unix://") && !strings.HasPrefix(got, "npipe://") {
		t.Errorf("expected unix:// or npipe:// prefix, got %q", got)
	}
}

func TestHostIsUnderParent(t *testing.T) {
	cases := []struct {
		host, parent string
		want         bool
	}{
		{"example.com", "example.com", true},
		{"sub.example.com", "example.com", true},
		{"deep.sub.example.com", "example.com", true},
		{"EXAMPLE.COM", "example.com", true},
		{"example.com", "EXAMPLE.COM", true},
		{"otherexample.com", "example.com", false},
		{"example.com", "sub.example.com", false},
		{"", "example.com", false},
		{"example.com", "", false},
	}
	for _, c := range cases {
		t.Run(c.host+"_under_"+c.parent, func(t *testing.T) {
			if got := hostIsUnderParent(c.host, c.parent); got != c.want {
				t.Errorf("hostIsUnderParent(%q, %q) = %v, want %v", c.host, c.parent, got, c.want)
			}
		})
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config.yaml")
	method := "POST"
	confirm := true
	showToast := false
	original := &Config{
		Apps: []AppConfig{{
			Name: "Webhook", URL: "https://example.com/hook", Enabled: true,
			OpenMode: "http_action", HTTPActionMethod: method,
			HTTPActionHeaders:   map[string]string{"Authorization": "Bearer t"},
			HTTPActionConfirm:   confirm,
			HTTPActionShowToast: &showToast,
		}, {
			Name: "Plain", URL: "https://plain.example", Enabled: true, OpenMode: "iframe",
		}},
	}
	if err := original.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Apps) != 2 {
		t.Fatalf("len(Apps) = %d, want 2", len(loaded.Apps))
	}
	if loaded.Apps[0].OpenMode != "http_action" {
		t.Errorf("OpenMode lost: %q", loaded.Apps[0].OpenMode)
	}
	if loaded.Apps[0].HTTPActionMethod != "POST" {
		t.Errorf("HTTPActionMethod lost: %q", loaded.Apps[0].HTTPActionMethod)
	}
	if loaded.Apps[0].HTTPActionHeaders["Authorization"] != "Bearer t" {
		t.Errorf("Authorization header lost: %q", loaded.Apps[0].HTTPActionHeaders["Authorization"])
	}
	if !loaded.Apps[0].HTTPActionConfirm {
		t.Error("HTTPActionConfirm lost")
	}
	if loaded.Apps[0].HTTPActionShowToast == nil || *loaded.Apps[0].HTTPActionShowToast != false {
		t.Errorf("HTTPActionShowToast lost: %v", loaded.Apps[0].HTTPActionShowToast)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("config file perms = %o, want 0600", info.Mode().Perm())
	}
}

func TestValidateGatewayListen_BadAddresses(t *testing.T) {
	cases := []struct {
		name    string
		addr    string
		wantErr string
	}{
		{"empty ok", "", ""},
		{"port only", ":8443", ""},
		{"host:port", "0.0.0.0:8443", ""},
		{"missing port", "0.0.0.0", "not a valid bind address"},
		{"bad port chars", ":notaport", "invalid port"},
		{"port out of range", ":99999", "invalid port"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateGatewayListen(c.addr)
			if c.wantErr == "" {
				if err != nil {
					t.Errorf("expected ok, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("expected error containing %q, got %v", c.wantErr, err)
			}
		})
	}
}

func TestIsValidHeaderName_FullCoverage(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"Authorization", true},
		{"X-Tok_123", true},
		{"X.Y", true},
		{"!#$%&'*+-.^_`|~", true},
		{"AaZz09", true},
		{"X Bad", false}, // space
		{"X:Bad", false}, // colon
		{"X@Bad", false}, // @ not allowed
		{"XéBad", false}, // accented char
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := isValidHeaderName(c.in); got != c.want {
				t.Errorf("isValidHeaderName(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestSave_MkdirFailure(t *testing.T) {
	// Create a regular file, then try to Save to a path that requires
	// creating a subdirectory under that file. MkdirAll should fail.
	tmpDir := t.TempDir()
	blocker := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blocker, []byte("regular file"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := defaultConfig()
	err := cfg.Save(filepath.Join(blocker, "sub", "config.yaml"))
	if err == nil {
		t.Error("expected error when MkdirAll cannot create dir under a regular file")
	}
}

func TestSave_DotDirNoSubdirCreated(t *testing.T) {
	// When path's directory is ".", Save should skip MkdirAll. We test
	// from a temp dir so the cwd doesn't leak.
	tmpDir := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	cfg := defaultConfig()
	if err := cfg.Save("local-config.yaml"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "local-config.yaml")); err != nil {
		t.Errorf("config was not saved: %v", err)
	}
}

func TestValidate_TLSCombinations(t *testing.T) {
	cases := []struct {
		name    string
		tls     TLSConfig
		wantErr string
	}{
		{"empty", TLSConfig{}, ""},
		{"domain without email", TLSConfig{Domain: "example.com"}, "tls.email is required"},
		{"email without domain", TLSConfig{Email: "a@b.com"}, "tls.email is set but tls.domain is empty"},
		{"cert without key", TLSConfig{Cert: "/a.pem"}, "must both be set"},
		{"key without cert", TLSConfig{Key: "/a.key"}, "must both be set"},
		{"domain plus cert", TLSConfig{Domain: "example.com", Email: "a@b.com", Cert: "/a.pem", Key: "/a.key"}, "not both"},
		{"acme ok", TLSConfig{Domain: "example.com", Email: "a@b.com"}, ""},
		{"custom-cert ok", TLSConfig{Cert: "/a.pem", Key: "/a.key"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := &Config{Server: ServerConfig{Listen: ":8080", TLS: c.tls}}
			err := cfg.Validate()
			if c.wantErr == "" {
				if err != nil {
					t.Errorf("expected ok, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("want error containing %q, got %v", c.wantErr, err)
			}
		})
	}
}

func TestValidate_LegacyGatewayRejected(t *testing.T) {
	cfg := &Config{Server: ServerConfig{Listen: ":8080", Gateway: "/etc/caddy/Caddyfile"}}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "no longer supported") {
		t.Errorf("expected rejection of legacy gateway, got %v", err)
	}
}

func TestIsSelfLoop(t *testing.T) {
	cases := []struct {
		name, backendHost, backendPort, listen string
		want                                   bool
	}{
		{"empty listen", "127.0.0.1", "8080", "", false},
		{"malformed listen", "127.0.0.1", "8080", "bad-no-port", false},
		{"port mismatch", "127.0.0.1", "8080", ":9090", false},
		{"wildcard listen, loopback backend", "127.0.0.1", "8080", "0.0.0.0:8080", true},
		{"wildcard listen, localhost backend", "localhost", "8080", "0.0.0.0:8080", true},
		{"wildcard listen empty-host, loopback", "127.0.0.1", "8080", ":8080", true},
		{"loopback listen, loopback backend", "127.0.0.1", "8080", "127.0.0.1:8080", true},
		{"localhost listen, loopback backend", "localhost", "8080", "localhost:8080", true},
		{"same host both", "192.168.1.5", "8080", "192.168.1.5:8080", true},
		{"different hosts", "10.0.0.5", "8080", "192.168.1.1:8080", false},
		{"wildcard listen, non-loopback backend", "10.0.0.5", "8080", "0.0.0.0:8080", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isSelfLoop(c.backendHost, c.backendPort, c.listen); got != c.want {
				t.Errorf("isSelfLoop(%q,%q,%q) = %v, want %v", c.backendHost, c.backendPort, c.listen, got, c.want)
			}
		})
	}
}

func TestValidate_LifecycleMinRole_AcceptsKnownRoles(t *testing.T) {
	for _, role := range []string{"", "admin", "power-user", "user"} {
		t.Run(role, func(t *testing.T) {
			c := &Config{}
			c.Discovery.Docker.LifecycleEnabled = true
			c.Discovery.Docker.LifecycleMinRole = role
			if err := c.validate(); err != nil {
				t.Fatalf("expected nil err for role %q, got %v", role, err)
			}
		})
	}
}

func TestValidate_LifecycleMinRole_RejectsUnknown(t *testing.T) {
	c := &Config{}
	c.Discovery.Docker.LifecycleEnabled = true
	c.Discovery.Docker.LifecycleMinRole = "superuser"
	err := c.validate()
	if err == nil || !strings.Contains(err.Error(), "lifecycle_min_role") {
		t.Fatalf("expected lifecycle_min_role error, got %v", err)
	}
}

func TestValidate_HealthBadgePlacement_AcceptsKnownValues(t *testing.T) {
	for _, v := range []string{"", "off", "overview", "overview_and_nav"} {
		c := &Config{}
		c.Discovery.Docker.HealthBadgePlacement = v
		if err := c.validate(); err != nil {
			t.Fatalf("placement %q: %v", v, err)
		}
	}
}

func TestValidate_HealthBadgePlacement_RejectsUnknown(t *testing.T) {
	c := &Config{}
	c.Discovery.Docker.HealthBadgePlacement = "popup"
	err := c.validate()
	if err == nil || !strings.Contains(err.Error(), "health_badge_placement") {
		t.Fatalf("expected health_badge_placement error, got %v", err)
	}
}

func TestValidate_LifecycleAllowedGroups_RejectsUnknownGroupName(t *testing.T) {
	c := &Config{
		Groups: []GroupConfig{{Name: "family"}},
	}
	c.Discovery.Docker.LifecycleEnabled = true
	c.Discovery.Docker.LifecycleAllowedGroups = []string{"family", "ghosts"}
	err := c.validate()
	if err == nil || !strings.Contains(err.Error(), "ghosts") {
		t.Fatalf("expected unknown-group error mentioning ghosts, got %v", err)
	}
}

func TestDefaults_LifecycleMinRole_DefaultsToAdminWhenLifecycleEnabled(t *testing.T) {
	c := &Config{}
	c.Discovery.Docker.Enabled = true
	c.Discovery.Docker.LifecycleEnabled = true
	applyDiscoveryDefaults(c)
	if c.Discovery.Docker.LifecycleMinRole != "admin" {
		t.Fatalf("want admin, got %q", c.Discovery.Docker.LifecycleMinRole)
	}
	if c.Discovery.Docker.HealthBadgePlacement != "overview" {
		t.Fatalf("want overview, got %q", c.Discovery.Docker.HealthBadgePlacement)
	}
}

func TestNormalizeAutoImport(t *testing.T) {
	cases := map[AutoImportMode]AutoImportMode{
		"":        AutoImportOff, // absent defaults to off
		"off":     AutoImportOff,
		"add":     AutoImportAdd,
		"update":  AutoImportUpdate,
		"sync":    AutoImportSync,
		"SYNC":    AutoImportSync, // case-insensitive
		"garbage": AutoImportOff,  // unknown fails closed
	}
	for in, want := range cases {
		if got := NormalizeAutoImport(in); got != want {
			t.Errorf("NormalizeAutoImport(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLoadDetachClearsAutoImported(t *testing.T) {
	// An auto-imported app whose URL was hand-edited (URL != DockerManagedURL)
	// must detach: DockerKey and DockerAutoImported both cleared.
	app := AppConfig{
		Name: "Sonarr", URL: "http://edited:8989",
		DockerKey: "label:sonarr", DockerManagedURL: "http://old:8989",
		DockerAutoImported: true,
	}
	detachIfHandEdited(&app) // the helper Load() already calls
	if app.DockerKey != "" || app.DockerAutoImported {
		t.Errorf("expected detach to clear tracking, got key=%q auto=%v",
			app.DockerKey, app.DockerAutoImported)
	}
}

// TestSave_RemovesTempFileOnRenameFailure: every Save error path cleans
// up its temp file except the final rename, which used to leak a
// .config-*.yaml on failure. Force a rename failure by pointing Save at
// an existing (non-empty) directory and assert no temp file is left.
func TestSave_RemovesTempFileOnRenameFailure(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "config-as-dir")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "keep"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{Apps: []AppConfig{{Name: "a", URL: "http://a"}}}
	if err := cfg.Save(target); err == nil {
		t.Fatal("expected Save to fail renaming onto a directory")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".config-") {
			t.Errorf("temp file leaked after failed rename: %s", e.Name())
		}
	}
}
