package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/stym06/rebuf/rebuf"
	"github.com/stym06/rebuf/server/internal/config"
	"github.com/stym06/rebuf/server/internal/database"
	"github.com/stym06/rebuf/server/internal/handlers"
	"github.com/stym06/rebuf/server/internal/middleware"
	"github.com/stym06/rebuf/server/internal/worker"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if err := run(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// Connect to database
	db, err := database.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer db.Close()

	// Run migrations
	if err := db.Migrate(ctx); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	slog.Info("database migrations complete")

	// Initialize rebuf WAL for delivery durability
	wal, err := rebuf.New(ctx, cfg.WALDir,
		rebuf.WithMaxLogSize(cfg.WALMaxLogSize),
		rebuf.WithMaxSegments(cfg.WALMaxSegments),
		rebuf.WithSyncStrategy(rebuf.SyncPeriodic),
		rebuf.WithFsyncTime(time.Second),
	)
	if err != nil {
		return fmt.Errorf("rebuf WAL: %w", err)
	}
	defer wal.Close()
	slog.Info("rebuf WAL initialized", "dir", cfg.WALDir)

	// Start delivery worker
	delivery := worker.NewDeliveryWorker(
		db.Pool, wal, cfg.DeliveryWorkers, cfg.DeliveryTimeout, cfg.MaxRetryAttempts,
	)
	delivery.Start()
	defer delivery.Stop()

	// Build router
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Public routes
	authHandler := handlers.NewAuthHandler(db.Pool, cfg.JWTSecret)
	r.Mount("/api/v1/auth", authHandler.Routes())

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWTSecret, db.Pool))

		// API keys
		apiKeyHandler := handlers.NewAPIKeyHandler(db.Pool)
		r.Mount("/api/v1/api-keys", apiKeyHandler.Routes())

		// Applications
		appHandler := handlers.NewApplicationHandler(db.Pool)
		r.Mount("/api/v1/app", appHandler.Routes())

		// Endpoints (nested under app)
		endpointHandler := handlers.NewEndpointHandler(db.Pool)
		r.Mount("/api/v1/app/{app_id}/endpoint", endpointHandler.Routes())

		// Messages (nested under app)
		messageHandler := handlers.NewMessageHandler(db.Pool, delivery)
		r.Mount("/api/v1/app/{app_id}/msg", messageHandler.Routes())

		// Event types
		eventTypeHandler := handlers.NewEventTypeHandler(db.Pool)
		r.Mount("/api/v1/event-type", eventTypeHandler.Routes())
	})

	// Start HTTP server
	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("server starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			cancel()
		}
	}()

	<-done
	slog.Info("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	return srv.Shutdown(shutdownCtx)
}
