package proxy

import (
	"errors"
	"strings"
	"testing"
)

// gateway_listen tests cover Phase H: the Caddyfile generator's
// behavior when an operator points server.gateway_listen at a
// non-default port (deployment behind another reverse proxy or on a
// box without CAP_NET_BIND_SERVICE).

func TestBuildCaddyfile_GatewayListen_RewritesSiteSelectorToHTTPPort(t *testing.T) {
	p := New(&Config{
		ListenAddr:    ":8080",
		InternalAddr:  "127.0.0.1:18080",
		GatewayListen: ":8443",
		GatewaySites: []GatewaySite{
			{Domain: "vw.example.com", BackendURL: "http://10.0.0.1:80", TLS: "auto"},
			{Domain: "upk.example.com", BackendURL: "http://10.0.0.2:3001", TLS: "none"},
		},
	})
	out := p.buildCaddyfile()

	// auto-https off so Caddy doesn't try to bind 80 for HTTP-01.
	if !strings.Contains(out, "auto_https off") {
		t.Errorf("expected auto_https off when GatewayListen is set; got:\n%s", out)
	}
	// Sites get explicit http:// + port. tls=auto downgrades silently
	// because HTTP-01 challenge cannot reach a high port.
	if !strings.Contains(out, "http://vw.example.com:8443 {") {
		t.Errorf("expected http://vw.example.com:8443 site selector; got:\n%s", out)
	}
	if !strings.Contains(out, "http://upk.example.com:8443 {") {
		t.Errorf("expected http://upk.example.com:8443 site selector; got:\n%s", out)
	}
	// No bare-domain site selector should appear (which would re-
	// enable auto-https for that name).
	if strings.Contains(out, "\nvw.example.com {") {
		t.Errorf("bare-domain selector leaked through; got:\n%s", out)
	}
}

func TestBuildCaddyfile_GatewayListen_CustomTLS_StaysHTTPS(t *testing.T) {
	// When the operator sets gateway_listen AND a site uses
	// TLS=custom (their own cert), Caddy should keep serving TLS on
	// the gateway port. Useful for split-DNS topologies where the
	// upstream proxy passes through TCP and Caddy handles certs.
	p := New(&Config{
		ListenAddr:    ":8080",
		InternalAddr:  "127.0.0.1:18080",
		GatewayListen: ":8443",
		GatewaySites: []GatewaySite{
			{Domain: "secure.example.com", BackendURL: "http://10.0.0.1:80",
				TLS: "custom", TLSCert: "/cert.pem", TLSKey: "/key.pem"},
		},
	})
	out := p.buildCaddyfile()

	if !strings.Contains(out, "https://secure.example.com:8443 {") {
		t.Errorf("custom TLS should stay https://; got:\n%s", out)
	}
	if !strings.Contains(out, "tls /cert.pem /key.pem") {
		t.Errorf("custom cert directive missing; got:\n%s", out)
	}
}

func TestBuildCaddyfile_NoGatewayListen_LegacyShape(t *testing.T) {
	// Empty GatewayListen retains the pre-Phase-H behavior: bare
	// domain selectors with auto-HTTPS for tls=auto/empty, http://
	// prefix for tls=none.
	p := New(&Config{
		ListenAddr:   ":8080",
		InternalAddr: "127.0.0.1:18080",
		GatewaySites: []GatewaySite{
			{Domain: "auto.example.com", BackendURL: "http://10.0.0.1:80", TLS: "auto"},
			{Domain: "plain.example.com", BackendURL: "http://10.0.0.2:80", TLS: "none"},
		},
	})
	out := p.buildCaddyfile()

	if !strings.Contains(out, "\nauto.example.com {") {
		t.Errorf("expected bare-domain selector for tls=auto; got:\n%s", out)
	}
	if !strings.Contains(out, "http://plain.example.com {") {
		t.Errorf("expected http:// prefix for tls=none; got:\n%s", out)
	}
	// auto_https should NOT be off here - we need it on to manage
	// the auto.example.com cert.
	if strings.Contains(out, "auto_https off") {
		t.Errorf("auto_https off should not appear when a tls=auto site exists; got:\n%s", out)
	}
}

