package server

import (
	"fmt"
	"io"
	"log"
	"testing"
)

func newBenchConnection(h *Hub, user string) *Connection {
	conn := &Connection{
		hub:    h,
		send:   make(chan []byte, 1),
		log:    log.New(io.Discard, "", 0),
		UserID: user,
	}

	go func(ch <-chan []byte) {
		for range ch {
		}
	}(conn.send)

	return conn
}

func BenchmarkHubBroadcast(b *testing.B) {
	hub := NewHub()

	for i := range 1000 {
		conn := newBenchConnection(hub, fmt.Sprintf("user-%d", i))
		hub.addConnection(conn)
	}

	msg := Broadcast{Payload: []byte("bench")}
	b.ResetTimer()

	for b.Loop() {
		hub.sendToTargets(msg)
	}
}
