package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/handlers"
	"github.com/mescon/muximux/v3/internal/health"
	"github.com/mescon/muximux/v3/internal/icons"
	"github.com/mescon/muximux/v3/internal/proxy"
	"github.com/mescon/muximux/v3/internal/websocket"
)

//go:embed all:dist
var embeddedFiles embed.FS

// validThemeName only allows safe CSS theme filenames (allowlist approach)
var validThemeName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*\.css$`)

// Server holds the HTTP server and related components
type Server struct {
	config         *config.Config
	configPath     string
	httpServer     *http.Server
	healthMonitor  *health.Monitor
	wsHub          *websocket.Hub
	sessionStore   *auth.SessionStore
	userStore      *auth.UserStore
	authMiddleware *auth.Middleware
	proxyServer    *proxy.Proxy
	oidcProvider   *auth.OIDCProvider
	needsSetup     atomic.Bool
}

// adminGuard is a function that wraps a handler to require admin role.
type adminGuard func(next http.HandlerFunc) http.HandlerFunc

// New creates a new server instance
func New(cfg *config.Config, configPath string) (*Server, error) {
	// Create WebSocket hub
	wsHub := websocket.NewHub()

	// Set up authentication
	sessionStore, userStore, authMiddleware := setupAuth(cfg)

	s := &Server{
		config:         cfg,
		configPath:     configPath,
		wsHub:          wsHub,
		sessionStore:   sessionStore,
		userStore:      userStore,
		authMiddleware: authMiddleware,
	}
	s.needsSetup.Store(cfg.NeedsSetup())

	// Set up routes
	mux := http.NewServeMux()

	// Auth endpoints (always accessible)
	authHandler := handlers.NewAuthHandler(sessionStore, userStore)
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

	registerAuthRoutes(mux, authHandler, wsHub)

	setupLimiter := newRateLimiter(5, 1*time.Minute)
	mux.HandleFunc("/api/auth/setup", setupLimiter.wrap(s.handleSetup))

	// API routes
	api := handlers.NewAPIHandler(cfg, configPath)
	registerAPIRoutes(mux, api, requireAdmin)

	// Health monitoring
	s.setupHealthRoutes(mux, cfg, wsHub)

	// Forward-declare so /themes/ handler closure can reference it
	var staticHandler http.Handler

	// Extract embedded dist filesystem (used for bundled themes + static serving)
	distFS, distErr := fs.Sub(embeddedFiles, "dist")

	registerThemeRoutes(mux, distFS, requireAdmin, &staticHandler)
	registerIconRoutes(mux, cfg, requireAdmin)

	// Integrated reverse proxy on main server (handles /proxy/{slug}/*)
	reverseProxyHandler := handlers.NewReverseProxyHandler(cfg.Apps)
	if reverseProxyHandler.HasRoutes() {
		mux.Handle(proxyPathPrefix, reverseProxyHandler)
		fmt.Printf("Integrated reverse proxy enabled for: %v\n", reverseProxyHandler.GetRoutes())
	}

	// Caddy setup
	goListenAddr := setupCaddy(s, cfg)

	// Proxy API routes (always registered so the endpoint responds gracefully)
	proxyHandler := handlers.NewProxyHandler(s.proxyServer, cfg.Server)
	mux.HandleFunc("/api/proxy/status", proxyHandler.GetStatus)

	// Auth-protected endpoints
	mux.HandleFunc("/api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		authMiddleware.RequireAuth(http.HandlerFunc(authHandler.Me)).ServeHTTP(w, r)
	})
	mux.HandleFunc("/api/auth/password", func(w http.ResponseWriter, r *http.Request) {
		authMiddleware.RequireAuth(http.HandlerFunc(authHandler.ChangePassword)).ServeHTTP(w, r)
	})

	// Serve embedded frontend files
	if distErr != nil {
		// Fallback to serving from web/dist during development
		fileServer := http.FileServer(http.Dir("web/dist"))
		staticHandler = spaHandlerDev(fileServer, "web/dist", "index.html")
	} else {
		staticHandler = spaHandlerEmbed(http.FileServer(http.FS(distFS)), distFS, "index.html")
	}

	// Serve static files with SPA fallback
	mux.Handle("/", staticHandler)

	// Create the final handler with security middleware
	handler := s.setupGuardMiddleware(wrapMiddleware(mux, cfg, authMiddleware))

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
	{Path: "/*.js"},
	{Path: "/*.css"},
	{Path: "/*.ico"},
	{Path: "/*.png"},
	{Path: "/*.svg"},
	{Path: "/login"},
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
		Headers: auth.ForwardAuthHeaders{
			User:   cfg.Auth.Headers["user"],
			Email:  cfg.Auth.Headers["email"],
			Groups: cfg.Auth.Headers["groups"],
			Name:   cfg.Auth.Headers["name"],
		},
		BypassRules: defaultBypassRules,
	}
	authMiddleware := auth.NewMiddleware(authConfig, sessionStore, userStore)

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
	}
	return auth.NewOIDCProvider(oidcConfig, sessionStore, userStore)
}

// registerAuthRoutes registers authentication and WebSocket endpoints.
func registerAuthRoutes(mux *http.ServeMux, authHandler *handlers.AuthHandler, wsHub *websocket.Hub) {
	loginLimiter := newRateLimiter(5, 1*time.Minute) // 5 attempts per IP per minute
	mux.HandleFunc("/api/auth/login", loginLimiter.wrap(authHandler.Login))
	mux.HandleFunc("/api/auth/logout", authHandler.Logout)
	mux.HandleFunc("/api/auth/status", authHandler.AuthStatus)
	mux.HandleFunc("/api/auth/oidc/login", authHandler.OIDCLogin)
	mux.HandleFunc("/api/auth/oidc/callback", authHandler.OIDCCallback)

	// WebSocket endpoint
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		websocket.ServeWs(wsHub, w, r)
	})
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

	// Configure apps for health monitoring
	healthApps := make([]health.AppConfig, 0, len(cfg.Apps))
	for _, app := range cfg.Apps {
		healthApps = append(healthApps, health.AppConfig{
			Name:      app.Name,
			URL:       app.URL,
			HealthURL: app.HealthURL,
			Enabled:   app.Enabled,
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
		if strings.HasSuffix(path, "/health/check") {
			healthHandler.CheckAppHealth(w, r)
		} else if strings.HasSuffix(path, "/health") {
			healthHandler.GetAppHealth(w, r)
		} else {
			http.Error(w, "Not found", http.StatusNotFound)
		}
	})
}

// registerThemeRoutes registers theme API and CSS serving routes.
// staticHandler is a pointer so the /themes/ closure can reference the handler
// that gets assigned later (forward declaration pattern).
func registerThemeRoutes(mux *http.ServeMux, distFS fs.FS, requireAdmin adminGuard, staticHandler *http.Handler) {
	themeHandler := handlers.NewThemeHandler(themesDataDir, distFS)
	mux.HandleFunc("/api/themes", func(w http.ResponseWriter, r *http.Request) {
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
	// Serve theme CSS files: try data/themes/ first (user-created), fall back to static assets (bundled)
	mux.HandleFunc("/themes/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/themes/")
		// Only allow safe CSS theme filenames (allowlist approach)
		if !validThemeName.MatchString(name) {
			http.NotFound(w, r)
			return
		}
		localPath := filepath.Join(themesDataDir, name)
		// Double-check resolved path is within the themes directory
		absPath, err := filepath.Abs(localPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		absThemesDir, _ := filepath.Abs(themesDataDir)
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
func registerIconRoutes(mux *http.ServeMux, cfg *config.Config, requireAdmin adminGuard) {
	cacheTTL := parseDuration(cfg.Icons.DashboardIcons.CacheTTL, 7*24*time.Hour)
	dashboardClient := icons.NewDashboardIconsClient(cfg.Icons.DashboardIcons.CacheDir, cacheTTL)
	lucideClient := icons.NewLucideClient("data/icons/lucide", cacheTTL)
	iconHandler := handlers.NewIconHandler(dashboardClient, lucideClient, "data/icons/custom")

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
	s.proxyServer = proxy.New(proxyConfig)

	// Configure proxy routes for apps with proxy enabled
	var proxyRoutes []proxy.AppRoute
	for _, app := range cfg.Apps {
		if app.Proxy && app.Enabled {
			proxyRoutes = append(proxyRoutes, proxy.AppRoute{
				Name:      app.Name,
				Slug:      proxy.Slugify(app.Name),
				TargetURL: app.URL,
				Enabled:   true,
			})
		}
	}
	s.proxyServer.SetRoutes(proxyRoutes)

	return internalAddr
}

// wrapMiddleware applies auth and security middleware around the mux.
func wrapMiddleware(mux *http.ServeMux, cfg *config.Config, authMiddleware *auth.Middleware) http.Handler {
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
		if !s.needsSetup.Load() {
			next.ServeHTTP(w, r)
			return
		}

		path := r.URL.Path

		// Allow auth endpoints
		if strings.HasPrefix(path, "/api/auth/") {
			next.ServeHTTP(w, r)
			return
		}

		// Allow health endpoint
		if path == "/api/health" {
			next.ServeHTTP(w, r)
			return
		}

		// Allow non-API paths (static assets, SPA)
		if !strings.HasPrefix(path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		// Block all other API endpoints during setup
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "setup_required"})
	})
}

// setupRequest is the JSON body for POST /api/auth/setup
type setupRequest struct {
	Method         string            `json:"method"`
	Username       string            `json:"username"`
	Password       string            `json:"password"`
	TrustedProxies []string          `json:"trusted_proxies"`
	Headers        map[string]string `json:"headers"`
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	if !s.needsSetup.Load() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "Setup already completed"})
		return
	}

	var req setupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	switch req.Method {
	case "builtin":
		if err := s.setupBuiltin(w, req); err != nil {
			return // error already written
		}
	case "forward_auth":
		if err := s.setupForwardAuth(req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
	case "none":
		s.setupNone()
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid method. Must be builtin, forward_auth, or none"})
		return
	}

	s.config.Auth.SetupComplete = true
	if err := s.config.Save(s.configPath); err != nil {
		log.Printf("Failed to save config after setup: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save configuration"})
		return
	}

	s.needsSetup.Store(false)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"method":  req.Method,
	})
}

func (s *Server) setupBuiltin(w http.ResponseWriter, req setupRequest) error {
	if strings.TrimSpace(req.Username) == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Username is required"})
		return fmt.Errorf("username required")
	}
	if len(req.Password) < 8 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Password must be at least 8 characters"})
		return fmt.Errorf("password too short")
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to hash password"})
		return err
	}

	// Update config
	s.config.Auth.Method = "builtin"
	s.config.Auth.Users = []config.UserConfig{
		{
			Username:     req.Username,
			PasswordHash: hash,
			Role:         "admin",
		},
	}

	// Update live user store
	s.userStore.LoadFromConfig([]auth.UserConfig{
		{
			Username:     req.Username,
			PasswordHash: hash,
			Role:         "admin",
		},
	})

	// Update auth middleware
	s.authMiddleware.UpdateConfig(auth.AuthConfig{
		Method:      auth.AuthMethodBuiltin,
		BypassRules: defaultBypassRules,
		APIKey:      s.config.Auth.APIKey,
	})

	// Create session so user is immediately logged in
	session, err := s.sessionStore.Create(req.Username, req.Username, "admin")
	if err != nil {
		log.Printf("Failed to create session after setup: %v", err)
		// Non-fatal — setup still succeeds, user just needs to log in manually
		return nil
	}
	s.sessionStore.SetCookie(w, session)

	return nil
}

func (s *Server) setupForwardAuth(req setupRequest) error {
	if len(req.TrustedProxies) == 0 {
		return fmt.Errorf("At least one trusted proxy is required for forward_auth")
	}

	// Update config
	s.config.Auth.Method = "forward_auth"
	s.config.Auth.TrustedProxies = req.TrustedProxies
	if req.Headers != nil {
		s.config.Auth.Headers = req.Headers
	}

	// Update auth middleware
	headers := auth.ForwardAuthHeaders{}
	if req.Headers != nil {
		headers.User = req.Headers["user"]
		headers.Email = req.Headers["email"]
		headers.Groups = req.Headers["groups"]
		headers.Name = req.Headers["name"]
	}

	s.authMiddleware.UpdateConfig(auth.AuthConfig{
		Method:         auth.AuthMethodForwardAuth,
		TrustedProxies: req.TrustedProxies,
		Headers:        headers,
		BypassRules:    defaultBypassRules,
		APIKey:         s.config.Auth.APIKey,
	})

	return nil
}

func (s *Server) setupNone() {
	s.config.Auth.Method = "none"
	// Middleware stays as-is (virtual admin)
}

// Start begins serving HTTP requests
func (s *Server) Start() error {
	// Start WebSocket hub
	go s.wsHub.Run()

	// Start health monitoring if enabled
	if s.healthMonitor != nil {
		s.healthMonitor.Start()
	}

	// Start Caddy if configured (must start before Go server claims its port)
	if s.proxyServer != nil {
		if err := s.proxyServer.Start(); err != nil {
			return fmt.Errorf("failed to start caddy: %w", err)
		}
		log.Printf("Muximux started on %s (Caddy on user port, Go on %s)",
			s.config.Server.Listen, s.proxyServer.GetInternalAddr())
	} else {
		log.Printf("Muximux started on %s", s.config.Server.Listen)
	}

	return s.httpServer.ListenAndServe()
}

// GetHub returns the WebSocket hub for broadcasting events
func (s *Server) GetHub() *websocket.Hub {
	return s.wsHub
}

// Stop gracefully shuts down the server
func (s *Server) Stop() error {
	// Stop health monitoring
	if s.healthMonitor != nil {
		s.healthMonitor.Stop()
	}

	// Stop Caddy
	if s.proxyServer != nil {
		if err := s.proxyServer.Stop(); err != nil {
			fmt.Printf("Warning: failed to stop caddy: %v\n", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

// spaHandlerDev wraps a file server for development (filesystem-based)
func spaHandlerDev(fileServer http.Handler, distDir string, indexPath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// For root or paths without extension (likely SPA routes), serve index.html directly
		// We use http.ServeFile instead of FileServer to avoid redirect loops
		// Exclude /api/, /ws, /proxy/, and /icons/ paths
		if path == "/" || (!strings.Contains(path, ".") && !strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/ws") && !strings.HasPrefix(path, proxyPathPrefix) && !strings.HasPrefix(path, "/icons/")) {
			http.ServeFile(w, r, distDir+"/"+indexPath)
			return
		}

		fileServer.ServeHTTP(w, r)
	})
}

// spaHandlerEmbed wraps a file server for embedded files
func spaHandlerEmbed(fileServer http.Handler, fsys fs.FS, indexPath string) http.Handler {
	// Pre-read the index.html content for SPA routes
	indexContent, err := fs.ReadFile(fsys, indexPath)
	if err != nil {
		// Fallback to letting fileServer handle it
		return fileServer
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// For root or paths without extension (likely SPA routes), serve index.html directly
		// Exclude /api/, /ws, /proxy/, and /icons/ paths
		if path == "/" || (!strings.Contains(path, ".") && !strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/ws") && !strings.HasPrefix(path, proxyPathPrefix) && !strings.HasPrefix(path, "/icons/")) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
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
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") && !strings.HasPrefix(ct, "multipart/form-data") {
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
			if r.URL.Path == "/api/config" || strings.HasPrefix(r.URL.Path, "/api/themes") {
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
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		attempts: make(map[string][]time.Time),
		max:      max,
		window:   window,
	}
	go rl.cleanup()
	return rl
}

// cleanup periodically removes stale rate-limit entries.
func (rl *rateLimiter) cleanup() {
	for range time.Tick(5 * time.Minute) {
		rl.purgeStaleEntries()
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
			log.Printf("Rate limit exceeded for IP %s on %s", ip, r.URL.Path)
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(rl.window.Seconds())))
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		handler(w, r)
	}
}
