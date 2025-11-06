package admindashboard

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"ride-hail/internal/general/config"
	"ride-hail/internal/general/jwt"
	"ride-hail/internal/general/logger"
	"ride-hail/internal/general/postgres"
	"ride-hail/internal/software/adminboard/handler"
	"ride-hail/internal/software/adminboard/service"
)

// Run wires the admin dashboard service and blocks until ctx is cancelled.
func Run(ctx context.Context, maxConcurrent int) error {
	// set up a new logger for admin dashboard service with a static request ID for startup logs
	logger := logger.New("admin-service")
	ctx = logger.WithRequestID(ctx, "startup-001")

	// load a config from filez
	cfg, err := config.LoadFromFile("config/config.yaml")
	if err != nil {
		logger.Error(ctx, "config_load_failed", "Failed to load configuration", err, nil)
		return err
	}

	// set up a Postgres connection pool
	pool, err := postgres.NewPool(ctx, cfg, logger)
	if err != nil {
		logger.Error(ctx, "db_connection_failed", "Failed to initialize Postgres pool", err, nil)
		return err
	}
	defer pool.Close()

	// set up the JWT manager
	jwtManager := jwt.NewManager(cfg.JWT.SecretKey, 2*time.Hour)

	// set up the necessary repos
	uow := postgres.NewUnitOfWork(pool)
	rideRepo := postgres.NewRideRepo()
	driverRepo := postgres.NewDriverRepo()
	locHistoryRepo := postgres.NewLocationHistoryRepo()
	coordsRepo := postgres.NewCoordinatesRepo(locHistoryRepo)

	// set up the service
	svc := service.NewAdminService(uow, rideRepo, driverRepo, coordsRepo)

	// set up the HTTP handler and its routes
	mux := http.NewServeMux()
	httpHandler := handler.NewAdminHTTPHandler(svc, logger, jwtManager)
	httpHandler.RegisterRoutes(mux)

	// concurrency limiter (global) — blocks when capacity is full
	limitedHandler := withConcurrencyLimit(maxConcurrent, mux)

	// log service start
	logger.Info(ctx, "service_started",
		fmt.Sprintf("Admin dashboard started on port %d", cfg.Services.AdminServicePort),
		map[string]any{"port": cfg.Services.AdminServicePort, "max_concurrent": maxConcurrent},
	)

	// set up the server configurations
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Services.AdminServicePort), // listen on the specified port
		Handler:           limitedHandler,                                    // apply the concurrency limiter to the HTTP handler
		ReadHeaderTimeout: 5 * time.Second,                                   // time to read headers
		ReadTimeout:       10 * time.Second,                                  // time to read full request body
		WriteTimeout:      15 * time.Second,                                  // full response write timeout
		IdleTimeout:       60 * time.Second,                                  // keep-alive window
		BaseContext:       func(net.Listener) context.Context { return ctx }, // pass base ctx to all handlers
	}

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
			logger.Error(ctx, "http_server_error", "HTTP server terminated with error", err, map[string]any{"port": cfg.Services.AdminServicePort})
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
