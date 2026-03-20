package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"gs-api/internal/analytics"
	"gs-api/internal/config"
	"gs-api/internal/db"
)

func main() {
	cfg := config.FromEnv()

	sqlDB, err := db.Open(cfg.DBDSN)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer sqlDB.Close()

	analyticsSender := analytics.NewSender(cfg.Analytics)

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(analytics.Middleware(analyticsSender))
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Expose root-level OpenAPI spec for Scalar (/openapi.json -> /api/v1/openapi.json).
	r.Get("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/api/v1/openapi.json", http.StatusTemporaryRedirect)
	})

	// Register traditional Chi routes and Huma-powered routes.
	registerRoutes(r, sqlDB, &cfg)

	addr := ":" + cfg.Port
	server := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	shutdownSignals, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
	}()

	log.Printf("Go API listening on %s", addr)

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Println("server error:", err)
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if shutdownErr := analyticsSender.Shutdown(shutdownCtx); shutdownErr != nil {
				log.Println("analytics shutdown error:", shutdownErr)
			}
			os.Exit(1)
		}
	case <-shutdownSignals.Done():
		log.Println("shutdown signal received")
	}

	serverShutdownCtx, serverShutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer serverShutdownCancel()
	if err := server.Shutdown(serverShutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Println("server error:", err)
	}

	analyticsShutdownCtx, analyticsShutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer analyticsShutdownCancel()
	if err := analyticsSender.Shutdown(analyticsShutdownCtx); err != nil {
		log.Println("analytics shutdown error:", err)
	}
}
