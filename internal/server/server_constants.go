package server

import "net/http"

const (
	errMethodNotAllowed = "Method not allowed"
	proxyPathPrefix     = "/proxy/"
	headerContentType   = "Content-Type"
	contentTypeJSON     = "application/json"
	apiThemesPath       = "/api/themes"
	errInvalidBody      = "Invalid request body"
)

// setJSONContentType sets the Content-Type header to application/json.
func setJSONContentType(w http.ResponseWriter) {
	w.Header().Set(headerContentType, contentTypeJSON)
}
