package websocket

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewHub(t *testing.T) {
	hub := NewHub()
	if hub == nil {
		t.Fatal("expected non-nil hub")
	}
	if hub.clients == nil {
		t.Error("expected initialized clients map")
	}
	if hub.broadcast == nil {
		t.Error("expected initialized broadcast channel")
	}
	if hub.register == nil {
		t.Error("expected initialized register channel")
	}
	if hub.unregister == nil {
		t.Error("expected initialized unregister channel")
	}
}

func TestHub_ClientCount_Empty(t *testing.T) {
	hub := NewHub()
	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", hub.ClientCount())
	}
}

func TestHub_RegisterUnregister(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create a mock client
	client := &Client{
		hub:  hub,
		send: make(chan []byte, 256),
	}

	// Register
	hub.Register(client)

	// Give the hub goroutine time to process
	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Errorf("expected 1 client after register, got %d", hub.ClientCount())
	}

	// Unregister
	hub.Unregister(client)

	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients after unregister, got %d", hub.ClientCount())
	}
}

func TestHub_UnregisterNonExistent(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create a client but don't register it
	client := &Client{
		hub:  hub,
		send: make(chan []byte, 256),
	}

	// Unregistering a non-registered client should not panic
	hub.Unregister(client)

	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", hub.ClientCount())
	}
}

func TestHub_Broadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create and register two clients
	client1 := &Client{hub: hub, send: make(chan []byte, 256)}
	client2 := &Client{hub: hub, send: make(chan []byte, 256)}

	hub.Register(client1)
	hub.Register(client2)
	time.Sleep(50 * time.Millisecond)

	// Broadcast an event
	event := Event{
		Type:    EventConfigUpdated,
		Payload: map[string]string{"key": "value"},
	}
	hub.Broadcast(event)

	// Wait for broadcast to be processed
	time.Sleep(50 * time.Millisecond)

	// Both clients should have received the message
	select {
	case msg := <-client1.send:
		var received Event
		if err := json.Unmarshal(msg, &received); err != nil {
			t.Fatalf("failed to unmarshal message: %v", err)
		}
		if received.Type != EventConfigUpdated {
			t.Errorf("expected event type %s, got %s", EventConfigUpdated, received.Type)
		}
	default:
		t.Error("client1 did not receive broadcast message")
	}

	select {
	case msg := <-client2.send:
		var received Event
		if err := json.Unmarshal(msg, &received); err != nil {
			t.Fatalf("failed to unmarshal message: %v", err)
		}
		if received.Type != EventConfigUpdated {
			t.Errorf("expected event type %s, got %s", EventConfigUpdated, received.Type)
		}
	default:
		t.Error("client2 did not receive broadcast message")
	}

	// Clean up
	hub.Unregister(client1)
	hub.Unregister(client2)
}

func TestHub_BroadcastConfigUpdate(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{hub: hub, send: make(chan []byte, 256)}
	hub.Register(client)
	time.Sleep(50 * time.Millisecond)

	hub.BroadcastConfigUpdate(map[string]string{"title": "New Title"})
	time.Sleep(50 * time.Millisecond)

	select {
	case msg := <-client.send:
		var event Event
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if event.Type != EventConfigUpdated {
			t.Errorf("expected type %s, got %s", EventConfigUpdated, event.Type)
		}
	default:
		t.Error("client did not receive config update")
	}

	hub.Unregister(client)
}

func TestHub_BroadcastHealthUpdate(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{hub: hub, send: make(chan []byte, 256)}
	hub.Register(client)
	time.Sleep(50 * time.Millisecond)

	hub.BroadcastHealthUpdate(map[string]string{"status": "healthy"})
	time.Sleep(50 * time.Millisecond)

	select {
	case msg := <-client.send:
		var event Event
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if event.Type != EventHealthChanged {
			t.Errorf("expected type %s, got %s", EventHealthChanged, event.Type)
		}
	default:
		t.Error("client did not receive health update")
	}

	hub.Unregister(client)
}

func TestHub_BroadcastAppHealthUpdate(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{hub: hub, send: make(chan []byte, 256)}
	hub.Register(client)
	time.Sleep(50 * time.Millisecond)

	hub.BroadcastAppHealthUpdate("myapp", map[string]string{"status": "healthy"})
	time.Sleep(50 * time.Millisecond)

	select {
	case msg := <-client.send:
		var event Event
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if event.Type != EventAppHealthChanged {
			t.Errorf("expected type %s, got %s", EventAppHealthChanged, event.Type)
		}
		// Check payload structure
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map payload, got %T", event.Payload)
		}
		if payload["app"] != "myapp" {
			t.Errorf("expected app 'myapp', got %v", payload["app"])
		}
	default:
		t.Error("client did not receive app health update")
	}

	hub.Unregister(client)
}

func TestHub_MultipleRegistrations(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	clients := make([]*Client, 10)
	for i := range clients {
		clients[i] = &Client{hub: hub, send: make(chan []byte, 256)}
		hub.Register(clients[i])
	}

	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 10 {
		t.Errorf("expected 10 clients, got %d", hub.ClientCount())
	}

	// Unregister half
	for i := 0; i < 5; i++ {
		hub.Unregister(clients[i])
	}

	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 5 {
		t.Errorf("expected 5 clients, got %d", hub.ClientCount())
	}

	// Unregister the rest
	for i := 5; i < 10; i++ {
		hub.Unregister(clients[i])
	}

	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", hub.ClientCount())
	}
}

func TestHub_BroadcastNoClients(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Broadcast with no clients should not panic or block
	hub.Broadcast(Event{Type: EventConfigUpdated, Payload: "test"})
	time.Sleep(50 * time.Millisecond)

	// If we get here without hanging, the test passes
}

func TestEvent_JSONSerialization(t *testing.T) {
	event := Event{
		Type:    EventConfigUpdated,
		Payload: map[string]string{"key": "value"},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if decoded.Type != EventConfigUpdated {
		t.Errorf("expected type %s, got %s", EventConfigUpdated, decoded.Type)
	}
}

func TestEventTypes(t *testing.T) {
	if EventConfigUpdated != "config_updated" {
		t.Errorf("expected 'config_updated', got %s", EventConfigUpdated)
	}
	if EventHealthChanged != "health_changed" {
		t.Errorf("expected 'health_changed', got %s", EventHealthChanged)
	}
	if EventAppHealthChanged != "app_health_changed" {
		t.Errorf("expected 'app_health_changed', got %s", EventAppHealthChanged)
	}
}
