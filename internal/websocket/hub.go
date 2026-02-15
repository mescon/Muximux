package websocket

import (
	"encoding/json"
	"sync"

	"github.com/mescon/muximux/v3/internal/logging"
)

// EventType defines the type of WebSocket event
type EventType string

const (
	EventConfigUpdated    EventType = "config_updated"
	EventHealthChanged    EventType = "health_changed"
	EventAppHealthChanged EventType = "app_health_changed"
)

// Event represents a WebSocket event
type Event struct {
	Type    EventType   `json:"type"`
	Payload interface{} `json:"payload"`
}

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan Event
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan Event, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			logging.Info("WebSocket client connected", "source", "websocket", "total_clients", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			logging.Info("WebSocket client disconnected", "source", "websocket", "total_clients", len(h.clients))

		case event := <-h.broadcast:
			data, err := json.Marshal(event)
			if err != nil {
				logging.Error("Error marshaling event", "source", "websocket", "error", err)
				continue
			}

			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- data:
				default:
					// Client's send buffer is full, remove it
					h.mu.RUnlock()
					h.mu.Lock()
					delete(h.clients, client)
					close(client.send)
					h.mu.Unlock()
					h.mu.RLock()
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Register adds a client to the hub
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Broadcast sends an event to all connected clients
func (h *Hub) Broadcast(event Event) {
	h.broadcast <- event
}

// BroadcastConfigUpdate sends a config update event
func (h *Hub) BroadcastConfigUpdate(config interface{}) {
	h.Broadcast(Event{
		Type:    EventConfigUpdated,
		Payload: config,
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

// ClientCount returns the number of connected clients
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
