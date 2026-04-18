package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
)

func TestGetProxyStatus(t *testing.T) {
	tests := []struct {
		name            string
		proxyNil        bool
		serverConfig    config.ServerConfig
		expectedEnabled bool
		expectedTLS     bool
		expectedDomain  string
		expectedGateway string
	}{
		{
			name:     "proxy nil",
			proxyNil: true,
			serverConfig: config.ServerConfig{
				Listen: ":8080",
			},
			expectedEnabled: false,
			expectedTLS:     false,
		},
		{
			name:     "proxy disabled, no TLS",
			proxyNil: true,
			serverConfig: config.ServerConfig{
				Listen: ":8080",
			},
			expectedEnabled: false,
			expectedTLS:     false,
		},
		{
			name:     "TLS with domain",
			proxyNil: true,
			serverConfig: config.ServerConfig{
				Listen: ":443",
				TLS: config.TLSConfig{
					Domain: "example.com",
					Email:  "admin@example.com",
				},
			},
			expectedEnabled: false,
			expectedTLS:     true,
			expectedDomain:  "example.com",
		},
		{
			name:     "TLS with cert",
			proxyNil: true,
			serverConfig: config.ServerConfig{
				Listen: ":443",
				TLS: config.TLSConfig{
					Cert: "/path/to/cert.pem",
					Key:  "/path/to/key.pem",
				},
			},
			expectedEnabled: false,
			expectedTLS:     true,
		},
		{
			name:     "with gateway",
			proxyNil: true,
			serverConfig: config.ServerConfig{
				Listen:  ":8080",
				Gateway: "/etc/caddy/gateway.conf",
			},
			expectedEnabled: false,
			expectedGateway: "/etc/caddy/gateway.conf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewProxyHandler(nil, &tt.serverConfig)

			req := httptest.NewRequest(http.MethodGet, "/api/proxy/status", nil)
			w := httptest.NewRecorder()

			handler.GetStatus(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}

			var resp ProxyStatusResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.Enabled != tt.expectedEnabled {
				t.Errorf("expected enabled=%v, got %v", tt.expectedEnabled, resp.Enabled)
			}
			if resp.TLS != tt.expectedTLS {
				t.Errorf("expected tls=%v, got %v", tt.expectedTLS, resp.TLS)
			}
			if resp.Domain != tt.expectedDomain {
				t.Errorf("expected domain=%q, got %q", tt.expectedDomain, resp.Domain)
			}
			if resp.Gateway != tt.expectedGateway {
				t.Errorf("expected gateway=%q, got %q", tt.expectedGateway, resp.Gateway)
			}
		})
	}
}

func TestGetProxyStatusWrongMethod(t *testing.T) {
	handler := NewProxyHandler(nil, &config.ServerConfig{Listen: ":8080"})

	req := httptest.NewRequest(http.MethodPost, "/api/proxy/status", nil)
	w := httptest.NewRecorder()

	handler.GetStatus(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}
