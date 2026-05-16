package config

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/mescon/muximux/v3/internal/logging"
)

// backupSuffix is the marker appended to legacy files we rewrite
// during the 3.1.0 auto-migration. Operators can keep these around
// indefinitely; they're harmless and document what we replaced.
const backupSuffix = ".pre-3.1.0.bak"

// autoMigrateGateway handles the one-shot upgrade path for operators
// arriving from 3.0.x with `server.gateway:` set to a Caddyfile path.
//
// On a clean conversion: the Caddyfile is parsed via the same converter
// that powers the `muximux migrate-gateway` CLI, the resulting sites
// are merged into cfg.Server.GatewaySites, the original config.yaml
// and Caddyfile are renamed with a .pre-3.1.0.bak suffix, and the
// rewritten config.yaml replaces the original on disk.
//
// On a lossy conversion (the converter emitted any warning, or the
// parser failed, or both the legacy field AND a non-empty
// gateway_sites: are present so we can't safely combine them), the
// function returns a clear error instructing the operator to run
// `muximux migrate-gateway` by hand. Hard fail beats silent data
// loss.
//
// On a missing Caddyfile (the path is set but the file is gone), we
// log a warning and clear the legacy field so startup proceeds with
// whatever gateway_sites: the operator already has.
func autoMigrateGateway(cfg *Config, configPath string) error {
	if cfg.Server.Gateway == "" {
		return nil
	}
	if len(cfg.Server.GatewaySites) > 0 {
		// Both forms set: we can't safely combine, since the operator
		// may already have edited gateway_sites: by hand. Refuse.
		return fmt.Errorf("server.gateway (Caddyfile path %q) and server.gateway_sites: are both set; remove one. Recommended: run `muximux migrate-gateway %q > sites.yaml` to convert the Caddyfile, paste the result into config.yaml under server.gateway_sites:, then delete the server.gateway: line", cfg.Server.Gateway, cfg.Server.Gateway)
	}

	caddyfilePath := cfg.Server.Gateway
	src, readErr := os.ReadFile(caddyfilePath) //nolint:gosec // operator-supplied path from config.yaml
	if os.IsNotExist(readErr) {
		// The Caddyfile is missing on disk. This can happen if the
		// operator already moved or deleted it but left the config
		// pointer dangling. Clear the field in memory and continue;
		// they have no sites to migrate anyway.
		logging.Warn("server.gateway points at a missing Caddyfile, clearing the field for this boot",
			"source", "config",
			"path", caddyfilePath)
		cfg.Server.Gateway = ""
		return nil
	}
	if readErr != nil {
		return fmt.Errorf("server.gateway: cannot read %q: %w", caddyfilePath, readErr)
	}

	sites, warnings, err := MigrateCaddyfileToSites(src)
	if err != nil {
		return fmt.Errorf("server.gateway: cannot auto-migrate %q (%w). Run `muximux migrate-gateway %q > sites.yaml` to handle this by hand, paste the output into config.yaml under server.gateway_sites:, then delete the server.gateway: line", caddyfilePath, err, caddyfilePath)
	}
	if len(warnings) > 0 {
		// Lossy conversion: refuse to silently rewrite the operator's
		// config when the converter could not represent every
		// directive. Surface each warning in the error so the
		// operator can see exactly what's at stake.
		return fmt.Errorf("server.gateway: auto-migration of %q would lose information (%d warning(s)):\n  - %s\nRun `muximux migrate-gateway %q > sites.yaml` to see the warnings, decide what to migrate by hand, paste the converted YAML into config.yaml under server.gateway_sites:, then delete the server.gateway: line", caddyfilePath, len(warnings), joinWarnings(warnings), caddyfilePath)
	}

	// Clean conversion. Mutate the in-memory config so subsequent
	// validate() + downstream code see gateway_sites and an empty
	// legacy field.
	cfg.Server.GatewaySites = sites
	cfg.Server.Gateway = ""

	// Persist to disk. Skip the rewrite when configPath is empty
	// (defaultConfig() path; Load() never hits that branch with a
	// legacy gateway: set, but defensive coding keeps tests simple).
	if configPath == "" {
		logging.Info("Auto-migrated server.gateway -> server.gateway_sites (in-memory only; configPath was empty)",
			"source", "config",
			"sites", len(sites))
		return nil
	}

	if err := rewriteConfigWithMigratedGateway(configPath, caddyfilePath, sites); err != nil {
		return fmt.Errorf("server.gateway: auto-migration converted the Caddyfile but failed to persist: %w. Re-run with the operator's gateway_sites pasted by hand", err)
	}

	logging.Info("Auto-migrated server.gateway -> server.gateway_sites",
		"source", "config",
		"sites", len(sites),
		"config_backup", configPath+backupSuffix,
		"caddyfile_backup", caddyfilePath+backupSuffix)
	return nil
}

