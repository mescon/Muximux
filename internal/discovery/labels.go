package discovery

import (
	"regexp"
	"strconv"
	"strings"
)

// Recognised label names. Listed here so a typo in operator labels
// surfaces clearly (we log unknowns and ignore them rather than
// silently dropping a misspelled directive).
const (
	LabelDiscoveryID = "muximux.discovery.id" // operator-supplied stable tracking key

	// muximux.app.* namespace - per-app fields. Anything an operator
	// would normally set in the App edit form can be pinned here.
	LabelAppEnabled            = "muximux.app.enabled" // "true" to opt in; defaults true when image matches catalog
	LabelAppName               = "muximux.app.name"
	LabelAppIcon               = "muximux.app.icon"
	LabelAppGroup              = "muximux.app.group"
	LabelAppPort               = "muximux.app.port"
	LabelAppScheme             = "muximux.app.scheme" // http | https
	LabelAppPath               = "muximux.app.path"
	LabelAppHealth             = "muximux.app.health"
	LabelAppColor              = "muximux.app.color"     // accent color, "#rrggbb"
	LabelAppOrder              = "muximux.app.order"     // sort order within group
	LabelAppDefault            = "muximux.app.default"   // "true" to load on dashboard startup
	LabelAppOpenMode           = "muximux.app.open_mode" // iframe | new_tab | new_window | redirect
	LabelAppProxy              = "muximux.app.proxy"     // "true" to route through Muximux's built-in reverse proxy
	LabelAppProxySkipTLSVerify = "muximux.app.proxy_skip_tls_verify"
	LabelAppMinRole            = "muximux.app.min_role"            // user | power-user | admin
	LabelAppAllowedGroups      = "muximux.app.allowed_groups"      // comma-separated
	LabelAppPermissions        = "muximux.app.permissions"         // comma-separated; iframe permission delegations
	LabelAppAllowNotifications = "muximux.app.allow_notifications" // "true" to enable notification bridge
	LabelAppShortcut           = "muximux.app.shortcut"            // keyboard digit 1..9
	LabelAppGatewayDomain      = "muximux.app.gateway.domain"      // suggest as gateway site

	// muximux.gateway.* namespace - per-gateway-site fields. Only
	// consulted when the same container also has app.gateway.domain
	// set. Lets operators pin the full Settings -> Gateway form
	// without post-import editing.
	LabelGatewayTLS                = "muximux.gateway.tls"                  // auto | none | custom
	LabelGatewayStreaming          = "muximux.gateway.streaming"            // "true" to disable Caddy response buffering
	LabelGatewayStripFrameBlockers = "muximux.gateway.strip_frame_blockers" // "true" to drop X-Frame-Options on responses
	LabelGatewayForwardedHeaders   = "muximux.gateway.forwarded_headers"    // "true" to forward X-Forwarded-* headers
	LabelGatewayRequireAuth        = "muximux.gateway.require_auth"         // "true" to gate the site behind Muximux login
	LabelGatewayMinRole            = "muximux.gateway.min_role"             // user | power-user | admin
	LabelGatewayAllowedGroups      = "muximux.gateway.allowed_groups"       // comma-separated
)

// AppLabels is the parsed shape of the muximux.app.* label namespace.
// Empty-when-missing fields are zero values; callers default to
// catalog or container facts when a field is unset.
type AppLabels struct {
	Enabled            *bool // pointer so we can distinguish "absent" from "false"
	Name               string
	Icon               string
	Group              string
	Port               int    // 0 = unset
	Scheme             string // "" = unset
	Path               string
	Health             string
	Color              string
	Order              int   // 0 = unset
	Default            *bool // pointer to distinguish absent from false
	OpenMode           string
	Proxy              *bool
	ProxySkipTLSVerify *bool
	MinRole            string
	AllowedGroups      []string
	Permissions        []string
	AllowNotifications *bool
	Shortcut           int // 0 = unset
	GatewayDomain      string

	// Unknown collects label keys in the muximux.* namespace we don't
	// recognise. The scan path logs them at Debug so a typo surfaces
	// when the operator runs Discover but doesn't see the expected
	// suggestion shape.
	Unknown []string
}

// GatewayLabels is the parsed shape of the muximux.gateway.* label
// namespace. Only consulted when AppLabels.GatewayDomain is set,
// since these settings describe a gateway-site entry.
type GatewayLabels struct {
	TLS                string // "" = unset; auto | none | custom
	Streaming          *bool
	StripFrameBlockers *bool
	ForwardedHeaders   *bool
	RequireAuth        *bool
	MinRole            string
	AllowedGroups      []string
}

