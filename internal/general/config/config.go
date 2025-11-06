package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Database struct {
		Host     string
		Port     int
		User     string
		Password string
		Name     string // YAML key: "database"
	}
	RabbitMQ struct {
		Host     string
		Port     int
		User     string
		Password string
	}
	WebSocket struct {
		Port int
	}
	Services struct {
		RideServicePort           int
		DriverLocationServicePort int
		AdminServicePort          int
	}
	JWT struct {
		SecretKey string `yaml:"secret_key"`
	}
}

// LoadFromFile loads config from a YAML file to a Config struct, applies defaults, and validates required fields.
func LoadFromFile(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var cfg Config
	if err := parseYAML(file, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	applyDefaults(&cfg)

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// applyDefaults sets safe defaults for some fields.
func applyDefaults(cfg *Config) {
	// Database
	if cfg.Database.Host == "" {
		cfg.Database.Host = "localhost"
	}
	if cfg.Database.Port == 0 {
		cfg.Database.Port = 5432
	}

	// RabbitMQ
	if cfg.RabbitMQ.Host == "" {
		cfg.RabbitMQ.Host = "localhost"
	}
	if cfg.RabbitMQ.Port == 0 {
		cfg.RabbitMQ.Port = 5672
	}

	// WebSocket
	if cfg.WebSocket.Port == 0 {
		cfg.WebSocket.Port = 8080
	}

	// Services
	if cfg.Services.RideServicePort == 0 {
		cfg.Services.RideServicePort = 3000
	}
	if cfg.Services.DriverLocationServicePort == 0 {
		cfg.Services.DriverLocationServicePort = 3001
	}
	if cfg.Services.AdminServicePort == 0 {
		cfg.Services.AdminServicePort = 3004
	}

	if cfg.JWT.SecretKey == "" {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			// fallback: time-based bytes
			key = []byte(fmt.Sprintf("%d", time.Now().UnixNano()))
		}
		cfg.JWT.SecretKey = base64.StdEncoding.EncodeToString(key)
	}
}

// validate checks required fields and basic ranges.
func (c *Config) validate() error {
	var problems []string

	// DB
	if c.Database.Port <= 0 || c.Database.Port > 65535 {
		problems = append(problems, "database.port must be in 1..65535")
	}
	if c.Database.User == "" {
		problems = append(problems, "database.user is required")
	}
	if c.Database.Password == "" {
		problems = append(problems, "database.password is required")
	}
	if c.Database.Name == "" {
		problems = append(problems, "database.name is required")
	}

	// RabbitMQ
	if c.RabbitMQ.Port <= 0 || c.RabbitMQ.Port > 65535 {
		problems = append(problems, "rabbitmq.port must be in 1..65535")
	}
	if c.RabbitMQ.User == "" {
		problems = append(problems, "rabbitmq.user is required")
	}
	if c.RabbitMQ.Password == "" {
		problems = append(problems, "rabbitmq.password is required")
	}

	// WebSocket
	if c.WebSocket.Port <= 0 || c.WebSocket.Port > 65535 {
		problems = append(problems, "websocket.port must be in 1..65535")
	}

	// Services
	if c.Services.RideServicePort <= 0 || c.Services.RideServicePort > 65535 {
		problems = append(problems, "services.ride_service must be in 1..65535")
	}
	if c.Services.DriverLocationServicePort <= 0 || c.Services.DriverLocationServicePort > 65535 {
		problems = append(problems, "services.driver_location_service must be in 1..65535")
	}
	if c.Services.AdminServicePort <= 0 || c.Services.AdminServicePort > 65535 {
		problems = append(problems, "services.admin_service must be in 1..65535")
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}
