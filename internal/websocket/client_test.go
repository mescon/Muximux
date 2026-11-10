package websocket

import (
	"encoding/json"
	"errors"
	"fmt"
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
	client := NewClient(hub, nil, Identity{IsAdmin: true})

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
		ServeWs(hub, w, r, Identity{IsAdmin: isAdmin})
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
	hub.BroadcastConfigUpdate()

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
	hub.BroadcastConfigUpdate()

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

	// findings.md M5: a missing Origin header used to pass CheckOrigin
	// because browsers always send one; but a non-browser tool with a
	// stolen session cookie could connect without the header and
	// bypass the same-origin check. CheckOrigin now requires Origin.
	t.Run("no origin rejected", func(t *testing.T) {
		dialer := websocket.Dialer{}
		_, resp, err := dialer.Dial(wsURL, nil) // No origin header
		if resp != nil {
			resp.Body.Close()
		}
		if err == nil {
			t.Error("expected error for missing Origin header")
		}
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
// C3: admin-only events (raw log entries) must not reach non-admin
// subscribers, while health updates continue to reach everyone.
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
	hub.BroadcastLogEntry(map[string]string{"message": "sensitive"})
	hub.BroadcastAppHealthUpdate("sonarr", map[string]string{"status": "up"}, false)
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

	if countContains(adminMsgs, "log_entry") != 1 {
		t.Errorf("admin expected 1 log_entry, got %d (%v)", countContains(adminMsgs, "log_entry"), adminMsgs)
	}
	if countContains(adminMsgs, "app_health_changed") != 1 {
		t.Errorf("admin expected 1 app_health_changed, got %d (%v)", countContains(adminMsgs, "app_health_changed"), adminMsgs)
	}
	if countContains(userMsgs, "log_entry") != 0 {
		t.Errorf("non-admin received admin-only broadcast: %v", userMsgs)
	}
	if countContains(userMsgs, "app_health_changed") != 1 {
		t.Errorf("non-admin expected 1 app_health_changed, got %v", userMsgs)
	}
}

// S-03: config_updated carries no config, so it goes to every client,
// including non-admins, who refetch the role-filtered GET /api/config.
func TestBroadcastConfigUpdate_ReachesNonAdmin(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Close()

	srv := testWSServerAs(t, hub, false)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
		"Origin": []string{srv.URL},
	})
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	resp.Body.Close()
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)
	hub.BroadcastConfigUpdate()

	if err = conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("failed to set read deadline: %v", err)
	}
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("non-admin did not receive config_updated: %v", err)
	}
	var event struct {
		Type    EventType       `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(msg, &event); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if event.Type != EventConfigUpdated {
		t.Errorf("expected %s, got %s", EventConfigUpdated, event.Type)
	}
	if string(event.Payload) != "{}" {
		t.Errorf("expected empty payload, got %s", event.Payload)
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

// dialSession opens a socket on a server that tags it with sessionID.
func dialSession(t *testing.T, hub *Hub, sessionID string) *websocket.Conn {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r, Identity{IsAdmin: true, SessionID: sessionID, UserID: "u-" + sessionID})
	}))
	t.Cleanup(srv.Close)
	conn, resp, err := (&websocket.Dialer{}).Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/", http.Header{"Origin": []string{srv.URL}})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	resp.Body.Close()
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestCloseSessions_ClosesOnlyMatchingSockets(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Close()

	ended := dialSession(t, hub, "sess-ended")
	kept := dialSession(t, hub, "sess-kept")
	anon := dialSession(t, hub, "")
	time.Sleep(50 * time.Millisecond)
	if hub.ClientCount() != 3 {
		t.Fatalf("expected 3 clients, got %d", hub.ClientCount())
	}

	hub.CloseSessions([]string{"sess-ended", ""})

	if err := ended.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, _, err := ended.ReadMessage()
	var ce *websocket.CloseError
	if !errors.As(err, &ce) || ce.Code != CloseSessionEnded {
		t.Fatalf("expected close code %d, got %v", CloseSessionEnded, err)
	}

	time.Sleep(50 * time.Millisecond)
	if hub.ClientCount() != 2 {
		t.Fatalf("expected 2 clients left, got %d", hub.ClientCount())
	}

	// The other sockets still receive broadcasts.
	hub.BroadcastConfigUpdate()
	for name, c := range map[string]*websocket.Conn{"kept": kept, "anon": anon} {
		if err := c.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, _, err := c.ReadMessage(); err != nil {
			t.Errorf("%s socket: expected a broadcast, got %v", name, err)
		}
	}
}

func TestCloseSessions_NoOpWhenEmptyOrClosed(t *testing.T) {
	hub := NewHub()
	hub.CloseSessions(nil) // no Run loop: must not block
	hub.Close()
	hub.CloseSessions([]string{"x"}) // closed hub: must not block
}

func TestRegister_RefusesSocketWhoseSessionIsGone(t *testing.T) {
	hub := NewHub()
	hub.SetSessionCheck(func(id string) (bool, bool) { return false, id == "sess-live" })
	go hub.Run()
	defer hub.Close()

	dead := dialSession(t, hub, "sess-dead")
	if err := dead.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, _, err := dead.ReadMessage()
	var ce *websocket.CloseError
	if !errors.As(err, &ce) || ce.Code != CloseSessionEnded {
		t.Fatalf("expected close code %d, got %v", CloseSessionEnded, err)
	}

	dialSession(t, hub, "sess-live")
	time.Sleep(50 * time.Millisecond)
	if hub.ClientCount() != 1 {
		t.Fatalf("expected only the live socket registered, got %d", hub.ClientCount())
	}
}

func TestCloseUser_ClosesOnlyThatUsersSockets(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Close()

	// dialSession sets UserID to "u-" + sessionID.
	target := dialSession(t, hub, "a")
	other := dialSession(t, hub, "b")
	time.Sleep(50 * time.Millisecond)

	hub.CloseUser("u-a")
	hub.CloseUser("") // ignored

	if err := target.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, _, err := target.ReadMessage()
	var ce *websocket.CloseError
	if !errors.As(err, &ce) || ce.Code == CloseSessionEnded {
		t.Fatalf("expected a plain close (reconnect), got %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1 client left, got %d", hub.ClientCount())
	}
	hub.BroadcastConfigUpdate()
	if err := other.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := other.ReadMessage(); err != nil {
		t.Fatalf("other user's socket should stay open: %v", err)
	}

	hub.Close()
	hub.CloseUser("u-b") // closed hub: must not block
}

func TestRegister_RederivesAdminFromSession(t *testing.T) {
	hub := NewHub()
	// The socket was opened as admin (dialSession), but the session's
	// user is no longer an admin by the time it registers.
	hub.SetSessionCheck(func(string) (bool, bool) { return false, true })
	go hub.Run()
	defer hub.Close()

	conn := dialSession(t, hub, "demoted")
	time.Sleep(50 * time.Millisecond)

	hub.BroadcastLogEntry(map[string]string{"msg": "secret"}) // admin-only
	hub.BroadcastHealthUpdate(map[string]string{"ok": "yes"})
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret") || !strings.Contains(string(data), string(EventHealthChanged)) {
		t.Fatalf("demoted socket got %s", data)
	}
}

func TestCloseSessions_NeverBlocksAndLosesNothing(t *testing.T) {
	hub := NewHub()
	// Run is not started yet: the requests pile up without blocking.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			hub.CloseSessions([]string{fmt.Sprintf("s%d", i)})
			hub.CloseUser(fmt.Sprintf("u%d", i))
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("CloseSessions/CloseUser blocked without a running hub")
	}
	hub.kickMu.Lock()
	n, m := len(hub.kickPending.sessions), len(hub.kickPending.users)
	hub.kickMu.Unlock()
	if n != 1000 || m != 1000 {
		t.Fatalf("queued %d sessions and %d users, want 1000 each", n, m)
	}

	// Once Run starts, the whole queue is processed.
	go hub.Run()
	defer hub.Close()
	conn := dialSession(t, hub, "s999")
	time.Sleep(50 * time.Millisecond)
	hub.CloseSessions([]string{"s999"})
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, _, err := conn.ReadMessage()
	var ce *websocket.CloseError
	if !errors.As(err, &ce) || ce.Code != CloseSessionEnded {
		t.Fatalf("expected close code %d, got %v", CloseSessionEnded, err)
	}
	hub.kickMu.Lock()
	left := len(hub.kickPending.sessions) + len(hub.kickPending.users)
	hub.kickMu.Unlock()
	if left != 0 {
		t.Fatalf("%d requests left in the queue", left)
	}
}
