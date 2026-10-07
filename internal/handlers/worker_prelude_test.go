package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
)

// runPreludeInNode evaluates the worker prelude inside a fake worker global
// whose location is workerURL, then runs body (JS) and returns every URL the
// stubbed network APIs were handed. Skips when node is not installed.
func runPreludeInNode(t *testing.T, prelude, workerURL, body string) []string {
	t.Helper()
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not installed; the prelude's behaviour is exercised in a real JS engine only")
	}
	harness := `
const seen = [];
globalThis.self = globalThis;
Object.defineProperty(globalThis, "location", { value: new URL(` + jsString(workerURL) + `), configurable: true });
globalThis.fetch = (i) => { seen.push("fetch " + (typeof i === "string" ? i : (i && i.url) || String(i))); return Promise.resolve(); };
globalThis.XMLHttpRequest = class { open(m, u) { seen.push("xhr " + u); } };
globalThis.WebSocket = class { constructor(u) { seen.push("ws " + u); } };
globalThis.EventSource = class { constructor(u) { seen.push("sse " + u); } };
globalThis.importScripts = (...u) => { seen.push("import " + u.join(",")); };
` + prelude + `
` + body + `
process.stdout.write(JSON.stringify(seen));
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(harness)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, stderr.String())
	}
	var seen []string
	if err := json.Unmarshal(out, &seen); err != nil {
		t.Fatalf("decode %q: %v", out, err)
	}
	return seen
}

func jsString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// TestWorkerPrelude_BlobWorkerReachesApp reproduces the Dispatcharr player:
// the page builds `${location.origin}/proxy/ts/stream/<uuid>` and mpegts.js
// fetches it from a Web Worker created from a blob: URL. The page's
// interceptor does not run in workers, so without the prelude the request
// left as /proxy/ts/stream/<uuid> and Muximux answered "App not found: ts".
func TestWorkerPrelude_BlobWorkerReachesApp(t *testing.T) {
	prelude := string(newContentRewriter("/proxy/dispatcharr", "", "").workerPrelude())
	seen := runPreludeInNode(t, prelude, "blob:http://dash.example/0b7e5c1a-uuid", `
fetch("http://dash.example/proxy/ts/stream/edb8d92b-f527-468c-b042-489064aea119?output_format=mpegts");
fetch("/api/channels/");
fetch(new URL("http://dash.example/proxy/stats/"));
fetch("/proxy/dispatcharr/already/prefixed");
fetch("https://cdn.other.example/lib.js");
fetch("http://dash.example.evil/proxy/ts/x");
new XMLHttpRequest().open("GET", "http://dash.example/proxy/ts/stream/x");
new WebSocket("ws://dash.example/ws/?token=t");
new EventSource("/events");
importScripts("/assets/decoder.js");
`)
	want := []string{
		"fetch http://dash.example/proxy/dispatcharr/proxy/ts/stream/edb8d92b-f527-468c-b042-489064aea119?output_format=mpegts",
		"fetch http://dash.example/proxy/dispatcharr/api/channels/",
		"fetch http://dash.example/proxy/dispatcharr/proxy/stats/",
		"fetch http://dash.example/proxy/dispatcharr/already/prefixed",
		"fetch https://cdn.other.example/lib.js",
		"fetch http://dash.example.evil/proxy/ts/x",
		"xhr http://dash.example/proxy/dispatcharr/proxy/ts/stream/x",
		"ws ws://dash.example/proxy/dispatcharr/ws/?token=t",
		"sse http://dash.example/proxy/dispatcharr/events",
		"import http://dash.example/proxy/dispatcharr/assets/decoder.js",
	}
	if strings.Join(seen, "\n") != strings.Join(want, "\n") {
		t.Errorf("worker requests:\n got  %q\n want %q", seen, want)
	}
}

// TestWorkerPrelude_URLWorkerReachesApp covers a worker loaded from a script
// URL under the proxy: root-relative requests resolve against the origin and
// would otherwise skip the app's prefix too.
func TestWorkerPrelude_URLWorkerReachesApp(t *testing.T) {
	prelude := string(newContentRewriter("/proxy/dispatcharr", "", "").workerPrelude())
	seen := runPreludeInNode(t, prelude, "http://dash.example/proxy/dispatcharr/assets/worker.js", `
