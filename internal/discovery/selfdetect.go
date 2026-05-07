package discovery

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strings"
)

// SelfDetectMethod records which fallback strategy succeeded so the
// Settings UI can show "self-identified via /etc/hostname" or similar.
type SelfDetectMethod string

const (
	SelfDetectCgroup    SelfDetectMethod = "cgroup"    // /proc/self/cgroup contained a recognisable docker ID
	SelfDetectHostname  SelfDetectMethod = "hostname"  // /etc/hostname value cross-checked against a running container ID
	SelfDetectContainer SelfDetectMethod = "container" // hostname matched the first 12 chars of a container Id
	SelfDetectNone      SelfDetectMethod = "none"      // all fallbacks failed
)

// SelfInfo describes the container Muximux is running in. Empty when
// Muximux runs on the host (not in a container) - in that case
// Method is SelfDetectNone and the caller is expected to treat the
// network-strategy filters as "no membership info available".
type SelfInfo struct {
	ContainerID string
	Networks    []string // names of docker networks the container is attached to
	Method      SelfDetectMethod
}

// InspectSelf attempts to identify which container Muximux is running
// in by walking three increasingly-fragile fallback strategies. Each
// strategy is independently testable.
//
// If all three fail, returns ErrSelfNotIdentified - this is the
// documented signal for "we cannot apply the container_ip /
// container_dns network strategy without an explicit network_filter".
func (c *Client) InspectSelf(ctx context.Context) (*SelfInfo, error) {
	// Strategy 1: cgroups v1 (/proc/self/cgroup contains the
	// container ID at the end of one of the lines). Works on:
	//   - Older kernels with cgroup v1 still active
	//   - Some hybrid v1+v2 distros
	if id := readContainerIDFromCgroup(); id != "" {
		if info, err := c.fillSelfInfo(ctx, id, SelfDetectCgroup); err == nil {
			return info, nil
		}
	}

	// Strategy 2: /etc/hostname. Docker sets this to the short
	// container ID for default-config containers. Containers started
	// with --hostname=foo land here as "foo" and the cross-check
	// fails - that's the documented edge case.
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		if info, err := c.fillSelfInfo(ctx, hostname, SelfDetectHostname); err == nil {
			return info, nil
		}
	}

	// Strategy 3: list all containers and find one whose Id starts
	// with our hostname. Same brittleness as strategy 2 (depends on
	// hostname being a container ID prefix) but exhaustive.
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		containers, listErr := c.ListContainers(ctx, ListContainersOpts{All: true})
		if listErr == nil {
			for i := range containers {
				if strings.HasPrefix(containers[i].ID, hostname) {
					return c.fillSelfInfo(ctx, containers[i].ID, SelfDetectContainer)
				}
			}
		}
	}

	return nil, ErrSelfNotIdentified
}

// readContainerIDFromCgroup parses /proc/self/cgroup. On cgroup v1 the
// per-controller lines look like:
//
//	12:devices:/docker/<container-id>
//	11:cpuset:/docker/<container-id>
//
// On cgroup v2 there is a single line:
//
//	0::/system.slice/docker-<container-id>.scope
//
// We try both shapes and return the 64-hex-char container ID, or ""
// if neither shape matches. Returning "" is the right signal for
// "this isn't a docker container" or "cgroup namespace is opaque".
func readContainerIDFromCgroup() string {
	f, err := os.Open("/proc/self/cgroup")
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// Look for either "/docker/" (v1) or "docker-" (v2).
		if idx := strings.LastIndex(line, "/docker/"); idx != -1 {
			id := line[idx+len("/docker/"):]
			id = strings.TrimSuffix(id, ".scope")
			if isContainerID(id) {
				return id
			}
		}
		if idx := strings.LastIndex(line, "docker-"); idx != -1 {
			id := line[idx+len("docker-"):]
			id = strings.TrimSuffix(id, ".scope")
			if isContainerID(id) {
				return id
			}
		}
	}
	return ""
}

// isContainerID checks the 64-hex-char shape of a Docker container
// ID. Short IDs (12 chars) also pass since some daemons emit the
// short form in cgroup paths.
func isContainerID(s string) bool {
	if len(s) != 64 && len(s) != 12 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// fillSelfInfo runs InspectContainer on the candidate ID and parses
// the Networks list out of the result. If inspect fails the candidate
// is wrong; bubble the error so the caller falls through to the next
// strategy.
func (c *Client) fillSelfInfo(ctx context.Context, candidate string, method SelfDetectMethod) (*SelfInfo, error) {
	raw, err := c.InspectContainer(ctx, candidate)
	if err != nil {
		return nil, err
	}
	// The inspect payload nests networks under
	// .NetworkSettings.Networks - decode just that.
	var inspected struct {
		ID              string `json:"Id"`
		NetworkSettings struct {
			Networks map[string]struct{} `json:"Networks"`
		} `json:"NetworkSettings"`
	}
	if err := json.Unmarshal(raw, &inspected); err != nil {
		return nil, err
	}
	networks := make([]string, 0, len(inspected.NetworkSettings.Networks))
	for name := range inspected.NetworkSettings.Networks {
		networks = append(networks, name)
	}
	return &SelfInfo{
		ContainerID: inspected.ID,
		Networks:    networks,
		Method:      method,
	}, nil
}
