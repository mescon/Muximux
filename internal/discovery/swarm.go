package discovery

import "strings"

// swarmServiceName returns the Swarm service name a task container
// belongs to, or "" when the container carries no swarm service label.
// The service name is stable across task reschedules, unlike the task
// name (which embeds a slot number and a random task ID).
func swarmServiceName(c *ContainerSummary) string {
	return strings.TrimSpace(c.Labels[LabelSwarmServiceName])
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
