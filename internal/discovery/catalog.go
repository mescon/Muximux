package discovery

import (
	"strings"

	"github.com/mescon/muximux/v3/internal/config"
)

// CatalogEntry is one row of the hand-curated image-to-app catalog.
// Operators see these suggestions in the Discover modal; they can
// edit any field before importing.
type CatalogEntry struct {
	// Image matches against the container's Image field. Matching is
	// done by stripping the registry/tag and comparing the
	// repo/name fragment, so "ghcr.io/foo/sonarr:v4" matches
	// "linuxserver/sonarr" via the trailing "sonarr".
	Image string

	Name        string // suggested app name (operator-friendly)
	Icon        string // dashboard-icons icon name (no extension)
	Group       string // suggested group ("Media", "Smart Home", ...)
	Port        int    // default container-internal port
	Scheme      string // "http" or "https"; default "http" if blank
	Path        string // optional sub-path (rarely used)
	HealthURL   string // relative health URL (default "/" if blank)
	Description string // shown as tooltip in the Discover modal

	// PrefersStrategy, when non-empty, overrides the global
	// network_strategy for suggestions matching this image. Useful
	// for frontdoor apps (swag, nginx-proxy-manager) that typically
	// sit on host_port even when other services use container_ip.
	PrefersStrategy config.NetworkStrategy
}

// builtinCatalog is the hand-curated list. Order matters: when two
// entries match the same image, the first-defined wins. We keep
// LinuxServer.io entries together because that's the most common
// homelab origin, and add bare-image entries for projects that
// publish under their own org.
//
// To extend without recompiling, operators can place a YAML file at
// data/discovery_catalog.yaml that gets merged on top at startup.
// Operator entries override built-ins by exact-image match.
var builtinCatalog = []CatalogEntry{
	// LinuxServer.io - *Arr stack
	{Image: "linuxserver/sonarr", Name: "Sonarr", Icon: "sonarr", Group: "Media", Port: 8989},
	{Image: "linuxserver/radarr", Name: "Radarr", Icon: "radarr", Group: "Media", Port: 7878},
	{Image: "linuxserver/bazarr", Name: "Bazarr", Icon: "bazarr", Group: "Media", Port: 6767},
	{Image: "linuxserver/prowlarr", Name: "Prowlarr", Icon: "prowlarr", Group: "Media", Port: 9696},
	{Image: "linuxserver/lidarr", Name: "Lidarr", Icon: "lidarr", Group: "Media", Port: 8686},
	{Image: "linuxserver/readarr", Name: "Readarr", Icon: "readarr", Group: "Media", Port: 8787},

	// Media servers
	{Image: "linuxserver/jellyfin", Name: "Jellyfin", Icon: "jellyfin", Group: "Media", Port: 8096},
	{Image: "jellyfin/jellyfin", Name: "Jellyfin", Icon: "jellyfin", Group: "Media", Port: 8096},
	{Image: "linuxserver/plex", Name: "Plex", Icon: "plex", Group: "Media", Port: 32400},
	{Image: "plexinc/pms-docker", Name: "Plex", Icon: "plex", Group: "Media", Port: 32400},
	{Image: "linuxserver/emby", Name: "Emby", Icon: "emby", Group: "Media", Port: 8096},
	{Image: "linuxserver/tautulli", Name: "Tautulli", Icon: "tautulli", Group: "Media", Port: 8181},

	// Download clients
	{Image: "linuxserver/qbittorrent", Name: "qBittorrent", Icon: "qbittorrent", Group: "Downloads", Port: 8080},
	{Image: "linuxserver/transmission", Name: "Transmission", Icon: "transmission", Group: "Downloads", Port: 9091},
	{Image: "linuxserver/deluge", Name: "Deluge", Icon: "deluge", Group: "Downloads", Port: 8112},
	{Image: "linuxserver/sabnzbd", Name: "SABnzbd", Icon: "sabnzbd", Group: "Downloads", Port: 8080},
	{Image: "linuxserver/nzbget", Name: "NZBGet", Icon: "nzbget", Group: "Downloads", Port: 6789},

	// Request managers
	{Image: "linuxserver/overseerr", Name: "Overseerr", Icon: "overseerr", Group: "Media", Port: 5055},
	{Image: "fallenbagel/jellyseerr", Name: "Jellyseerr", Icon: "jellyseerr", Group: "Media", Port: 5055},

	// Smart home / monitoring
	{Image: "homeassistant/home-assistant", Name: "Home Assistant", Icon: "home-assistant", Group: "Smart Home", Port: 8123},
	{Image: "ghcr.io/home-assistant/home-assistant", Name: "Home Assistant", Icon: "home-assistant", Group: "Smart Home", Port: 8123},
	{Image: "grafana/grafana", Name: "Grafana", Icon: "grafana", Group: "Monitoring", Port: 3000, Description: "Streaming dashboards: enable Streaming if you use live panels."},
	{Image: "prom/prometheus", Name: "Prometheus", Icon: "prometheus", Group: "Monitoring", Port: 9090},
	{Image: "louislam/uptime-kuma", Name: "Uptime Kuma", Icon: "uptime-kuma", Group: "Monitoring", Port: 3001},

	// Network / DNS
	{Image: "pihole/pihole", Name: "Pi-hole", Icon: "pi-hole", Group: "Network", Port: 80, Path: "/admin"},
	{Image: "adguard/adguardhome", Name: "AdGuard Home", Icon: "adguard-home", Group: "Network", Port: 80},
	{Image: "linuxserver/unifi-controller", Name: "UniFi Controller", Icon: "unifi", Group: "Network", Port: 8443, Scheme: "https"},

	// Reverse proxies (prefer host_port - they bind privileged ports)
	{Image: "linuxserver/swag", Name: "SWAG", Icon: "swag", Group: "Network", Port: 443, Scheme: "https", PrefersStrategy: "host_port"},
	{Image: "jc21/nginx-proxy-manager", Name: "Nginx Proxy Manager", Icon: "nginx-proxy-manager", Group: "Network", Port: 81, PrefersStrategy: "host_port"},
	{Image: "caddy", Name: "Caddy", Icon: "caddy", Group: "Network", Port: 80, PrefersStrategy: "host_port"},

	// Storage / files / sync
	{Image: "linuxserver/nextcloud", Name: "Nextcloud", Icon: "nextcloud", Group: "Files", Port: 443, Scheme: "https"},
	{Image: "nextcloud", Name: "Nextcloud", Icon: "nextcloud", Group: "Files", Port: 80},
	{Image: "linuxserver/syncthing", Name: "Syncthing", Icon: "syncthing", Group: "Files", Port: 8384},
	{Image: "syncthing/syncthing", Name: "Syncthing", Icon: "syncthing", Group: "Files", Port: 8384},
	{Image: "paperlessngx/paperless-ngx", Name: "Paperless-ngx", Icon: "paperless-ngx", Group: "Files", Port: 8000},

	// Photo / media archives
	{Image: "photoprism/photoprism", Name: "PhotoPrism", Icon: "photoprism", Group: "Media", Port: 2342},
	{Image: "ghcr.io/immich-app/immich-server", Name: "Immich", Icon: "immich", Group: "Media", Port: 3001},

	// Productivity / dev / ops
	{Image: "vaultwarden/server", Name: "Vaultwarden", Icon: "vaultwarden", Group: "Productivity", Port: 80},
	{Image: "linuxserver/code-server", Name: "code-server", Icon: "vscode", Group: "Development", Port: 8443, Scheme: "https"},
	{Image: "gitea/gitea", Name: "Gitea", Icon: "gitea", Group: "Development", Port: 3000},
	{Image: "portainer/portainer-ce", Name: "Portainer", Icon: "portainer", Group: "DevOps", Port: 9443, Scheme: "https"},
}

