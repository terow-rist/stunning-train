package postgres

import (
	"context"

	"ride-hail/internal/domain/driver"
	"ride-hail/internal/ports"
)

// DriverSessionRepo persists driver session records using pgx and plain SQL.
type DriverSessionRepo struct{}

// NewDriverSessionRepo constructs a new DriverSessionRepo.
func NewDriverSessionRepo() ports.DriverSessionRepository {
	return &DriverSessionRepo{}
}

// Start creates a new driver session row and returns its generated session ID.
func (repo *DriverSessionRepo) Start(ctx context.Context, driverID string) (string, error) {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return "", err
	}

	session, err := driver.NewSession(driverID)
	if err != nil {
		return "", err
	}

	var sessionID string
	err = tx.QueryRow(ctx, `
		INSERT INTO driver_sessions (driver_id, started_at, total_rides, total_earnings)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`,
		session.DriverID,
		session.StartedAt,
		session.TotalRides,
		session.TotalEarnings,
	).Scan(&sessionID)
	if err != nil {
		return "", err
	}

	return sessionID, nil
}

// End updates an existing session with its summary and marks it ended.
func (repo *DriverSessionRepo) End(ctx context.Context, sessionID string, summary driver.DriverSession) error {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return err
	}

	if summary.EndedAt == nil {
		if err := summary.End(); err != nil {
			return err
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE driver_sessions
		SET ended_at = $1,
		    total_rides = $2,
		    total_earnings = $3
		WHERE id = $4
	`, summary.EndedAt, summary.TotalRides, summary.TotalEarnings, sessionID)

	return err
}

// GetActiveForDriver fetches the information about the active session.
func (repo *DriverSessionRepo) GetActiveForDriver(ctx context.Context, driverID string) (*driver.DriverSession, error) {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var session driver.DriverSession

	err = tx.QueryRow(ctx, `
		SELECT 
			id,
			driver_id,
			started_at,
			ended_at,
			total_rides,
			total_earnings
		FROM driver_sessions
		WHERE driver_id = $1 AND ended_at IS NULL
		ORDER BY started_at DESC
		LIMIT 1
	`, driverID).Scan(
		&session.ID,
		&session.DriverID,
		&session.StartedAt,
		&session.EndedAt,
		&session.TotalRides,
		&session.TotalEarnings,
	)
	if err != nil {
		return nil, err
	}

	return &session, nil
}

// IncrementCounters updates aggregate counters for an active session.
func (repo *DriverSessionRepo) IncrementCounters(ctx context.Context, sessionID string, totalRides int, totalEarnings float64) error {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		UPDATE driver_sessions
		SET total_rides = $1,
		    total_earnings = $2
		WHERE id = $3
	`, totalRides, totalEarnings, sessionID)

	return err
}
