package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/mescon/muximux/v3/internal/logging"
)

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
