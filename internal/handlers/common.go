package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/mescon/muximux/v3/internal/logging"
)

// writeFileAtomic writes data to filename atomically: write to a temp file
// in the same directory, then rename over the target. An abrupt exit
// (crash, kill, power loss) mid-write leaves either the old file intact
// or the new file complete, never a truncated/corrupt one. Used for
// state files whose consumers glob a directory and would otherwise
// surface a partial write as a valid entry.
func writeFileAtomic(filename string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(filename)
	tmp, err := os.CreateTemp(dir, filepath.Base(filename)+".tmp.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpName, filename); err != nil {
		cleanup()
		return err
	}
	return nil
}

// sendJSON writes a JSON response with the given status code.
func sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set(headerContentType, contentTypeJSON)
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

const (
	headerContentType     = "Content-Type"
	headerContentEncoding = "Content-Encoding"
	headerCacheControl    = "Cache-Control"
	contentTypeJSON       = "application/json"
	cachePublic24h        = "public, max-age=86400"
	errMethodNotAllowed   = "Method not allowed"
	errFailedSaveConfig   = "Failed to save configuration"
	errAppNotFound        = "App not found"
	errGroupNotFound      = "Group not found"
	errIconNameRequired   = "Icon name required"
	errInvalidBody        = "Invalid request body"
	errInvalidJSON        = "Invalid JSON: "
	errUserNotFound       = "User not found"
	proxyPathPrefix       = "/proxy/"
	headerSetCookie       = "Set-Cookie"
	headerXForwardedFor   = "X-Forwarded-For"
	errBadGateway         = "Bad Gateway"
)

// respondError sends an HTTP error response and logs at the appropriate level.
// 5xx → Error, 401/403 → Warn, other 4xx → Debug.
// Optional attrs are appended as slog-style key-value pairs for structured context.
func respondError(w http.ResponseWriter, r *http.Request, status int, msg string, attrs ...any) {
	all := make([]any, 0, 4+len(attrs))
	all = append(all, "status", status, "response", msg)
	all = append(all, attrs...)

	switch {
	case status >= 500:
		logging.From(r.Context()).Error("HTTP error", all...)
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		logging.From(r.Context()).Warn("HTTP error", all...)
	default:
		logging.From(r.Context()).Debug("HTTP error", all...)
	}

	http.Error(w, msg, status)
}
