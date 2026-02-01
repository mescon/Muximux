package server

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"time"

	"github.com/mescon/muximux3/internal/config"
	"github.com/mescon/muximux3/internal/handlers"
)

//go:embed all:dist
var embeddedFiles embed.FS

// Server holds the HTTP server and related components
type Server struct {
	config     *config.Config
	httpServer *http.Server
	// TODO: Add proxy server when implemented
}

// New creates a new server instance
func New(cfg *config.Config) (*Server, error) {
	s := &Server{
		config: cfg,
	}

	// Set up routes
	mux := http.NewServeMux()

	// API routes
	api := handlers.NewAPIHandler(cfg)
	mux.HandleFunc("/api/config", api.GetConfig)
	mux.HandleFunc("/api/apps", api.GetApps)
	mux.HandleFunc("/api/groups", api.GetGroups)
	mux.HandleFunc("/api/health", api.Health)

	// Serve embedded frontend files
	distFS, err := fs.Sub(embeddedFiles, "dist")
	if err != nil {
		// Fallback to serving from web/dist during development
		mux.Handle("/", http.FileServer(http.Dir("web/dist")))
	} else {
		mux.Handle("/", http.FileServer(http.FS(distFS)))
	}

	s.httpServer = &http.Server{
		Addr:         cfg.Server.Listen,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s, nil
}

// Start begins serving HTTP requests
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// Stop gracefully shuts down the server
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}
