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

	"ride-hail/internal/common/config"
	"ride-hail/internal/common/db"
	"ride-hail/internal/common/log"
	"ride-hail/internal/common/rabbitmq"
	"ride-hail/internal/driver_location/adapters/api"
	"ride-hail/internal/driver_location/adapters/queue"
	"ride-hail/internal/driver_location/adapters/repository"
	"ride-hail/internal/driver_location/app"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.New("driver-location-service")
	log.Info(ctx, logger, "init_start", "Driver & Location Service initializing...")

	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Error(ctx, logger, "config_load_fail", "Failed to load config file", err)
		os.Exit(1)
	}
	log.Info(ctx, logger, "config_loaded", "Configuration loaded successfully")

	// Postgres
	dbPool, err := db.ConnectPostgres(ctx, cfg.DB)
	if err != nil {
		log.Error(ctx, logger, "connect_db_fail", "Failed to connect to database", err)
		os.Exit(1)
	}
	log.Info(ctx, logger, "db_connected", "Connected to PostgreSQL")

	// RabbitMQ manager (handles reconnect loop internally)
	rmq := rabbitmq.NewMQ(cfg.RMQ, logger)
	if err := rmq.Connect(ctx); err != nil {
		log.Error(ctx, logger, "rmq_connect_fail", "Failed to connect to RabbitMQ", err)
		os.Exit(1)
	}
	if err := rmq.DeclareTopology(); err != nil {
		log.Error(ctx, logger, "rmq_declare_topology_fail", "Failed to declare RabbitMQ topology", err)
		os.Exit(1)
	}
	log.Info(ctx, logger, "rmq_ready", "RabbitMQ topology declared")

	// Repositories, publisher, and HTTP handler

	driverRepo := repository.NewDriverRepository(dbPool)
	locationRepo := repository.NewLocationRepository(dbPool)
	publisher := queue.NewDriverPublisher(rmq, logger)
	coreService := app.NewAppService(driverRepo, locationRepo, publisher)

	handler := api.NewHandler(coreService, logger)

	// Start HTTP server in goroutine
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", 3001),
		Handler: handler.Router(),
	}

	go func() {
		log.Info(ctx, logger, "http_server_start", "Starting HTTP server")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error(ctx, logger, "http_server_fail", "HTTP server failed", err)
			cancel()
		}
	}()

	// Wait for termination signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-stop:
		log.Info(ctx, logger, "shutdown_signal", "Shutdown signal received")
	case <-ctx.Done():
		log.Info(ctx, logger, "shutdown_ctx", "Context canceled")
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error(ctx, logger, "http_shutdown_fail", "HTTP server shutdown failed", err)
	} else {
		log.Info(ctx, logger, "http_shutdown", "HTTP server stopped")
	}

	// Close RMQ and DB
	rmq.Close()
	dbPool.Close()

	// wait a short moment for background goroutines/logs
	time.Sleep(500 * time.Millisecond)
	log.InfoX(logger, "shutdown_complete", "Driver & Location Service stopped")
	_ = slog.New(slog.NewJSONHandler(os.Stdout, nil)) // no-op to satisfy gofumpt style of imports usage
}
