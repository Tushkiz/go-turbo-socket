package server

import "sync"

type Hub struct {
	mu          sync.RWMutex
	connections map[*Connection]struct{}
}

func NewHub() *Hub {
	return &Hub{
		connections: make(map[*Connection]struct{}),
	}
}

func (h *Hub) Register(c *Connection) {
	h.mu.Lock()
	h.connections[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) Unregister(c *Connection) {
	h.mu.Lock()
	delete(h.connections, c)
	h.mu.Unlock()
}
