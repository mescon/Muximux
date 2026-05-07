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
