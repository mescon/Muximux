package handlers

const (
	headerContentType    = "Content-Type"
	headerContentEncoding = "Content-Encoding"
	headerCacheControl   = "Cache-Control"
	contentTypeJSON      = "application/json"
	cachePublic24h       = "public, max-age=86400"
	errMethodNotAllowed  = "Method not allowed"
	errFailedSaveConfig  = "Failed to save configuration"
	errAppNotFound       = "App not found"
	errGroupNotFound     = "Group not found"
	errIconNameRequired  = "Icon name required"
	errInvalidBody       = "Invalid request body"
	errInvalidJSON       = "Invalid JSON: "
	errUserNotFound      = "User not found"
	proxyPathPrefix      = "/proxy/"
	headerSetCookie      = "Set-Cookie"
	headerXForwardedFor  = "X-Forwarded-For"
	errBadGateway        = "Bad Gateway"
)
