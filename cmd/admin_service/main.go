package main

import (
	"context"
	"fmt"
	"os"
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
}