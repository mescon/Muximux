package handlers

import (
	"path"
	"strings"
	"testing"
	"unicode/utf8"
)

// The embedding proxy is the one component that consumes bytes it does not
// control: request paths chosen by the client, and HTML/CSS/JS bodies
// returned by whatever backend the operator pointed an app at. These fuzz
// targets cover both. They assert invariants rather than exact output, so
// they stay useful as the rewriter's regexes evolve.
//
// Run the seed corpus (fast, what CI does):
//
//	go test ./internal/handlers/ -run Fuzz
//
// Run an actual fuzzing campaign:
//
//	go test ./internal/handlers/ -fuzz FuzzContentRewriter -fuzztime 60s

// FuzzResolveBackendRequestPath checks that joining a client-supplied path
// onto an app's configured base path cannot walk out of that base.
//
// targetPath is how an operator says "this app lives under /admin on the
// backend" (Pi-hole being the usual case). Everything the proxy forwards is
// supposed to land under it. If a request path containing dot segments can
// escape, an authenticated user reaches parts of the backend the operator
// did not intend to expose through this app.
func FuzzResolveBackendRequestPath(f *testing.F) {
	seeds := []struct{ reqPath, targetPath string }{
		{"/", "/admin"},
		{"/api/status", "/admin"},
		{"/../etc/passwd", "/admin"},
		{"/../../root", "/admin/"},
		{"..%2f..%2froot", "/admin"},
		{"/./././x", "/admin"},
		{"//evil.example.com/x", "/admin"},
		{"/a/../../b", "/admin/sub"},
		{"", ""},
		{"/x", ""},
		{"\\..\\..\\x", "/admin"},
	}
	for _, s := range seeds {
		f.Add(s.reqPath, s.targetPath)
	}

	f.Fuzz(func(t *testing.T, reqPath, targetPath string) {
		got := resolveBackendRequestPath(reqPath, targetPath)

		// targetPath is always url.Parse(app.URL).Path, so it is either empty
		// (normalised to "/" by the caller) or absolute. Relative values like
		// "." cannot occur and the containment check is meaningless for them.
		if !strings.HasPrefix(targetPath, "/") {
			return
		}

		base := strings.TrimSuffix(targetPath, "/")
		if base == "" {
			// No base path configured: the request path passes through and
			// there is nothing to escape from.
			return
		}

		// Dot segments never reach this function: rejectDotSegmentsMiddleware
		// sits outside auth and 400s them, precisely because a percent-encoded
		// traversal survives http.ServeMux's redirect and would otherwise both
		// satisfy an auth_bypass prefix rule and escape the base path here.
		// The join itself is deliberately a plain concatenation; the security
		// boundary is that middleware, and server_test.go asserts it.
		if hasDotSegmentPath(reqPath) {
			return
		}

		// path.Clean resolves the dot segments a backend (or any normalising
		// intermediary) would resolve. After that the result must still sit
		// under the configured base.
		cleaned := path.Clean(got)
		cleanBase := path.Clean(base)
		if cleanBase == "/" {
			// Root base: every absolute path is trivially contained.
			return
		}
		if cleaned == cleanBase || strings.HasPrefix(cleaned, cleanBase+"/") {
			return
		}
		t.Errorf("resolved path escapes the app's base path:\n"+
			"  reqPath    = %q\n  targetPath = %q\n  joined     = %q\n  cleaned    = %q\n  base       = %q",
			reqPath, targetPath, got, cleaned, cleanBase)
	})
}

// FuzzContentRewriter runs arbitrary bytes through the full rewrite pipeline.
// The bodies it models are backend responses, so they are entirely outside
// Muximux's control and may be malformed, truncated, or hostile.
//
// The contract asserted here is deliberately weak, because the rewriter is a
// pile of regexes whose exact output is expected to change: it must not
// panic, and it must not corrupt valid UTF-8 into invalid UTF-8. The second
// half matters because the result is served to a browser; mangled encoding
// is how parser-confusion bugs start.
func FuzzContentRewriter(f *testing.F) {
	seeds := []string{
		`<html><head><base href="/admin/"></head><body></body></html>`,
		`<a href="/admin/page">x</a>`,
		`<img srcset="/admin/a.png 1x, /admin/b.png 2x">`,
		`<script src="/admin/app.js" integrity="sha384-abc"></script>`,
		`<meta http-equiv="Content-Security-Policy" content="default-src 'self'">`,
		`<meta http-equiv="refresh" content="0; url=/admin/next">`,
		`@import url("/admin/theme.css");`,
		`import x from "/admin/mod.js";`,
		`background: image-set("/admin/a.png" 1x);`,
		`url(/admin/bg.png)`,
		`<svg><use href="/admin/sprite.svg#i"/></svg>`,
		`urlBase = "/admin"`,
		"<html>\x00\xff\xfe truncated",
		strings.Repeat(`<a href="/admin/x">`, 64),
		"",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	rw := newContentRewriter("/proxy/app", "/admin", "backend.internal")

	f.Fuzz(func(t *testing.T, body string) {
		in := []byte(body)

		// Must not panic on any input.
		out := rw.rewrite(in)

		// Valid UTF-8 in must stay valid UTF-8 out. Invalid input is not
		// held to that standard -- the rewriter is not a sanitiser and is
		// not expected to repair a broken body.
		if utf8.Valid(in) && !utf8.Valid(out) {
			t.Errorf("rewriter turned valid UTF-8 into invalid UTF-8\n  in  = %q\n  out = %q", in, out)
		}
	})
}

// FuzzRewriteScript covers the JavaScript path separately. It is reached for
// script bodies rather than documents and applies a different subset of the
// rewrites, so document-shaped seeds do not exercise it.
func FuzzRewriteScript(f *testing.F) {
	for _, s := range []string{
		`fetch("/admin/api")`,
		`const u = '/admin/x';`,
		`import("/admin/chunk.js")`,
		`new URL("/admin/a", location.origin)`,
		`//# sourceMappingURL=/admin/app.js.map`,
		"",
		"\xff\xfe",
	} {
		f.Add(s)
	}

	rw := newContentRewriter("/proxy/app", "/admin", "backend.internal")

	f.Fuzz(func(t *testing.T, script string) {
		in := []byte(script)
		out := rw.rewriteScript(in)
		if utf8.Valid(in) && !utf8.Valid(out) {
			t.Errorf("rewriteScript turned valid UTF-8 into invalid UTF-8\n  in  = %q\n  out = %q", in, out)
		}
	})
}

// FuzzExpandHeaderValue drives identity values that originate at an external
// identity provider (OIDC claims, forward-auth headers) into the per-app
// custom-header templates. A username carrying CR or LF would be header
// injection into the upstream request, so the expansion must strip them.
func FuzzExpandHeaderValue(f *testing.F) {
	for _, s := range []string{
		"alice",
		"alice\r\nX-Admin: true",
		"alice\nX-Admin: true",
		"\r\n\r\nGET /evil HTTP/1.1",
		"a\x00b",
		strings.Repeat("a", 300),
		"",
	} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, username string) {
		got := sanitizeHeaderValue(username)
		for _, r := range []string{"\r", "\n", "\x00"} {
			if strings.Contains(got, r) {
				t.Errorf("sanitizeHeaderValue kept %q\n  in  = %q\n  out = %q", r, username, got)
			}
		}
	})
}

// hasDotSegmentPath mirrors rejectDotSegmentsMiddleware's check. Duplicated
// rather than exported because it exists to document this fuzz target's
// precondition, not to be reused.
func hasDotSegmentPath(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg == "." || seg == ".." {
			return true
		}
	}
	return false
}
