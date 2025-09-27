package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tushkiz/go-turbo-socket/internal/server"
)

type broadcastRequest struct {
	Targets []string        `json:"targets"` // empty slice => broadcast to everybody
	Payload json.RawMessage `json:"payload"`
}

func main() {
	logger := log.New(os.Stdout, "[ws-server] ", log.LstdFlags|log.Lshortfile)
	hub := server.NewHub()
	go hub.Run(logger)

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		server.ServeWS(hub, logger, w, r)
	})
	mux.HandleFunc("/broadcast", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req broadcastRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			logger.Printf("invalid request payload: %s", r.Body)
			return
		}
		logger.Printf("broadcast request payload: %s", string(req.Payload))

		hub.Broadcast(server.Broadcast{
			TargetUsers: req.Targets,
			Payload:     req.Payload,
		})

		w.WriteHeader(http.StatusAccepted)
	})

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Printf("listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logger.Println("shutting down...")
	if err := srv.Shutdown(ctx); err != nil {
		logger.Printf("graceful shutdown failed: %v", err)
	}
	logger.Println("server stopped")
}