// appLabelHandlers maps each recognised muximux.app.* label name to
// the small mutator that applies its parsed value onto AppLabels.
// Splitting the dispatch into a table keeps ParseAppLabels itself
// low-complexity (one map lookup per label, no giant switch) and
// makes adding a new label a single-line entry.
var appLabelHandlers = map[string]func(out *AppLabels, v string){
	LabelAppEnabled: func(out *AppLabels, v string) { b := boolish(v); out.Enabled = &b },
	LabelAppName:    func(out *AppLabels, v string) { out.Name = v },
	LabelAppIcon:    func(out *AppLabels, v string) { out.Icon = v },
	LabelAppGroup:   func(out *AppLabels, v string) { out.Group = v },
	LabelAppPort: func(out *AppLabels, v string) {
		if p, err := strconv.Atoi(v); err == nil && p >= 1 && p <= 65535 {
			out.Port = p
		}
	},
	LabelAppScheme: func(out *AppLabels, v string) {
		lv := strings.ToLower(v)
		if lv == "http" || lv == "https" {
			out.Scheme = lv
		}
	},
	LabelAppPath:   func(out *AppLabels, v string) { out.Path = v },
	LabelAppHealth: func(out *AppLabels, v string) { out.Health = v },
	LabelAppColor: func(out *AppLabels, v string) {
		if isHexColor(v) {
			out.Color = v
		}
	},
	LabelAppOrder: func(out *AppLabels, v string) {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 9999 {
			out.Order = n
		}
	},
	LabelAppDefault: func(out *AppLabels, v string) { b := boolish(v); out.Default = &b },
	LabelAppOpenMode: func(out *AppLabels, v string) {
		lv := strings.ToLower(strings.TrimSpace(v))
		if lv == "iframe" || lv == "new_tab" || lv == "new_window" || lv == "redirect" {
			out.OpenMode = lv
		}
	},
	LabelAppProxy:              func(out *AppLabels, v string) { b := boolish(v); out.Proxy = &b },
	LabelAppProxySkipTLSVerify: func(out *AppLabels, v string) { b := boolish(v); out.ProxySkipTLSVerify = &b },
	LabelAppMinRole: func(out *AppLabels, v string) {
		lv := strings.ToLower(strings.TrimSpace(v))
		if lv == "user" || lv == "power-user" || lv == "admin" {
			out.MinRole = lv
		}
	},
	LabelAppAllowedGroups:      func(out *AppLabels, v string) { out.AllowedGroups = splitCSV(v) },
	LabelAppPermissions:        func(out *AppLabels, v string) { out.Permissions = splitCSV(v) },
	LabelAppAllowNotifications: func(out *AppLabels, v string) { b := boolish(v); out.AllowNotifications = &b },
	LabelAppShortcut: func(out *AppLabels, v string) {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 9 {
			out.Shortcut = n
		}
	},
	LabelAppGatewayDomain: func(out *AppLabels, v string) { out.GatewayDomain = v },
}

// knownNonAppLabels are recognised muximux.* names that ParseAppLabels
// intentionally ignores (they're consumed by other parsers). Listed
// here so they don't end up flagged as Unknown.
var knownNonAppLabels = map[string]struct{}{
	LabelDiscoveryID:               {},
	LabelGatewayTLS:                {},
	LabelGatewayStreaming:          {},
	LabelGatewayStripFrameBlockers: {},
	LabelGatewayForwardedHeaders:   {},
	LabelGatewayRequireAuth:        {},
	LabelGatewayMinRole:            {},
	LabelGatewayAllowedGroups:      {},
}

// ParseAppLabels extracts known muximux.app.* and muximux.discovery.*
// labels from a container's label map. Validates ranges
// (port 1..65535, scheme http|https, open_mode shape, etc.) and
// returns an empty zero-value when no labels are present.
//
// Unknown labels in the muximux.* namespace land in Unknown so callers
// can log a "did you mean ...?" hint.
func ParseAppLabels(labels map[string]string) AppLabels {
	out := AppLabels{}
	if len(labels) == 0 {
		return out
	}
	for k, v := range labels {
		if !strings.HasPrefix(k, "muximux.") {
			continue
		}
		if h, ok := appLabelHandlers[k]; ok {
			h(&out, v)
			continue
		}
		if _, known := knownNonAppLabels[k]; known {
			continue
		}
		out.Unknown = append(out.Unknown, k)
	}
	return out
}

