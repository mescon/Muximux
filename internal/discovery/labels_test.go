package discovery

import (
	"strings"
	"testing"
)

func TestParseAppLabels_KnownFields(t *testing.T) {
	labels := map[string]string{
		"muximux.app.name":           "My Sonarr",
		"muximux.app.icon":           "sonarr",
		"muximux.app.group":          "Media",
		"muximux.app.port":           "8989",
		"muximux.app.scheme":         "https",
		"muximux.app.path":           "/sonarr",
		"muximux.app.health":         "/ping",
		"muximux.app.gateway.domain": "sonarr.example.com",
		"muximux.app.enabled":        "true",
		"unrelated.label":            "ignored",
	}
	got := ParseAppLabels(labels)
	if got.Name != "My Sonarr" {
		t.Errorf("Name = %q", got.Name)
	}
	if got.Port != 8989 {
		t.Errorf("Port = %d", got.Port)
	}
	if got.Scheme != "https" {
		t.Errorf("Scheme = %q", got.Scheme)
	}
	if got.GatewayDomain != "sonarr.example.com" {
		t.Errorf("GatewayDomain = %q", got.GatewayDomain)
	}
	if got.Enabled == nil || *got.Enabled != true {
		t.Errorf("Enabled = %v, want pointer-to-true", got.Enabled)
	}
}

func TestParseAppLabels_RejectsBadValues(t *testing.T) {
	labels := map[string]string{
		"muximux.app.port":   "not-a-number",
		"muximux.app.scheme": "ftp",
	}
	got := ParseAppLabels(labels)
	if got.Port != 0 {
		t.Errorf("Port should be 0 for non-numeric value, got %d", got.Port)
	}
	if got.Scheme != "" {
		t.Errorf("Scheme should be empty for unknown value, got %q", got.Scheme)
	}
}

func TestParseAppLabels_UnknownLabelsCollected(t *testing.T) {
	labels := map[string]string{
		"muximux.app.name":     "OK",
		"muximux.app.typo":     "bad",
		"muximux.discovery.id": "stable-key", // handled by KeyForContainer; not unknown
	}
	got := ParseAppLabels(labels)
	if len(got.Unknown) != 1 || got.Unknown[0] != "muximux.app.typo" {
		t.Errorf("Unknown = %v, want [muximux.app.typo]", got.Unknown)
	}
}

// TestParseAppLabels_ExtendedFields covers every label added for the
// 3.1.0 GitOps expansion. Each field's parser has its own validation
// (hex colour, role enum, comma-split list, etc.) and this test
// pins the contract that valid values land and invalid ones fall
// through to zero.
func TestParseAppLabels_ExtendedFields(t *testing.T) {
	t.Run("all extended fields parse", func(t *testing.T) {
		labels := map[string]string{
			"muximux.app.color":                 "#3b82f6",
			"muximux.app.order":                 "7",
			"muximux.app.default":               "true",
			"muximux.app.open_mode":             "new_tab",
			"muximux.app.proxy":                 "true",
			"muximux.app.proxy_skip_tls_verify": "yes",
			"muximux.app.min_role":              "power-user",
			"muximux.app.allowed_groups":        "family, admins , staff",
			"muximux.app.permissions":           "camera,microphone,geolocation",
			"muximux.app.allow_notifications":   "1",
			"muximux.app.shortcut":              "3",
		}
		got := ParseAppLabels(labels)
		if got.Color != "#3b82f6" {
			t.Errorf("Color = %q", got.Color)
		}
		if got.Order != 7 {
			t.Errorf("Order = %d", got.Order)
		}
		if got.Default == nil || !*got.Default {
			t.Errorf("Default = %v, want pointer-to-true", got.Default)
		}
		if got.OpenMode != "new_tab" {
			t.Errorf("OpenMode = %q", got.OpenMode)
		}
		if got.Proxy == nil || !*got.Proxy {
			t.Errorf("Proxy = %v, want pointer-to-true", got.Proxy)
		}
		if got.ProxySkipTLSVerify == nil || !*got.ProxySkipTLSVerify {
			t.Errorf("ProxySkipTLSVerify = %v, want pointer-to-true", got.ProxySkipTLSVerify)
		}
		if got.MinRole != "power-user" {
			t.Errorf("MinRole = %q", got.MinRole)
		}
		if want := []string{"family", "admins", "staff"}; !equalStrings(got.AllowedGroups, want) {
			t.Errorf("AllowedGroups = %v, want %v", got.AllowedGroups, want)
		}
		if want := []string{"camera", "microphone", "geolocation"}; !equalStrings(got.Permissions, want) {
			t.Errorf("Permissions = %v, want %v", got.Permissions, want)
		}
		if got.AllowNotifications == nil || !*got.AllowNotifications {
			t.Errorf("AllowNotifications = %v, want pointer-to-true", got.AllowNotifications)
		}
		if got.Shortcut != 3 {
			t.Errorf("Shortcut = %d", got.Shortcut)
		}
	})

	t.Run("rejects invalid extended values", func(t *testing.T) {
		labels := map[string]string{
			"muximux.app.color":     "blue", // not hex
			"muximux.app.order":     "abc",
			"muximux.app.open_mode": "modal", // not one of the supported set
			"muximux.app.min_role":  "owner", // not in enum
			"muximux.app.shortcut":  "12",    // out of range
		}
		got := ParseAppLabels(labels)
		if got.Color != "" || got.Order != 0 || got.OpenMode != "" || got.MinRole != "" || got.Shortcut != 0 {
			t.Errorf("invalid values should fall through to zero; got %+v", got)
		}
	})
}

