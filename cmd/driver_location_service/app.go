package driverlocationservice

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"ride-hail/internal/general/config"
	"ride-hail/internal/general/jwt"
	"ride-hail/internal/general/logger"
	"ride-hail/internal/general/postgres"
	"ride-hail/internal/general/rabbitmq"
	"ride-hail/internal/general/websocket"
	"ride-hail/internal/software/dandl/handler"
	"ride-hail/internal/software/dandl/service"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Run(ctx context.Context, prefetch, maxConcurrent int) error {
	// set up a new logger for driver & location service with a static request ID for startup logs
	logger := logger.New("driver-location-service")
	ctx = logger.WithRequestID(ctx, "startup-001")

	// load configuration
	cfg, err := config.LoadFromFile("./config/config.yaml")
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// set up a Postgres connection pool
	pool, err := postgres.NewPool(ctx, cfg, logger)
	if err != nil {
		logger.Error(ctx, "db_connection_failed", "Failed to initialize Postgres pool", err, nil)
		return err
	}
	defer pool.Close()

	// connect to RabbitMQ
	rmq, err := rabbitmq.ConnectRabbitMQ(ctx, cfg, logger)
	if err != nil {
		logger.Error(ctx, "rabbitmq_connection_failed", "Failed to connect to RabbitMQ", err, nil)
		return err
	}
	defer rmq.Close()

	// set up the RabbitMQ publisher
	pub := &rabbitmq.MQPublisher{Client: rmq}

	// set up the JWT manager
	jwtManager := jwt.NewManager(cfg.JWT.SecretKey, 2*time.Hour)

	// set up the necessary repos
	uow := postgres.NewUnitOfWork(pool)
	driverRepo := postgres.NewDriverRepo()
	driverSessionRepo := postgres.NewDriverSessionRepo()
	locHistoryRepo := postgres.NewLocationHistoryRepo()
	coordsRepo := postgres.NewCoordinatesRepo(locHistoryRepo)
	rideRepo := postgres.NewRideRepo()
	rideEventRepo := postgres.NewRideEventRepo()

	// set up the websocket handler
	ws := websocket.NewWebSocket(logger, jwtManager, pub, coordsRepo, rideRepo)

	// set up the ride service
	svc := service.NewDriverLocationService(logger, uow, driverRepo, driverSessionRepo, coordsRepo, locHistoryRepo, rideRepo, rideEventRepo, pub, rmq, ws)

	// start the background RabbitMQ consumer that is used for matching the ride requests
	svc.StartBackgroundConsumer(ctx)

	// set up the HTTP handler and its routes
	mux := http.NewServeMux()
	httpHandler := handler.NewDriverHTTPHandler(svc, logger, jwtManager, ws)
	httpHandler.RegisterRoutes(mux)

	// concurrency limiter (global) — blocks when capacity is full
	limitedHandler := withConcurrencyLimit(maxConcurrent, mux)

	// set up the server configurations
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Services.DriverLocationServicePort), // listen on the specified port
		Handler:           limitedHandler,                                             // apply the concurrency limiter to HTTP handler
		ReadHeaderTimeout: 5 * time.Second,                                            // time to read headers
		ReadTimeout:       10 * time.Second,                                           // time to read full request body
		WriteTimeout:      15 * time.Second,                                           // full response write timeout
		IdleTimeout:       60 * time.Second,                                           // keep-alive window
		BaseContext:       func(net.Listener) context.Context { return ctx },          // pass base ctx to all handlers
	}

	// log service start
	logger.Info(ctx, "service_started",
		fmt.Sprintf("Driver & Location Service started on port %d", cfg.Services.DriverLocationServicePort),
		map[string]any{"port": cfg.Services.DriverLocationServicePort, "max_concurrent": maxConcurrent},
	)

	// start the server in a background goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	// wait for context cancellation or server error
	select {
	case <-ctx.Done():
		// graceful HTTP shutdown on context cancel
		shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shCtx); err != nil && err != http.ErrServerClosed {
			logger.Error(ctx, "http_shutdown_failed", "Failed to gracefully shut down HTTP server", err, nil)
		}
	case err := <-errCh:
		// server returned a terminal error at startup or during run
		if err != nil && err != http.ErrServerClosed {
			logger.Error(ctx, "http_server_error", "HTTP server terminated with error", err, map[string]any{"port": cfg.Services.DriverLocationServicePort})
			return err
		}
		return nil
	}

	return nil
}

// withConcurrencyLimit wraps an http.Handler with a semaphore-based limiter.
// It controls how many HTTP requests can be in-progress at the same time.
func withConcurrencyLimit(n int, next http.Handler) http.Handler {
	if n <= 0 {
		return next
	}
	sem := make(chan struct{}, n)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case sem <- struct{}{}: // acquire
			defer func() { <-sem }() // release
			next.ServeHTTP(w, r)
		case <-r.Context().Done():
			// client canceled or server is shutting down
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		}
	})
}