// MatchImage returns the catalog entry matching the given container
// image, or (zero, false) if no entry matches. Match strategy:
//
//  1. Strip tag (everything after the last ":") and digest (after "@").
//  2. Try exact match against each entry's Image (after the same strip).
//  3. Fall back to last-path-segment match, e.g. "ghcr.io/foo/sonarr"
//     matches a catalog entry whose Image ends with "/sonarr".
//
// Returns the FIRST matching entry, so order in builtinCatalog
// determines precedence between two matches.
func MatchImage(image string) (CatalogEntry, bool) {
	stripped := stripImageTag(image)
	for i := range builtinCatalog {
		entry := builtinCatalog[i]
		entryStripped := stripImageTag(entry.Image)
		if stripped == entryStripped {
			return entry, true
		}
	}
	// Fall back to last-segment match.
	imageBase := lastSegment(stripped)
	for i := range builtinCatalog {
		entry := builtinCatalog[i]
		if lastSegment(stripImageTag(entry.Image)) == imageBase {
			return entry, true
		}
	}
	return CatalogEntry{}, false
}

func stripImageTag(s string) string {
	if i := strings.IndexByte(s, '@'); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndexByte(s, ':'); i >= 0 {
		// Don't strip the colon in a port spec like "registry:5000/foo".
		// The colon-followed-by-tag form has no slash after the colon.
		if !strings.ContainsRune(s[i:], '/') {
			s = s[:i]
		}
	}
	return s
}

func lastSegment(s string) string {
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		return s[i+1:]
	}
	return s
}
