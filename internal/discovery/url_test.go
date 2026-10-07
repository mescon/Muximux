package discovery

import (
	"strings"
	"testing"
)

func TestBuildURLForSuggestion_ContainerIP(t *testing.T) {
	c := &ContainerSummary{
		Names: []string{"/sonarr"},
		NetworkSettings: ContainerNetworks{
			Networks: map[string]ContainerNetwork{
				"media": {IPAddress: "10.0.0.5"},
			},
		},
	}
	got, err := buildURLForSuggestion("container_ip", c, 8989, "http", "")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != "http://10.0.0.5:8989" {
		t.Errorf("got %q", got)
	}
}

func TestBuildURLForSuggestion_ContainerIP_NoIPErrors(t *testing.T) {
	c := &ContainerSummary{Names: []string{"/sonarr"}}
	_, err := buildURLForSuggestion("container_ip", c, 8989, "http", "")
	if err == nil {
		t.Error("expected error when no IP available")
	}
}

func TestBuildURLForSuggestion_ContainerDNS(t *testing.T) {
	c := &ContainerSummary{Names: []string{"/sonarr"}}
	got, err := buildURLForSuggestion("container_dns", c, 8989, "http", "")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != "http://sonarr:8989" {
		t.Errorf("got %q", got)
	}
}

func TestBuildURLForSuggestion_HostPort(t *testing.T) {
	c := &ContainerSummary{
		Ports: []ContainerPort{{PrivatePort: 8989, PublicPort: 32768, Type: "tcp"}},
	}
	got, err := buildURLForSuggestion("host_port", c, 8989, "http", "192.168.1.10")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != "http://192.168.1.10:32768" {
		t.Errorf("got %q", got)
	}

	// Empty hostIP defaults to 127.0.0.1.
	got2, _ := buildURLForSuggestion("host_port", c, 8989, "http", "")
	if !strings.HasPrefix(got2, "http://127.0.0.1:") {
		t.Errorf("got %q, want 127.0.0.1 default", got2)
	}
}

func TestBuildURLForSuggestion_HostPort_NoPublicPortErrors(t *testing.T) {
	c := &ContainerSummary{
		Ports: []ContainerPort{{PrivatePort: 8989, PublicPort: 0}},
	}
	_, err := buildURLForSuggestion("host_port", c, 8989, "http", "192.168.1.10")
	if err == nil {
		t.Error("expected error when port not published")
	}
}

func TestBuildURLForSuggestion_HostDockerInternal(t *testing.T) {
	c := &ContainerSummary{
		Ports: []ContainerPort{{PrivatePort: 8989, PublicPort: 32768}},
	}
	got, err := buildURLForSuggestion("host_docker_internal", c, 8989, "https", "")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != "https://host.docker.internal:32768" {
		t.Errorf("got %q", got)
	}
}

func TestBuildURLForSuggestion_UnknownStrategy(t *testing.T) {
	c := &ContainerSummary{Names: []string{"/x"}}
	_, err := buildURLForSuggestion("moonbeam", c, 8989, "http", "")
	if err == nil {
		t.Error("expected error for unknown strategy")
	}
}

func TestPrimaryContainerIP_DeterministicAcrossCalls(t *testing.T) {
	// Map iteration order is randomised; primaryContainerIP should
	// pick the same one across calls (alphabetical).
	c := &ContainerSummary{
		NetworkSettings: ContainerNetworks{
			Networks: map[string]ContainerNetwork{
				"zzz": {IPAddress: "10.0.0.99"},
				"aaa": {IPAddress: "10.0.0.1"},
				"mmm": {IPAddress: "10.0.0.5"},
			},
		},
	}
	for i := 0; i < 50; i++ {
		ip := primaryContainerIP(c)
		if ip != "10.0.0.1" {
			t.Fatalf("non-deterministic on iteration %d: got %q", i, ip)
		}
	}
}

func TestParseFixedURL(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
		ok   bool
	}{
		{"https://sonarr.example.com", "https://sonarr.example.com", true},
		{"  http://10.0.0.5:8989/web  ", "http://10.0.0.5:8989/web", true},
		{"HTTPS://Sonarr.Example.com/", "HTTPS://Sonarr.Example.com/", true},
		{"sonarr.example.com", "", false},
		{"/relative", "", false},
		{"ftp://files.example.com", "", false},
		{"https://", "", false},
		{"", "", false},
		{"http://bad host/", "", false},
	} {
		got, ok := parseFixedURL(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("parseFixedURL(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestWithPath(t *testing.T) {
	for _, tc := range []struct{ base, path, want string }{
		{"http://10.0.0.5:80", "", "http://10.0.0.5:80"},
		{"http://10.0.0.5:80", "/", "http://10.0.0.5:80"},
		{"http://10.0.0.5:80", "/admin", "http://10.0.0.5:80/admin"},
		{"http://10.0.0.5:80", "admin/", "http://10.0.0.5:80/admin/"},
		{"http://10.0.0.5:80/", "/admin", "http://10.0.0.5:80/admin"},
		{"http://10.0.0.5:80", "  /web  ", "http://10.0.0.5:80/web"},
	} {
		if got := withPath(tc.base, tc.path); got != tc.want {
			t.Errorf("withPath(%q, %q) = %q, want %q", tc.base, tc.path, got, tc.want)
		}
	}
}

func TestResolveHealth(t *testing.T) {
	for _, tc := range []struct{ base, health, want string }{
		{"http://10.0.0.5:8989", "", ""},
		{"http://10.0.0.5:8989", "/api/v3/health", "http://10.0.0.5:8989/api/v3/health"},
		{"http://10.0.0.5:80/admin", "/ping", "http://10.0.0.5:80/ping"},
		{"http://10.0.0.5:8989", "https://status.example.com/sonarr", "https://status.example.com/sonarr"},
		{"", "/api/health", "/api/health"},
		{"http://10.0.0.5:8989", "health", "health"},
	} {
		if got := resolveHealth(tc.base, tc.health); got != tc.want {
			t.Errorf("resolveHealth(%q, %q) = %q, want %q", tc.base, tc.health, got, tc.want)
		}
	}
}