fetch("/proxy/ts/stream/abc");
fetch("relative/chunk.bin");
`)
	want := []string{
		"fetch http://dash.example/proxy/dispatcharr/proxy/ts/stream/abc",
		"fetch relative/chunk.bin",
	}
	if strings.Join(seen, "\n") != strings.Join(want, "\n") {
		t.Errorf("worker requests:\n got  %q\n want %q", seen, want)
	}
}

// TestWorkerPrelude_RunsOnce guards against double prefixing when the same
// prelude ends up evaluated twice in one scope (a cached script that was
// served both as a worker and as a page script).
func TestWorkerPrelude_RunsOnce(t *testing.T) {
	prelude := string(newContentRewriter("/proxy/app", "", "").workerPrelude())
	seen := runPreludeInNode(t, prelude+"\n"+prelude, "blob:http://h.example/u", `fetch("/x");`)
	if len(seen) != 1 || seen[0] != "fetch http://h.example/proxy/app/x" {
		t.Errorf("seen = %q, want a single prefixed fetch", seen)
	}
}

func TestWorkerPrelude_StrictDirectiveKept(t *testing.T) {
	r := newContentRewriter("/proxy/app", "", "")
	for _, tc := range []struct{ name, src string }{
		{"single quotes", "'use strict';\nself.onmessage=()=>{};"},
		{"double quotes after comments", "/* banner */\n// more\n\"use strict\";\nx();"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := string(r.prependWorkerPrelude([]byte(tc.src)))
			if !strings.HasPrefix(out, `"use strict";`) {
				t.Errorf("strict worker lost its directive: %q", out[:min(len(out), 60)])
			}
			if !strings.HasSuffix(out, tc.src) {
				t.Error("original source must follow the prelude unchanged")
			}
		})
	}
	out := string(r.prependWorkerPrelude([]byte("self.onmessage=()=>{};")))
	if strings.HasPrefix(out, `"use strict"`) {
		t.Error("sloppy worker must not be made strict")
	}
}

// TestReverseProxy_WorkerScriptGetsPrelude: the proxy prepends the prelude
// only when the browser is loading the script as a worker, and marks the
// response as varying on that so caches keep the two variants apart.
func TestReverseProxy_WorkerScriptGetsPrelude(t *testing.T) {
	const src = "self.onmessage=function(e){fetch('/api/x')};"
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		_, _ = io.WriteString(w, src)
	}))
	defer backend.Close()
	h := NewReverseProxyHandler([]config.AppConfig{{Name: "Dispatcharr", URL: backend.URL, Enabled: true, Proxy: true}}, "30s")
	marker := string(newContentRewriter("/proxy/dispatcharr", "", "").workerPrelude())[:40]

	for _, tc := range []struct {
		dest        string
		wantPrelude bool
	}{
		{"worker", true},
		{"sharedworker", true},
		{"script", false},
		{"", false},
	} {
		t.Run("dest="+tc.dest, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/proxy/dispatcharr/assets/worker.js", nil)
			if tc.dest != "" {
				req.Header.Set("Sec-Fetch-Dest", tc.dest)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			body := rec.Body.String()
			if got := strings.HasPrefix(body, marker); got != tc.wantPrelude {
				t.Errorf("prelude present = %v, want %v; body starts %q", got, tc.wantPrelude, body[:min(len(body), 60)])
			}
			if !strings.HasSuffix(body, src) {
				t.Errorf("worker source must be forwarded intact, got %q", body)
			}
			if !strings.Contains(rec.Header().Get("Vary"), "Sec-Fetch-Dest") {
				t.Errorf("Vary = %q, want it to include Sec-Fetch-Dest", rec.Header().Get("Vary"))
			}
		})
	}
}

// TestReverseProxy_AppPathNamedProxyReachesApp: an app whose own routes live
// under /proxy/ (Dispatcharr streams from /proxy/ts/stream/<uuid>) is reached
// once the request carries the app's prefix; the bare path stays a 404
// because "ts" is not an app.
func TestReverseProxy_AppPathNamedProxyReachesApp(t *testing.T) {
	var gotPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		w.Header().Set("Content-Type", "video/mp2t")
		_, _ = io.WriteString(w, "ts-bytes")
	}))
	defer backend.Close()
	h := NewReverseProxyHandler([]config.AppConfig{{Name: "Dispatcharr", URL: backend.URL, Enabled: true, Proxy: true}}, "30s")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/proxy/dispatcharr/proxy/ts/stream/edb8d92b?output_format=mpegts", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "ts-bytes" {
		t.Fatalf("prefixed stream: code=%d body=%q", rec.Code, rec.Body.String())
	}
	if gotPath != "/proxy/ts/stream/edb8d92b?output_format=mpegts" {
		t.Errorf("backend saw %q, want the app's own /proxy/ts/stream path", gotPath)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/proxy/ts/stream/edb8d92b", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unprefixed stream path: code=%d, want 404", rec.Code)
	}
}
