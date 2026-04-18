package server

import "net/http"

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
