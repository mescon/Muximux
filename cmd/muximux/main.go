package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/server"
)

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

// envOrDefault returns the environment variable value if set, otherwise the fallback.
func envOrDefault(envKey, fallback string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return fallback
}

func main() {
	// Command line flags (env vars used as defaults where applicable)
	configPath := flag.String("config", envOrDefault("MUXIMUX_CONFIG", "config.yaml"), "Path to configuration file (env: MUXIMUX_CONFIG)")
	listenAddr := flag.String("listen", "", "Override listen address, e.g. :9090 (env: MUXIMUX_LISTEN)")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Muximux %s (commit: %s, built: %s)\n", version, commit, buildDate)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Apply CLI/env overrides
	if *listenAddr != "" {
		cfg.Server.Listen = *listenAddr
	} else if v := os.Getenv("MUXIMUX_LISTEN"); v != "" {
		cfg.Server.Listen = v
	}

	// Create and start server
	srv, err := server.New(cfg, *configPath)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	log.Printf("Muximux %s started on %s", version, cfg.Server.Listen)

	<-quit
	log.Println("Shutting down...")

	if err := srv.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Goodbye!")
}
