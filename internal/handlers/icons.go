package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/mescon/muximux/v3/internal/icons"
	"github.com/mescon/muximux/v3/internal/logging"
)

// cgnatNet is RFC 6598 (Carrier-Grade NAT shared address space,
// 100.64.0.0/10). net.IP.IsPrivate does not cover it.
var cgnatNet = &net.IPNet{
	IP:   net.IPv4(100, 64, 0, 0),
	Mask: net.CIDRMask(10, 32),
}

// validateIP returns an error if the IP falls inside any of the address
// classes that must not be reachable from user-supplied URLs (loopback,
// RFC1918, link-local, multicast, unspecified, CGNAT). IPv4-mapped IPv6
// addresses are unwrapped so `::ffff:10.0.0.1` cannot sneak past the
// IsPrivate check by wearing an IPv6 disguise.
func validateIP(ip net.IP) error {
	if ip == nil {
		return errors.New("invalid IP address")
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	switch {
	case ip.IsLoopback(),
		ip.IsPrivate(),
		ip.IsLinkLocalUnicast(),
		ip.IsLinkLocalMulticast(),
		ip.IsUnspecified(),
		ip.IsMulticast(),
		ip.IsInterfaceLocalMulticast():
		return fmt.Errorf("blocked address: %s", ip)
	}
	if ip.To4() != nil && cgnatNet.Contains(ip) {
		return fmt.Errorf("blocked CGNAT address: %s", ip)
	}
	return nil
}

// validateHostSSRF resolves a hostname and rejects any IP that would let a
// user-supplied URL reach internal infrastructure. Kept as a pre-flight
// check so obviously-bad inputs fail fast; the http.Transport's
// DialContext re-validates at connect time to close the DNS TOCTOU gap.
// Defined as a variable so tests can override it for localhost test servers.
var validateHostSSRF = func(hostname string) error {
	ips, err := net.LookupHost(hostname)
	if err != nil {
		return err
	}
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if err := validateIP(ip); err != nil {
			return &net.AddrError{Err: err.Error(), Addr: ipStr}
		}
	}
	return nil
}

// safeSSRFDialContext re-resolves the target host at connect time and
// validates every returned IP. This is the TOCTOU-safe counterpart to the
// pre-flight validateHostSSRF: an attacker-controlled DNS with a short TTL
// can no longer serve a public IP to the validator and a loopback IP to
// the fetcher, because the actual socket uses these validated IPs.
// Exposed as a variable so tests can substitute a trusting dialer when
// exercising the handler against a loopback httptest server.
var safeSSRFDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if err := validateIP(ip); err != nil {
			return nil, err
		}
	}
	// Dial the first validated address directly so the transport doesn't
	// resolve the name again on its own.
	var dialer net.Dialer
	return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
}

// resolveIconContentType chooses the content type for a downloaded icon,
// preferring bytes over the server's declared header. Returns an empty
// string when neither source yields a supported image type, which the
// caller treats as an unsupported-type error.
func resolveIconContentType(data []byte, headerValue string) string {
	detected := http.DetectContentType(data)
	if idx := strings.Index(detected, ";"); idx != -1 {
		detected = strings.TrimSpace(detected[:idx])
	}
	if _, ok := icons.AllowedMimeTypes[detected]; ok {
		return detected
	}

	header := strings.TrimSpace(headerValue)
	if idx := strings.Index(header, ";"); idx != -1 {
		header = strings.TrimSpace(header[:idx])
	}
	if strings.EqualFold(header, "image/svg+xml") && looksLikeSVG(data) {
		return "image/svg+xml"
	}
	return ""
}

// looksLikeSVG returns true when the payload begins with an XML or SVG
// marker, ignoring leading whitespace / UTF-8 BOM. Used to gate the
// `image/svg+xml` server header so a text/html blob cannot put on an
// SVG hat.
func looksLikeSVG(data []byte) bool {
	s := data
	// Strip UTF-8 BOM
	if len(s) >= 3 && s[0] == 0xEF && s[1] == 0xBB && s[2] == 0xBF {
		s = s[3:]
	}
	s = []byte(strings.TrimLeft(string(s), " \t\r\n"))
	lower := strings.ToLower(string(s))
	return strings.HasPrefix(lower, "<?xml") || strings.HasPrefix(lower, "<svg")
}

