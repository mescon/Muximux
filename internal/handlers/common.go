package handlers

const (
	headerContentType   = "Content-Type"
	headerCacheControl  = "Cache-Control"
	contentTypeJSON     = "application/json"
	cachePublic24h      = "public, max-age=86400"
	errMethodNotAllowed = "Method not allowed"
	errFailedSaveConfig = "Failed to save configuration"
	errAppNotFound      = "App not found"
	errGroupNotFound    = "Group not found"
	errIconNameRequired = "Icon name required"
	proxyPathPrefix     = "/proxy/"
	headerSetCookie     = "Set-Cookie"
	headerXForwardedFor = "X-Forwarded-For"
	errBadGateway       = "Bad Gateway"
)
