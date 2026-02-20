package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/logging"
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
	dataDir := flag.String("data", envOrDefault("MUXIMUX_DATA", "data"), "Data directory for config, themes, icons (env: MUXIMUX_DATA)")
	configPath := flag.String("config", envOrDefault("MUXIMUX_CONFIG", ""), "Override config file path (env: MUXIMUX_CONFIG)")
	listenAddr := flag.String("listen", "", "Override listen address, e.g. :9090 (env: MUXIMUX_LISTEN)")
	basePath := flag.String("base-path", "", "Base URL path for reverse proxy subpath, e.g. /muximux (env: MUXIMUX_BASE_PATH)")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	// Resolve relative data directory against the binary's location so that
	// muximux always looks for "data/" beside the executable regardless of CWD.
	if !filepath.IsAbs(*dataDir) {
		if exe, err := os.Executable(); err == nil {
			if resolved, err := filepath.EvalSymlinks(exe); err == nil {
				*dataDir = filepath.Join(filepath.Dir(resolved), *dataDir)
			}
		}
	}

	// Derive config path from data dir if not explicitly set
	if *configPath == "" {
		*configPath = filepath.Join(*dataDir, "config.yaml")
	}

	if *showVersion {
		fmt.Printf("Muximux %s (commit: %s, built: %s)\n", version, commit, buildDate)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Run config migrations (e.g. role renames)
	if cfg.Migrate() {
		if err := cfg.Save(*configPath); err != nil {
			log.Printf("Warning: failed to save migrated config: %v", err)
		}
	}

	// Initialize structured logging — always write to stdout + data/muximux.log
	logFile := filepath.Join(*dataDir, "muximux.log")
	if err := logging.Init(logging.Config{
		Level:   logging.Level(cfg.Server.LogLevel),
		Format:  "text",
		Output:  "stdout",
		LogFile: logFile,
	}); err != nil {
		log.Fatalf("Failed to initialize logging: %v", err)
	}

	// Apply CLI/env overrides
	if *listenAddr != "" {
		cfg.Server.Listen = *listenAddr
	} else if v := os.Getenv("MUXIMUX_LISTEN"); v != "" {
		cfg.Server.Listen = v
	}
	if *basePath != "" {
		cfg.Server.BasePath = *basePath
	} else if v := os.Getenv("MUXIMUX_BASE_PATH"); v != "" {
		cfg.Server.BasePath = v
	}

	// Create and start server
	srv, err := server.New(cfg, *configPath, *dataDir, version, commit, buildDate)
	if err != nil {
		logging.Error("Failed to create server", "source", "server", "error", err)
		os.Exit(1)
	}

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil {
			logging.Error("Server error", "source", "server", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	logging.Info("Shutting down", "source", "server")

	if err := srv.Stop(); err != nil {
		logging.Error("Error during shutdown", "source", "server", "error", err)
	}

	logging.Info("Goodbye!", "source", "server")
}
