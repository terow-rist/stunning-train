package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"ride-hail/internal/common/config"
	"ride-hail/internal/common/db"
	"ride-hail/internal/common/log"
	"ride-hail/internal/common/rabbitmq"
	"ride-hail/internal/ride/adapters/repository"
	"syscall"
	"time"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.New("ride-service")
	log.Info(ctx, logger, "init_start", "Ride Service initializing...")

	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Error(ctx, logger, "config_load_fail", "Failed to load config file", err)
		os.Exit(1)
	}
	log.Info(ctx, logger, "config_loaded", "Configuration loaded successfully")

	dbPool, err := db.ConnectPostgres(ctx, cfg.DB)
	if err != nil {
		log.Error(ctx, logger, "connect_db_fail", "Failed to connect to database", err)
		os.Exit(1)
	}

	rmq := rabbitmq.NewMQ(cfg.RMQ, logger)
	if err := rmq.Connect(ctx); err != nil {
		log.Error(ctx, logger, "rmq_connect_fail", "Failed to connect rabbit MQ", err)
		os.Exit(1)
	}
	if err := rmq.DeclareTopology(); err != nil {
		log.Error(ctx, logger, "rmq_declare_topology_fail", "Failed to declare RMQ topology", err)
		os.Exit(1)
	}

	_ = repository.NewRideRepository(dbPool)

	log.Info(ctx, logger, "db_connected", "Successfully connected to database")

	fmt.Println(cfg)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Info(ctx, logger, "shutdown", "Ride Service shutting down...")

	time.Sleep(1 * time.Second)
	log.Info(ctx, logger, "shutdown_complete", "Service stopped successfully")
}
