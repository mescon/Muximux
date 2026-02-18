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
			name: "missing gateway file",
			yaml: `
server:
  listen: ":8080"
  gateway: /nonexistent/sites.Caddyfile
`,
			wantErr: "gateway file not found",
		},
		{
			name: "valid gateway file",
			yaml: `
server:
  listen: ":8080"
  gateway: "` + gatewayPath + `"
`,
			wantErr: "",
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

func TestMigrate(t *testing.T) {
	t.Run("v1 to v2 renames roles", func(t *testing.T) {
		cfg := &Config{
			ConfigVersion: 0,
			Auth: AuthConfig{
				Users: []UserConfig{
					{Username: "admin1", Role: "admin"},
					{Username: "user1", Role: "user"},
					{Username: "guest1", Role: "guest"},
				},
			},
			Apps: []AppConfig{
				{Name: "app1", MinRole: "user"},
				{Name: "app2", MinRole: "guest"},
				{Name: "app3", MinRole: "admin"},
				{Name: "app4", MinRole: ""},
			},
		}

		changed := cfg.Migrate()
		if !changed {
			t.Error("expected Migrate to return true")
		}
		if cfg.ConfigVersion != CurrentConfigVersion {
			t.Errorf("expected config_version=%d, got %d", CurrentConfigVersion, cfg.ConfigVersion)
		}

		// Check user roles
		roles := map[string]string{}
		for _, u := range cfg.Auth.Users {
			roles[u.Username] = u.Role
		}
		if roles["admin1"] != "admin" {
			t.Errorf("expected admin1 role 'admin', got %q", roles["admin1"])
		}
		if roles["user1"] != "power-user" {
			t.Errorf("expected user1 role 'power-user', got %q", roles["user1"])
		}
		if roles["guest1"] != "user" {
			t.Errorf("expected guest1 role 'user', got %q", roles["guest1"])
		}

		// Check app min_role
		appRoles := map[string]string{}
		for _, a := range cfg.Apps {
			appRoles[a.Name] = a.MinRole
		}
		if appRoles["app1"] != "power-user" {
			t.Errorf("expected app1 min_role 'power-user', got %q", appRoles["app1"])
		}
		if appRoles["app2"] != "user" {
			t.Errorf("expected app2 min_role 'user', got %q", appRoles["app2"])
		}
		if appRoles["app3"] != "admin" {
			t.Errorf("expected app3 min_role 'admin', got %q", appRoles["app3"])
		}
		if appRoles["app4"] != "" {
			t.Errorf("expected app4 min_role '', got %q", appRoles["app4"])
		}
	})

	t.Run("already current version is no-op", func(t *testing.T) {
		cfg := &Config{
			ConfigVersion: CurrentConfigVersion,
			Auth: AuthConfig{
				Users: []UserConfig{
					{Username: "u1", Role: "user"},
				},
			},
		}

		changed := cfg.Migrate()
		if changed {
			t.Error("expected Migrate to return false for current version")
		}
		if cfg.Auth.Users[0].Role != "user" {
			t.Errorf("role should not change, got %q", cfg.Auth.Users[0].Role)
		}
	})
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