// ParseGatewayLabels extracts the muximux.gateway.* namespace from a
// container's label map. Returned as a separate struct so callers
// only consult it when AppLabels.GatewayDomain is set.
// gatewayLabelHandlers mirrors appLabelHandlers but for the
// muximux.gateway.* namespace. Same dispatch-table shape keeps
// ParseGatewayLabels free of nested switch + low-complexity.
var gatewayLabelHandlers = map[string]func(out *GatewayLabels, v string){
	LabelGatewayTLS: func(out *GatewayLabels, v string) {
		lv := strings.ToLower(strings.TrimSpace(v))
		if lv == "auto" || lv == "none" || lv == "custom" {
			out.TLS = lv
		}
	},
	LabelGatewayStreaming:          func(out *GatewayLabels, v string) { b := boolish(v); out.Streaming = &b },
	LabelGatewayStripFrameBlockers: func(out *GatewayLabels, v string) { b := boolish(v); out.StripFrameBlockers = &b },
	LabelGatewayForwardedHeaders:   func(out *GatewayLabels, v string) { b := boolish(v); out.ForwardedHeaders = &b },
	LabelGatewayRequireAuth:        func(out *GatewayLabels, v string) { b := boolish(v); out.RequireAuth = &b },
	LabelGatewayMinRole: func(out *GatewayLabels, v string) {
		lv := strings.ToLower(strings.TrimSpace(v))
		if lv == "user" || lv == "power-user" || lv == "admin" {
			out.MinRole = lv
		}
	},
	LabelGatewayAllowedGroups: func(out *GatewayLabels, v string) { out.AllowedGroups = splitCSV(v) },
}

func ParseGatewayLabels(labels map[string]string) GatewayLabels {
	out := GatewayLabels{}
	if len(labels) == 0 {
		return out
	}
	for k, v := range labels {
		if h, ok := gatewayLabelHandlers[k]; ok {
			h(&out, v)
		}
	}
	return out
}

// boolish parses common truthy strings (case-insensitive "true",
// "1", "yes", "on") into bool. Anything else is false. Centralised
// so all boolean labels agree on parsing.
func boolish(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes", "on":
		return true
	}
	return false
}

// hexColorPattern matches "#" followed by 3, 4, 6, or 8 hex digits.
var hexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`)

func isHexColor(s string) bool {
	s = strings.TrimSpace(s)
	if !hexColorPattern.MatchString(s) {
		return false
	}
	switch len(s) {
	case 4, 5, 7, 9:
		return true
	}
	return false
}

// splitCSV splits a comma-separated label value into trimmed,
// non-empty entries. Used for allowed_groups and permissions.
func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Stability hints surface in the Discover modal next to each
// suggestion so the operator can see whether the tracking key will
// survive a docker-compose --force-recreate.
type Stability string

const (
	StabilityStable          Stability = "stable"           // label-based or plain non-suffixed name
	StabilityRecreateFragile Stability = "recreate-fragile" // compose-style suffix that changes on recreate
	StabilityTaskFragile     Stability = "task-fragile"     // swarm task name with random suffix
)

// composeV1Suffix matches names like "myproject_sonarr_1" - V1 default.
var composeV1Suffix = regexp.MustCompile(`_\d+$`)

// composeV2Suffix matches names like "myproject-sonarr-1" - V2 default.
var composeV2Suffix = regexp.MustCompile(`-\d+$`)

// swarmTaskPattern matches names like "stack_service.1.abcdef0123456789abcd"
// emitted by docker swarm. The trailing 20+ char ID changes per
// reschedule, so the name is permanently unstable.
var swarmTaskPattern = regexp.MustCompile(`\.\d+\.[a-z0-9]{20,}$`)

// KeyForContainer picks the most stable identifier available for
// tracking the container across restarts. Resolution order (most
// stable first):
//
//  1. operator label muximux.discovery.id  -> "label:<value>"
//  2. plain container name                  -> "name:<name>"   (with stability hint)
//  3. container ID (full SHA)               -> "id:<id>"        (last resort)
//
// The returned stability lets the modal surface a warning when the
// chosen key will likely shift on docker-compose --force-recreate or
// swarm task reschedule.
func KeyForContainer(c *ContainerSummary) (key string, stability Stability) {
	if v, ok := c.Labels[LabelDiscoveryID]; ok && strings.TrimSpace(v) != "" {
		return "label:" + strings.TrimSpace(v), StabilityStable
	}
	name := c.PrimaryName()
	if name != "" {
		switch {
		case swarmTaskPattern.MatchString(name):
			return "name:" + name, StabilityTaskFragile
		case composeV1Suffix.MatchString(name) || composeV2Suffix.MatchString(name):
			return "name:" + name, StabilityRecreateFragile
		default:
			return "name:" + name, StabilityStable
		}
	}
	return "id:" + c.ID, StabilityRecreateFragile
}
