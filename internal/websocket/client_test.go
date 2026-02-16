package websocket

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestNewClient(t *testing.T) {
	hub := NewHub()
	// Create a mock connection (nil for unit test of constructor)
	client := NewClient(hub, nil)

	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.hub != hub {
		t.Error("expected client hub to match")
	}
	if client.send == nil {
		t.Error("expected non-nil send channel")
	}
	if cap(client.send) != 256 {
		t.Errorf("expected send channel capacity 256, got %d", cap(client.send))
	}
}

// testWSServer creates a test HTTP server with WebSocket support.
func testWSServer(t *testing.T, hub *Hub) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r)
	}))
}

func TestServeWs(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	srv := testWSServer(t, hub)
	defer srv.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/"

	// Connect a WebSocket client
	dialer := websocket.Dialer{}
	conn, resp, err := dialer.Dial(wsURL, http.Header{
		"Origin": []string{srv.URL},
	})
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("expected 101, got %d", resp.StatusCode)
	}

	// Wait for registration
	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", hub.ClientCount())
	}

	// Close connection
	if err = conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
		t.Fatalf("failed to write close message: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients after close, got %d", hub.ClientCount())
	}
}

func TestClient_ReceiveBroadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	srv := testWSServer(t, hub)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/"

	dialer := websocket.Dialer{}
	conn, resp, err := dialer.Dial(wsURL, http.Header{
		"Origin": []string{srv.URL},
	})
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()
	defer resp.Body.Close()

	// Wait for registration
	time.Sleep(50 * time.Millisecond)

	// Broadcast a message
	hub.BroadcastConfigUpdate(map[string]string{"title": "Updated"})

	// Read the message from WebSocket
	if err = conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("failed to set read deadline: %v", err)
	}
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}

	if len(msg) == 0 {
		t.Error("expected non-empty message")
	}

	// Verify it contains the event type
	if !strings.Contains(string(msg), "config_updated") {
		t.Errorf("expected message to contain 'config_updated', got: %s", string(msg))
	}
}

func TestClient_MultipleConnections(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	srv := testWSServer(t, hub)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/"

	// Connect multiple clients
	connections := make([]*websocket.Conn, 5)
	for i := 0; i < 5; i++ {
		dialer := websocket.Dialer{}
		conn, resp, err := dialer.Dial(wsURL, http.Header{
			"Origin": []string{srv.URL},
		})
		if err != nil {
			t.Fatalf("failed to connect client %d: %v", i, err)
		}
		resp.Body.Close()
		connections[i] = conn
	}

	time.Sleep(100 * time.Millisecond)

	if hub.ClientCount() != 5 {
		t.Errorf("expected 5 clients, got %d", hub.ClientCount())
	}

	// Broadcast to all
	hub.BroadcastConfigUpdate("test-broadcast")

	// All should receive
	for i, conn := range connections {
		if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Errorf("client %d failed to set read deadline: %v", i, err)
			continue
		}
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("client %d failed to read: %v", i, err)
			continue
		}
		if len(msg) == 0 {
			t.Errorf("client %d received empty message", i)
		}
	}

	// Close all
	for _, conn := range connections {
		if err := conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
			t.Errorf("failed to write close message: %v", err)
		}
		conn.Close()
	}

	time.Sleep(200 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients after closing all, got %d", hub.ClientCount())
	}
}

func TestUpgrader_OriginCheck(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	srv := testWSServer(t, hub)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/"

	t.Run("same origin allowed", func(t *testing.T) {
		dialer := websocket.Dialer{}
		conn, resp, err := dialer.Dial(wsURL, http.Header{
			"Origin": []string{srv.URL},
		})
		if err != nil {
			t.Fatalf("same origin should be allowed: %v", err)
		}
		resp.Body.Close()
		conn.Close()
		time.Sleep(50 * time.Millisecond)
	})

	t.Run("no origin allowed", func(t *testing.T) {
		dialer := websocket.Dialer{}
		conn, resp, err := dialer.Dial(wsURL, nil) // No origin header
		if err != nil {
			t.Fatalf("no origin should be allowed: %v", err)
		}
		resp.Body.Close()
		conn.Close()
		time.Sleep(50 * time.Millisecond)
	})

	t.Run("cross origin rejected", func(t *testing.T) {
		dialer := websocket.Dialer{}
		_, resp, err := dialer.Dial(wsURL, http.Header{
			"Origin": []string{"http://evil.com"},
		})
		if resp != nil {
			resp.Body.Close()
		}
		if err == nil {
			t.Error("cross-origin should be rejected")
		}
	})
}

func TestClient_ConnectionDrop(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	srv := testWSServer(t, hub)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/"

	dialer := websocket.Dialer{}
	conn, resp, err := dialer.Dial(wsURL, http.Header{
		"Origin": []string{srv.URL},
	})
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", hub.ClientCount())
	}

	// Abruptly close the connection (simulate drop)
	conn.Close()

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients after drop, got %d", hub.ClientCount())
	}
}
