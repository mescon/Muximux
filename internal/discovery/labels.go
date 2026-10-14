package discovery

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/logging"
)

// Recognised label names. Listed here so a typo in operator labels
// surfaces clearly (we log unknowns and ignore them rather than
// silently dropping a misspelled directive).
const (
	LabelDiscoveryID = "muximux.discovery.id" // operator-supplied stable tracking key

	// Docker-owned labels used for stable swarm and compose keys.
	LabelSwarmServiceID   = "com.docker.swarm.service.id"
	LabelSwarmServiceName = "com.docker.swarm.service.name"
	LabelComposeProject   = "com.docker.compose.project"
	LabelComposeService   = "com.docker.compose.service"

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
	LabelAppURL                = "muximux.app.url"                 // absolute URL the app opens at; health still follows the container
	LabelAppHealthCheck        = "muximux.app.health_check"        // "true" enables health monitoring, "false" disables, unset keeps the app's own setting

	LabelAppHTTPActionMethod    = "muximux.app.http_action_method"     // GET | POST | PUT | DELETE | PATCH
	LabelAppHTTPActionHeaders   = "muximux.app.http_action_headers"    // Key=Value,Key2=Value2 (CSV)
	LabelAppHTTPActionConfirm   = "muximux.app.http_action_confirm"    // boolean
	LabelAppHTTPActionShowToast = "muximux.app.http_action_show_toast" // boolean; default true

	// muximux.gateway.* namespace - per-gateway-site fields. Only
	// consulted when the same container also has app.gateway.domain
	// set. Lets operators pin the full Settings -> Gateway form
	// without post-import editing.
	LabelGatewayTLS                = "muximux.gateway.tls"                  // auto | none | custom
	LabelGatewayStreaming          = "muximux.gateway.streaming"            // "true" to disable Caddy response buffering
	LabelGatewayStripFrameBlockers = "muximux.gateway.strip_frame_blockers" // "true" to drop X-Frame-Options on responses
	LabelGatewayForwardedHeaders   = "muximux.gateway.forwarded_headers"    // "true" to forward X-Forwarded-* headers
	LabelGatewaySkipTLSVerify      = "muximux.gateway.skip_tls_verify"      // "true" to skip verifying a self-signed HTTPS backend
	LabelGatewayRequireAuth        = "muximux.gateway.require_auth"         // "true" to gate the site behind Muximux login
	LabelGatewayMinRole            = "muximux.gateway.min_role"             // user | power-user | admin
	LabelGatewayAllowedGroups      = "muximux.gateway.allowed_groups"       // comma-separated

	// muximux.group.* namespace - fields of the group the container's app
	// is in. Applied only to a group Docker discovery created and still
	// manages (GroupConfig.DockerManaged); see grouplabels.go.
	LabelGroupIcon  = "muximux.group.icon"  // same forms as muximux.app.icon
	LabelGroupColor = "muximux.group.color" // "#rrggbb"
	LabelGroupOrder = "muximux.group.order" // sort order of the group, 0..9999
)

