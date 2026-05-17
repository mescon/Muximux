package discovery

import (
	"fmt"
	"strings"

	"github.com/mescon/muximux/v3/internal/config"
)

// Confidence rates how trustworthy a Suggestion's auto-filled
// fields are. The frontend renders a coloured chip per row so
// operators can scan a batch and focus review on the low-
// confidence rows.
type Confidence string

const (
	// ConfidenceHigh: operator-supplied muximux.app.* labels on
	// the container drove every field. No guessing.
	ConfidenceHigh Confidence = "high"
	// ConfidenceMedium: the image matches a catalog entry, so
	// name/icon/port came from a curated source. Still worth a
	// glance before importing.
	ConfidenceMedium Confidence = "medium"
	// ConfidenceLow: no labels, no catalog match - all fields
	// fell through to heuristics (titleized container name,
	// first exposed port). Operator review strongly recommended.
	ConfidenceLow Confidence = "low"
)

// Suggestion is one row in the Discover modal. The frontend treats
// these as the canonical input to the import flow - the operator
// edits inline before submitting, but every field has a sensible
// default already.
type Suggestion struct {
	// Tracking
	Key       string    `json:"key"`       // "label:foo" | "name:bar" | "id:..."
	Stability Stability `json:"stability"` // see Stability constants

	// Display
	Name      string `json:"name"`
	Icon      string `json:"icon,omitempty"`
	Group     string `json:"group,omitempty"`
	URL       string `json:"url"` // ready-to-use URL
	HealthURL string `json:"health_url,omitempty"`

	// Network
	EffectiveStrategy config.NetworkStrategy `json:"effective_strategy"` // strategy used to build URL

	// Diagnostic / display
	ContainerID   string     `json:"container_id"`
	ContainerName string     `json:"container_name,omitempty"`
	ImageRef      string     `json:"image_ref"`
	Confidence    Confidence `json:"confidence"`               // see Confidence constants
	RequiresInput bool       `json:"requires_input,omitempty"` // true when scan can't pick a port etc.
	Notes         []string   `json:"notes,omitempty"`

	// Label-derived App fields. Populated when the container has the
	// matching muximux.app.* labels set. Frontend copies these through
	// to the ClientAppConfig on import so operators get a fully
	// configured app from labels alone.
	Color              string   `json:"color,omitempty"`
	Order              int      `json:"order,omitempty"`
	OpenMode           string   `json:"open_mode,omitempty"`
	Proxy              *bool    `json:"proxy,omitempty"`
	ProxySkipTLSVerify *bool    `json:"proxy_skip_tls_verify,omitempty"`
	MinRole            string   `json:"min_role,omitempty"`
	AllowedGroups      []string `json:"allowed_groups,omitempty"`
	Permissions        []string `json:"permissions,omitempty"`
	AllowNotifications *bool    `json:"allow_notifications,omitempty"`
	Default            *bool    `json:"default,omitempty"`
	Shortcut           int      `json:"shortcut,omitempty"`

	// Suggested gateway-site fields, used when the modal's
	// "Add gateway site" toggle is on. SuggestedDomain comes from
	// muximux.app.gateway.domain; the rest come from the
	// muximux.gateway.* namespace.
	SuggestedDomain  string                  `json:"suggested_domain,omitempty"`
	SuggestedGateway *SuggestedGatewayConfig `json:"suggested_gateway,omitempty"`
}

// SuggestedGatewayConfig carries muximux.gateway.* label values
// through the scan/import flow. Present only when at least one
// gateway-namespace label is set on the container.
type SuggestedGatewayConfig struct {
	TLS                string   `json:"tls,omitempty"` // auto | none | custom
	Streaming          *bool    `json:"streaming,omitempty"`
	StripFrameBlockers *bool    `json:"strip_frame_blockers,omitempty"`
	ForwardedHeaders   *bool    `json:"forwarded_headers,omitempty"`
	RequireAuth        *bool    `json:"require_auth,omitempty"`
	MinRole            string   `json:"min_role,omitempty"`
	AllowedGroups      []string `json:"allowed_groups,omitempty"`
}

