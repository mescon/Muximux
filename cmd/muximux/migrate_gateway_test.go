package main

import (
	"strings"
	"testing"
)

// TestMigrateCaddyfileToSites covers the migrator's happy paths and
// the "unrecognised directive" warning surface. Each case feeds a
// Caddyfile through the parser and asserts the structured result.
func TestMigrateCaddyfileToSites(t *testing.T) {
	t.Run("simple reverse_proxy block", func(t *testing.T) {
		src := `plex.example.com {
    reverse_proxy http://plex:32400
}`
		sites, warns, err := migrateCaddyfileToSites([]byte(src))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(warns) != 0 {
			t.Errorf("unexpected warnings: %v", warns)
		}
		if len(sites) != 1 {
			t.Fatalf("expected 1 site, got %d", len(sites))
		}
		if sites[0].Domain != "plex.example.com" || sites[0].BackendURL != "http://plex:32400" {
			t.Errorf("unexpected site: %+v", sites[0])
		}
		if sites[0].TLS != "" {
			t.Errorf("expected default TLS (auto), got %q", sites[0].TLS)
		}
	})

	t.Run("flush_interval -1 maps to streaming", func(t *testing.T) {
		src := `plex.example.com {
    reverse_proxy http://plex:32400 {
        flush_interval -1
    }
}`
		sites, _, err := migrateCaddyfileToSites([]byte(src))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(sites) != 1 || !sites[0].Streaming {
			t.Errorf("expected streaming=true, got %+v", sites)
		}
	})

	t.Run("header_up maps to proxy_headers", func(t *testing.T) {
		src := `sonarr.example.com {
    reverse_proxy http://sonarr:8989 {
        header_up X-Api-Key "abc-123"
    }
}`
		sites, _, err := migrateCaddyfileToSites([]byte(src))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(sites) != 1 {
			t.Fatalf("expected 1 site, got %d", len(sites))
		}
		got := sites[0].ProxyHeaders["X-Api-Key"]
		if got != "abc-123" {
			t.Errorf("X-Api-Key = %q, want abc-123", got)
		}
	})

	t.Run("X-Forwarded-* are dropped from proxy_headers", func(t *testing.T) {
		// The structured generator emits X-Forwarded-* itself, so
		// migrating an explicit header_up X-Forwarded-Proto should not
		// duplicate it in the YAML output.
		src := `sonarr.example.com {
    reverse_proxy http://sonarr:8989 {
        header_up X-Forwarded-Proto {scheme}
        header_up X-Api-Key "abc-123"
    }
}`
		sites, _, err := migrateCaddyfileToSites([]byte(src))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(sites) != 1 {
			t.Fatalf("expected 1 site, got %d", len(sites))
		}
		if _, leaked := sites[0].ProxyHeaders["X-Forwarded-Proto"]; leaked {
			t.Errorf("X-Forwarded-Proto should not have been migrated into proxy_headers")
		}
		if sites[0].ProxyHeaders["X-Api-Key"] != "abc-123" {
			t.Errorf("X-Api-Key not migrated: %+v", sites[0].ProxyHeaders)
		}
	})

	t.Run("response header X-Frame-Options delete maps to strip_frame_blockers", func(t *testing.T) {
		src := `embedded.example.com {
    reverse_proxy http://app:8080
    header -X-Frame-Options
}`
		sites, _, err := migrateCaddyfileToSites([]byte(src))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(sites) != 1 || !sites[0].StripFrameBlockers {
			t.Errorf("expected strip_frame_blockers=true, got %+v", sites)
		}
	})

	t.Run("listen :80 maps to tls=none", func(t *testing.T) {
		src := `internal.lan:80 {
    reverse_proxy http://app:8080
}`
		sites, _, err := migrateCaddyfileToSites([]byte(src))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(sites) != 1 || sites[0].TLS != "none" {
			t.Errorf("expected tls=none, got %+v", sites)
		}
	})

	t.Run("multiple upstreams keeps first and warns", func(t *testing.T) {
		src := `lb.example.com {
    reverse_proxy http://a:80 http://b:80
}`
		sites, warns, err := migrateCaddyfileToSites([]byte(src))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(sites) != 1 || sites[0].BackendURL != "http://a:80" {
			t.Errorf("expected first upstream kept, got %+v", sites)
		}
		if !anyContains(warns, "2 upstreams") {
			t.Errorf("expected multi-upstream warning, got %v", warns)
		}
	})

	t.Run("multi-host route keeps first and warns", func(t *testing.T) {
		src := `a.example.com, b.example.com {
    reverse_proxy http://shared:80
}`
		sites, warns, err := migrateCaddyfileToSites([]byte(src))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(sites) != 1 || sites[0].Domain != "a.example.com" {
			t.Errorf("expected first host kept, got %+v", sites)
		}
		if !anyContains(warns, "multiple hosts") {
			t.Errorf("expected multi-host warning, got %v", warns)
		}
	})

	t.Run("unsupported handler warns and skips", func(t *testing.T) {
		src := `respond.example.com {
    handle /* {
        respond "hi"
    }
}`
		sites, warns, err := migrateCaddyfileToSites([]byte(src))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(sites) != 0 {
			t.Errorf("expected the site to be skipped, got %+v", sites)
		}
		if !anyContains(warns, "static_response") {
			t.Errorf("expected unsupported-handler warning, got %v", warns)
		}
		if !anyContains(warns, "no reverse_proxy directive found") {
			t.Errorf("expected no-reverse-proxy warning, got %v", warns)
		}
	})

	t.Run("output is sorted by domain", func(t *testing.T) {
		src := `zeta.example.com {
    reverse_proxy http://z:80
}

alpha.example.com {
    reverse_proxy http://a:80
}

mike.example.com {
    reverse_proxy http://m:80
}`
		sites, _, err := migrateCaddyfileToSites([]byte(src))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(sites) != 3 {
			t.Fatalf("expected 3 sites, got %d", len(sites))
		}
		want := []string{"alpha.example.com", "mike.example.com", "zeta.example.com"}
		for i, w := range want {
			if sites[i].Domain != w {
				t.Errorf("site[%d].Domain = %q, want %q", i, sites[i].Domain, w)
			}
		}
	})

	t.Run("invalid Caddyfile surfaces parse error", func(t *testing.T) {
		_, _, err := migrateCaddyfileToSites([]byte(`unbalanced.example.com {`))
		if err == nil {
			t.Fatal("expected parse error for unbalanced braces")
		}
	})

	t.Run("default-port catch-all with reverse_proxy emits a warning instead of silent drop", func(t *testing.T) {
		// `:443 { reverse_proxy backend:8080 }` is a default-site
		// pattern — operator's primary backend, no host matcher.
		// The migrator can't produce a structured site without a
		// domain, but silently dropping the directive means the
		// operator's backend disappears from the YAML output. Warn
		// so they know to set a domain manually.
		src := `:443 {
    reverse_proxy http://backend:8080
}`
		sites, warns, err := migrateCaddyfileToSites([]byte(src))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(sites) != 0 {
			t.Errorf("catch-all route should not produce a site, got %+v", sites)
		}
		if !anyContains(warns, "no host matcher contains reverse_proxy") {
			t.Errorf("expected catch-all warning, got %v", warns)
		}
	})

	t.Run("upstream without scheme defaults to http://", func(t *testing.T) {
		src := `bare.example.com {
    reverse_proxy app:8080
}`
		sites, _, err := migrateCaddyfileToSites([]byte(src))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(sites) != 1 || sites[0].BackendURL != "http://app:8080" {
			t.Errorf("expected http://app:8080, got %+v", sites)
		}
	})
}

func anyContains(warns []string, sub string) bool {
	for _, w := range warns {
		if strings.Contains(w, sub) {
			return true
		}
	}
	return false
}
