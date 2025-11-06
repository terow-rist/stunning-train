package postgres

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"ride-hail/internal/general/config"
	"ride-hail/internal/general/logger"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool builds a DSN from cfg, configures pgxpool, verifies connectivity, and returns the pool.
func NewPool(ctx context.Context, cfg *config.Config, logger *logger.Logger) (*pgxpool.Pool, error) {
	start := time.Now()

	// build DSN
	u := &url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(cfg.Database.Host, strconv.Itoa(cfg.Database.Port)),
		Path:   "/" + cfg.Database.Name,
		User:   url.UserPassword(cfg.Database.User, cfg.Database.Password),
	}
	q := url.Values{}
	q.Set("sslmode", "disable")
	u.RawQuery = q.Encode()
	dsn := u.String()

	// one-time sanity log (do not print the password)
	logger.Info(ctx, "db_config_check", "Effective DB connection parameters", map[string]any{
		"host":           cfg.Database.Host,
		"port":           cfg.Database.Port,
		"user":           cfg.Database.User,
		"database":       cfg.Database.Name,
		"password_empty": cfg.Database.Password == "",
		"sslmode":        "disable",
	})

	// ---- parse and tune pool config ----
	pcfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres parse dsn: %w", err)
	}

	// connection-level settings
	pcfg.ConnConfig.ConnectTimeout = 5 * time.Second
	if pcfg.ConnConfig.RuntimeParams == nil {
		pcfg.ConnConfig.RuntimeParams = make(map[string]string, 2)
	}
	pcfg.ConnConfig.RuntimeParams["timezone"] = "UTC"

	// pool hygiene (keep reasonable, simple defaults)
	pcfg.HealthCheckPeriod = 30 * time.Second
	pcfg.MaxConnIdleTime = 5 * time.Minute

	// create pool
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.NewWithConfig: %w", err)
	}

	// verify connectivity with a bounded timeout
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}

	logger.Info(ctx, "db_connected", "Connected to PostgreSQL database", map[string]any{
		"duration_ms": time.Since(start).Milliseconds(),
	})

	return pool, nil
}