// AppLabels is the parsed shape of the muximux.app.* label namespace.
// Empty-when-missing fields are zero values; callers default to
// catalog or container facts when a field is unset.
type AppLabels struct {
	Any     bool  // at least one muximux.* label was present (known or unknown)
	Enabled *bool // pointer so we can distinguish "absent" from "false"
	Name    string
	Icon    IconRef // see ParseIconLabel; zero = unset or invalid
	// IconInvalid is why a set icon label was ignored; "" = usable or unset.
	IconInvalid string
	// BoolInvalid lists, in label order, the boolean labels whose value was
	// not a recognised boolean and so was ignored (as if unset).
	BoolInvalid []InvalidLabel
	Group       string
	Port        int    // 0 = unset
	Scheme      string // "" = unset
	Path        string
	Health      string
	Color       string
	Order       int // 0 when unset; see OrderSet
	// OrderSet is true when muximux.app.order holds a valid value, so an
	// explicit 0 can be told apart from an unset label.
	OrderSet           bool
	Default            *bool // pointer to distinguish absent from false
	OpenMode           string
	Proxy              *bool
	ProxySkipTLSVerify *bool
	MinRole            string
	AllowedGroups      []string
	Permissions        []string
	AllowNotifications *bool
	HealthCheck        *bool // pointer so unset keeps the app's own setting
	Shortcut           int   // 0 = unset
	GatewayDomain      string

	// URL is the trimmed muximux.app.url value. Validated where it is
	// used (suggest.go applyFixedURL) so an invalid value can produce a
	// scan note instead of vanishing silently.
	URL string

	HTTPActionMethod    string
	HTTPActionHeaders   map[string]string
	HTTPActionConfirm   *bool
	HTTPActionShowToast *bool

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
	SkipTLSVerify      *bool
	RequireAuth        *bool
	MinRole            string
	AllowedGroups      []string
	// Invalid lists, in label order, the boolean labels whose value was not
	// a recognised boolean and so was ignored (as if unset).
	Invalid []InvalidLabel
}

// appLabelHandlers maps each recognised muximux.app.* label name to
// the small mutator that applies its parsed value onto AppLabels.
// Splitting the dispatch into a table keeps ParseAppLabels itself
// low-complexity (one map lookup per label, no giant switch) and
// makes adding a new label a single-line entry.
var appLabelHandlers = map[string]func(out *AppLabels, v string){
	LabelAppEnabled: appBool(LabelAppEnabled, func(o *AppLabels) **bool { return &o.Enabled }),
	// Name, icon and group are trimmed so a stray space in a compose file
	// neither renames the app nor points at a missing icon or group. The icon
	// is parsed by ParseIconLabel (dashboard slug, lucide:, url: or a URL).
	LabelAppName:  func(out *AppLabels, v string) { out.Name = strings.TrimSpace(v) },
	LabelAppIcon:  func(out *AppLabels, v string) { out.Icon, out.IconInvalid = ParseIconLabel(v) },
	LabelAppGroup: func(out *AppLabels, v string) { out.Group = strings.TrimSpace(v) },
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
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 0 && n <= 9999 {
			out.Order = n
			out.OrderSet = true
		}
	},
	LabelAppDefault: appBool(LabelAppDefault, func(o *AppLabels) **bool { return &o.Default }),
	LabelAppOpenMode: func(out *AppLabels, v string) {
		lv := strings.ToLower(strings.TrimSpace(v))
		if lv == "iframe" || lv == "new_tab" || lv == "new_window" || lv == "redirect" || lv == "http_action" {
			out.OpenMode = lv
		}
	},
	LabelAppProxy:              appBool(LabelAppProxy, func(o *AppLabels) **bool { return &o.Proxy }),
	LabelAppProxySkipTLSVerify: appBool(LabelAppProxySkipTLSVerify, func(o *AppLabels) **bool { return &o.ProxySkipTLSVerify }),
	LabelAppMinRole: func(out *AppLabels, v string) {
		lv := strings.ToLower(strings.TrimSpace(v))
		if lv == "user" || lv == "power-user" || lv == "admin" {
			out.MinRole = lv
		}
	},
	LabelAppAllowedGroups:      func(out *AppLabels, v string) { out.AllowedGroups = splitCSV(v) },
	LabelAppPermissions:        func(out *AppLabels, v string) { out.Permissions = splitCSV(v) },
	LabelAppAllowNotifications: appBool(LabelAppAllowNotifications, func(o *AppLabels) **bool { return &o.AllowNotifications }),
	LabelAppShortcut: func(out *AppLabels, v string) {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 9 {
			out.Shortcut = n
		}
	},
	LabelAppHealthCheck:   appBool(LabelAppHealthCheck, func(o *AppLabels) **bool { return &o.HealthCheck }),
	LabelAppGatewayDomain: func(out *AppLabels, v string) { out.GatewayDomain = v },
	LabelAppURL:           func(out *AppLabels, v string) { out.URL = strings.TrimSpace(v) },
	LabelAppHTTPActionMethod: func(out *AppLabels, v string) {
		uv := strings.ToUpper(strings.TrimSpace(v))
		switch uv {
		case "GET", "POST", "PUT", "DELETE", "PATCH":
			out.HTTPActionMethod = uv
		}
	},
	LabelAppHTTPActionHeaders: func(out *AppLabels, v string) {
		out.HTTPActionHeaders = parseHTTPActionHeadersCSV(v)
	},
	LabelAppHTTPActionConfirm:   appBool(LabelAppHTTPActionConfirm, func(o *AppLabels) **bool { return &o.HTTPActionConfirm }),
	LabelAppHTTPActionShowToast: appBool(LabelAppHTTPActionShowToast, func(o *AppLabels) **bool { return &o.HTTPActionShowToast }),
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
	LabelGatewaySkipTLSVerify:      {},
	LabelGatewayRequireAuth:        {},
	LabelGatewayMinRole:            {},
	LabelGatewayAllowedGroups:      {},
	LabelGroupIcon:                 {},
	LabelGroupColor:                {},
	LabelGroupOrder:                {},
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
		out.Any = true
		if h, ok := appLabelHandlers[k]; ok {
			h(&out, v)
			continue
		}
		if _, known := knownNonAppLabels[k]; known {
			continue
		}
		out.Unknown = append(out.Unknown, k)
	}
	// Map order is random; keep the notes and the WARN stable.
	sortInvalid(out.BoolInvalid)
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
	LabelGatewayStreaming:          gatewayBool(LabelGatewayStreaming, func(o *GatewayLabels) **bool { return &o.Streaming }),
	LabelGatewayStripFrameBlockers: gatewayBool(LabelGatewayStripFrameBlockers, func(o *GatewayLabels) **bool { return &o.StripFrameBlockers }),
	LabelGatewayForwardedHeaders:   gatewayBool(LabelGatewayForwardedHeaders, func(o *GatewayLabels) **bool { return &o.ForwardedHeaders }),
	LabelGatewaySkipTLSVerify:      gatewayBool(LabelGatewaySkipTLSVerify, func(o *GatewayLabels) **bool { return &o.SkipTLSVerify }),
	LabelGatewayRequireAuth:        gatewayBool(LabelGatewayRequireAuth, func(o *GatewayLabels) **bool { return &o.RequireAuth }),
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
	sortInvalid(out.Invalid)
	return out
}

