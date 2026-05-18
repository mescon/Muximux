package discovery

import (
	"strings"
	"testing"
)

func sonarrContainer() ContainerSummary {
	return ContainerSummary{
		ID:    strings.Repeat("a", 64),
		Names: []string{"/sonarr"},
		Image: "linuxserver/sonarr:latest",
		State: "running",
		Ports: []ContainerPort{{PrivatePort: 8989, Type: "tcp"}},
		NetworkSettings: ContainerNetworks{
			Networks: map[string]ContainerNetwork{
				"media": {IPAddress: "10.0.0.5"},
			},
		},
	}
}

func TestSuggest_CatalogMatch_ContainerIPStrategy(t *testing.T) {
	c := sonarrContainer()
	s := suggestForContainer(&c, "container_ip", "", "")
	if s.Name != "Sonarr" {
		t.Errorf("Name = %q, want Sonarr", s.Name)
	}
	if s.Icon != "sonarr" {
		t.Errorf("Icon = %q, want sonarr", s.Icon)
	}
	if s.Group != "Media" {
		t.Errorf("Group = %q, want Media", s.Group)
	}
	if s.URL != "http://10.0.0.5:8989" {
		t.Errorf("URL = %q, want http://10.0.0.5:8989", s.URL)
	}
	if s.Confidence != "medium" {
		t.Errorf("Confidence = %q, want medium", s.Confidence)
	}
	if s.EffectiveStrategy != "container_ip" {
		t.Errorf("EffectiveStrategy = %q, want container_ip", s.EffectiveStrategy)
	}
}

func TestSuggest_LabelOverridesCatalog(t *testing.T) {
	c := sonarrContainer()
	c.Labels = map[string]string{
		"muximux.app.name":  "📺 Sonarr (TV)",
		"muximux.app.group": "Custom",
		"muximux.app.port":  "12345",
	}
	c.Ports = append(c.Ports, ContainerPort{PrivatePort: 12345, Type: "tcp"})
	s := suggestForContainer(&c, "container_ip", "", "")
	if s.Name != "📺 Sonarr (TV)" {
		t.Errorf("label name lost: %q", s.Name)
	}
	if s.Group != "Custom" {
		t.Errorf("label group lost: %q", s.Group)
	}
	if !strings.Contains(s.URL, ":12345") {
		t.Errorf("label port lost: %q", s.URL)
	}
	if s.Confidence != "high" {
		t.Errorf("Confidence = %q, want high (label-driven)", s.Confidence)
	}
}

func TestSuggest_CatalogPrefersStrategyOverridesGlobal(t *testing.T) {
	c := ContainerSummary{
		ID:    strings.Repeat("a", 64),
		Names: []string{"/swag"},
		Image: "linuxserver/swag:latest",
		Ports: []ContainerPort{{PrivatePort: 443, PublicPort: 443, Type: "tcp"}},
	}
	// Global strategy is container_ip; catalog says swag prefers
	// host_port. The suggestion should use host_port.
	s := suggestForContainer(&c, "container_ip", "", "")
	if s.EffectiveStrategy != "host_port" {
		t.Errorf("EffectiveStrategy = %q, want host_port (catalog override)", s.EffectiveStrategy)
	}
}

func TestSuggest_NoCatalogNoLabels_LowConfidence(t *testing.T) {
	c := ContainerSummary{
		ID:    strings.Repeat("b", 64),
		Names: []string{"/myapp"},
		Image: "private.io/myapp:1.0",
		Ports: []ContainerPort{{PrivatePort: 8080, Type: "tcp"}},
		NetworkSettings: ContainerNetworks{
			Networks: map[string]ContainerNetwork{"bridge": {IPAddress: "172.18.0.5"}},
		},
	}
	s := suggestForContainer(&c, "container_ip", "", "")
	if s.Confidence != "low" {
		t.Errorf("Confidence = %q, want low", s.Confidence)
	}
	if s.URL != "http://172.18.0.5:8080" {
		t.Errorf("URL = %q", s.URL)
	}
	if s.Name != "Myapp" {
		t.Errorf("Name = %q (expected titleized fallback)", s.Name)
	}
}

