package server

import (
	"bytes"
	"compress/gzip"
	"io/fs"
	"path"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"

	"github.com/mescon/muximux/v3/internal/logging"
)

// precompressedAsset holds the on-startup-computed brotli + gzip
// variants of a single embedded static asset, alongside the
// original bytes for clients that don't accept either encoding.
//
// All three byte slices are immutable after build and shared
// across goroutines; the server hands out aliasing references via
// bytes.NewReader, never the slices themselves.
type precompressedAsset struct {
	contentType string
	original    []byte
	gzip        []byte // gzip-encoded, default level
	brotli      []byte // brotli-encoded, quality 6
}

// precompressedAssets is a path -> variant lookup populated at
// server startup from the embedded SPA dist. Read-only after
// init; safe to consult concurrently without locking.
//
// Paths are stored canonical-style with a leading "/" so they
// match http.Request.URL.Path directly (e.g.
// "/assets/index-DEBuH9mI.js").
type precompressedAssets struct {
	byPath map[string]*precompressedAsset
}

// brotliQuality chooses a static-asset compression quality. 6 is
// the practical sweet spot: noticeably smaller payloads than
// gzip but encoding stays fast enough that the startup hit on
// the full SPA bundle is under a second on a typical homelab
// machine. Levels 10-11 squeeze another few percent at the cost
// of multi-second encodes.
const brotliQuality = 6

// minPrecompressSize skips files small enough that compression
// overhead (headers, dictionary) would erase any size win. Below
// ~150 bytes the gzip / brotli output is often larger than the
// input.
const minPrecompressSize = 256

// compressibleExtensions enumerates the MIME-mappable file types
// where text compression actually wins. PNG, JPEG, WOFF2, etc.
// are already-compressed binary formats; re-compressing them
// costs CPU at boot and doesn't shrink them.
var compressibleExtensions = map[string]string{
	".html": "text/html; charset=utf-8",
	".css":  "text/css; charset=utf-8",
	".js":   "application/javascript; charset=utf-8",
	".mjs":  "application/javascript; charset=utf-8",
	".json": "application/json; charset=utf-8",
	".xml":  "application/xml; charset=utf-8",
	".svg":  "image/svg+xml",
	".txt":  "text/plain; charset=utf-8",
	".map":  "application/json; charset=utf-8",
}

// buildPrecompressedAssets walks the embedded SPA dist FS and
// produces brotli + gzip variants for every compressible file.
// Called once at server startup; the resulting cache is read-only
// for the lifetime of the process.
//
// Failures during the walk are non-fatal: a per-file error logs
// at Warn and the file is omitted from the cache. The handler
// then falls back to serving the original from disk on that path
// so a partial cache never breaks the SPA.
func buildPrecompressedAssets(fsys fs.FS) *precompressedAssets {
	out := &precompressedAssets{byPath: make(map[string]*precompressedAsset, 32)}

	var (
		mu        sync.Mutex
		wg        sync.WaitGroup
		jobs      = make(chan string, 16)
		totalIn   int64
		totalGZ   int64
		totalBR   int64
		fileCount int
	)

	// Workers compress files in parallel. The walk is fast (FS
	// iteration); brotli at quality 6 on a 1.5 MB file takes ~50 ms
	// single-threaded. Four workers covers most desktop CPUs
	// without becoming the dominant startup cost.
	const workerCount = 4
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				if asset, ok := compressOne(fsys, p); ok {
					mu.Lock()
					out.byPath["/"+p] = asset
					totalIn += int64(len(asset.original))
					totalGZ += int64(len(asset.gzip))
					totalBR += int64(len(asset.brotli))
					fileCount++
					mu.Unlock()
				}
			}
		}()
	}

	_ = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		// Skip directories and per-entry walk errors. We intentionally
		// swallow err here: a stat failure on one embedded file means
		// "no compressed variant cached for this path", which the
		// runtime SPA handler degrades to serving the raw bytes from
		// the file server. Propagating the error would abort the walk
		// and lose every later compressible file too.
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // intentional: per-entry skip, not a propagated failure
		}
		ext := strings.ToLower(path.Ext(p))
		if _, ok := compressibleExtensions[ext]; !ok {
			return nil
		}
		jobs <- p
		return nil
	})
	close(jobs)
	wg.Wait()

	if fileCount > 0 {
		logging.Info("Pre-compressed static assets",
			"source", "server",
			"files", fileCount,
			"raw_bytes", totalIn,
			"gzip_bytes", totalGZ,
			"brotli_bytes", totalBR,
		)
	}
	return out
}

