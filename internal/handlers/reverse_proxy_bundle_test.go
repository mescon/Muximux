package handlers

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRewriteScriptKeepsRealBundlesValid is a corpus regression guard for
// the #371 class of bug: an unanchored rewrite pattern that false-matches
// inside a minified JS string/regex literal and turns valid JavaScript
// into a syntax error. It runs every real minified bundle it can find
// (dependency *.min.js plus Muximux's own built chunks) through
// rewriteScript and asserts that anything valid before the rewrite is
// still valid after it, in the same parse goal (module or script).
//
// It is a best-effort guard: it skips cleanly when node is not on PATH or
// when no corpus is present (e.g. a pure-Go CI job with no installed
// node_modules and no built frontend), so it never fails spuriously. It
// complements the deterministic, node-free assertions in
// TestRewriteScriptPreservesJSStringLiterals and TestRewriteModuleImports.
func TestRewriteScriptKeepsRealBundlesValid(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not on PATH; skipping real-bundle corpus check")
	}

	var corpus []string
	for _, root := range []string{"../../web/node_modules", "../server/dist/assets"} {
		_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil //nolint:nilerr // intentional: per-entry skip, not a propagated failure
			}
			if strings.HasSuffix(p, ".min.js") ||
				(strings.Contains(filepath.ToSlash(p), "/dist/assets/") && strings.HasSuffix(p, ".js")) {
				corpus = append(corpus, p)
			}
			return nil
		})
	}
	if len(corpus) == 0 {
		t.Skip("no minified-bundle corpus available; skipping")
	}
	t.Logf("checking %d real bundles", len(corpus))

	tmp := t.TempDir()
	// node infers the parse goal from the file extension: .mjs = module,
	// .cjs = script. A bundle is "valid" in a mode if node --check passes.
	valid := func(mode string, src []byte) bool {
		ext := ".cjs"
		if mode == "module" {
			ext = ".mjs"
		}
		f := filepath.Join(tmp, "chk"+ext)
		if err := os.WriteFile(f, src, 0o600); err != nil { //nolint:gosec // f is filepath.Join(t.TempDir(), constant); no user-tainted path
			t.Fatal(err)
		}
		return exec.Command("node", "--check", f).Run() == nil //nolint:gosec // fixed args; f is a controlled temp path
	}

	// Two representative deployments: the common same-host/no-subpath
	// case, and a subpath+host case that additionally exercises the
	// target-path and absolute-URL patterns.
	configs := []struct{ prefix, targetPath, targetHost string }{
		{"/proxy/app", "", ""},
		{"/proxy/app", "/app", "192.0.2.10"},
	}

	for _, file := range corpus {
		src, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		okModule, okScript := valid("module", src), valid("script", src)
		if !okModule && !okScript {
			continue // not something node can parse; not our concern
		}
		for _, c := range configs {
			out := newContentRewriter(c.prefix, c.targetPath, c.targetHost).rewriteScript(src)
			if okModule && !valid("module", out) {
				t.Errorf("%s: valid ES module became invalid after rewriteScript (prefix=%q path=%q host=%q)",
					file, c.prefix, c.targetPath, c.targetHost)
			}
			if okScript && !valid("script", out) {
				t.Errorf("%s: valid script became invalid after rewriteScript (prefix=%q path=%q host=%q)",
					file, c.prefix, c.targetPath, c.targetHost)
			}
		}
	}
}
