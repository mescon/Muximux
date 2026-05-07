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

	LabelAppEnabled       = "muximux.app.enabled" // "true" to opt in; defaults true when image matches catalog
	LabelAppName          = "muximux.app.name"
	LabelAppIcon          = "muximux.app.icon"
	LabelAppGroup         = "muximux.app.group"
	LabelAppPort          = "muximux.app.port"
	LabelAppScheme        = "muximux.app.scheme" // http | https
	LabelAppPath          = "muximux.app.path"
	LabelAppHealth        = "muximux.app.health"
	LabelAppGatewayDomain = "muximux.app.gateway.domain" // suggest as gateway site
)

// AppLabels is the parsed shape of the muximux.app.* label namespace.
// Empty-when-missing fields are zero values; callers default to
// catalog or container facts when a field is unset.
type AppLabels struct {
	Enabled       *bool // pointer so we can distinguish "absent" from "false"
	Name          string
	Icon          string
	Group         string
	Port          int    // 0 = unset
	Scheme        string // "" = unset
	Path          string
	Health        string
	GatewayDomain string

	// Unknown collects label keys in the muximux.* namespace we don't
	// recognise. The scan path logs them at Debug so a typo surfaces
	// when the operator runs Discover but doesn't see the expected
	// suggestion shape.
	Unknown []string
}

// ParseAppLabels extracts known muximux.* labels from a container's
// label map. Validates ranges (port 1..65535, scheme http|https) and
// returns an empty zero-value when no labels are present.
func ParseAppLabels(labels map[string]string) AppLabels {
	out := AppLabels{}
	if len(labels) == 0 {
		return out
	}
	for k, v := range labels {
		if !strings.HasPrefix(k, "muximux.") {
			continue
		}
		switch k {
		case LabelAppEnabled:
			b := strings.EqualFold(v, "true") || v == "1"
			out.Enabled = &b
		case LabelAppName:
			out.Name = v
		case LabelAppIcon:
			out.Icon = v
		case LabelAppGroup:
			out.Group = v
		case LabelAppPort:
			if p, err := strconv.Atoi(v); err == nil && p >= 1 && p <= 65535 {
				out.Port = p
			}
		case LabelAppScheme:
			lv := strings.ToLower(v)
			if lv == "http" || lv == "https" {
				out.Scheme = lv
			}
		case LabelAppPath:
			out.Path = v
		case LabelAppHealth:
			out.Health = v
		case LabelAppGatewayDomain:
			out.GatewayDomain = v
		case LabelDiscoveryID:
			// Handled by KeyForContainer, not this struct.
		default:
			out.Unknown = append(out.Unknown, k)
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
