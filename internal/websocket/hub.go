package websocket

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/mescon/muximux/v3/internal/logging"
)

// EventType defines the type of WebSocket event
type EventType string

const (
	EventConfigUpdated      EventType = "config_updated"
	EventHealthChanged      EventType = "health_changed"
	EventAppHealthChanged   EventType = "app_health_changed"
	EventLogEntry           EventType = "log_entry"
	EventDockerStateChanged EventType = "docker_state_changed"
)

// Event represents a WebSocket event
type Event struct {
	Type    EventType   `json:"type"`
	Payload interface{} `json:"payload"`
	// adminOnly restricts the event to clients flagged as admin. Kept
	// unexported (no JSON tag) so the wire format is unchanged and clients
	// cannot learn that a sensitive event exists.
	adminOnly bool
}

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan Event
	register   chan *Client
	unregister chan *Client
	done       chan struct{}
	mu         sync.RWMutex
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan Event, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		done:       make(chan struct{}),
	}
}

// Close signals the hub's Run loop to exit. Safe to call multiple times.
// After Close returns any further Broadcast / Register / Unregister
// calls become non-blocking no-ops: each path selects on h.done so a
// caller racing with shutdown drops the message instead of wedging on
// an undrained channel (the broadcast buffer is finite, register and
// unregister are unbuffered).
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	select {
	case <-h.done:
		// already closed
	default:
		close(h.done)
	}
}

// Run starts the hub's main loop. Returns when Close is called.
func (h *Hub) Run() {
	for {
		select {
		case <-h.done:
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			total := len(h.clients)
			h.mu.Unlock()
			logging.Debug("WebSocket client connected", "source", "websocket", "total_clients", total)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			total := len(h.clients)
			h.mu.Unlock()
			logging.Debug("WebSocket client disconnected", "source", "websocket", "total_clients", total)

		case event := <-h.broadcast:
			data, err := json.Marshal(event)
			if err != nil {
				logging.Error("Error marshaling event", "source", "websocket", "error", err)
				continue
			}

			var toDrop []*Client
			h.mu.RLock()
			for client := range h.clients {
				// Admin-only events must not be delivered to
				// non-admin clients: the payload contains full
				// config (user table, API key hash, trusted
				// proxies) or raw log lines (audit usernames).
				if event.adminOnly && !client.isAdmin {
					continue
				}
				select {
				case client.send <- data:
				default:
					toDrop = append(toDrop, client)
				}
			}
			h.mu.RUnlock()

			if len(toDrop) > 0 {
				h.mu.Lock()
				for _, client := range toDrop {
					if _, ok := h.clients[client]; ok {
						delete(h.clients, client)
						close(client.send)
						logging.Warn("WebSocket client dropped: send buffer full", "source", "websocket")
					}
				}
				h.mu.Unlock()
			}
		}
	}
}

// Register adds a client to the hub. Becomes a no-op once Close has run.
func (h *Hub) Register(client *Client) {
	select {
	case <-h.done:
		return
	default:
	}
	select {
	case h.register <- client:
	case <-h.done:
	}
}

// Unregister removes a client from the hub. Becomes a no-op once Close has run.
func (h *Hub) Unregister(client *Client) {
	select {
	case <-h.done:
		return
	default:
	}
	select {
	case h.unregister <- client:
	case <-h.done:
	}
}

// Broadcast sends an event to all connected clients. Drops the event if the
// hub has been closed or the buffer is full at shutdown - blocking would
// wedge whichever goroutine raced with Close (config save, health poll,
// log fanout) and stall the surrounding HTTP handler.
func (h *Hub) Broadcast(event Event) {
	select {
	case <-h.done:
		return
	default:
	}
	select {
	case h.broadcast <- event:
	case <-h.done:
	}
}

// BroadcastConfigUpdate sends a config update event. Restricted to admin
// clients because the payload includes user records, API-key hashes, and
// trusted-proxy networks.
func (h *Hub) BroadcastConfigUpdate(config interface{}) {
	h.Broadcast(Event{
		Type:      EventConfigUpdated,
		Payload:   config,
		adminOnly: true,
	})
}

// BroadcastHealthUpdate sends a health update event
func (h *Hub) BroadcastHealthUpdate(health interface{}) {
	h.Broadcast(Event{
		Type:    EventHealthChanged,
		Payload: health,
	})
}

// BroadcastAppHealthUpdate sends an app-specific health update event
func (h *Hub) BroadcastAppHealthUpdate(appName string, health interface{}) {
	h.Broadcast(Event{
		Type: EventAppHealthChanged,
		Payload: map[string]interface{}{
			"app":    appName,
			"health": health,
		},
	})
}

// BroadcastLogEntry sends a log entry event. Restricted to admin clients
// because log lines include audit entries (usernames, client IPs) and
// panic stack traces.
func (h *Hub) BroadcastLogEntry(entry interface{}) {
	h.Broadcast(Event{
		Type:      EventLogEntry,
		Payload:   entry,
		adminOnly: true,
	})
}

// DockerStatePayload mirrors discovery.DockerState on the wire. Kept
// in this package (not discovery) so the websocket package stays
// independent of the docker discovery types - a cyclic import would
// otherwise pull discovery into websocket.
type DockerStatePayload struct {
	Status       string    `json:"status"`
	Health       string    `json:"health"`
	StartedAt    time.Time `json:"started_at,omitempty"`
	FinishedAt   time.Time `json:"finished_at,omitempty"`
	ExitCode     int       `json:"exit_code,omitempty"`
	RestartCount int       `json:"restart_count"`
	Image        string    `json:"image"`
}

// DockerStateChangedEvent is the payload of EventDockerStateChanged.
type DockerStateChangedEvent struct {
	AppName string             `json:"app_name"`
	State   DockerStatePayload `json:"state"`
}

// BroadcastDockerStateChanged fans a per-app state update out to every
// connected client. Not adminOnly: state visibility is for everyone
// authenticated, mirroring the existing HealthIndicator behaviour.
// Mutations are gated by the handler-side role/group check, not by
// event filtering. state is taken by pointer to avoid copying the
// (relatively heavy) payload on every call.
func (h *Hub) BroadcastDockerStateChanged(appName string, state *DockerStatePayload) {
	h.Broadcast(Event{
		Type: EventDockerStateChanged,
		Payload: DockerStateChangedEvent{
			AppName: appName,
			State:   *state,
		},
	})
}

// ClientCount returns the number of connected clients
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
