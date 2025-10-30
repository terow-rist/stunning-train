package repository

import (
	"context"
	"fmt"
	"ride-hail/internal/driver_location/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DriverRepository struct {
	db *pgxpool.Pool
}

func NewDriverRepository(db *pgxpool.Pool) *DriverRepository {
	return &DriverRepository{db: db}
}

func (r *DriverRepository) StartSession(ctx context.Context, driverID string) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var existing string
	err = tx.QueryRow(ctx, `
		SELECT id
		FROM driver_sessions
		WHERE driver_id = $1 AND ended_at IS NULL
		LIMIT 1
	`, driverID).Scan(&existing)

	if err == nil {
		return "", fmt.Errorf("driver already has active session: %w", domain.ErrAlreadyOnline)
	}
	if err != nil && err != pgx.ErrNoRows {
		return "", fmt.Errorf("check existing session: %w", err)
	}

	var sessionID string
	err = tx.QueryRow(ctx, `
		INSERT INTO driver_sessions (driver_id)
		VALUES ($1)
		RETURNING id
	`, driverID).Scan(&sessionID)
	if err != nil {
		return "", fmt.Errorf("insert driver_session: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit tx: %w", err)
	}

	return sessionID, nil
}

func (r *DriverRepository) UpdateStatus(ctx context.Context, driverID, status string) error {
	ct, err := r.db.Exec(ctx, `
		UPDATE drivers
		SET status = $2, updated_at = now()
		WHERE id = $1
	`, driverID, status)
	if err != nil {
		return fmt.Errorf("update drivers status: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *DriverRepository) EndSession(ctx context.Context, driverID string) (string, domain.SessionSummary, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", domain.SessionSummary{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var sessionID string
	row := tx.QueryRow(ctx, `
		SELECT id
		FROM driver_sessions
		WHERE driver_id = $1 AND ended_at IS NULL
		ORDER BY started_at DESC
		LIMIT 1
	`, driverID)
	if err := row.Scan(&sessionID); err != nil {
		if err == pgx.ErrNoRows {
			return "", domain.SessionSummary{}, domain.ErrAlreadyOffline
		}
		return "", domain.SessionSummary{}, fmt.Errorf("query active session: %w", err)
	}

	var ridesCompleted int
	var totalEarnings float64
	err = tx.QueryRow(ctx, `
		SELECT 
			COUNT(*) AS rides_completed,
			COALESCE(SUM(final_fare), 0)
		FROM rides
		WHERE driver_id = $1
		  AND status = 'COMPLETED'
		  AND completed_at >= (
		      SELECT started_at FROM driver_sessions WHERE id = $2
		  )
	`, driverID, sessionID).Scan(&ridesCompleted, &totalEarnings)
	if err != nil {
		return "", domain.SessionSummary{}, fmt.Errorf("query rides summary: %w", err)
	}

	ct, err := tx.Exec(ctx, `
		UPDATE driver_sessions
		SET ended_at = now(), total_rides = $2, total_earnings = $3
		WHERE id = $1
	`, sessionID, ridesCompleted, totalEarnings)
	if err != nil {
		return "", domain.SessionSummary{}, fmt.Errorf("update session: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return "", domain.SessionSummary{}, domain.ErrAlreadyOffline
	}

	var durationHours float64
	err = tx.QueryRow(ctx, `
		SELECT EXTRACT(EPOCH FROM (now() - started_at)) / 3600.0
		FROM driver_sessions
		WHERE id = $1
	`, sessionID).Scan(&durationHours)
	if err != nil {
		return "", domain.SessionSummary{}, fmt.Errorf("compute duration: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", domain.SessionSummary{}, fmt.Errorf("commit tx: %w", err)
	}

	summary := domain.SessionSummary{
		DurationHours:  durationHours,
		RidesCompleted: ridesCompleted,
		Earnings:       totalEarnings,
	}

	return sessionID, summary, nil
}

func (r *DriverRepository) HasActiveSession(ctx context.Context, driverID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM driver_sessions
			WHERE driver_id = $1 AND ended_at IS NULL
		)
	`, driverID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check active session: %w", err)
	}
	return exists, nil
}