// TestParseGatewayLabels covers the separate gateway-namespace parser
// that drives auto-creation of `gateway_sites:` entries from labels.
func TestParseGatewayLabels(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		labels := map[string]string{
			"muximux.gateway.tls":                  "auto",
			"muximux.gateway.streaming":            "true",
			"muximux.gateway.strip_frame_blockers": "true",
			"muximux.gateway.forwarded_headers":    "false",
			"muximux.gateway.require_auth":         "true",
			"muximux.gateway.min_role":             "admin",
			"muximux.gateway.allowed_groups":       "admins",
		}
		got := ParseGatewayLabels(labels)
		if got.TLS != "auto" {
			t.Errorf("TLS = %q", got.TLS)
		}
		if got.Streaming == nil || !*got.Streaming {
			t.Errorf("Streaming = %v", got.Streaming)
		}
		if got.StripFrameBlockers == nil || !*got.StripFrameBlockers {
			t.Errorf("StripFrameBlockers = %v", got.StripFrameBlockers)
		}
		if got.ForwardedHeaders == nil || *got.ForwardedHeaders {
			t.Errorf("ForwardedHeaders should be pointer-to-false, got %v", got.ForwardedHeaders)
		}
		if got.RequireAuth == nil || !*got.RequireAuth {
			t.Errorf("RequireAuth = %v", got.RequireAuth)
		}
		if got.MinRole != "admin" {
			t.Errorf("MinRole = %q", got.MinRole)
		}
		if len(got.AllowedGroups) != 1 || got.AllowedGroups[0] != "admins" {
			t.Errorf("AllowedGroups = %v", got.AllowedGroups)
		}
	})

	t.Run("rejects invalid tls / role", func(t *testing.T) {
		got := ParseGatewayLabels(map[string]string{
			"muximux.gateway.tls":      "wildcard",
			"muximux.gateway.min_role": "moderator",
		})
		if got.TLS != "" || got.MinRole != "" {
			t.Errorf("invalid values should be dropped; got %+v", got)
		}
	})
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestKeyForContainer_LabelWins(t *testing.T) {
	c := &ContainerSummary{
		ID:    "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
		Names: []string{"/myproject_sonarr_1"},
		Labels: map[string]string{
			"muximux.discovery.id": "sonarr-prod",
		},
	}
	key, stab := KeyForContainer(c)
	if key != "label:sonarr-prod" {
		t.Errorf("key = %q, want label:sonarr-prod", key)
	}
	if stab != StabilityStable {
		t.Errorf("stability = %q, want stable", stab)
	}
}

func TestKeyForContainer_NameDetectsCompose(t *testing.T) {
	cases := []struct {
		name      string
		container ContainerSummary
		wantKey   string
		wantStab  Stability
	}{
		{
			name:      "compose v1 underscore",
			container: ContainerSummary{ID: strings.Repeat("a", 64), Names: []string{"/myproject_sonarr_1"}},
			wantKey:   "name:myproject_sonarr_1",
			wantStab:  StabilityRecreateFragile,
		},
		{
			name:      "compose v2 hyphen",
			container: ContainerSummary{ID: strings.Repeat("b", 64), Names: []string{"/myproject-sonarr-1"}},
			wantKey:   "name:myproject-sonarr-1",
			wantStab:  StabilityRecreateFragile,
		},
		{
			name:      "swarm task",
			container: ContainerSummary{ID: strings.Repeat("c", 64), Names: []string{"/stack_svc.1.abcdefghij1234567890"}},
			wantKey:   "name:stack_svc.1.abcdefghij1234567890",
			wantStab:  StabilityTaskFragile,
		},
		{
			name:      "plain name stable",
			container: ContainerSummary{ID: strings.Repeat("d", 64), Names: []string{"/sonarr"}},
			wantKey:   "name:sonarr",
			wantStab:  StabilityStable,
		},
		{
			name:      "no name falls back to id",
			container: ContainerSummary{ID: "deadbeef00000000000000000000000000000000000000000000000000000000"},
			wantKey:   "id:deadbeef00000000000000000000000000000000000000000000000000000000",
			wantStab:  StabilityRecreateFragile,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c2 := c.container
			gotKey, gotStab := KeyForContainer(&c2)
			if gotKey != c.wantKey {
				t.Errorf("key = %q, want %q", gotKey, c.wantKey)
			}
			if gotStab != c.wantStab {
				t.Errorf("stability = %q, want %q", gotStab, c.wantStab)
			}
		})
	}
}