// fetchWithSSRFSafeRedirects performs at most maxRedirects hops,
// re-validating each intermediate URL (scheme + host) instead of letting
// net/http follow redirects blindly.
func fetchWithSSRFSafeRedirects(client *http.Client, initial string, maxRedirects int) (*http.Response, error) {
	current := initial
	var lastParsed *url.URL
	for i := 0; i <= maxRedirects; i++ {
		parsed, err := url.Parse(current)
		if err != nil {
			return nil, err
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return nil, fmt.Errorf("blocked redirect scheme: %s", parsed.Scheme)
		}
		if err := validateHostSSRF(parsed.Hostname()); err != nil {
			return nil, fmt.Errorf("redirect to blocked host %s: %w", parsed.Hostname(), err)
		}
		resp, err := client.Get(current)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			loc := resp.Header.Get("Location")
			resp.Body.Close()
			if loc == "" {
				return nil, errors.New("redirect without Location header")
			}
			base := parsed
			if lastParsed != nil {
				base = lastParsed
			}
			next, err := base.Parse(loc)
			if err != nil {
				return nil, fmt.Errorf("invalid redirect location: %w", err)
			}
			lastParsed = next
			current = next.String()
			continue
		}
		return resp, nil
	}
	return nil, errors.New("too many redirects")
}

// IconHandler handles icon-related requests
type IconHandler struct {
	dashboardClient *icons.DashboardIconsClient
	lucideClient    *icons.LucideClient
	customManager   *icons.CustomIconsManager
}

// NewIconHandler creates a new icon handler
func NewIconHandler(dashboardClient *icons.DashboardIconsClient, lucideClient *icons.LucideClient, customIconsDir string) *IconHandler {
	return &IconHandler{
		dashboardClient: dashboardClient,
		lucideClient:    lucideClient,
		customManager:   icons.NewCustomIconsManager(customIconsDir),
	}
}

