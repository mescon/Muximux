package server

import (
	"encoding/json"
	"net/http"

	"github.com/mescon/muximux/v3/internal/logging"
)

const (
	errMethodNotAllowed = "Method not allowed"
	proxyPathPrefix     = "/proxy/"
	headerContentType   = "Content-Type"
	contentTypeJSON     = "application/json"
	apiThemesPath       = "/api/themes"
	errInvalidBody      = "Invalid request body"
	// sessionCookieName is the cookie that stores the Muximux session ID.
	// Exposed as a constant so the reverse proxy can strip it from outgoing
	// requests (preventing the session from leaking to proxied backends).
	sessionCookieName = "muximux_session"
)

// setJSONContentType sets the Content-Type header to application/json.
func setJSONContentType(w http.ResponseWriter) {
	w.Header().Set(headerContentType, contentTypeJSON)
}

// writeJSON sets Content-Type, writes status, and encodes data as JSON.
// Encode errors are logged at warning level (findings.md H8); the
// response status and headers are already committed by the time we
// reach the encoder so nothing can be recovered for the client, but an
// invisible truncation no longer becomes a silent audit gap.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	setJSONContentType(w)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logging.Warn("Failed to write JSON response", "source", "server", "status", status, "error", err)
	}
}