func TestSuggest_NoPortRequiresInput(t *testing.T) {
	c := ContainerSummary{
		ID:    strings.Repeat("c", 64),
		Names: []string{"/mystery"},
		Image: "private.io/mystery:1.0",
		// no ports
	}
	s := suggestForContainer(&c, "container_ip", "", "")
	if !s.RequiresInput {
		t.Errorf("RequiresInput = false, want true (no ports)")
	}
	if s.URL != "" {
		t.Errorf("URL = %q, want empty when no port", s.URL)
	}
}

func TestSuggest_HostPortStrategyNeedsPublicPort(t *testing.T) {
	c := ContainerSummary{
		ID:    strings.Repeat("d", 64),
		Names: []string{"/sonarr"},
		Image: "linuxserver/sonarr",
		// PrivatePort 8989 but PublicPort 0 (not published)
		Ports: []ContainerPort{{PrivatePort: 8989, Type: "tcp"}},
	}
	s := suggestForContainer(&c, "host_port", "192.168.1.10", "")
	if s.URL != "" {
		t.Errorf("URL = %q, want empty when host_port has no public port", s.URL)
	}
	if !s.RequiresInput {
		t.Errorf("RequiresInput = false, want true")
	}
}

func TestSuggest_HostPortStrategyWithPublicPort(t *testing.T) {
	c := ContainerSummary{
		ID:    strings.Repeat("e", 64),
		Names: []string{"/sonarr"},
		Image: "linuxserver/sonarr",
		Ports: []ContainerPort{{PrivatePort: 8989, PublicPort: 32768, Type: "tcp"}},
	}
	s := suggestForContainer(&c, "host_port", "192.168.1.10", "")
	if s.URL != "http://192.168.1.10:32768" {
		t.Errorf("URL = %q, want http://192.168.1.10:32768", s.URL)
	}
}

func TestSuggest_GatewayDomainSuggested(t *testing.T) {
	// dashboardDomain set, container name available.
	c := sonarrContainer()
	s := suggestForContainer(&c, "container_ip", "", "yi.se")
	if s.SuggestedDomain != "sonarr.yi.se" {
		t.Errorf("SuggestedDomain = %q, want sonarr.yi.se", s.SuggestedDomain)
	}

	// dashboardDomain empty - leave SuggestedDomain empty.
	s2 := suggestForContainer(&c, "container_ip", "", "")
	if s2.SuggestedDomain != "" {
		t.Errorf("SuggestedDomain = %q, want empty when no dashboardDomain", s2.SuggestedDomain)
	}

	// label override always wins.
	c.Labels = map[string]string{"muximux.app.gateway.domain": "myown.example.com"}
	s3 := suggestForContainer(&c, "container_ip", "", "yi.se")
	if s3.SuggestedDomain != "myown.example.com" {
		t.Errorf("SuggestedDomain label override: got %q", s3.SuggestedDomain)
	}
}

func TestSuggest_StabilityHintFromKey(t *testing.T) {
	c := sonarrContainer()
	c.Names = []string{"/myproj_sonarr_1"} // compose v1
	s := suggestForContainer(&c, "container_ip", "", "")
	if s.Stability != StabilityRecreateFragile {
		t.Errorf("Stability = %q, want recreate-fragile", s.Stability)
	}
}

func TestSuggest_UnknownLabelSurfacedAsNote(t *testing.T) {
	c := sonarrContainer()
	c.Labels = map[string]string{"muximux.app.typo": "bad"}
	s := suggestForContainer(&c, "container_ip", "", "")
	found := false
	for _, n := range s.Notes {
		if strings.Contains(n, "muximux.app.typo") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected unknown-label note, got %v", s.Notes)
	}
}

// Container-name catalog fallback (the "GitOps your apps" lenient
// match shipped after 3.1.0). When MatchImage misses because the
// image path is unrecognised, MatchByContainerName is consulted
// using the container's tokenised name. These tests pin the
// end-to-end suggestForContainer behaviour, not just the unit-
// level matcher.

