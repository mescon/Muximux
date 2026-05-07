package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
)

func TestService_Scan_DisabledReturnsBlock(t *testing.T) {
	svc := NewService(&config.DiscoveryDockerConfig{Enabled: false})
	res := svc.Scan(context.Background(), "example.com")
	if !strings.Contains(res.ScanBlocked, "disabled") {
		t.Errorf("expected scan_blocked to mention 'disabled', got %q", res.ScanBlocked)
	}
	if len(res.Suggestions) != 0 {
		t.Errorf("disabled scan returned %d suggestions, want 0", len(res.Suggestions))
	}
}

func TestService_Scan_NoClientReturnsError(t *testing.T) {
	// Enabled but with malformed endpoint -> NewClient fails ->
	// Service.client stays nil. Scan should surface that as a
	// config error rather than panic.
	svc := NewService(&config.DiscoveryDockerConfig{
		Enabled:  true,
		Endpoint: "invalid://nope",
	})
	if svc.client != nil {
		t.Skip("Service unexpectedly produced a client; skipping nil-path coverage")
	}
	res := svc.Scan(context.Background(), "example.com")
	if res.Error == "" {
		t.Errorf("expected scan error for missing client; got empty")
	}
}

func TestService_Scan_BlocksWhenSelfDetectFailsAndNoNetworkFilter(t *testing.T) {
	// Enabled + container_ip + no network_filter + self-detect
	// fails -> ScanBlocked mentions remediation steps.
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.41/containers/json", func(w http.ResponseWriter, r *http.Request) {
		// Self-detect lists All:true; the scan path lists All:false.
		// Either way we return an empty set so InspectSelf yields nil.
		_ = json.NewEncoder(w).Encode([]ContainerSummary{})
	})
	socket, cleanup := fakeDockerOverUnix(t, mux)
	defer cleanup()

	svc := NewService(&config.DiscoveryDockerConfig{
		Enabled:         true,
		Endpoint:        "unix://" + socket,
		NetworkStrategy: "container_ip",
	})
	if svc.client == nil {
		t.Fatalf("client unexpectedly nil")
	}

	res := svc.Scan(context.Background(), "example.com")
	if !strings.Contains(res.ScanBlocked, "identify the container Muximux is running in") {
		t.Errorf("expected gating message, got blocked=%q error=%q", res.ScanBlocked, res.Error)
	}
}

func TestService_Scan_HappyPath_ReturnsSuggestions(t *testing.T) {
	// network_filter set -> bypasses self-detect gating. We then
	// rely on the daemon returning a single container.
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.41/containers/json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]ContainerSummary{
			{
				ID:    "abc",
				Names: []string{"/sonarr"},
				Image: "linuxserver/sonarr",
				NetworkSettings: ContainerNetworks{
					Networks: map[string]ContainerNetwork{"media": {IPAddress: "10.0.0.5"}},
				},
				Ports: []ContainerPort{{PrivatePort: 8989, Type: "tcp"}},
			},
		})
	})
	socket, cleanup := fakeDockerOverUnix(t, mux)
	defer cleanup()

	svc := NewService(&config.DiscoveryDockerConfig{
		Enabled:         true,
		Endpoint:        "unix://" + socket,
		NetworkStrategy: "container_ip",
		NetworkFilter:   "media",
	})
	res := svc.Scan(context.Background(), "example.com")
	if res.Error != "" {
		t.Fatalf("scan returned error: %s", res.Error)
	}
	if len(res.Suggestions) != 1 {
		t.Fatalf("got %d suggestions, want 1", len(res.Suggestions))
	}
	if res.Suggestions[0].ContainerName != "sonarr" {
		t.Errorf("suggestion ContainerName = %q, want sonarr", res.Suggestions[0].ContainerName)
	}
}
