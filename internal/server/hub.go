package server

import (
	"log"
	"sync"
)

type Broadcast struct {
	TargetUsers []string
	Payload     []byte
}

type Hub struct {
	mu          sync.RWMutex
	connections map[*Connection]struct{}
	users       map[string]map[*Connection]struct{}

	register   chan *Connection
	unregister chan *Connection
	broadcast  chan Broadcast
}

func NewHub() *Hub {
	return &Hub{
		connections: make(map[*Connection]struct{}),
		users:       make(map[string]map[*Connection]struct{}),
		register:    make(chan *Connection, 128),
		unregister:  make(chan *Connection, 128),
		broadcast:   make(chan Broadcast, 128),
	}
}

func (h *Hub) Register(c *Connection) {
	h.register <- c
}

func (h *Hub) Unregister(c *Connection) {
	h.unregister <- c
}

func (h *Hub) Broadcast(msg Broadcast) {
	h.broadcast <- msg
}

func (h *Hub) Run(logger *log.Logger) {
	for {
		select {
		case conn := <-h.register:
			h.addConnection(conn)
			logger.Printf("registered user=%s conn=%p", conn.UserID, conn)

		case conn := <-h.unregister:
			h.removeConnection(conn)
			logger.Printf("unregistered user=%s conn=%p", conn.UserID, conn)

		case msg := <-h.broadcast:
			h.sendToTargets(msg)
			logger.Printf("broadcast to %d users", len(msg.TargetUsers))
		}
	}
}

func (h *Hub) addConnection(c *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.connections[c] = struct{}{}

	if c.UserID == "" {
		return
	}

	if _, ok := h.users[c.UserID]; !ok {
		h.users[c.UserID] = make(map[*Connection]struct{})
	}
	h.users[c.UserID][c] = struct{}{}

}
func (h *Hub) removeConnection(c *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.connections, c)

	if c.UserID == "" {
		return
	}

	if conns, ok := h.users[c.UserID]; ok {
		delete(conns, c)
		if len(conns) == 0 {
			delete(h.users, c.UserID)
		}
	}
}
func (h *Hub) sendToTargets(msg Broadcast) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(msg.TargetUsers) == 0 {
		for c := range h.connections {
			c.Enqueue(msg.Payload)
		}
		return
	}

	for _, user := range msg.TargetUsers {
		for c := range h.users[user] {
			c.Enqueue(msg.Payload)
		}
	}
}
