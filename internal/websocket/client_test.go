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
	client := NewClient(hub, nil, true)

	if client == nil {
		t.Fatal("expected non-nil client")
		return
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
	if !client.isAdmin {
		t.Error("expected isAdmin to be true")
	}
}

// testWSServer creates a test HTTP server that upgrades connections as
// admin. Tests that exercise role-based filtering use testWSServerAs
// directly so they can opt in to non-admin.
func testWSServer(t *testing.T, hub *Hub) *httptest.Server {
	t.Helper()
	return testWSServerAs(t, hub, true)
}

func testWSServerAs(t *testing.T, hub *Hub, isAdmin bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r, isAdmin)
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

// TestBroadcast_FilterByRole covers the gating introduced for findings.md
// C3: admin-only events (config snapshot, log entries) must not reach
// non-admin subscribers, while health updates continue to reach everyone.
func TestBroadcast_FilterByRole(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	adminSrv := testWSServerAs(t, hub, true)
	defer adminSrv.Close()
	userSrv := testWSServerAs(t, hub, false)
	defer userSrv.Close()

	dial := func(srv *httptest.Server) *websocket.Conn {
		wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/"
		conn, resp, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
			"Origin": []string{srv.URL},
		})
		if err != nil {
			t.Fatalf("dial failed: %v", err)
		}
		resp.Body.Close()
		return conn
	}

	adminConn := dial(adminSrv)
	defer adminConn.Close()
	userConn := dial(userSrv)
	defer userConn.Close()

	time.Sleep(50 * time.Millisecond)
	if hub.ClientCount() != 2 {
		t.Fatalf("expected 2 clients, got %d", hub.ClientCount())
	}

	// Fire both events first so the user connection's read stream has at
	// most the health update (the admin-only event was filtered before the
	// send channel). Then assert the admin saw both in order and the user
	// saw exactly the health update.
	hub.BroadcastConfigUpdate(map[string]string{"title": "sensitive"})
	hub.BroadcastAppHealthUpdate("sonarr", map[string]string{"status": "up"})
	time.Sleep(200 * time.Millisecond)

	readAll := func(conn *websocket.Conn) []string {
		var msgs []string
		for {
			_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
			_, m, readErr := conn.ReadMessage()
			if readErr != nil {
				return msgs
			}
			msgs = append(msgs, string(m))
		}
	}

	adminMsgs := readAll(adminConn)
	userMsgs := readAll(userConn)

	countContains := func(haystacks []string, needle string) int {
		n := 0
		for _, h := range haystacks {
			if strings.Contains(h, needle) {
				n++
			}
		}
		return n
	}

	if countContains(adminMsgs, "config_updated") != 1 {
		t.Errorf("admin expected 1 config_updated, got %d (%v)", countContains(adminMsgs, "config_updated"), adminMsgs)
	}
	if countContains(adminMsgs, "app_health_changed") != 1 {
		t.Errorf("admin expected 1 app_health_changed, got %d (%v)", countContains(adminMsgs, "app_health_changed"), adminMsgs)
	}
	if countContains(userMsgs, "config_updated") != 0 {
		t.Errorf("non-admin received admin-only broadcast: %v", userMsgs)
	}
	if countContains(userMsgs, "app_health_changed") != 1 {
		t.Errorf("non-admin expected 1 app_health_changed, got %v", userMsgs)
	}
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
