package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingInterval   = 30 * time.Second
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Connection struct {
	hub    *Hub
	ws     *websocket.Conn
	send   chan []byte
	log    *log.Logger
	UserID string
}

func ServeWS(h *Hub, logger *log.Logger, w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Printf("upgrade failed: %v", err)
		return
	}

	c := &Connection{
		hub:    h,
		ws:     conn,
		send:   make(chan []byte, 256),
		log:    logger,
		UserID: userID,
	}

	h.Register(c)

	go c.writeLoop()
	c.readLoop()
}

func (c *Connection) Enqueue(payload []byte) {
	select {
	case c.send <- payload:
	default:
		c.log.Printf("dropping message for user=%s: send buffer full", c.UserID)
		if c.ws != nil {
			c.ws.Close()
		}
	}
}

func (c *Connection) readLoop() {
	defer func() {
		c.hub.Unregister(c)
		if c.ws != nil {
			c.ws.Close()
		}
	}()

	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	type inbound struct {
		Type string          `json:"type"`
		To   []string        `json:"to"`
		Body json.RawMessage `json:"body"`
	}

	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				c.log.Printf("read error: %v", err)
			}
			break
		}

		c.log.Printf("read message: %s", string(message))

		var msg inbound
		if err := json.Unmarshal(message, &msg); err != nil {
			c.log.Printf("broadcast decode failed: %v", err)
		} else {
			c.log.Printf("parsed inbound type=%s targets=%v", msg.Type, msg.To)
			if msg.Type == "broadcast" {
				c.hub.Broadcast(Broadcast{TargetUsers: msg.To, Payload: msg.Body})
				continue
			}
		}

		c.send <- message // echo
	}

	close(c.send)
}

func (c *Connection) writeLoop() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.ws.WriteMessage(websocket.TextMessage, message); err != nil {
				c.log.Printf("write error: %v", err)
				return
			}
		case <-ticker.C:
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.log.Printf("ping error: %v", err)
				return
			}
		}
	}
}