// InvalidLabel is a boolean label whose value was ignored: the label name
// and the reason, for the scan note and the warning.
type InvalidLabel struct{ Name, Reason string }

func sortInvalid(l []InvalidLabel) {
	sort.Slice(l, func(i, j int) bool { return l[i].Name < l[j].Name })
}

// invalidReasons joins the reasons of l with "; ".
func invalidReasons(l ...[]InvalidLabel) []string {
	var out []string
	for _, x := range l {
		for i := range x {
			out = append(out, x[i].Reason)
		}
	}
	return out
}

// parseBoolLabel reads a boolean label. A blank value counts as unset. A
// value that is not true/1/yes/on or false/0/no/off (case-insensitive,
// trimmed) is ignored as if the label were unset and described in invalid,
// so a typo such as "tru" never opts a container out. The rule is shared
// with the environment overrides (config.ParseBool).
func parseBoolLabel(name, v string, dst **bool, invalid *[]InvalidLabel) {
	if strings.TrimSpace(v) == "" {
		return
	}
	b, ok := config.ParseBool(v)
	if !ok {
		*invalid = append(*invalid, InvalidLabel{Name: name, Reason: fmt.Sprintf("%s %q is not a boolean (use true or false)", name, v)})
		return
	}
	*dst = &b
}

