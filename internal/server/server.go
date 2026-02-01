package server

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/mescon/muximux3/internal/auth"
	"github.com/mescon/muximux3/internal/config"
	"github.com/mescon/muximux3/internal/handlers"
	"github.com/mescon/muximux3/internal/health"
	"github.com/mescon/muximux3/internal/icons"
	"github.com/mescon/muximux3/internal/proxy"
	"github.com/mescon/muximux3/internal/websocket"
)

//go:embed all:dist
var embeddedFiles embed.FS

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
}

// New creates a new server instance
func New(cfg *config.Config, configPath string) (*Server, error) {
	// Create WebSocket hub
	wsHub := websocket.NewHub()

	// Set up authentication
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
		Headers: auth.ForwardAuthHeaders{
			User:   cfg.Auth.Headers["user"],
			Email:  cfg.Auth.Headers["email"],
			Groups: cfg.Auth.Headers["groups"],
			Name:   cfg.Auth.Headers["name"],
		},
		BypassRules: []auth.BypassRule{
			// Always allow auth endpoints
			{Path: "/api/auth/login"},
			{Path: "/api/auth/logout"},
			{Path: "/api/auth/status"},
			// Always allow static assets
			{Path: "/assets/*"},
			{Path: "/*.js"},
			{Path: "/*.css"},
			{Path: "/*.ico"},
			{Path: "/*.png"},
			{Path: "/*.svg"},
			// Allow login page
			{Path: "/login"},
		},
	}
	authMiddleware := auth.NewMiddleware(authConfig, sessionStore, userStore)

	s := &Server{
		config:         cfg,
		configPath:     configPath,
		wsHub:          wsHub,
		sessionStore:   sessionStore,
		userStore:      userStore,
		authMiddleware: authMiddleware,
	}

	// Set up routes
	mux := http.NewServeMux()

	// Auth endpoints (always accessible)
	authHandler := handlers.NewAuthHandler(sessionStore, userStore)
	mux.HandleFunc("/api/auth/login", authHandler.Login)
	mux.HandleFunc("/api/auth/logout", authHandler.Logout)
	mux.HandleFunc("/api/auth/status", authHandler.AuthStatus)

	// WebSocket endpoint
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		websocket.ServeWs(wsHub, w, r)
	})

	// API routes
	api := handlers.NewAPIHandler(cfg, configPath)

	// Config endpoint - handle both GET and PUT
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			api.GetConfig(w, r)
		case http.MethodPut:
			api.SaveConfig(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/apps", api.GetApps)
	mux.HandleFunc("/api/groups", api.GetGroups)
	mux.HandleFunc("/api/health", api.Health)

	// Health monitoring
	if cfg.Health.Enabled {
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

	// Icon routes
	cacheTTL := parseDuration(cfg.Icons.DashboardIcons.CacheTTL, 7*24*time.Hour)
	dashboardClient := icons.NewDashboardIconsClient(cfg.Icons.DashboardIcons.CacheDir, cacheTTL)
	iconHandler := handlers.NewIconHandler(dashboardClient, "data/icons/custom")

	mux.HandleFunc("/api/icons/dashboard", iconHandler.ListDashboardIcons)
	mux.HandleFunc("/api/icons/dashboard/", iconHandler.GetDashboardIcon)
	mux.HandleFunc("/icons/", iconHandler.ServeIcon)

	// Proxy setup
	if cfg.Proxy.Enabled {
		proxyConfig := proxy.Config{
			Enabled:   cfg.Proxy.Enabled,
			Listen:    cfg.Proxy.Listen,
			AutoHTTPS: cfg.Proxy.AutoHTTPS,
			ACMEEmail: cfg.Proxy.ACMEEmail,
			TLSCert:   cfg.Proxy.TLSCert,
			TLSKey:    cfg.Proxy.TLSKey,
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

		// Proxy API routes
		proxyHandler := handlers.NewProxyHandler(s.proxyServer)
		mux.HandleFunc("/api/proxy/status", proxyHandler.GetStatus)
		mux.HandleFunc("/api/proxy/app", proxyHandler.GetAppProxyURL)
	}

	// Auth-protected endpoints
	mux.HandleFunc("/api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		authMiddleware.RequireAuth(http.HandlerFunc(authHandler.Me)).ServeHTTP(w, r)
	})
	mux.HandleFunc("/api/auth/password", func(w http.ResponseWriter, r *http.Request) {
		authMiddleware.RequireAuth(http.HandlerFunc(authHandler.ChangePassword)).ServeHTTP(w, r)
	})

	// Serve embedded frontend files
	distFS, err := fs.Sub(embeddedFiles, "dist")
	var fileServer http.Handler
	if err != nil {
		// Fallback to serving from web/dist during development
		fileServer = http.FileServer(http.Dir("web/dist"))
	} else {
		fileServer = http.FileServer(http.FS(distFS))
	}

	// Wrap static file serving with SPA fallback
	mux.Handle("/", spaHandler(fileServer, "index.html"))

	// Create the final handler with auth middleware wrapping where needed
	var handler http.Handler = mux
	if cfg.Auth.Method != "none" && cfg.Auth.Method != "" {
		// Wrap the entire handler with auth middleware
		// The middleware will check bypass rules and auth status
		handler = authMiddleware.RequireAuth(mux)
	}

	s.httpServer = &http.Server{
		Addr:         cfg.Server.Listen,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s, nil
}

// Start begins serving HTTP requests
func (s *Server) Start() error {
	// Start WebSocket hub
	go s.wsHub.Run()

	// Start health monitoring if enabled
	if s.healthMonitor != nil {
		s.healthMonitor.Start()
	}

	// Start proxy if enabled
	if s.proxyServer != nil {
		if err := s.proxyServer.Start(); err != nil {
			return fmt.Errorf("failed to start proxy: %w", err)
		}
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

	// Stop proxy
	if s.proxyServer != nil {
		if err := s.proxyServer.Stop(); err != nil {
			// Log but don't fail
			fmt.Printf("Warning: failed to stop proxy: %v\n", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

// spaHandler wraps a file server to serve index.html for SPA routes
func spaHandler(fileServer http.Handler, indexPath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file
		path := r.URL.Path

		// For root or paths without extension (likely SPA routes), serve index.html
		if path == "/" || (!strings.Contains(path, ".") && !strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/ws")) {
			r.URL.Path = "/" + indexPath
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
