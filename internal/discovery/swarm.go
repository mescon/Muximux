package discovery

import (
	"fmt"
	"sort"
	"strings"
)

// swarmServiceName returns the Swarm service name a task container
// belongs to, or "" when the container carries no swarm service label.
// The service name is stable across task reschedules, unlike the task
// name (which embeds a slot number and a random task ID).
func swarmServiceName(c *ContainerSummary) string {
	return strings.TrimSpace(c.Labels[LabelSwarmServiceName])
}

// baseName is the name a suggestion derives defaults from: the Swarm
// service name for a task container, otherwise the container name.
func baseName(c *ContainerSummary) string {
	if svc := swarmServiceName(c); svc != "" {
		return svc
	}
	return c.PrimaryName()
}

// composeIdentity returns the Compose project and service labels,
// trimmed. Either may be empty.
func composeIdentity(c *ContainerSummary) (project, service string) {
	return strings.TrimSpace(c.Labels[LabelComposeProject]), strings.TrimSpace(c.Labels[LabelComposeService])
}

// composeKeyValue returns "<project>:<service>" or "" unless both
// labels are set. Compose project and service names cannot contain
// ":" or "/", so the value is safe inside a tracking key.
func composeKeyValue(c *ContainerSummary) string {
	project, service := composeIdentity(c)
	if project == "" || service == "" {
		return ""
	}
	return project + ":" + service
}

// Notes added to every swarm suggestion when the services endpoint cannot
// be read, so the operator knows why ports and deploy labels are missing.
const (
	noteSwarmWorker    = "Swarm worker node: service ports and deploy labels are not visible; set muximux.app.port and put labels under the service's labels: key"
	noteSwarmForbidden = "Swarm services are not readable (403; a socket proxy needs SERVICES=1): service ports and deploy labels are not visible"
)

// hasSwarmTasks reports whether any container is a Swarm task.
func hasSwarmTasks(containers []ContainerSummary) bool {
	for i := range containers {
		if _, ok := containers[i].Labels[LabelSwarmServiceID]; ok {
			return true
		}
	}
	return false
}

// mergeSwarmServices folds service labels and ports into the task
// containers in place. Container labels win over service labels, and
// service Labels win over ContainerLabels.
func mergeSwarmServices(containers []ContainerSummary, services []ServiceSummary) {
	byID := make(map[string]*ServiceSummary, len(services))
	for i := range services {
		byID[services[i].ID] = &services[i]
	}
	for i := range containers {
		c := &containers[i]
		id := c.Labels[LabelSwarmServiceID]
		svc, ok := byID[id]
		if id == "" || !ok {
			continue // not a task (Labels non-nil past here)
		}
		for _, src := range []map[string]string{svc.Labels, svc.ContainerLabels} {
			for k, v := range src {
				if _, has := c.Labels[k]; has {
					continue
				}
				c.Labels[k] = v
			}
		}
		for _, sp := range svc.Ports {
			found := false
			for j := range c.Ports {
				if c.Ports[j].PrivatePort == sp.PrivatePort && c.Ports[j].Type == sp.Type {
					if c.Ports[j].PublicPort == 0 {
						c.Ports[j].PublicPort = sp.PublicPort
					}
					found = true
					break
				}
			}
			if !found {
				c.Ports = append(c.Ports, sp)
			}
		}
	}
}

// collapseDuplicateKeys keeps one suggestion per tracking key, whatever the
// source (swarm replicas, a scaled compose service, two containers sharing a
// discovery id): a suggestion that is eligible for auto-import
// (AutoImportSkip == nil) wins over an ineligible one, ties go to the lowest
// ContainerName, and the survivor gets the note
// "N containers share this key; one app is imported". Callers should compute
// AutoImportSkip before collapsing so an ineligible replica cannot hide an
// importable one.
func collapseDuplicateKeys(suggestions []Suggestion) []Suggestion {
	idx := make(map[string]int, len(suggestions))
	var groups [][]int
	for i := range suggestions {
		g, ok := idx[suggestions[i].Key]
		if !ok {
			g = len(groups)
			idx[suggestions[i].Key] = g
			groups = append(groups, nil)
		}
		groups[g] = append(groups[g], i)
	}
	out := make([]Suggestion, 0, len(groups))
	for _, members := range groups {
		sort.SliceStable(members, func(a, b int) bool {
			sa, sb := &suggestions[members[a]], &suggestions[members[b]]
			if ea, eb := sa.AutoImportSkip == nil, sb.AutoImportSkip == nil; ea != eb {
				return ea
			}
			return sa.ContainerName < sb.ContainerName
		})
		s := suggestions[members[0]]
		if len(members) > 1 {
			s.Notes = append(append([]string(nil), s.Notes...), fmt.Sprintf("%d containers share this key; one app is imported", len(members)))
		}
		out = append(out, s)
	}
	return out
}
