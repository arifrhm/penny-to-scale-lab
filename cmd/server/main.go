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
	"github.com/valyala/fasthttp"
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
		mode = "zeroalloc" // Default to v0.5.0 Zero-Alloc Engine
	}

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	// Mode 6, 5 & 4: FastHTTP Engine (RingBuffer, ZeroAlloc & FastHTTP)
	if mode == "ringbuffer" || mode == "zeroalloc" || mode == "fasthttp" {
		fastHandler := handler.NewFastHTTPHandler(workerID)
		zeroHandler := handler.NewZeroAllocHandler(workerID)
		ringHandler := handler.NewRingBufferHandler(workerID, 4)
		defer ringHandler.Close()

		versionStr := "0.6.0-ringbuffer"
		switch mode {
		case "zeroalloc":
			versionStr = "0.5.0-zeroalloc"
		case "fasthttp":
			versionStr = "0.4.0-fasthttp"
		}

		router := func(ctx *fasthttp.RequestCtx) {
			path := string(ctx.Path())
			switch path {
			case "/health":
				ctx.SetContentType("application/json")
				ctx.SetStatusCode(fasthttp.StatusOK)
				ctx.SetBodyString(fmt.Sprintf(`{"status":"HEALTHY","version":"%s","mode":"%s","ring_consumed":%d,"ring_dropped":%d}`, versionStr, mode, ringHandler.ConsumedCount(), ringHandler.DroppedCount()))
			case "/events":
				switch mode {
				case "ringbuffer":
					ringHandler.HandleFastHTTP(ctx)
				case "zeroalloc":
					zeroHandler.HandleFastHTTP(ctx)
				default:
					fastHandler.HandleFastHTTP(ctx)
				}
			case "/events/ringbuffer":
				ringHandler.HandleFastHTTP(ctx)
			case "/events/zeroalloc":
				zeroHandler.HandleFastHTTP(ctx)
			case "/events/fasthttp":
				fastHandler.HandleFastHTTP(ctx)
			default:
				ctx.SetStatusCode(fasthttp.StatusNotFound)
				ctx.SetBodyString("Not Found")
			}
		}

		server := &fasthttp.Server{
			Handler:            router,
			Name:               "PennyToScale-FastHTTP",
			Concurrency:        256 * 1024,
			ReadTimeout:        10 * time.Second,
			WriteTimeout:       10 * time.Second,
			MaxRequestBodySize: 10 * 1024 * 1024,
		}

		go func() {
			log.Printf("🚀 [V%s] Server listening on :%s (Mode: %s, Worker: %s)", versionStr, port, mode, workerID)
			if err := server.ListenAndServe(":" + port); err != nil {
				log.Fatalf("FastHTTP ListenAndServe failed: %v", err)
			}
		}()

		<-stopChan
		log.Println("🛑 Shutting down FastHTTP server gracefully...")
		if err := server.Shutdown(); err != nil {
			log.Printf("FastHTTP shutdown error: %v", err)
		}
		log.Println("✅ FastHTTP server stopped gracefully")
		return
	}

	// Net/HTTP Modes (Naive, Pooled, Sonic)
	naiveHandler := handler.NewNaiveHandler(workerID)
	pooledHandler := handler.NewPooledHandler(workerID)
	sonicHandler := handler.NewSonicHandler(workerID)

	mux := http.NewServeMux()

	versionStr := "0.3.0-sonic"
	switch mode {
	case "naive":
		versionStr = "0.1.0-naive"
	case "pooled":
		versionStr = "0.2.0-pooled"
	case "sonic":
		versionStr = "0.3.0-sonic"
	}

	// 1. Healthcheck
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"status":"HEALTHY","version":"%s","mode":"%s"}`, versionStr, mode)))
	})

	// 2. Primary Event Ingestion endpoint based on HANDLER_MODE
	switch mode {
	case "naive":
		mux.Handle("/events", naiveHandler)
	case "pooled":
		mux.Handle("/events", pooledHandler)
	default:
		mux.Handle("/events", sonicHandler)
	}

	// Explicit versioned endpoints
	mux.Handle("/events/naive", naiveHandler)
	mux.Handle("/events/pooled", pooledHandler)
	mux.Handle("/events/sonic", sonicHandler)

	// 3. Mount pprof for memory & cpu profiling
	mux.Handle("/debug/pprof/", http.DefaultServeMux)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

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
