package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/andybalholm/brotli"
)

// TestPickEncoding_PreferenceOrder pins the priority: brotli wins
// when both br and gzip are listed, gzip wins when only gzip is
// listed, empty/unrelated headers return empty string.
func TestPickEncoding_PreferenceOrder(t *testing.T) {
	cases := []struct {
		header string
		want   string
	}{
		// Both present -> brotli wins.
		{"br, gzip", "br"},
		{"gzip, br", "br"},
		{"gzip, deflate, br", "br"},
		// Only gzip -> gzip.
		{"gzip", "gzip"},
		{"gzip, deflate", "gzip"},
		{"deflate, gzip;q=0.9", "gzip"},
		// Only br -> br.
		{"br", "br"},
		// Neither -> empty.
		{"", ""},
		{"deflate", ""},
		{"identity", ""},
		// q=0 is an explicit disable per RFC 7231 §5.3.4. Treat as
		// absent so we don't surprise a client that opted out.
		{"gzip;q=0", ""},
		{"br;q=0, gzip", "gzip"},
		// Token must be exact - "gzipfoo" must not match "gzip".
		{"gzipfoo", ""},
		// Case insensitivity.
		{"GZIP", "gzip"},
		{"BR", "br"},
	}
	for _, c := range cases {
		if got := pickEncoding(c.header); got != c.want {
			t.Errorf("pickEncoding(%q) = %q, want %q", c.header, got, c.want)
		}
	}
}

// TestBuildPrecompressedAssets_HappyPath: a synthetic FS with one
// large JS file and one too-small file. The JS should land in the
// cache with both gzip and brotli variants smaller than the
// original; the tiny file should be skipped (compression overhead
// would inflate it).
func TestBuildPrecompressedAssets_HappyPath(t *testing.T) {
	largeJS := strings.Repeat("const compressMe = 'this string repeats so it compresses well'; ", 200)
	tinyFile := "x"
	fsys := fstest.MapFS{
		"assets/index-DEBuH9mI.js":  {Data: []byte(largeJS)},
		"assets/tiny-AAAAAAAA.css":  {Data: []byte(tinyFile)},
		"favicon.ico":               {Data: []byte("ICON-BYTES")}, // not compressible (extension not in map)
		"assets/messages-FOO.js":    {Data: []byte(largeJS + largeJS)},
		"assets/already-binary.png": {Data: []byte("PNG-DATA")}, // not compressible
	}

	cache := buildPrecompressedAssets(fsys)

	// Compressible: two .js files.
	jsKey := "/assets/index-DEBuH9mI.js"
	a, ok := cache.byPath[jsKey]
	if !ok || a == nil {
		t.Fatalf("missing entry for %s", jsKey)
	}
	if a.contentType != "application/javascript; charset=utf-8" {
		t.Errorf("contentType for .js = %q, want javascript", a.contentType)
	}
	if len(a.gzip) >= len(a.original) {
		t.Errorf("gzip didn't shrink: orig=%d gzip=%d", len(a.original), len(a.gzip))
	}
	if len(a.brotli) >= len(a.original) {
		t.Errorf("brotli didn't shrink: orig=%d brotli=%d", len(a.original), len(a.brotli))
	}
	// Brotli typically beats gzip on text. Pin that here so a
	// future quality-knob change that flips the ratio is loud.
	if len(a.brotli) >= len(a.gzip) {
		t.Errorf("brotli >= gzip on highly-compressible JS (brotli=%d gzip=%d) - quality knob regression?",
			len(a.brotli), len(a.gzip))
	}

	// Non-compressible extension (.png) must NOT be in the cache.
	if _, ok := cache.byPath["/assets/already-binary.png"]; ok {
		t.Errorf(".png entry leaked into precompress cache; we'd waste CPU re-encoding already-compressed bytes")
	}

	// Tiny file (1 byte) is below minPrecompressSize and must NOT
	// be cached - compression overhead would inflate it.
	if _, ok := cache.byPath["/assets/tiny-AAAAAAAA.css"]; ok {
		t.Errorf("tiny .css cached despite being under minPrecompressSize (overhead > savings)")
	}
}

// TestSelectVariant_ServesBrotliWhenSupported verifies the end-
// to-end selection path: a request with "Accept-Encoding: br,
// gzip" gets back brotli bytes with Content-Encoding: br and a
// matching Content-Length.
func TestSelectVariant_ServesBrotliWhenSupported(t *testing.T) {
	original := []byte(strings.Repeat("hello compressible world ", 50))
	a := buildSingleAsset(original, "text/html; charset=utf-8")
	if a.brotli == nil || a.gzip == nil {
		t.Fatalf("setup error: brotli or gzip missing on built asset")
	}

	h := http.Header{}
	body := selectVariant(a, "br, gzip", h)
	if !bytes.Equal(body, a.brotli) {
		t.Errorf("body != brotli variant")
	}
	if h.Get("Content-Encoding") != "br" {
		t.Errorf("Content-Encoding = %q, want %q", h.Get("Content-Encoding"), "br")
	}
	if h.Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", h.Get("Content-Type"))
	}
	// Decode the body to confirm the bytes are actually valid
	// brotli, not just the raw payload mislabeled.
	r := brotli.NewReader(bytes.NewReader(body))
	decoded, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("brotli decode failed: %v", err)
	}
	if !bytes.Equal(decoded, original) {
		t.Errorf("brotli round-trip mismatch: got %d bytes, want %d", len(decoded), len(original))
	}
}

// TestSelectVariant_ServesGzipWhenBrotliNotAccepted: client lists
// only gzip; we must fall back to gzip rather than serving raw.
func TestSelectVariant_ServesGzipWhenBrotliNotAccepted(t *testing.T) {
	original := []byte(strings.Repeat("hello compressible world ", 50))
	a := buildSingleAsset(original, "text/css; charset=utf-8")

	h := http.Header{}
	body := selectVariant(a, "gzip, deflate", h)
	if !bytes.Equal(body, a.gzip) {
		t.Errorf("body != gzip variant")
	}
	if h.Get("Content-Encoding") != "gzip" {
		t.Errorf("Content-Encoding = %q, want %q", h.Get("Content-Encoding"), "gzip")
	}
	// Round-trip gzip too.
	r, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	decoded, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("gzip decode: %v", err)
	}
	if !bytes.Equal(decoded, original) {
		t.Errorf("gzip round-trip mismatch")
	}
}

// TestSelectVariant_FallsBackToOriginalWhenClientAcceptsNothing
// makes sure we don't accidentally serve a compressed body to a
// client that didn't ask for it - that would produce garbage on
// the wire and break the page silently.
func TestSelectVariant_FallsBackToOriginalWhenClientAcceptsNothing(t *testing.T) {
	original := []byte(strings.Repeat("hello compressible world ", 50))
	a := buildSingleAsset(original, "text/html; charset=utf-8")

	h := http.Header{}
	body := selectVariant(a, "", h)
	if !bytes.Equal(body, a.original) {
		t.Errorf("body should be original when no Accept-Encoding")
	}
	if h.Get("Content-Encoding") != "" {
		t.Errorf("Content-Encoding should be unset, got %q", h.Get("Content-Encoding"))
	}
	if h.Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("Content-Type still set: %q", h.Get("Content-Type"))
	}
}