// GetDashboardIcon serves a dashboard icon
func (h *IconHandler) GetDashboardIcon(w http.ResponseWriter, r *http.Request) {
	// Extract icon name from path: /api/icons/dashboard/{name}.{ext}
	path := strings.TrimPrefix(r.URL.Path, "/api/icons/dashboard/")
	if path == "" {
		respondError(w, r, http.StatusBadRequest, errIconNameRequired)
		return
	}

	// Parse name and variant from extension or query param
	name := path
	variant := r.URL.Query().Get("variant")
	if variant == "" {
		ext := filepath.Ext(name)
		if ext != "" {
			variant = strings.TrimPrefix(ext, ".")
			name = strings.TrimSuffix(name, ext)
		} else {
			variant = "svg"
		}
	}

	// Get the icon (falls back through svg → webp → png)
	data, contentType, err := h.dashboardClient.GetIcon(name, variant)
	if err != nil {
		respondError(w, r, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set(headerContentType, contentType)
	w.Header().Set(headerCacheControl, cachePublic24h)
	w.Write(data)
}

// ListDashboardIcons returns a list of available dashboard icons
func (h *IconHandler) ListDashboardIcons(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	var iconList []icons.IconInfo
	var err error

	if query != "" {
		iconList, err = h.dashboardClient.SearchIcons(query)
	} else {
		iconList, err = h.dashboardClient.ListIcons()
	}

	if err != nil {
		respondError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	sendJSON(w, http.StatusOK, iconList)
}

// ListLucideIcons returns a list of available Lucide icons with optional search
func (h *IconHandler) ListLucideIcons(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	var iconList []icons.LucideIconInfo
	var err error

	if query != "" {
		iconList, err = h.lucideClient.SearchIcons(query)
	} else {
		iconList, err = h.lucideClient.ListIcons()
	}

	if err != nil {
		respondError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	sendJSON(w, http.StatusOK, iconList)
}

// GetLucideIcon serves a single Lucide icon by name
func (h *IconHandler) GetLucideIcon(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/icons/lucide/")
	if path == "" {
		respondError(w, r, http.StatusBadRequest, errIconNameRequired)
		return
	}

	data, contentType, err := h.lucideClient.GetIcon(path)
	if err != nil {
		respondError(w, r, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set(headerContentType, contentType)
	w.Header().Set(headerCacheControl, cachePublic24h)
	w.Write(data)
}

// ServeIcon serves an icon based on type (dashboard, lucide, or custom)
func (h *IconHandler) ServeIcon(w http.ResponseWriter, r *http.Request) {
	// Path format: /icons/{type}/{name}
	path := strings.TrimPrefix(r.URL.Path, "/icons/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		respondError(w, r, http.StatusBadRequest, "Invalid icon path")
		return
	}

	iconType := parts[0]
	iconName := parts[1]

	switch iconType {
	case "dashboard":
		variant := r.URL.Query().Get("variant")
		if variant == "" {
			// Try to determine from extension
			ext := filepath.Ext(iconName)
			if ext != "" {
				variant = strings.TrimPrefix(ext, ".")
				iconName = strings.TrimSuffix(iconName, ext)
			} else {
				variant = "svg"
			}
		}

		data, contentType, err := h.dashboardClient.GetIcon(iconName, variant)
		if err != nil {
			respondError(w, r, http.StatusNotFound, err.Error())
			return
		}

		w.Header().Set(headerContentType, contentType)
		w.Header().Set(headerCacheControl, cachePublic24h)
		w.Write(data)

	case "custom":
		// Serve from custom icons directory. Custom icons can be SVGs,
		// and SVG can contain <script> / onload handlers. Loading an SVG
		// as an <img> is safe, but navigating directly to the icon URL
		// would render it with full script privileges on Muximux's
		// origin (findings.md H3). Harden the response so a direct
		// navigation is neutered: download-as-attachment (which the
		// browser ignores for <img>), no script/resource loads allowed
		// by CSP, and no MIME sniffing.
		data, contentType, err := h.customManager.GetIcon(iconName)
		if err != nil {
			respondError(w, r, http.StatusNotFound, err.Error())
			return
		}

		w.Header().Set(headerContentType, contentType)
		w.Header().Set(headerCacheControl, cachePublic24h)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
		// Quote the filename so slashes/quotes in iconName cannot
		// smuggle header content. iconName has already been validated
		// by the customManager path check, but defence in depth.
		safeName := strings.ReplaceAll(iconName, "\"", "")
		w.Header().Set("Content-Disposition", `attachment; filename="`+safeName+`"`)
		w.Write(data)

	case "lucide":
		// Serve from Lucide CDN (cached locally)
		name := strings.TrimSuffix(iconName, filepath.Ext(iconName))
		data, contentType, err := h.lucideClient.GetIcon(name)
		if err != nil {
			respondError(w, r, http.StatusNotFound, err.Error())
			return
		}

		w.Header().Set(headerContentType, contentType)
		w.Header().Set(headerCacheControl, cachePublic24h)
		w.Write(data)

	default:
		respondError(w, r, http.StatusBadRequest, "Unknown icon type")
	}
}

// ListCustomIcons returns a list of custom uploaded icons
func (h *IconHandler) ListCustomIcons(w http.ResponseWriter, r *http.Request) {
	iconList, err := h.customManager.ListIcons()
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	sendJSON(w, http.StatusOK, iconList)
}

// UploadCustomIcon handles custom icon file uploads
func (h *IconHandler) UploadCustomIcon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, icons.MaxIconSize+1024) // Extra for form overhead

	// Parse multipart form
	if err := r.ParseMultipartForm(icons.MaxIconSize); err != nil {
		respondError(w, r, http.StatusBadRequest, "File too large or invalid form")
		return
	}

	// Get the file
	file, header, err := r.FormFile("icon")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "No icon file provided")
		return
	}
	defer file.Close()

	// Get icon name (from form or filename)
	name := r.FormValue("name")
	if name == "" {
		name = strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	}

	// Read file content
	data, err := io.ReadAll(file)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "Failed to read file")
		return
	}

	// Determine content type
	contentType := header.Header.Get(headerContentType)
	if contentType == "" || contentType == "application/octet-stream" {
		// Detect from file content
		contentType = http.DetectContentType(data)
	}

	// Save the icon
	if err := h.customManager.SaveIcon(name, data, contentType); err != nil {
		respondError(w, r, http.StatusBadRequest, err.Error(), "source", "icons", "name", name, "error", err)
		return
	}

	logging.From(r.Context()).Info("Custom icon uploaded", "source", "icons", "name", name, "size", len(data))
	sendJSON(w, http.StatusOK, map[string]string{
		"name":   name,
		"status": "uploaded",
	})
}

// fetchIconRequest is the JSON body for POST /api/icons/custom/fetch
type fetchIconRequest struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

