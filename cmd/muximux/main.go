package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/logging"
	"github.com/mescon/muximux/v3/internal/server"
)

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

func envOrDefault(envKey, fallback string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return fallback
}

func resolveDataDir(dir string) string {
	if !filepath.IsAbs(dir) {
		if exe, err := os.Executable(); err == nil {
			if resolved, err := filepath.EvalSymlinks(exe); err == nil {
				return filepath.Join(filepath.Dir(resolved), dir)
			}
		}
	}
	return dir
}

func applyOverrides(cfg *config.Config, listenAddr, basePath string) {
	if listenAddr != "" {
		cfg.Server.Listen = listenAddr
	} else if v := os.Getenv("MUXIMUX_LISTEN"); v != "" {
		cfg.Server.Listen = v
	}
	if basePath != "" {
		cfg.Server.BasePath = basePath
	} else if v := os.Getenv("MUXIMUX_BASE_PATH"); v != "" {
		cfg.Server.BasePath = v
	}
}

// runHash generates a bcrypt hash from a password/key provided as an argument or via stdin.
func runHash() {
	var password string

	if len(os.Args) > 2 {
		password = os.Args[2]
	} else {
		fmt.Print("Enter value to hash: ")
		fd := int(os.Stdin.Fd()) //nolint:gosec // Fd() returns 0 for stdin; overflow is not possible
		if term.IsTerminal(fd) {
			bytePassword, err := term.ReadPassword(fd)
			fmt.Println()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
				os.Exit(1)
			}
			password = string(bytePassword)
		} else {
			reader := bufio.NewReader(os.Stdin)
			var err error
			password, err = reader.ReadString('\n')
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
				os.Exit(1)
			}
			password = strings.TrimSpace(password)
		}
	}

	if password == "" {
		fmt.Fprintln(os.Stderr, "Value cannot be empty")
		os.Exit(1)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating hash: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(hash)
}

func main() {
	// Handle subcommands before flag parsing
	if len(os.Args) > 1 && os.Args[1] == "hash" {
		runHash()
		return
	}

	dataDir := flag.String("data", envOrDefault("MUXIMUX_DATA", "data"), "Data directory for config, themes, icons (env: MUXIMUX_DATA)")
	configPath := flag.String("config", envOrDefault("MUXIMUX_CONFIG", ""), "Override config file path (env: MUXIMUX_CONFIG)")
	listenAddr := flag.String("listen", "", "Override listen address, e.g. :9090 (env: MUXIMUX_LISTEN)")
	basePath := flag.String("base-path", "", "Base URL path for reverse proxy subpath, e.g. /muximux (env: MUXIMUX_BASE_PATH)")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	*dataDir = resolveDataDir(*dataDir)

	if *configPath == "" {
		*configPath = filepath.Join(*dataDir, "config.yaml")
	}

	if *showVersion {
		fmt.Printf("Muximux %s (commit: %s, built: %s)\n", version, commit, buildDate)
		os.Exit(0)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Environment variable overrides for logging (take precedence over config file)
	if v := os.Getenv("MUXIMUX_LOG_LEVEL"); v != "" {
		cfg.Server.LogLevel = v
	}
	if v := os.Getenv("MUXIMUX_LOG_FORMAT"); v != "" {
		cfg.Server.LogFormat = v
	}

	logFile := filepath.Join(*dataDir, "muximux.log")
	if err := logging.Init(logging.Config{
		Level:   logging.Level(cfg.Server.LogLevel),
		Format:  cfg.Server.LogFormat,
		Output:  "stdout",
		LogFile: logFile,
	}); err != nil {
		log.Fatalf("Failed to initialize logging: %v", err)
	}
	logging.LoadRecentFromFile()

	applyOverrides(cfg, *listenAddr, *basePath)

	// Warn if OIDC client_secret is stored as plaintext in config
	if cfg.Auth.OIDC.Enabled && cfg.Auth.OIDC.ClientSecret != "" && !config.IsBracedEnvRef(cfg.Auth.OIDC.ClientSecret) {
		logging.Warn("OIDC client_secret is stored in plaintext config — consider using ${ENV_VAR} syntax", "source", "config")
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

	logging.Close()
	logging.Info("Goodbye!", "source", "server")
}