// suggestForContainer builds a Suggestion for one container by
// combining catalog match (medium confidence), label overrides (high
// confidence), and fallback heuristics (low confidence).
//
// globalStrategy + hostIP come from the discovery config; they're
// passed in rather than read from the Service so this function stays
// pure and easy to test.
func suggestForContainer(c *ContainerSummary, globalStrategy config.NetworkStrategy, hostIP, dashboardDomain string) Suggestion {
	labels := ParseAppLabels(c.Labels)
	catalog, hasCatalog := MatchImage(c.Image)
	key, stability := KeyForContainer(c)

	s := Suggestion{
		Key:           key,
		Stability:     stability,
		ContainerID:   c.ID,
		ContainerName: c.PrimaryName(),
		ImageRef:      c.Image,
		Confidence:    ConfidenceLow,
		Notes:         []string{},
	}

	// Resolve fields with priority: label > catalog > heuristic.
	switch {
	case labels.Name != "":
		s.Name = labels.Name
		s.Confidence = ConfidenceHigh
		s.Notes = append(s.Notes, "Name from label muximux.app.name")
	case hasCatalog && catalog.Name != "":
		s.Name = catalog.Name
		s.Confidence = ConfidenceMedium
		s.Notes = append(s.Notes, fmt.Sprintf("Name suggested from catalog: %s", catalog.Image))
	default:
		s.Name = titleizeName(c.PrimaryName())
	}

	switch {
	case labels.Icon != "":
		s.Icon = labels.Icon
	case hasCatalog && catalog.Icon != "":
		s.Icon = catalog.Icon
	}

	switch {
	case labels.Group != "":
		s.Group = labels.Group
	case hasCatalog && catalog.Group != "":
		s.Group = catalog.Group
	}

	// Pick port: label > catalog > first exposed.
	port := 0
	switch {
	case labels.Port != 0:
		port = labels.Port
		s.Notes = append(s.Notes, fmt.Sprintf("Port %d from label muximux.app.port", port))
	case hasCatalog && catalog.Port != 0 && containerExposesPort(c, catalog.Port):
		port = catalog.Port
	default:
		port = pickFirstExposedPort(c)
		if port != 0 {
			s.Notes = append(s.Notes, fmt.Sprintf("Port %d picked from container's exposed ports (no catalog/label hint)", port))
		}
	}

	if port == 0 {
		s.RequiresInput = true
		s.Notes = append(s.Notes, "No port exposed and no muximux.app.port label set")
	}

	// Scheme: label > catalog > http
	scheme := "http"
	if labels.Scheme != "" {
		scheme = labels.Scheme
	} else if hasCatalog && catalog.Scheme != "" {
		scheme = catalog.Scheme
	}

	// Strategy: per-container catalog override > global config.
	strategy := globalStrategy
	if hasCatalog && catalog.PrefersStrategy != "" {
		strategy = catalog.PrefersStrategy
		s.Notes = append(s.Notes, fmt.Sprintf("Strategy %q suggested by catalog (this image typically binds host ports)", strategy))
	}
	s.EffectiveStrategy = strategy

	// URL.
	if port != 0 {
		urlStr, err := buildURLForSuggestion(string(strategy), c, port, scheme, hostIP)
		if err != nil {
			s.RequiresInput = true
			s.Notes = append(s.Notes, fmt.Sprintf("Cannot build URL: %s", err.Error()))
		} else {
			s.URL = urlStr
		}
	}

	// Path applied to URL only if non-trivial - we keep the App.URL
	// pointing at the host:port, paths are an App-config field today.
	// (Future: surface a Path field on the suggestion for apps that
	// live behind a sub-path.)
	_ = labels.Path
	_ = catalog.Path

	// Health URL.
	switch {
	case labels.Health != "":
		s.HealthURL = labels.Health
	case hasCatalog && catalog.HealthURL != "":
		s.HealthURL = catalog.HealthURL
	}

	// Suggested gateway-site domain. Priority: label > derived from
	// containername.dashboardDomain > empty.
	switch {
	case labels.GatewayDomain != "":
		s.SuggestedDomain = labels.GatewayDomain
	case dashboardDomain != "" && c.PrimaryName() != "":
		s.SuggestedDomain = sanitiseSubdomain(c.PrimaryName()) + "." + dashboardDomain
	}

	// Carry every other muximux.app.* label through to the
	// Suggestion so the frontend can pre-fill the App form (and the
	// import endpoint can write them straight into AppConfig)
	// without operator post-edit.
	s.Color = labels.Color
	s.Order = labels.Order
	s.OpenMode = labels.OpenMode
	s.Proxy = labels.Proxy
	s.ProxySkipTLSVerify = labels.ProxySkipTLSVerify
	s.MinRole = labels.MinRole
	s.AllowedGroups = labels.AllowedGroups
	s.Permissions = labels.Permissions
	s.AllowNotifications = labels.AllowNotifications
	s.Default = labels.Default
	s.Shortcut = labels.Shortcut
	// Bump confidence to High when label-set is rich enough to
	// drive the import without operator review.
	if (labels.Proxy != nil || labels.OpenMode != "" || labels.Color != "" || labels.MinRole != "") && s.Confidence == ConfidenceMedium {
		s.Confidence = ConfidenceHigh
	}

	// muximux.gateway.* labels only matter when a gateway domain
	// was set too. Parse and attach.
	if s.SuggestedDomain != "" {
		gw := ParseGatewayLabels(c.Labels)
		if gatewayLabelsNonEmpty(&gw) {
			s.SuggestedGateway = &SuggestedGatewayConfig{
				TLS:                gw.TLS,
				Streaming:          gw.Streaming,
				StripFrameBlockers: gw.StripFrameBlockers,
				ForwardedHeaders:   gw.ForwardedHeaders,
				RequireAuth:        gw.RequireAuth,
				MinRole:            gw.MinRole,
				AllowedGroups:      gw.AllowedGroups,
			}
		}
	}

	// Surface unknown muximux.* labels so a typo gets noticed.
	for _, u := range labels.Unknown {
		s.Notes = append(s.Notes, fmt.Sprintf("Unknown label ignored: %s", u))
	}

	return s
}

