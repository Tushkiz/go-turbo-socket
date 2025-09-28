package server

import (
	"fmt"
	"io"
	"log"
	"sync"
	"testing"
	"time"
)

func newTestConnection(user string, done *sync.WaitGroup) *Connection {
	conn := &Connection{
		send:   make(chan []byte, 1),
		UserID: user,
		log:    log.New(io.Discard, "", 0),
	}

	go func() {
		defer done.Done()
		<-conn.send
	}()

	return conn
}

func TestHubConcurrentBroadcast(t *testing.T) {
	hub := NewHub()

	logger := log.New(io.Discard, "", 0)
	go hub.Run(logger)

	const clients = 100
	var wg sync.WaitGroup
	wg.Add(clients)

	conns := make([]*Connection, 0, clients)
	for i := 0; i < clients; i++ {
		conn := newTestConnection(fmt.Sprintf("user-%d", i), &wg)
		conns = append(conns, conn)
	}

	// register in parallel to exercise hub.register channel
	var reg sync.WaitGroup
	reg.Add(clients)
	for _, conn := range conns {
		go func(c *Connection) {
			defer reg.Done()
			hub.Register(c)
		}(conn)
	}
	reg.Wait()

	deadline := time.Now().Add(time.Second)
	for {
		hub.mu.RLock()
		registered := len(hub.connections)
		hub.mu.RUnlock()
		if registered == clients {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d of %d connections registered", registered, clients)
		}
		time.Sleep(10 * time.Millisecond)
	}

	hub.Broadcast(Broadcast{Payload: []byte("hi")})

	done := make(chan struct{})
	go func() {
		wg.Wait() // each connection decrements when it receives
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("broadcast never reached all connections")
	}
}