// appBool builds the handler of one boolean muximux.app.* label.
func appBool(name string, field func(*AppLabels) **bool) func(*AppLabels, string) {
	return func(out *AppLabels, v string) { parseBoolLabel(name, v, field(out), &out.BoolInvalid) }
}

// gatewayBool builds the handler of one boolean muximux.gateway.* label.
func gatewayBool(name string, field func(*GatewayLabels) **bool) func(*GatewayLabels, string) {
	return func(out *GatewayLabels, v string) { parseBoolLabel(name, v, field(out), &out.Invalid) }
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

// parseHTTPActionHeadersCSV parses a `Key=Value,Key2=Value2` string
// into a map. Each pair is split on the first `=` so values may
// themselves contain `=`. Pairs with empty keys or missing `=` are
// dropped. Returns nil when no valid pairs are found so the caller
// can distinguish "no header label" from "label set to empty".
func parseHTTPActionHeadersCSV(v string) map[string]string {
	out := map[string]string{}
	for _, pair := range strings.Split(v, ",") {
		eq := strings.IndexByte(pair, '=')
		if eq < 0 {
			continue
		}
		k := strings.TrimSpace(pair[:eq])
		val := strings.TrimSpace(pair[eq+1:])
		if k == "" {
			continue
		}
		out[k] = val
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Stability hints surface in the Discover modal next to each
// suggestion so the operator can see whether the tracking key will
// survive a docker-compose --force-recreate or a swarm reschedule.
// Label, swarm service and compose service keys are all stable; only
// task-name and compose-suffixed container-name keys are fragile.
type Stability string

const (
	StabilityStable          Stability = "stable"           // label, swarm service, compose service, or plain non-suffixed name
	StabilityRecreateFragile Stability = "recreate-fragile" // compose-style name suffix (no compose labels) that changes on recreate
	StabilityTaskFragile     Stability = "task-fragile"     // swarm task name with random suffix (no swarm service label)
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
//  2. swarm service name label             -> "swarm:<service>"
//  3. compose project + service labels      -> "compose:<project>:<service>"
//  4. plain container name                  -> "name:<name>"   (with stability hint)
//  5. container ID (full SHA)               -> "id:<id>"        (last resort)
//
// The returned stability lets the modal surface a warning when the
// chosen key will likely shift on docker-compose --force-recreate or
// swarm task reschedule.
func KeyForContainer(c *ContainerSummary) (key string, stability Stability) {
	if v, ok := c.Labels[LabelDiscoveryID]; ok && strings.TrimSpace(v) != "" {
		return "label:" + strings.TrimSpace(v), StabilityStable
	}
	if svc := swarmServiceName(c); svc != "" {
		return "swarm:" + svc, StabilityStable
	}
	if v := composeKeyValue(c); v != "" {
		return "compose:" + v, StabilityStable
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

// warnInvalidBoolLabels logs, once per transition, the boolean labels of
// each container whose value was not a recognised boolean and so was
// ignored. Rebuilt every tick, so a fixed label is forgotten and a later
// regression warns again.
func (p *Poller) warnInvalidBoolLabels(containers []ContainerSummary) {
	invalid := map[string]string{}
	for i := range containers {
		c := &containers[i]
		app := ParseAppLabels(c.Labels)
		gw := ParseGatewayLabels(c.Labels)
		reasons := invalidReasons(app.BoolInvalid, gw.Invalid)
		if len(reasons) == 0 {
			continue
		}
		key, _ := KeyForContainer(c)
		invalid[key] = strings.Join(reasons, "; ")
	}
	for k, reason := range invalid {
		if p.boolLabelInvalid[k] != reason {
			logging.Warn("Docker boolean label ignored: invalid value",
				"source", "discovery", "key", k, "reason", reason)
		}
	}
	p.boolLabelInvalid = invalid
}
