// Package main is the entry point for the GoSIP application
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

	"github.com/btafoya/gosip/internal/api"
	"github.com/btafoya/gosip/internal/config"
	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/internal/twilio"
	"github.com/btafoya/gosip/pkg/sip"
)

func main() {
	// Initialize structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting GoSIP", "version", "1.0.0")

	// Load configuration
	cfg := config.Load()

	// Ensure data directories exist
	if err := cfg.EnsureDirectories(); err != nil {
		slog.Error("Failed to create data directories", "error", err)
		os.Exit(1)
	}

	// Initialize database
	database, err := db.New(cfg.DBPath())
	if err != nil {
		slog.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	// Run migrations
	if err := database.Migrate(); err != nil {
		slog.Error("Failed to run database migrations", "error", err)
		os.Exit(1)
	}

	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize SIP server
	sipServer, err := sip.NewServer(sip.Config{
		Port:      cfg.SIPPort,
		UserAgent: config.DefaultUserAgent,
	}, database)
	if err != nil {
		slog.Error("Failed to initialize SIP server", "error", err)
		os.Exit(1)
	}

	// Start SIP server
	if err := sipServer.Start(ctx); err != nil {
		slog.Error("Failed to start SIP server", "error", err)
		os.Exit(1)
	}
	slog.Info("SIP server started", "port", cfg.SIPPort)

	// Initialize Twilio client
	twilioClient := twilio.NewClient(cfg)
	twilioClient.Start(ctx)
	defer twilioClient.Stop()
	slog.Info("Twilio client initialized")

	// Initialize and start HTTP server
	router := api.NewRouter(&api.Dependencies{
		Config: cfg,
		DB:     database,
		SIP:    sipServer,
		Twilio: twilioClient,
	})

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start HTTP server in goroutine
	go func() {
		slog.Info("HTTP server started", "port", cfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
			cancel()
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	slog.Info("Shutdown signal received, initiating graceful shutdown...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown HTTP server
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}

	// Stop SIP server
	sipServer.Stop()

	slog.Info("GoSIP shutdown complete")
}