// FetchCustomIcon downloads an icon from a URL and saves it as a custom icon
func (h *IconHandler) FetchCustomIcon(w http.ResponseWriter, r *http.Request) {
	var req fetchIconRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidBody)
		return
	}

	if req.URL == "" {
		respondError(w, r, http.StatusBadRequest, "URL is required")
		return
	}

	// Parse and validate URL scheme
	parsed, err := url.Parse(req.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		respondError(w, r, http.StatusBadRequest, "Invalid URL: must be http or https")
		return
	}

	// SSRF protection: resolve hostname and reject private/internal IPs.
	// Distinguish a DNS failure (the hostname cannot be resolved at all)
	// from a resolved-but-blocked IP so an operator looking at the
	// audit log can tell "attacker sending bogus names" from "attacker
	// pointing at internal infrastructure" (findings.md M1).
	if err := validateHostSSRF(parsed.Hostname()); err != nil {
		if _, ok := err.(*net.AddrError); ok {
			respondError(w, r, http.StatusBadRequest, "URL must not point to a private or internal address",
				"source", "audit", "host", parsed.Hostname(), "reason", "blocked_ip")
		} else {
			respondError(w, r, http.StatusBadRequest, "Could not resolve hostname",
				"source", "audit", "host", parsed.Hostname(), "reason", "dns_error", "error", err)
		}
		return
	}

	// Download via a transport that re-validates every resolved IP at
	// connect time and a redirect chain that re-checks each hop's host
	// against the SSRF allow-list (findings.md C8).
	sanitizedURL := parsed.String()
	if !strings.HasPrefix(sanitizedURL, "http://") && !strings.HasPrefix(sanitizedURL, "https://") {
		respondError(w, r, http.StatusBadRequest, "Invalid URL scheme")
		return
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: safeSSRFDialContext,
		},
		// Disable built-in redirect following; each hop is validated by
		// fetchWithSSRFSafeRedirects before we issue the next request.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := fetchWithSSRFSafeRedirects(client, sanitizedURL, 5) //nolint:bodyclose // handled below
	if err != nil {
		respondError(w, r, http.StatusBadGateway, "Failed to download icon", "source", "icons", "url", sanitizedURL, "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respondError(w, r, http.StatusBadGateway, "Remote server returned "+resp.Status)
		return
	}

	// Read with size limit
	data, err := io.ReadAll(io.LimitReader(resp.Body, icons.MaxIconSize+1))
	if err != nil {
		respondError(w, r, http.StatusBadGateway, "Failed to read response")
		return
	}
	if len(data) > icons.MaxIconSize {
		respondError(w, r, http.StatusBadRequest, "File too large: max size is 2MB")
		return
	}

	// Content-type decision (findings.md L9): sniff the bytes ourselves.
	// http.DetectContentType reliably identifies PNG / JPEG / GIF / WEBP
	// / ICO, so if sniffing lands on one of the allowed image types that
	// is what we use, regardless of what the server claims. SVG is a
	// special case because sniffing reports it as text/xml: we accept the
	// server's `image/svg+xml` claim only after verifying the bytes
	// actually start with an SVG or XML marker, which stops a server
	// from declaring `image/svg+xml` for an HTML / JS payload.
	contentType := resolveIconContentType(data, resp.Header.Get(headerContentType))

	// Validate MIME type
	if _, ok := icons.AllowedMimeTypes[contentType]; !ok {
		respondError(w, r, http.StatusBadRequest, "Unsupported file type: "+contentType)
		return
	}

	// Derive name from URL filename if not provided
	name := req.Name
	if name == "" {
		name = filepath.Base(parsed.Path)
		name = strings.TrimSuffix(name, filepath.Ext(name))
		if name == "" || name == "." {
			name = "fetched-icon"
		}
	}

	// Save the icon (reuses same validation as file upload)
	if err := h.customManager.SaveIcon(name, data, contentType); err != nil {
		respondError(w, r, http.StatusBadRequest, err.Error(), "source", "icons", "name", name, "error", err)
		return
	}

	logging.From(r.Context()).Info("Custom icon fetched from URL", "source", "icons", "name", name, "url", req.URL, "size", len(data))
	sendJSON(w, http.StatusOK, map[string]string{
		"name":   name,
		"status": "uploaded",
	})
}

// DeleteCustomIcon handles custom icon deletion
func (h *IconHandler) DeleteCustomIcon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	// Extract icon name from path: /api/icons/custom/{name}
	path := strings.TrimPrefix(r.URL.Path, "/api/icons/custom/")
	if path == "" {
		respondError(w, r, http.StatusBadRequest, errIconNameRequired)
		return
	}

	if err := h.customManager.DeleteIcon(path); err != nil {
		respondError(w, r, http.StatusNotFound, err.Error())
		return
	}

	logging.From(r.Context()).Info("Custom icon deleted", "source", "icons", "name", path)
	sendJSON(w, http.StatusOK, map[string]string{
		"status": "deleted",
	})
}
