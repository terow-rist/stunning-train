package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DriverRepository struct {
	db *pgxpool.Pool
}

func NewDriverRepository(db *pgxpool.Pool) *DriverRepository {
	return &DriverRepository{db: db}
}

// StartSession creates a driver_sessions row and returns the session id.
// Uses a transaction to ensure atomicity.
func (r *DriverRepository) StartSession(ctx context.Context, driverID string) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx) // safe to call (no-op if committed)
	}()

	var sessionID string
	row := tx.QueryRow(ctx, `
		INSERT INTO driver_sessions (driver_id)
		VALUES ($1)
		RETURNING id
	`, driverID)
	if err := row.Scan(&sessionID); err != nil {
		return "", fmt.Errorf("insert driver_session: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit tx: %w", err)
	}
	return sessionID, nil
}

// UpdateStatus updates drivers.status; returns pgx.ErrNoRows if driver missing.
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
