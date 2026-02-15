package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/mescon/muximux/v3/internal/logging"
)

// LogsHandler handles log-related API endpoints.
type LogsHandler struct{}

// NewLogsHandler creates a new LogsHandler.
func NewLogsHandler() *LogsHandler {
	return &LogsHandler{}
}

// GetRecent returns recent log entries from the ring buffer.
// Query params: ?limit=200&level=info&source=proxy
func (h *LogsHandler) GetRecent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	buf := logging.Buffer()
	if buf == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]logging.LogEntry{})
		return
	}

	limit := 200
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	entries := buf.Recent(limit)

	// Optional filtering by level
	if levelFilter := r.URL.Query().Get("level"); levelFilter != "" {
		filtered := make([]logging.LogEntry, 0, len(entries))
		for _, e := range entries {
			if strings.EqualFold(e.Level, levelFilter) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// Optional filtering by source
	if sourceFilter := r.URL.Query().Get("source"); sourceFilter != "" {
		filtered := make([]logging.LogEntry, 0, len(entries))
		for _, e := range entries {
			if strings.EqualFold(e.Source, sourceFilter) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}