// rewriteConfigWithMigratedGateway backs up the original config.yaml
// and the legacy Caddyfile, then rewrites config.yaml with the
// converted gateway_sites and an empty server.gateway:. The rewrite
// is staged through a sibling temp file and renamed atomically so a
// crash mid-write never leaves a truncated config.yaml on disk.
func rewriteConfigWithMigratedGateway(configPath, caddyfilePath string, sites []GatewaySite) error {
	// 1. Back up the original config.yaml.
	if err := copyFile(configPath, configPath+backupSuffix); err != nil {
		return fmt.Errorf("backup config.yaml: %w", err)
	}
	// 2. Back up the legacy Caddyfile.
	if err := copyFile(caddyfilePath, caddyfilePath+backupSuffix); err != nil {
		return fmt.Errorf("backup caddyfile: %w", err)
	}

	// 3. Read + re-decode + re-encode the YAML so we preserve the
	// operator's full config shape (every other field). We round-trip
	// through yaml.Node rather than the typed Config struct so
	// fields the typed Config doesn't surface (e.g. yaml comments,
	// unknown-but-still-parsable shapes) survive.
	raw, err := os.ReadFile(configPath) //nolint:gosec // operator-supplied path
	if err != nil {
		return fmt.Errorf("re-read config.yaml: %w", err)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return fmt.Errorf("re-parse config.yaml: %w", err)
	}
	if err := patchServerGateway(&root, sites); err != nil {
		return fmt.Errorf("patch server.gateway: %w", err)
	}

	// 4. Atomically replace the file. Write to a sibling temp file
	// in the same directory (same filesystem, so rename is atomic),
	// then rename over the original.
	tmpPath := configPath + ".tmp"
	tmpFile, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open temp file: %w", err)
	}
	enc := yaml.NewEncoder(tmpFile)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("encode rewritten config: %w", err)
	}
	if err := enc.Close(); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("flush encoder: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, configPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// patchServerGateway walks the parsed YAML document and mutates the
// `server.gateway:` field (sets it to empty / drops the entry) and
// inserts `server.gateway_sites:` with the converted sites.
// Preserves all other fields and their order so the operator can
// diff the result against their backup.
func patchServerGateway(root *yaml.Node, sites []GatewaySite) error {
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return fmt.Errorf("expected document node")
	}
	top := root.Content[0]
	if top.Kind != yaml.MappingNode {
		return fmt.Errorf("expected top-level mapping")
	}
	serverNode := findMappingValue(top, "server")
	if serverNode == nil {
		return fmt.Errorf("no server: key found")
	}
	if serverNode.Kind != yaml.MappingNode {
		return fmt.Errorf("server: is not a mapping")
	}

	// Remove the legacy `gateway:` entry entirely (we don't want it
	// lingering at "" in the rewritten file; it would confuse a
	// diff). Leaves the rest of the keys in their original order.
	removeMappingKey(serverNode, "gateway")

	// Encode the sites slice and graft the result onto server.
	sitesBytes, err := yaml.Marshal(sites)
	if err != nil {
		return fmt.Errorf("marshal sites: %w", err)
	}
	var sitesNode yaml.Node
	if err := yaml.Unmarshal(sitesBytes, &sitesNode); err != nil {
		return fmt.Errorf("unmarshal sites: %w", err)
	}
	if sitesNode.Kind != yaml.DocumentNode || len(sitesNode.Content) == 0 {
		return fmt.Errorf("unexpected sites encoding")
	}

	keyNode := &yaml.Node{
		Kind:        yaml.ScalarNode,
		Value:       "gateway_sites",
		HeadComment: "Auto-migrated from server.gateway: by 3.1.0 startup. Original Caddyfile saved as <path>" + backupSuffix + ".",
	}
	serverNode.Content = append(serverNode.Content, keyNode, sitesNode.Content[0])
	return nil
}

// findMappingValue returns the value node for the given key in a
// YAML mapping, or nil if the key is not present.
func findMappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// removeMappingKey deletes the given key from a YAML mapping
// in place. No-op if the key isn't present.
func removeMappingKey(mapping *yaml.Node, key string) {
	if mapping.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			return
		}
	}
}

// copyFile copies src to dst with 0600 perms. Used for the .bak
// snapshots written before we mutate config.yaml.
func copyFile(src, dst string) error {
	in, err := os.Open(src) //nolint:gosec // operator-supplied path from config.yaml
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// joinWarnings concatenates converter warnings into a single
// human-readable string for the auto-migration error message.
func joinWarnings(warnings []string) string {
	if len(warnings) == 0 {
		return ""
	}
	joined := warnings[0]
	for _, w := range warnings[1:] {
		joined += "\n  - " + w
	}
	return joined
}
