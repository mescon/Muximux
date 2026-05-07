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
