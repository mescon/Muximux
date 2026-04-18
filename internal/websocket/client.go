package websocket

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/mescon/muximux/v3/internal/logging"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// The Muximux WebSocket is only ever opened by the in-page SPA,
		// which runs on the same origin as /ws. Require Origin to be
		// present AND to match r.Host; a missing Origin header is
		// enough for a non-browser tool with a stolen session cookie
		// to bypass the same-origin check (findings.md M5).
		origin := r.Header.Get("Origin")
		if origin == "" {
			logging.Debug("WebSocket upgrade rejected: missing Origin header", "source", "websocket")
			return false
		}
		host := r.Host
		if origin == "http://"+host || origin == "https://"+host {
			return true
		}
		logging.Debug("WebSocket upgrade rejected: origin mismatch", "source", "websocket", "origin", origin, "host", host)
		return false
	},
}

// Client represents a WebSocket connection
type Client struct {
	hub     *Hub
	conn    *websocket.Conn
	send    chan []byte
	isAdmin bool
}

// NewClient creates a new WebSocket client. isAdmin, when true, allows the
// client to receive events flagged adminOnly (config updates, raw log
// lines). Callers are responsible for deriving this from the authenticated
// user's role.
func NewClient(hub *Hub, conn *websocket.Conn, isAdmin bool) *Client {
	return &Client{
		hub:     hub,
		conn:    conn,
		send:    make(chan []byte, 256),
		isAdmin: isAdmin,
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub
// This is primarily for receiving pings and handling disconnection
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	// A deadline that fails to set means the next read can hang
	// forever on a broken peer, so log the error instead of the
	// previous _ = discard pattern (findings.md L14).
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		logging.Debug("WebSocket SetReadDeadline failed", "source", "websocket", "error", err)
		return
	}
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logging.Warn("Unexpected WebSocket close", "source", "websocket", "error", err)
			}
			break
		}
		// Currently we don't process incoming messages from clients
		// This is primarily a server-to-client broadcast channel
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				logging.Debug("WebSocket SetWriteDeadline failed", "source", "websocket", "error", err)
				return
			}
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				logging.Debug("WebSocket write failed, closing connection", "source", "websocket", "error", err)
				return
			}
			w.Write(message)

			// Add queued messages to the current WebSocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				logging.Debug("WebSocket ping SetWriteDeadline failed", "source", "websocket", "error", err)
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ServeWs handles WebSocket requests from the peer. isAdmin tags the client
// as privileged so it receives admin-only broadcasts; non-admin clients
// continue to receive health updates but not config/log events.
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request, isAdmin bool) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logging.Error("WebSocket upgrade failed", "source", "websocket", "error", err)
		return
	}

	client := NewClient(hub, conn, isAdmin)
	hub.Register(client)

	// Start the client pumps in new goroutines
	go client.WritePump()
	go client.ReadPump()
}
