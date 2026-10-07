package discovery

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/mescon/muximux/v3/internal/config"
)

// urlFormat is the shared format string for building `<scheme>://<host>:<port>`
// URLs across all the network strategies. Centralised so a future
// IPv6-aware change (e.g. wrapping the host in brackets) only has
// to update one spot.
const urlFormat = "%s://%s:%d"

// buildURLForSuggestion constructs the App URL / Gateway BackendURL
// from container facts using the strategy on the suggestion. Strategy
// rules per dev/docker-discovery-plan.md "Network strategies":
//
//   - container_ip: scheme://<container's primary IP in our network>:<port>
//   - container_dns: scheme://<container name>:<port>
//   - host_port:    scheme://<host_ip or 127.0.0.1>:<published port>
//   - host_docker_internal: scheme://host.docker.internal:<published port>
//
// Returns an error when the strategy cannot be satisfied (e.g.
// container_ip but no IP, host_port but no published port).
func buildURLForSuggestion(strategy string, c *ContainerSummary, port int, scheme, hostIP string) (string, error) {
	if scheme == "" {
		scheme = "http"
	}
	switch config.NetworkStrategy(strategy) {
	case "", config.StrategyContainerIP:
		ip := primaryContainerIP(c)
		if ip == "" {
			return "", fmt.Errorf("container has no network IP for container_ip strategy")
		}
		return fmt.Sprintf(urlFormat, scheme, ip, port), nil

	case config.StrategyContainerDNS:
		name := c.PrimaryName()
		if name == "" {
			return "", fmt.Errorf("container has no name for container_dns strategy")
		}
		return fmt.Sprintf(urlFormat, scheme, name, port), nil

	case config.StrategyHostPort:
		hostPort := hostBindingForPort(c, port)
		if hostPort == 0 {
			return "", fmt.Errorf("container does not publish container port %d on host", port)
		}
		host := hostIP
		if host == "" {
			host = "127.0.0.1"
		}
		return fmt.Sprintf(urlFormat, scheme, host, hostPort), nil

	case config.StrategyHostDockerInternal:
		hostPort := hostBindingForPort(c, port)
		if hostPort == 0 {
			return "", fmt.Errorf("container does not publish container port %d on host", port)
		}
		return fmt.Sprintf("%s://host.docker.internal:%d", scheme, hostPort), nil

	default:
		return "", fmt.Errorf("unknown network strategy: %q", strategy)
	}
}

// primaryContainerIP picks the first non-empty IP from the
// container's NetworkSettings. Containers attached to multiple
// networks: first deterministic one wins. Empty string when the
// container has no networks (host network mode, or ephemeral).
func primaryContainerIP(c *ContainerSummary) string {
	// Iterate networks in alphabetical order so the choice is stable
	// across calls (Go map iteration is randomised).
	names := make([]string, 0, len(c.NetworkSettings.Networks))
	for n := range c.NetworkSettings.Networks {
		names = append(names, n)
	}
	// Sort with a small inline sort (avoid pulling sort just for this).
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j-1] > names[j]; j-- {
			names[j-1], names[j] = names[j], names[j-1]
		}
	}
	for _, n := range names {
		if ip := c.NetworkSettings.Networks[n].IPAddress; ip != "" {
			return ip
		}
	}
	return ""
}

// hostBindingForPort returns the host-published port matching the
// given container-internal port, or 0 when none.
func hostBindingForPort(c *ContainerSummary, containerPort int) int {
	for _, p := range c.Ports {
		if int(p.PrivatePort) == containerPort && p.PublicPort != 0 {
			return int(p.PublicPort)
		}
	}
	return 0
}

// formatPort returns the port as a string. Tiny helper kept here so
// the URL builder reads top-to-bottom.
func formatPort(port int) string { return strconv.Itoa(port) }

var _ = formatPort // keep available for future use

// parseFixedURL validates a muximux.app.url value: an absolute http or
// https URL with a host. Returns the trimmed value as written.
func parseFixedURL(v string) (string, bool) {
	v = strings.TrimSpace(v)
	u, err := url.Parse(v)
	if err != nil || u.Host == "" {
		return "", false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return v, true
	}
	return "", false
}

// withPath appends a muximux.app.path / catalog path to a
// container-derived URL. "" and "/" mean no path.
func withPath(base, path string) string {
	p := strings.TrimSpace(path)
	if p == "" || p == "/" {
		return base
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return strings.TrimRight(base, "/") + p
}

// resolveHealth turns a muximux.app.health value into a usable health
// address. A root-relative value ("/api/v3/health") is resolved against
// the container's origin; an absolute http(s) URL is kept; anything else
// is returned unchanged.
func resolveHealth(containerURL, health string) string {
	if health == "" {
		return ""
	}
	if _, ok := parseFixedURL(health); ok {
		return health
	}
	if !strings.HasPrefix(health, "/") {
		return health
	}
	base, err := url.Parse(containerURL)
	if err != nil || base.Host == "" {
		return health
	}
	return base.Scheme + "://" + base.Host + health
}