func TestSuggest_ContainerNameFallback_HitsPrefixedName(t *testing.T) {
	// Image is a custom rebuild that won't match the catalog; the
	// container name however carries an explicit "sonarr" token.
	// The fallback should pick the Sonarr catalog entry and
	// auto-fill name + icon + group at medium confidence.
	c := ContainerSummary{
		ID:    strings.Repeat("a", 64),
		Names: []string{"/homelab-sonarr"},
		Image: "private.registry/me/arr-stack:custom",
		Ports: []ContainerPort{{PrivatePort: 8989, Type: "tcp"}},
		NetworkSettings: ContainerNetworks{
			Networks: map[string]ContainerNetwork{"media": {IPAddress: "10.0.0.7"}},
		},
	}
	s := suggestForContainer(&c, "container_ip", "", "")
	if s.Name != "Sonarr" {
		t.Errorf("Name = %q, want Sonarr from name-fallback catalog", s.Name)
	}
	if s.Icon != "sonarr" {
		t.Errorf("Icon = %q, want sonarr", s.Icon)
	}
	if s.Group != "Media" {
		t.Errorf("Group = %q, want Media", s.Group)
	}
	if s.Confidence != "medium" {
		t.Errorf("Confidence = %q, want medium (catalog matched via name fallback)", s.Confidence)
	}
}

func TestSuggest_ContainerNameFallback_MultiWordImage(t *testing.T) {
	// `home-assistant` splits into two tokens during tokenisation;
	// the adjacent-pair second pass must re-join them to match the
	// catalog entry. The image is unrecognised so the fallback is
	// the only path that can hit Home Assistant.
	c := ContainerSummary{
		ID:    strings.Repeat("b", 64),
		Names: []string{"/homelab-home-assistant"},
		Image: "private.registry/me/smart-home:custom",
		Ports: []ContainerPort{{PrivatePort: 8123, Type: "tcp"}},
		NetworkSettings: ContainerNetworks{
			Networks: map[string]ContainerNetwork{"iot": {IPAddress: "10.0.0.9"}},
		},
	}
	s := suggestForContainer(&c, "container_ip", "", "")
	if s.Name != "Home Assistant" {
		t.Errorf("Name = %q, want 'Home Assistant' from adjacent-token catalog match", s.Name)
	}
	if s.Confidence != "medium" {
		t.Errorf("Confidence = %q, want medium", s.Confidence)
	}
}

func TestSuggest_ContainerNameFallback_NoMatchStaysLow(t *testing.T) {
	// Neither the image nor any container-name token corresponds
	// to a catalog entry. The fallback must not invent a false
	// positive; confidence stays low and the titleized name takes
	// over.
	c := ContainerSummary{
		ID:    strings.Repeat("c", 64),
		Names: []string{"/my-bespoke-service"},
		Image: "private.io/bespoke:1.0",
		Ports: []ContainerPort{{PrivatePort: 8080, Type: "tcp"}},
		NetworkSettings: ContainerNetworks{
			Networks: map[string]ContainerNetwork{"bridge": {IPAddress: "172.18.0.5"}},
		},
	}
	s := suggestForContainer(&c, "container_ip", "", "")
	if s.Confidence != "low" {
		t.Errorf("Confidence = %q, want low (no catalog hit anywhere)", s.Confidence)
	}
	if s.Name != "My-bespoke-service" {
		t.Errorf("Name = %q, want titleized container name", s.Name)
	}
	if s.Group != "" {
		t.Errorf("Group = %q, want empty (no catalog match)", s.Group)
	}
}

func TestSuggest_ContainerNameFallback_DoesNotOverrideImageMatch(t *testing.T) {
	// When MatchImage already hit (canonical linuxserver/sonarr
	// image), the container-name fallback must not run a second
	// match. Renaming the container to suggest a different app
	// shouldn't switch the suggestion - the image is the
	// authoritative signal when present.
	c := sonarrContainer()
	c.Names = []string{"/this-is-actually-radarr"} // misleading rename
	s := suggestForContainer(&c, "container_ip", "", "")
	if s.Name != "Sonarr" {
		t.Errorf("Name = %q, want Sonarr (image wins over name)", s.Name)
	}
}