// compressOne reads a single file and produces both gzip and
// brotli variants. Files below minPrecompressSize are skipped
// (compression overhead exceeds savings). Returns (nil, false)
// for skip / error; the caller then falls through to the
// uncompressed file server.
func compressOne(fsys fs.FS, p string) (*precompressedAsset, bool) {
	data, err := fs.ReadFile(fsys, p)
	if err != nil {
		logging.Warn("Pre-compress read failed; serving raw",
			"source", "server", "path", p, "error", err.Error())
		return nil, false
	}
	if len(data) < minPrecompressSize {
		return nil, false
	}

	gzData, err := encodeGzip(data)
	if err != nil {
		logging.Warn("Pre-compress gzip failed; serving raw",
			"source", "server", "path", p, "error", err.Error())
		return nil, false
	}
	brData, err := encodeBrotli(data)
	if err != nil {
		logging.Warn("Pre-compress brotli failed; serving raw",
			"source", "server", "path", p, "error", err.Error())
		return nil, false
	}

	ext := strings.ToLower(path.Ext(p))
	return &precompressedAsset{
		contentType: compressibleExtensions[ext],
		original:    data,
		gzip:        gzData,
		brotli:      brData,
	}, true
}

func encodeGzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.DefaultCompression)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeBrotli(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := brotli.NewWriterLevel(&buf, brotliQuality)
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// pickEncoding parses the client's Accept-Encoding header and
// returns the best supported encoding our cache holds. Returns
// ("br" | "gzip" | "") - empty means the client either didn't
// list a supported encoding or explicitly disallowed all of them.
//
// Implementation is intentionally simple: we only check for the
// presence of the encoding token, not the q-value preference
// order. Brotli is preferred over gzip when both are listed.
func pickEncoding(acceptEncoding string) string {
	if acceptEncoding == "" {
		return ""
	}
	ae := strings.ToLower(acceptEncoding)
	if containsEncodingToken(ae, "br") {
		return "br"
	}
	if containsEncodingToken(ae, "gzip") {
		return "gzip"
	}
	return ""
}

// containsEncodingToken does a tokenized lookup so "gzip" doesn't
// false-match on "gzipfoobar" or get blocked by a "gzip;q=0".
// The token-with-q=0 case (explicit disable, per RFC 7231 §5.3.4)
// is uncommon but worth honouring; we treat any "<token>;q=0" as
// absent. Be careful with the q-value check: "q=0.9" is non-zero
// and means "still acceptable, just lower priority" - we must
// not confuse it with "q=0" (disabled).
func containsEncodingToken(header, token string) bool {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		name := part
		params := ""
		if i := strings.IndexByte(part, ';'); i >= 0 {
			name = strings.TrimSpace(part[:i])
			params = part[i:]
		}
		if name == token && !hasZeroQValue(params) {
			return true
		}
	}
	return false
}

// hasZeroQValue returns true when the parameter string disables
// the encoding via a literal q=0 (with optional .0/.00/.000
// trailing zeros) - distinct from non-zero q values like q=0.5
// or q=0.9 which keep the encoding usable.
func hasZeroQValue(params string) bool {
	idx := strings.Index(params, "q=")
	if idx < 0 {
		return false
	}
	rest := params[idx+2:]
	// Skip the leading "0".
	if !strings.HasPrefix(rest, "0") {
		return false
	}
	rest = rest[1:]
	// Allow an optional ".0", ".00", ".000" tail.
	if strings.HasPrefix(rest, ".") {
		rest = rest[1:]
		for i := 0; i < len(rest); i++ {
			if rest[i] != '0' {
				// Any non-zero digit (or anything else) after the
				// decimal point means a non-zero weight.
				if rest[i] >= '1' && rest[i] <= '9' {
					return false
				}
				// End of the q-value (followed by ; or whitespace etc.).
				break
			}
		}
	}
	return true
}