func TestRemapCaddyLoadError_PermissionDenied(t *testing.T) {
	// Real shape of the wrapped error caddy.Load returns when the
	// process can't bind a privileged port. The remap must turn
	// this into something with the three concrete fixes (setcap /
	// root / gateway_listen).
	raw := errors.New("loading new config: http app module: start: listening on :443: listen tcp :443: bind: permission denied")
	got := remapCaddyLoadError(raw)
	msg := got.Error()
	if !strings.Contains(msg, "CAP_NET_BIND_SERVICE") {
		t.Errorf("remap should mention CAP_NET_BIND_SERVICE; got:\n%s", msg)
	}
	if !strings.Contains(msg, "server.gateway_listen") {
		t.Errorf("remap should mention server.gateway_listen as a fix; got:\n%s", msg)
	}
	if !strings.Contains(msg, ":443") {
		t.Errorf("remap should preserve the offending port (:443); got:\n%s", msg)
	}
	// errors.Is still passes through to the underlying error so
	// callers that match on the original can do so.
	if !errors.Is(got, raw) {
		t.Errorf("remap broke errors.Is chain")
	}
}

func TestRemapCaddyLoadError_OtherErrorsPassThrough(t *testing.T) {
	// Unrelated errors should keep their original shape (just
	// wrapped with "caddy load failed:") rather than getting the
	// privileged-port remediation hint.
	raw := errors.New("malformed json")
	got := remapCaddyLoadError(raw)
	if strings.Contains(got.Error(), "CAP_NET_BIND_SERVICE") {
		t.Errorf("unrelated errors should not get permission-hint; got:\n%s", got)
	}
	if !strings.Contains(got.Error(), "malformed json") {
		t.Errorf("original error message should remain visible; got:\n%s", got)
	}
}

func TestPortOnly_VariousInputs(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{":8443", ":8443"},
		{"0.0.0.0:8443", ":8443"},
		{"127.0.0.1:8443", ":8443"},
		{"[::]:8443", ":8443"},
		{"", ""},
		// A bare host with no port is not valid input but the helper
		// returns it verbatim so Caddy's parser surfaces the real
		// problem rather than us silently mangling the value.
		{"nonsense", "nonsense"},
	}
	for _, c := range cases {
		if got := portOnly(c.in); got != c.want {
			t.Errorf("portOnly(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestBuildCaddyfile_RequireAuth_EmitsForwardAuth pins the
// forward_auth directive emission when a site has RequireAuth=true.
// The directive's address is the proxy's InternalAddr so Caddy calls
// back into Muximux's Go server (not the public dashboard URL).
func TestBuildCaddyfile_RequireAuth_EmitsForwardAuth(t *testing.T) {
	p := New(&Config{
		ListenAddr:   ":8080",
		InternalAddr: "127.0.0.1:18080",
		GatewaySites: []GatewaySite{
			{Domain: "sonarr.example.com", BackendURL: "http://10.0.0.5:8989", TLS: "auto", RequireAuth: true},
			{Domain: "ungated.example.com", BackendURL: "http://10.0.0.6:80", TLS: "auto", RequireAuth: false},
		},
	})
	out := p.buildCaddyfile()

	if !strings.Contains(out, "forward_auth 127.0.0.1:18080 {") {
		t.Errorf("expected forward_auth pointing at InternalAddr; got:\n%s", out)
	}
	if !strings.Contains(out, "uri /api/auth/forward") {
		t.Errorf("expected uri directive in forward_auth block; got:\n%s", out)
	}
	if !strings.Contains(out, "copy_headers X-Muximux-User X-Muximux-Role") {
		t.Errorf("expected copy_headers in forward_auth block; got:\n%s", out)
	}
	// The ungated site must NOT have a forward_auth directive of
	// its own. We look for the second site's block specifically.
	ungatedBlockStart := strings.Index(out, "ungated.example.com")
	if ungatedBlockStart < 0 {
		t.Fatalf("ungated site not in output:\n%s", out)
	}
	ungatedBlock := out[ungatedBlockStart:]
	if strings.Contains(ungatedBlock, "forward_auth") {
		t.Errorf("ungated site should not have forward_auth; got:\n%s", ungatedBlock)
	}
}

func TestBuildCaddyfile_NoRequireAuth_OmitsForwardAuth(t *testing.T) {
	p := New(&Config{
		ListenAddr:   ":8080",
		InternalAddr: "127.0.0.1:18080",
		GatewaySites: []GatewaySite{
			{Domain: "x.example.com", BackendURL: "http://10.0.0.5:80", TLS: "auto"},
		},
	})
	out := p.buildCaddyfile()
	if strings.Contains(out, "forward_auth") {
		t.Errorf("forward_auth should not appear when no site has RequireAuth; got:\n%s", out)
	}
}
