package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LocationUpdate is reused in API; placed here for repo boundaries.
type LocationUpdate struct {
	DriverID  string
	Latitude  float64
	Longitude float64
	// optional: AccuracyMeters, SpeedKmh, HeadingDegrees
	RecordedAt time.Time
}

type LocationRepository struct {
	db *pgxpool.Pool
}

func NewLocationRepository(db *pgxpool.Pool) *LocationRepository {
	return &LocationRepository{db: db}
}

// SaveLocation updates is_current flags and inserts a new coordinates row.
// This operation is transactional.
func (r *LocationRepository) SaveLocation(ctx context.Context, loc LocationUpdate) error {
	if loc.RecordedAt.IsZero() {
		loc.RecordedAt = time.Now().UTC()
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// mark previous as not current
	if _, err := tx.Exec(ctx, `
		UPDATE coordinates
		SET is_current = false, updated_at = now()
		WHERE entity_id = $1 AND entity_type = 'driver' AND is_current = true
	`, loc.DriverID); err != nil {
		return fmt.Errorf("update previous coords: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO coordinates (entity_id, entity_type, address, latitude, longitude, is_current, created_at, updated_at)
		VALUES ($1, 'driver', '', $2, $3, true, now(), now())
	`, loc.DriverID, loc.Latitude, loc.Longitude)
	if err != nil {
		return fmt.Errorf("insert coordinates: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
