package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/heisenberg-os/backend/internal/config"
	"github.com/heisenberg-os/backend/internal/db"
	"github.com/heisenberg-os/backend/internal/server"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found - using the process environment")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid configuration:\n%v", err)
	}

	gdb, err := db.Connect(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Migrate(gdb); err != nil {
		log.Fatal(err)
	}
	log.Println("database connected and migrated")

	if !cfg.SaulConfigured() {
		log.Println("Saul Goodman is offline: ANTHROPIC_API_KEY is not set")
	}
	if cfg.MLServiceURL == "" {
		log.Println("ML service disabled: built-in rules will be used")
	}

	srv := &http.Server{
		Addr:              net.JoinHostPort(cfg.Host, cfg.Port),
		Handler:           server.New(cfg),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second, // Saul may take up to ANTHROPIC_TIMEOUT_SEC
		IdleTimeout:       120 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("listening on %s (%s)", srv.Addr, cfg.Env)
		serverErr <- srv.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server stopped: %v", err)
		}
	case sig := <-stop:
		log.Printf("received %s, shutting down...", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
	}

	if raw, err := gdb.DB(); err == nil {
		_ = raw.Close()
	}
	log.Println("bye")
}
