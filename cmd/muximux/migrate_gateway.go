package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/mescon/muximux/v3/internal/config"
)

// runMigrateGateway implements the `muximux migrate-gateway <path>`
// subcommand. It reads an operator-written Caddyfile (the v3.0.x
// `server.gateway:` form), runs it through the package-level
// converter in internal/config, and emits the equivalent YAML on
// stdout for pasting under `server:` in `config.yaml`.
//
// Warnings emitted by the converter go to stderr so the operator
// knows which directives the structured form cannot represent.
//
// Usage:
//
//	muximux migrate-gateway /app/data/sites.Caddyfile > sites.yaml
func runMigrateGateway() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: muximux migrate-gateway <caddyfile>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Reads the given Caddyfile (the v3.0.x server.gateway: form), prints")
		fmt.Fprintln(os.Stderr, "equivalent YAML for server.gateway_sites: on stdout, and warnings on")
		fmt.Fprintln(os.Stderr, "stderr for any directive that cannot be auto-migrated.")
		os.Exit(2)
	}
	path := os.Args[2]

	// Cap the input at 10 MiB so a misdirected --file at a giant log
	// or a deliberately-huge Caddyfile cannot OOM the operator's
	// host. Real-world Caddyfiles are tens to hundreds of KiB.
	const maxCaddyfileBytes = 10 * 1024 * 1024
	info, statErr := os.Stat(path) //nolint:gosec // operator-supplied path from argv; same source as the size-capped ReadFile below
	if statErr != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", path, statErr)
		os.Exit(1)
	}
	if info.Size() > maxCaddyfileBytes {
		fmt.Fprintf(os.Stderr, "Error: %s is %d bytes; refusing to parse files larger than %d bytes\n", path, info.Size(), maxCaddyfileBytes)
		os.Exit(1)
	}
	src, err := os.ReadFile(path) //nolint:gosec // operator-supplied path; size-capped above
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", path, err)
		os.Exit(1)
	}

	sites, warnings, err := config.MigrateCaddyfileToSites(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing Caddyfile: %v\n", err)
		os.Exit(1)
	}

	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}

	if len(sites) == 0 {
		fmt.Fprintln(os.Stderr, "No host-based sites found. Nothing to migrate.")
		os.Exit(0)
	}

	// Emit a stub `gateway_sites:` block so the operator can paste the
	// output directly under `server:` in config.yaml.
	wrapper := struct {
		GatewaySites []config.GatewaySite `yaml:"gateway_sites"`
	}{GatewaySites: sites}

	enc := yaml.NewEncoder(os.Stdout)
	enc.SetIndent(2)
	if err := enc.Encode(wrapper); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing YAML: %v\n", err)
		os.Exit(1)
	}
	if err := enc.Close(); err != nil {
		// Surface flush errors (broken stdout, full disk on a redirect)
		// so the operator notices a truncated YAML file rather than
		// silently using it.
		fmt.Fprintf(os.Stderr, "Error finalising YAML: %v\n", err)
		os.Exit(1)
	}
}
