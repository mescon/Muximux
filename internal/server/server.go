package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/handlers"
	"github.com/mescon/muximux/v3/internal/health"
	"github.com/mescon/muximux/v3/internal/icons"
	"github.com/mescon/muximux/v3/internal/logging"
	"github.com/mescon/muximux/v3/internal/proxy"
	"github.com/mescon/muximux/v3/internal/websocket"
)

// validThemeName only allows safe CSS theme filenames (allowlist approach)
var validThemeName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*\.css$`)

// Server holds the HTTP server and related components
type Server struct {
	config         *config.Config
	configMu       sync.RWMutex // protects config reads/writes
	configPath     string
	dataDir        string
	httpServer     *http.Server
	healthMonitor  *health.Monitor
	wsHub          *websocket.Hub
	sessionStore   *auth.SessionStore
	userStore      *auth.UserStore
	authMiddleware *auth.Middleware
	proxyServer    *proxy.Proxy
	oidcProvider   *auth.OIDCProvider
	needsSetup     atomic.Bool
	setupMu        sync.Mutex // serializes setup requests
	loginLimiter   *rateLimiter
	setupLimiter   *rateLimiter
	logCh          chan logging.LogEntry
	version        string
	commit         string
	buildDate      string
}

// adminGuard is a function that wraps a handler to require admin role.
type adminGuard func(next http.HandlerFunc) http.HandlerFunc

// New creates a new server instance.
// dataDir is the base directory for all mutable data (config, themes, icons).
func New(cfg *config.Config, configPath string, dataDir string, version, commit, buildDate string) (*Server, error) {
	// Create WebSocket hub
	wsHub := websocket.NewHub()

	// Set up authentication
	sessionStore, userStore, authMiddleware := setupAuth(cfg)

	s := &Server{
		config:         cfg,
		configPath:     configPath,
		dataDir:        dataDir,
		wsHub:          wsHub,
		sessionStore:   sessionStore,
		userStore:      userStore,
		authMiddleware: authMiddleware,
		version:        version,
		commit:         commit,
		buildDate:      buildDate,
	}
	s.needsSetup.Store(cfg.NeedsSetup())

	// Set up routes
	mux := http.NewServeMux()

	// Auth endpoints (always accessible)
	authHandler := handlers.NewAuthHandler(sessionStore, userStore, cfg, configPath, authMiddleware, &s.configMu)
	authHandler.SetBypassRules(defaultBypassRules)
	authHandler.SetSetupChecker(func() bool { return s.needsSetup.Load() })

	// Set up OIDC provider if configured
	if cfg.Auth.OIDC.Enabled {
		oidcProvider := setupOIDC(cfg, sessionStore, userStore)
		authHandler.SetOIDCProvider(oidcProvider)
		s.oidcProvider = oidcProvider
	}

	// requireAdmin checks that the authenticated user has admin role.
	// Used to protect state-changing API endpoints.
	requireAdmin := adminGuard(func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			user := auth.GetUserFromContext(r.Context())
			if user == nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			if user.Role != auth.RoleAdmin {
				http.Error(w, "Forbidden: admin role required", http.StatusForbidden)
				return
			}
			next(w, r)
		}
	})

	s.loginLimiter = registerAuthRoutes(mux, authHandler, wsHub)

	s.setupLimiter = newRateLimiter(5, 1*time.Minute)
	mux.HandleFunc("/api/auth/setup", s.setupLimiter.wrap(s.handleSetup))
	mux.HandleFunc("/api/config/restore", s.setupLimiter.wrap(s.handleConfigRestore))

	// API routes
	api := handlers.NewAPIHandler(cfg, configPath, &s.configMu)
	registerAPIRoutes(mux, api, requireAdmin)

	// Health monitoring
	s.setupHealthRoutes(mux, cfg, wsHub)

	// System info endpoints (no auth required — non-sensitive)
	systemHandler := handlers.NewSystemHandler(version, commit, buildDate, dataDir)
	mux.HandleFunc("/api/system/info", systemHandler.GetInfo)
	mux.HandleFunc("/api/system/updates", systemHandler.CheckUpdate)

	// Logs endpoint
	logsHandler := handlers.NewLogsHandler()
	mux.HandleFunc("/api/logs/recent", logsHandler.GetRecent)

	// Forward-declare so /themes/ handler closure can reference it
	var staticHandler http.Handler

	// Extract embedded dist filesystem (used for bundled themes + static serving)
	var distFS fs.FS
	var distErr error
	if hasEmbeddedAssets {
		distFS, distErr = fs.Sub(embeddedFiles, "dist")
	} else {
		distErr = fmt.Errorf("no embedded assets (dev mode)")
	}

	themesDir := filepath.Join(dataDir, "themes")
	registerThemeRoutes(mux, distFS, requireAdmin, &staticHandler, themesDir)
	registerIconRoutes(mux, cfg, requireAdmin, dataDir)

	// Integrated reverse proxy on main server (handles /proxy/{slug}/*)
	// Always registered so routes added at runtime (via Settings) work without restart.
	reverseProxyHandler := handlers.NewReverseProxyHandler(cfg.Apps, cfg.Server.ProxyTimeout)
	mux.Handle(proxyPathPrefix, reverseProxyHandler)
	if reverseProxyHandler.HasRoutes() {
		logging.Info("Integrated reverse proxy enabled", "source", "server", "routes", reverseProxyHandler.GetRoutes())
	}

	// Rebuild proxy routes whenever config is saved
	api.SetOnConfigSave(func() {
		reverseProxyHandler.RebuildRoutes(cfg.Apps)
	})

	// Caddy setup
	goListenAddr := setupCaddy(s, cfg)

	// Proxy API routes (always registered so the endpoint responds gracefully)
	proxyHandler := handlers.NewProxyHandler(s.proxyServer, &cfg.Server)
	mux.HandleFunc("/api/proxy/status", proxyHandler.GetStatus)

	// Auth-protected endpoints
	mux.HandleFunc("/api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		authMiddleware.RequireAuth(http.HandlerFunc(authHandler.Me)).ServeHTTP(w, r)
	})
	mux.HandleFunc("/api/auth/password", func(w http.ResponseWriter, r *http.Request) {
		authMiddleware.RequireAuth(http.HandlerFunc(authHandler.ChangePassword)).ServeHTTP(w, r)
	})

	// User management (auth-protected, admin-only)
	mux.HandleFunc("/api/auth/users", func(w http.ResponseWriter, r *http.Request) {
		authMiddleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				requireAdmin(authHandler.ListUsers)(w, r)
			case http.MethodPost:
				requireAdmin(authHandler.CreateUser)(w, r)
			default:
				http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
			}
		})).ServeHTTP(w, r)
	})

	// /api/auth/users/{username} — PUT, DELETE
	mux.HandleFunc("/api/auth/users/", func(w http.ResponseWriter, r *http.Request) {
		authMiddleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPut:
				requireAdmin(authHandler.UpdateUser)(w, r)
			case http.MethodDelete:
				requireAdmin(authHandler.DeleteUser)(w, r)
			default:
				http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
			}
		})).ServeHTTP(w, r)
	})

	// Auth method switching
	mux.HandleFunc("/api/auth/method", func(w http.ResponseWriter, r *http.Request) {
		authMiddleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
				return
			}
			requireAdmin(authHandler.UpdateAuthMethod)(w, r)
		})).ServeHTTP(w, r)
	})

	// Serve embedded frontend files
	if distErr != nil {
		// Fallback to serving from web/dist during development
		fileServer := http.FileServer(http.Dir("web/dist"))
		staticHandler = spaHandlerDev(fileServer, "web/dist", "index.html")
	} else {
		staticHandler = spaHandlerEmbed(http.FileServer(http.FS(distFS)), distFS, cfg.Server.NormalizedBasePath())
	}

	// Serve static files with SPA fallback
	mux.Handle("/", staticHandler)

	// Create the final handler with security middleware
	handler := s.setupGuardMiddleware(wrapMiddleware(mux, cfg, authMiddleware))

	// Wrap with base path stripping if configured
	basePath := cfg.Server.NormalizedBasePath()
	if basePath != "" {
		inner := handler
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Redirect requests to base path without trailing slash
			if r.URL.Path == basePath {
				http.Redirect(w, r, basePath+"/", http.StatusMovedPermanently)
				return
			}
			if !strings.HasPrefix(r.URL.Path, basePath+"/") {
				http.NotFound(w, r)
				return
			}
			http.StripPrefix(basePath, inner).ServeHTTP(w, r)
		})
	}

	s.httpServer = &http.Server{
		Addr:         goListenAddr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s, nil
}

// defaultBypassRules defines paths that bypass authentication.
var defaultBypassRules = []auth.BypassRule{
	{Path: "/api/auth/login"},
	{Path: "/api/auth/logout"},
	{Path: "/api/auth/status"},
	{Path: "/api/auth/setup"},
	{Path: "/api/auth/oidc/*"},
	{Path: "/assets/*"},
	{Path: "/themes/*"},
	{Path: "/manifest.json"},
	{Path: "/favicon.ico"},
	{Path: "/favicon-16x16.png"},
	{Path: "/favicon-32x32.png"},
	{Path: "/apple-touch-icon.png"},
	{Path: "/android-chrome-192x192.png"},
	{Path: "/android-chrome-256x256.png"},
	{Path: "/safari-pinned-tab.svg"},
	{Path: "/browserconfig.xml"},
	{Path: "/mstile-150x150.png"},
	{Path: "/login"},
	{Path: "/api/health"},
}

// setupAuth creates the session store, user store, and auth middleware from config.
func setupAuth(cfg *config.Config) (*auth.SessionStore, *auth.UserStore, *auth.Middleware) {
	sessionMaxAge := parseDuration(cfg.Auth.SessionMaxAge, 24*time.Hour)
	sessionStore := auth.NewSessionStore("muximux_session", sessionMaxAge, cfg.Auth.SecureCookies)
	userStore := auth.NewUserStore()

	// Load users from config
	userConfigs := make([]auth.UserConfig, 0, len(cfg.Auth.Users))
	for _, u := range cfg.Auth.Users {
		userConfigs = append(userConfigs, auth.UserConfig{
			Username:     u.Username,
			PasswordHash: u.PasswordHash,
			Role:         u.Role,
			Email:        u.Email,
			DisplayName:  u.DisplayName,
		})
	}
	userStore.LoadFromConfig(userConfigs)

	// Create auth middleware with default bypass rules
	authConfig := auth.AuthConfig{
		Method:         auth.AuthMethod(cfg.Auth.Method),
		TrustedProxies: cfg.Auth.TrustedProxies,
		APIKey:         cfg.Auth.APIKey,
		BasePath:       cfg.Server.NormalizedBasePath(),
		Headers:        auth.ForwardAuthHeadersFromMap(cfg.Auth.Headers),
		BypassRules:    defaultBypassRules,
	}
	authMiddleware := auth.NewMiddleware(&authConfig, sessionStore, userStore)

	return sessionStore, userStore, authMiddleware
}

// setupOIDC configures and returns the OIDC provider.
func setupOIDC(cfg *config.Config, sessionStore *auth.SessionStore, userStore *auth.UserStore) *auth.OIDCProvider {
	oidcConfig := auth.OIDCConfig{
		Enabled:          cfg.Auth.OIDC.Enabled,
		IssuerURL:        cfg.Auth.OIDC.IssuerURL,
		ClientID:         cfg.Auth.OIDC.ClientID,
		ClientSecret:     cfg.Auth.OIDC.ClientSecret,
		RedirectURL:      cfg.Auth.OIDC.RedirectURL,
		Scopes:           cfg.Auth.OIDC.Scopes,
		UsernameClaim:    cfg.Auth.OIDC.UsernameClaim,
		EmailClaim:       cfg.Auth.OIDC.EmailClaim,
		GroupsClaim:      cfg.Auth.OIDC.GroupsClaim,
		DisplayNameClaim: cfg.Auth.OIDC.DisplayNameClaim,
		AdminGroups:      cfg.Auth.OIDC.AdminGroups,
		BasePath:         cfg.Server.NormalizedBasePath(),
	}
	return auth.NewOIDCProvider(&oidcConfig, sessionStore, userStore)
}

// registerAuthRoutes registers authentication and WebSocket endpoints.
func registerAuthRoutes(mux *http.ServeMux, authHandler *handlers.AuthHandler, wsHub *websocket.Hub) *rateLimiter {
	loginLimiter := newRateLimiter(5, 1*time.Minute)
	mux.HandleFunc("/api/auth/login", loginLimiter.wrap(authHandler.Login))
	mux.HandleFunc("/api/auth/logout", authHandler.Logout)
	mux.HandleFunc("/api/auth/status", authHandler.AuthStatus)
	mux.HandleFunc("/api/auth/oidc/login", authHandler.OIDCLogin)
	mux.HandleFunc("/api/auth/oidc/callback", authHandler.OIDCCallback)

	// WebSocket endpoint
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		websocket.ServeWs(wsHub, w, r)
	})

	return loginLimiter
}

// registerAPIRoutes registers config, apps, and groups CRUD endpoints.
func registerAPIRoutes(mux *http.ServeMux, api *handlers.APIHandler, requireAdmin adminGuard) {
	// Config endpoint - handle both GET and PUT
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			api.GetConfig(w, r)
		case http.MethodPut:
			requireAdmin(api.SaveConfig)(w, r)
		default:
			http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/config/export", requireAdmin(api.ExportConfig))
	mux.HandleFunc("/api/config/import", requireAdmin(api.ParseImportedConfig))

	// Apps collection endpoint
	mux.HandleFunc("/api/apps", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			api.GetApps(w, r)
		case http.MethodPost:
			requireAdmin(api.CreateApp)(w, r)
		default:
			http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		}
	})

	// Groups collection endpoint
	mux.HandleFunc("/api/groups", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			api.GetGroups(w, r)
		case http.MethodPost:
			requireAdmin(api.CreateGroup)(w, r)
		default:
			http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		}
	})

	// Individual app endpoint
	mux.HandleFunc("/api/app/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/api/app/")
		if name == "" {
			http.Error(w, "App name required", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodGet:
			api.GetApp(w, r, name)
		case http.MethodPut:
			requireAdmin(func(w http.ResponseWriter, r *http.Request) {
				api.UpdateApp(w, r, name)
			})(w, r)
		case http.MethodDelete:
			requireAdmin(func(w http.ResponseWriter, r *http.Request) {
				api.DeleteApp(w, r, name)
			})(w, r)
		default:
			http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		}
	})

	// Individual group endpoint
	mux.HandleFunc("/api/group/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/api/group/")
		if name == "" {
			http.Error(w, "Group name required", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodGet:
			api.GetGroup(w, r, name)
		case http.MethodPut:
			requireAdmin(func(w http.ResponseWriter, r *http.Request) {
				api.UpdateGroup(w, r, name)
			})(w, r)
		case http.MethodDelete:
			requireAdmin(func(w http.ResponseWriter, r *http.Request) {
				api.DeleteGroup(w, r, name)
			})(w, r)
		default:
			http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/health", api.Health)
}

// setupHealthRoutes configures health monitoring and registers health check routes.
func (s *Server) setupHealthRoutes(mux *http.ServeMux, cfg *config.Config, wsHub *websocket.Hub) {
	if !cfg.Health.Enabled {
		return
	}

	healthInterval := parseDuration(cfg.Health.Interval, 30*time.Second)
	healthTimeout := parseDuration(cfg.Health.Timeout, 5*time.Second)
	s.healthMonitor = health.NewMonitor(healthInterval, healthTimeout)

	// Configure apps for health monitoring.
	// Health checks are opt-in: only apps with health_check: true are monitored.
	healthApps := make([]health.AppConfig, 0, len(cfg.Apps))
	for i := range cfg.Apps {
		hcEnabled := cfg.Apps[i].HealthCheck != nil && *cfg.Apps[i].HealthCheck
		healthApps = append(healthApps, health.AppConfig{
			Name:      cfg.Apps[i].Name,
			URL:       cfg.Apps[i].URL,
			HealthURL: cfg.Apps[i].HealthURL,
			Enabled:   cfg.Apps[i].Enabled && hcEnabled,
		})
	}
	s.healthMonitor.SetApps(healthApps)

	// Broadcast health changes via WebSocket
	s.healthMonitor.SetHealthChangeCallback(func(appName string, appHealth *health.AppHealth) {
		wsHub.BroadcastAppHealthUpdate(appName, appHealth)
	})

	// Health check routes
	healthHandler := handlers.NewHealthHandler(s.healthMonitor)
	mux.HandleFunc("/api/apps/health", healthHandler.GetAllHealth)
	mux.HandleFunc("/api/apps/", func(w http.ResponseWriter, r *http.Request) {
		// Route /api/apps/{name}/health and /api/apps/{name}/health/check
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/health/check"):
			healthHandler.CheckAppHealth(w, r)
		case strings.HasSuffix(path, "/health"):
			healthHandler.GetAppHealth(w, r)
		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	})
}

// registerThemeRoutes registers theme API and CSS serving routes.
// staticHandler is a pointer so the /themes/ closure can reference the handler
// that gets assigned later (forward declaration pattern).
func registerThemeRoutes(mux *http.ServeMux, distFS fs.FS, requireAdmin adminGuard, staticHandler *http.Handler, themesDir string) {
	themeHandler := handlers.NewThemeHandler(themesDir, distFS)
	mux.HandleFunc(apiThemesPath, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			themeHandler.ListThemes(w, r)
		case http.MethodPost:
			requireAdmin(themeHandler.SaveTheme)(w, r)
		default:
			http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/themes/", requireAdmin(themeHandler.DeleteTheme))
	// Serve theme CSS files: try themesDir first (user-created), fall back to static assets (bundled)
	mux.HandleFunc("/themes/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/themes/")
		// Only allow safe CSS theme filenames (allowlist approach)
		if !validThemeName.MatchString(name) {
			http.NotFound(w, r)
			return
		}
		localPath := filepath.Join(themesDir, name)
		// Double-check resolved path is within the themes directory
		absPath, err := filepath.Abs(localPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		absThemesDir, _ := filepath.Abs(themesDir)
		if !strings.HasPrefix(absPath, absThemesDir+string(filepath.Separator)) {
			http.NotFound(w, r)
			return
		}
		f, openErr := os.Open(localPath)
		if openErr == nil {
			defer f.Close()
			stat, _ := f.Stat()
			http.ServeContent(w, r, name, stat.ModTime(), f)
			return
		}
		// Fall through to static handler (web/dist/themes/ or embedded)
		(*staticHandler).ServeHTTP(w, r)
	})
}

// registerIconRoutes registers icon API and serving routes.
func registerIconRoutes(mux *http.ServeMux, cfg *config.Config, requireAdmin adminGuard, dataDir string) {
	cacheTTL := parseDuration(cfg.Icons.DashboardIcons.CacheTTL, 7*24*time.Hour)
	// Resolve CacheDir relative to dataDir unless it's an absolute path
	cacheDir := cfg.Icons.DashboardIcons.CacheDir
	if !filepath.IsAbs(cacheDir) {
		cacheDir = filepath.Join(dataDir, cacheDir)
	}
	dashboardClient := icons.NewDashboardIconsClient(cacheDir, cacheTTL)
	lucideClient := icons.NewLucideClient(filepath.Join(dataDir, "icons", "lucide"), cacheTTL)
	iconHandler := handlers.NewIconHandler(dashboardClient, lucideClient, filepath.Join(dataDir, "icons", "custom"))

	mux.HandleFunc("/api/icons/dashboard", iconHandler.ListDashboardIcons)
	mux.HandleFunc("/api/icons/dashboard/", iconHandler.GetDashboardIcon)
	mux.HandleFunc("/api/icons/lucide", iconHandler.ListLucideIcons)
	mux.HandleFunc("/api/icons/lucide/", iconHandler.GetLucideIcon)
	mux.HandleFunc("/api/icons/custom", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			iconHandler.ListCustomIcons(w, r)
		case http.MethodPost:
			requireAdmin(iconHandler.UploadCustomIcon)(w, r)
		default:
			http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/icons/custom/", requireAdmin(iconHandler.DeleteCustomIcon))
	mux.HandleFunc("/icons/", iconHandler.ServeIcon)
}

// setupCaddy configures the Caddy reverse proxy when TLS or Gateway is active.
// It sets s.proxyServer and returns the Go server's listen address.
func setupCaddy(s *Server, cfg *config.Config) string {
	goListenAddr := cfg.Server.Listen
	if !cfg.Server.NeedsCaddy() {
		return goListenAddr
	}

	internalAddr := proxy.ComputeInternalAddr(cfg.Server.Listen)
	proxyConfig := proxy.Config{
		ListenAddr:   cfg.Server.Listen,
		InternalAddr: internalAddr,
		Domain:       cfg.Server.TLS.Domain,
		Email:        cfg.Server.TLS.Email,
		TLSCert:      cfg.Server.TLS.Cert,
		TLSKey:       cfg.Server.TLS.Key,
		Gateway:      cfg.Server.Gateway,
	}
	s.proxyServer = proxy.New(&proxyConfig)

	// Configure proxy routes for apps with proxy enabled
	var proxyRoutes []proxy.AppRoute
	for i := range cfg.Apps {
		if cfg.Apps[i].Proxy && cfg.Apps[i].Enabled {
			proxyRoutes = append(proxyRoutes, proxy.AppRoute{
				Name:      cfg.Apps[i].Name,
				Slug:      handlers.Slugify(cfg.Apps[i].Name),
				TargetURL: cfg.Apps[i].URL,
				Enabled:   true,
			})
		}
	}
	s.proxyServer.SetRoutes(proxyRoutes)

	return internalAddr
}

// wrapMiddleware applies auth and security middleware around the mux.
func wrapMiddleware(mux *http.ServeMux, _ *config.Config, authMiddleware *auth.Middleware) http.Handler {
	// Always apply RequireAuth — it injects a virtual admin when auth is
	// "none", which downstream adminGuard checks rely on.
	handler := authMiddleware.RequireAuth(mux)
	// Wrap with security middleware (outermost = runs first)
	handler = securityHeadersMiddleware(handler)
	handler = csrfMiddleware(handler)
	handler = bodySizeLimitMiddleware(handler)
	return handler
}

// setupGuardMiddleware blocks non-auth API endpoints while setup is pending.
func (s *Server) setupGuardMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.needsSetup.Load() || s.isSetupAllowed(r) {
			next.ServeHTTP(w, r)
			return
		}

		logging.Debug("API blocked: setup not complete", "source", "server", "path", r.URL.Path)
		setJSONContentType(w)
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "setup_required"})
	})
}

// isSetupAllowed returns true if the request should be allowed through during setup.
func (s *Server) isSetupAllowed(r *http.Request) bool {
	path := r.URL.Path

	// Non-API paths (static assets, SPA) are always allowed
	if !strings.HasPrefix(path, "/api/") {
		return true
	}

	// Auth, health, and config restore endpoints are always allowed
	if strings.HasPrefix(path, "/api/auth/") || path == "/api/health" || path == "/api/config/restore" {
		return true
	}

	// Read-only endpoints needed for the onboarding wizard
	if r.Method == http.MethodGet {
		return path == apiThemesPath ||
			strings.HasPrefix(path, "/api/icons/") ||
			strings.HasPrefix(path, "/api/system/") ||
			strings.HasPrefix(path, "/api/logs/")
	}

	return false
}

// setupRequest is the JSON body for POST /api/auth/setup
type setupRequest struct {
	Method         string            `json:"method"`
	Username       string            `json:"username"`
	Password       string            `json:"password"`
	TrustedProxies []string          `json:"trusted_proxies"`
	Headers        map[string]string `json:"headers"`
	LogoutURL      string            `json:"logout_url"`
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	// Serialize setup requests to prevent TOCTOU races
	s.setupMu.Lock()
	defer s.setupMu.Unlock()

	if !s.needsSetup.Load() {
		setJSONContentType(w)
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "Setup already completed"})
		return
	}

	var req setupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		setJSONContentType(w)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": errInvalidBody})
		return
	}

	switch req.Method {
	case "builtin":
		if err := s.setupBuiltin(w, &req); err != nil {
			return // error already written
		}
	case "forward_auth":
		if err := s.setupForwardAuth(&req); err != nil {
			setJSONContentType(w)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
	case "none":
		s.setupNone()
	default:
		setJSONContentType(w)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid method. Must be builtin, forward_auth, or none"})
		return
	}

	if err := func() error {
		s.configMu.Lock()
		defer s.configMu.Unlock()
		s.config.Auth.SetupComplete = true
		return s.config.Save(s.configPath)
	}(); err != nil {
		logging.Error("Failed to save config after setup", "source", "server", "error", err)
		setJSONContentType(w)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save configuration"})
		return
	}

	logging.Info("Setup completed", "source", "config", "method", req.Method)

	s.needsSetup.Store(false)

	setJSONContentType(w)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"method":  req.Method,
	})
}

// handleConfigRestore accepts a full YAML config file and saves it directly,
// bypassing the normal setup wizard. Only allowed while setup is pending.
func (s *Server) handleConfigRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	s.setupMu.Lock()
	defer s.setupMu.Unlock()

	if !s.needsSetup.Load() {
		setJSONContentType(w)
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "Setup already completed"})
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB limit
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var cfg config.Config
	if err := yaml.Unmarshal(body, &cfg); err != nil {
		setJSONContentType(w)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Invalid YAML: %s", err.Error())})
		return
	}

	// Ensure the restored config is marked as setup complete
	cfg.Auth.SetupComplete = true
	if cfg.ConfigVersion == 0 {
		cfg.ConfigVersion = config.CurrentConfigVersion
	}

	s.configMu.Lock()
	*s.config = cfg
	err = s.config.Save(s.configPath)
	s.configMu.Unlock()

	if err != nil {
		logging.Error("Failed to save restored config", "source", "config", "error", err)
		setJSONContentType(w)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save configuration"})
		return
	}

	logging.Info("Config restored from backup", "source", "config", "apps", len(cfg.Apps), "groups", len(cfg.Groups))
	s.needsSetup.Store(false)

	setJSONContentType(w)
	json.NewEncoder(w).Encode(map[string]string{"success": "true"})
}

func (s *Server) setupBuiltin(w http.ResponseWriter, req *setupRequest) error {
	if strings.TrimSpace(req.Username) == "" {
		setJSONContentType(w)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Username is required"})
		return fmt.Errorf("username required")
	}
	if len(req.Password) < 8 {
		setJSONContentType(w)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Password must be at least 8 characters"})
		return fmt.Errorf("password too short")
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		setJSONContentType(w)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to hash password"})
		return err
	}

	s.configMu.Lock()
	s.config.Auth.Method = "builtin"
	s.config.Auth.Users = []config.UserConfig{
		{
			Username:     req.Username,
			PasswordHash: hash,
			Role:         "admin",
		},
	}
	s.configMu.Unlock()

	// Update live user store
	s.userStore.LoadFromConfig([]auth.UserConfig{
		{
			Username:     req.Username,
			PasswordHash: hash,
			Role:         "admin",
		},
	})

	// Update auth middleware
	s.authMiddleware.UpdateConfig(&auth.AuthConfig{
		Method:      auth.AuthMethodBuiltin,
		BypassRules: defaultBypassRules,
		APIKey:      s.config.Auth.APIKey,
		BasePath:    s.config.Server.NormalizedBasePath(),
	})

	logging.Info("Admin user created", "source", "auth", "user", req.Username)

	// Create session so user is immediately logged in
	session, err := s.sessionStore.Create(req.Username, req.Username, "admin")
	if err != nil {
		logging.Warn("Failed to create session after setup", "source", "server", "error", err)
		// Non-fatal — setup still succeeds, user just needs to log in manually
		return nil
	}
	s.sessionStore.SetCookie(w, session)

	return nil
}

func (s *Server) setupForwardAuth(req *setupRequest) error {
	if len(req.TrustedProxies) == 0 {
		return fmt.Errorf("at least one trusted proxy is required for forward_auth")
	}

	s.configMu.Lock()
	s.config.Auth.Method = "forward_auth"
	s.config.Auth.TrustedProxies = req.TrustedProxies
	s.config.Auth.LogoutURL = req.LogoutURL
	if req.Headers != nil {
		s.config.Auth.Headers = req.Headers
	}
	apiKey := s.config.Auth.APIKey
	s.configMu.Unlock()

	s.authMiddleware.UpdateConfig(&auth.AuthConfig{
		Method:         auth.AuthMethodForwardAuth,
		TrustedProxies: req.TrustedProxies,
		Headers:        auth.ForwardAuthHeadersFromMap(req.Headers),
		BypassRules:    defaultBypassRules,
		APIKey:         apiKey,
		BasePath:       s.config.Server.NormalizedBasePath(),
	})

	logging.Info("Forward auth configured", "source", "auth", "proxies", strings.Join(req.TrustedProxies, ","))

	return nil
}

func (s *Server) setupNone() {
	s.configMu.Lock()
	s.config.Auth.Method = "none"
	s.configMu.Unlock()
	logging.Info("Auth disabled (method: none)", "source", "auth")
	// Middleware stays as-is (virtual admin)
}

// Start begins serving HTTP requests
func (s *Server) Start() error {
	// Start WebSocket hub
	go s.wsHub.Run()

	// Bridge log entries to WebSocket
	if buf := logging.Buffer(); buf != nil {
		s.logCh = buf.Subscribe()
		go func() {
			for entry := range s.logCh {
				s.wsHub.BroadcastLogEntry(entry)
			}
		}()
	}

	// Start health monitoring if enabled
	if s.healthMonitor != nil {
		s.healthMonitor.Start()
	}

	// Start Caddy if configured (must start before Go server claims its port)
	if s.proxyServer != nil {
		if err := s.proxyServer.Start(); err != nil {
			return fmt.Errorf("failed to start caddy: %w", err)
		}
		logging.Info("Muximux started", "source", "server", "version", s.version, "listen", s.config.Server.Listen, "internal_addr", s.proxyServer.GetInternalAddr(), "caddy", true)
	} else {
		logging.Info("Muximux started", "source", "server", "version", s.version, "listen", s.config.Server.Listen)
	}

	return s.httpServer.ListenAndServe()
}

// GetHub returns the WebSocket hub for broadcasting events
func (s *Server) GetHub() *websocket.Hub {
	return s.wsHub
}

// Stop gracefully shuts down the server
func (s *Server) Stop() error {
	logging.Info("Server shutting down", "source", "server")
	// Stop health monitoring
	if s.healthMonitor != nil {
		s.healthMonitor.Stop()
	}

	// Stop Caddy
	if s.proxyServer != nil {
		if err := s.proxyServer.Stop(); err != nil {
			logging.Warn("Failed to stop Caddy", "source", "server", "error", err)
		}
	}

	// Stop rate limiter cleanup goroutines
	if s.loginLimiter != nil {
		s.loginLimiter.stop()
	}
	if s.setupLimiter != nil {
		s.setupLimiter.stop()
	}

	// Unsubscribe log channel to stop the bridge goroutine
	if s.logCh != nil {
		if buf := logging.Buffer(); buf != nil {
			buf.Unsubscribe(s.logCh)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

// isSPARoute returns true if the path should be served by the SPA's index.html.
// This covers root and paths without file extensions, excluding backend paths.
func isSPARoute(path string) bool {
	if path == "/" {
		return true
	}
	return !strings.Contains(path, ".") &&
		!strings.HasPrefix(path, "/api/") &&
		!strings.HasPrefix(path, "/ws") &&
		!strings.HasPrefix(path, proxyPathPrefix) &&
		!strings.HasPrefix(path, "/icons/")
}

// spaHandlerDev wraps a file server for development (filesystem-based)
func spaHandlerDev(fileServer http.Handler, distDir string, indexPath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSPARoute(r.URL.Path) {
			http.ServeFile(w, r, filepath.Join(distDir, indexPath))
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

// spaHandlerEmbed wraps a file server for embedded files.
// basePath is injected into index.html as window.__MUXIMUX_BASE__ for frontend API calls.
func spaHandlerEmbed(fileServer http.Handler, fsys fs.FS, basePath string) http.Handler {
	indexContent, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		return fileServer
	}

	if basePath != "" {
		injection := []byte(fmt.Sprintf(`<script>window.__MUXIMUX_BASE__=%q;</script></head>`, basePath))
		indexContent = bytes.Replace(indexContent, []byte("</head>"), injection, 1)
	}

	// Pre-compute headers at startup for zero-allocation index serving
	indexLen := strconv.Itoa(len(indexContent))
	indexETag := fmt.Sprintf(`"%x"`, sha256.Sum256(indexContent))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSPARoute(r.URL.Path) {
			h := w.Header()
			h.Set(headerContentType, "text/html; charset=utf-8")
			h.Set("Content-Length", indexLen)
			h.Set("ETag", indexETag)
			h.Set("Cache-Control", "no-cache")
			if r.Header.Get("If-None-Match") == indexETag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Write(indexContent)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

// parseDuration parses a duration string like "7d", "24h", "30m"
func parseDuration(s string, defaultVal time.Duration) time.Duration {
	if s == "" {
		return defaultVal
	}

	// Handle day suffix
	if strings.HasSuffix(s, "d") {
		s = strings.TrimSuffix(s, "d")
		var days int
		if _, err := fmt.Sscanf(s, "%d", &days); err == nil {
			return time.Duration(days) * 24 * time.Hour
		}
		return defaultVal
	}

	// Try standard Go duration parsing
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}

	return defaultVal
}

// securityHeadersMiddleware adds standard security headers to all responses
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

// csrfMiddleware protects state-changing API requests from cross-origin attacks.
// POST requests are the main CSRF vector (browsers can send cross-origin POSTs via forms).
// We require JSON Content-Type on POST/PUT, which triggers CORS preflight from cross-origin.
// DELETE/PATCH are not "simple" HTTP methods, so browsers always preflight them.
func csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && (r.Method == http.MethodPost || r.Method == http.MethodPut) {
			ct := r.Header.Get(headerContentType)
			if !strings.HasPrefix(ct, contentTypeJSON) && !strings.HasPrefix(ct, "multipart/form-data") && !strings.HasPrefix(ct, "application/x-yaml") {
				logging.Warn("CSRF check failed: invalid content-type", "source", "server", "path", r.URL.Path, "method", r.Method, "content_type", ct)
				http.Error(w, "Forbidden: JSON Content-Type required", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// bodySizeLimitMiddleware limits request body size for API endpoints
func bodySizeLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Body != nil {
			// 1MB limit for config endpoints, 64KB for others
			var maxBytes int64 = 64 * 1024
			if r.URL.Path == "/api/config" || strings.HasPrefix(r.URL.Path, apiThemesPath) {
				maxBytes = 1 * 1024 * 1024
			} else if strings.HasPrefix(r.URL.Path, "/api/icons/custom") {
				maxBytes = 5 * 1024 * 1024 // 5MB for icon uploads
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// rateLimiter implements a simple per-IP rate limiter
type rateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	max      int
	window   time.Duration
	done     chan struct{}
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		attempts: make(map[string][]time.Time),
		max:      max,
		window:   window,
		done:     make(chan struct{}),
	}
	go rl.cleanup()
	return rl
}

// stop signals the cleanup goroutine to exit.
func (rl *rateLimiter) stop() {
	close(rl.done)
}

// cleanup periodically removes stale rate-limit entries.
func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.purgeStaleEntries()
		case <-rl.done:
			return
		}
	}
}

// purgeStaleEntries removes entries older than the rate-limit window.
func (rl *rateLimiter) purgeStaleEntries() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for ip, times := range rl.attempts {
		valid := times[:0]
		for _, t := range times {
			if now.Sub(t) < rl.window {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(rl.attempts, ip)
		} else {
			rl.attempts[ip] = valid
		}
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Filter to only recent attempts
	times := rl.attempts[ip]
	valid := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.max {
		rl.attempts[ip] = valid
		return false
	}

	rl.attempts[ip] = append(valid, now)
	return true
}

func (rl *rateLimiter) wrap(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only rate-limit POST requests (actual login attempts)
		if r.Method != http.MethodPost {
			handler(w, r)
			return
		}

		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if ip == "" {
			ip = r.RemoteAddr
		}
		if !rl.allow(ip) {
			logging.Warn("Rate limit exceeded", "source", "server", "ip", ip, "path", r.URL.Path)
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(rl.window.Seconds())))
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		handler(w, r)
	}
}
