// Comando server: modulo de ingestion y procesamiento de GameStats (Go).
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"gamestats/internal/analysis"
	"gamestats/internal/api"
	"gamestats/internal/pipeline"
	"gamestats/internal/store"
	"gamestats/internal/validator"
)

func main() {
	logger := log.New(os.Stdout, "[gamestats-go] ", log.LstdFlags)

	cfgPath := getenv("EVENT_TYPES_PATH", "config/event_types.json")
	cfg, err := validator.LoadConfig(cfgPath)
	if err != nil {
		logger.Fatalf("cannot load event types: %v", err)
	}

	st := store.New()
	rej := store.NewRejectedLog(1000)

	var client *analysis.Client
	if url := os.Getenv("SCALA_URL"); url != "" {
		client = analysis.NewClient(url)
		logger.Printf("analysis module: %s", url)
	} else {
		logger.Println("SCALA_URL not set: finished matches will not be sent to Scala")
	}

	p := pipeline.New(validator.New(cfg), st, rej, client, pipeline.Options{
		IngestWorkers:     getenvInt("INGEST_WORKERS", 4),
		AnalysisWorkers:   getenvInt("ANALYSIS_WORKERS", 2),
		QueueSize:         getenvInt("QUEUE_SIZE", 1024),
		InactivityTimeout: getenvDuration("INACTIVITY_TIMEOUT", 0),
	}, logger)

	srv := &http.Server{
		Addr:              ":" + getenv("PORT", "8080"),
		Handler:           api.New(p, st, rej, 5<<20).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Printf("listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("http server: %v", err)
		}
	}()

	<-ctx.Done()
	logger.Println("shutting down...")

	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		logger.Printf("http shutdown: %v", err)
	}
	if err := p.Shutdown(shutCtx); err != nil {
		logger.Printf("pipeline shutdown: %v", err)
	}
	logger.Println("bye")
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func getenvDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