// gatewayLabelsNonEmpty reports whether any gateway-namespace label
// was actually set. Used so we don't attach an all-zero
// SuggestedGateway payload that would clutter the JSON output for
// containers that only set the app.gateway.domain label. Takes a
// pointer so the 88-byte struct doesn't cross the function boundary
// by value (gocritic hugeParam).
func gatewayLabelsNonEmpty(g *GatewayLabels) bool {
	return g.TLS != "" || g.Streaming != nil || g.StripFrameBlockers != nil ||
		g.ForwardedHeaders != nil || g.RequireAuth != nil ||
		g.MinRole != "" || len(g.AllowedGroups) > 0
}

// pickFirstExposedPort returns the lowest privileged-looking port if
// any, else the first listed port. Falls back to 0 when no ports.
func pickFirstExposedPort(c *ContainerSummary) int {
	if len(c.Ports) == 0 {
		return 0
	}
	// Preferred ports for HTTP-ish services.
	preferred := []uint16{80, 8080, 8000, 8443, 443, 3000, 5000, 9000}
	for _, p := range preferred {
		if containerExposesPort(c, int(p)) {
			return int(p)
		}
	}
	// Fallback: first port in the list.
	return int(c.Ports[0].PrivatePort)
}

func containerExposesPort(c *ContainerSummary, port int) bool {
	for _, p := range c.Ports {
		if int(p.PrivatePort) == port {
			return true
		}
	}
	return false
}

// titleizeName turns "sonarr" into "Sonarr". Keeps mixed-case names
// (e.g., "qBittorrent") untouched so labelling stays readable.
func titleizeName(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] -= 32
	}
	return string(r)
}

// sanitiseSubdomain strips characters that cannot appear in a DNS
// label so we can splice a container name into a domain default.
// Rejects empty / fully-stripped results and falls back to nothing.
func sanitiseSubdomain(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	return out
}
