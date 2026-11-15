package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/arifrhm/penny-to-scale-lab/internal/handler"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	workerID := os.Getenv("WORKER_ID")
	if workerID == "" {
		workerID = "node-lab-xeon-01"
	}

	mode := os.Getenv("HANDLER_MODE")
	if mode == "" {
		mode = "pooled" // Default to v0.2.0 pooled handler
	}

	naiveHandler := handler.NewNaiveHandler(workerID)
	pooledHandler := handler.NewPooledHandler(workerID)

	mux := http.NewServeMux()

	versionStr := "0.2.0-pooled"
	if mode == "naive" {
		versionStr = "0.1.0-naive"
	}

	// 1. Healthcheck
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"status":"HEALTHY","version":"%s","mode":"%s"}`, versionStr, mode)))
	})

	// 2. Primary Event Ingestion endpoint based on HANDLER_MODE
	if mode == "naive" {
		mux.Handle("/events", naiveHandler)
	} else {
		mux.Handle("/events", pooledHandler)
	}

	// Explicit versioned endpoints for direct testing
	mux.Handle("/events/naive", naiveHandler)
	mux.Handle("/events/pooled", pooledHandler)

	// 3. Mount pprof for memory & cpu profiling
	mux.Handle("/debug/pprof/", http.DefaultServeMux)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("🚀 [V%s] Server listening on :%s (Mode: %s, Worker: %s)", versionStr, port, mode, workerID)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server ListenAndServe failed: %v", err)
		}
	}()

	<-stopChan
	log.Println("🛑 Shutting down server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	log.Println("✅ Server stopped gracefully")
}
