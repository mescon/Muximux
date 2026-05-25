package handlers

import "strings"

// mapDockerError maps the verbose Docker daemon error strings to
// short, operator-readable messages suitable for a toast. The full
// daemon error continues to land in the audit log so an operator
// can grep the actual cause; this function only shapes the UI text.
func mapDockerError(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "port is already allocated"):
		return "Port already in use"
	case strings.Contains(s, "no such image"):
		return "Image not found"
	case strings.Contains(s, "No such container"):
		return "Container not found"
	case strings.Contains(s, "permission denied"):
		return "Permission denied (socket access)"
	case strings.Contains(s, "is already started"):
		return "Already running"
	case strings.Contains(s, "is not running"):
		return "Already stopped"
	case strings.Contains(s, "context deadline exceeded"):
		return "Docker daemon timeout"
	default:
		return "Action failed (see audit log)"
	}
}
