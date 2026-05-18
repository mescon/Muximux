package discovery

import "testing"

func TestMatchImage_Exact(t *testing.T) {
	cases := []struct {
		image    string
		wantName string
		wantHit  bool
	}{
		{"linuxserver/sonarr", "Sonarr", true},
		{"linuxserver/sonarr:latest", "Sonarr", true},
		{"linuxserver/sonarr:v4.0.0", "Sonarr", true},
		{"linuxserver/sonarr@sha256:abc", "Sonarr", true},
		{"jellyfin/jellyfin:latest", "Jellyfin", true},
		{"plexinc/pms-docker:latest", "Plex", true},
		{"unknown/whatever:1.0", "", false},
	}
	for _, c := range cases {
		t.Run(c.image, func(t *testing.T) {
			got, ok := MatchImage(c.image)
			if ok != c.wantHit {
				t.Fatalf("MatchImage(%q) ok = %v, want %v", c.image, ok, c.wantHit)
			}
			if ok && got.Name != c.wantName {
				t.Errorf("Name = %q, want %q", got.Name, c.wantName)
			}
		})
	}
}

func TestMatchImage_LastSegmentFallback(t *testing.T) {
	// Operator-rebuilt image at a private registry. The last path
	// segment should still match a catalog entry.
	got, ok := MatchImage("ghcr.io/mycorp/sonarr:custom-build")
	if !ok {
		t.Fatal("expected last-segment fallback to match sonarr")
	}
	if got.Name != "Sonarr" {
		t.Errorf("Name = %q, want Sonarr", got.Name)
	}
}

func TestMatchByContainerName(t *testing.T) {
	cases := []struct {
		name     string
		wantName string
		wantHit  bool
	}{
		// Plain container name (no prefix).
		{"sonarr", "Sonarr", true},
		{"radarr", "Radarr", true},
		// Operator-prefixed names. The whole "GitOps your apps"
		// path depends on the catalog still firing here.
		{"homelab-sonarr", "Sonarr", true},
		{"homelab-radarr-arm64", "Radarr", true},
		{"homelab_radarr", "Radarr", true},
		{"homelab.plex", "Plex", true},
		{"prod-sonarr-stable", "Sonarr", true},
		// Compose adds a numeric suffix on recreate; the
		// tokeniser drops digit-only fragments so we still match.
		{"myproject_sonarr_1", "Sonarr", true},
		// Multi-word catalog images: home-assistant splits into
		// two tokens, but the adjacent-pair pass catches it.
		{"homelab-home-assistant", "Home Assistant", true},
		// Leading `/` (Docker engine name shape).
		{"/homelab-sonarr", "Sonarr", true},
		// Hostile false-positives: longer tokens that simply
		// contain a catalog name as a SUBSTRING must NOT match,
		// because we use exact-token comparison.
		{"transmissionic", "", false},
		{"sonarrr", "", false},
		// Truly unknown apps fall through.
		{"my-custom-app", "", false},
		// Empty name returns nothing.
		{"", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := MatchByContainerName(c.name)
			if ok != c.wantHit {
				t.Fatalf("MatchByContainerName(%q) ok = %v, want %v", c.name, ok, c.wantHit)
			}
			if ok && got.Name != c.wantName {
				t.Errorf("Name = %q, want %q", got.Name, c.wantName)
			}
		})
	}
}

func TestStripImageTag(t *testing.T) {
	cases := []struct{ in, want string }{
		{"foo", "foo"},
		{"foo:latest", "foo"},
		{"foo/bar", "foo/bar"},
		{"foo/bar:tag", "foo/bar"},
		{"registry:5000/foo", "registry:5000/foo"}, // colon in registry, no tag
		{"registry:5000/foo:tag", "registry:5000/foo"},
		{"foo@sha256:abc", "foo"},
		{"foo:tag@sha256:abc", "foo"},
	}
	for _, c := range cases {
		if got := stripImageTag(c.in); got != c.want {
			t.Errorf("stripImageTag(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPrefersStrategyOnFrontdoorImages(t *testing.T) {
	// SWAG / Nginx Proxy Manager / Caddy expect to bind host ports.
	for _, image := range []string{"linuxserver/swag", "jc21/nginx-proxy-manager", "caddy"} {
		entry, ok := MatchImage(image)
		if !ok {
			t.Fatalf("expected catalog hit for %s", image)
		}
		if entry.PrefersStrategy != "host_port" {
			t.Errorf("%s: PrefersStrategy = %q, want host_port", image, entry.PrefersStrategy)
		}
	}
}
