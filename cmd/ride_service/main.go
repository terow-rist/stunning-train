package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ride-hail/internal/common/config"
	"ride-hail/internal/common/log"
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
	
	fmt.Println(cfg)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Info(ctx, logger, "shutdown", "Ride Service shutting down...")

	time.Sleep(1 * time.Second)
	log.Info(ctx, logger, "shutdown_complete", "Service stopped successfully")
}
