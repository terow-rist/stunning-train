package repository

import (
	"context"
	"crypto/rand"
	"fmt"
	"ride-hail/internal/driver_location/domain"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LocationRepository struct {
	db *pgxpool.Pool
}

func NewLocationRepository(db *pgxpool.Pool) *LocationRepository {
	return &LocationRepository{db: db}
}

func (r *LocationRepository) SaveLocation(ctx context.Context, loc domain.LocationUpdate) error {
	if loc.RecordedAt.IsZero() {
		loc.RecordedAt = time.Now().UTC()
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		UPDATE coordinates
		SET is_current = false, updated_at = now()
		WHERE entity_id = $1 AND entity_type = 'driver' AND is_current = true
	`, loc.DriverID); err != nil {
		return fmt.Errorf("update previous coords: %w", err)
	}

	id, err := newUUID()
	if err != nil {
		return fmt.Errorf("generate uuid: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO coordinates (
			id, entity_id, entity_type, address, latitude, longitude,
			is_current, created_at, updated_at
		)
		VALUES ($1, $2, 'driver', '', $3, $4, true, now(), now())
	`, id, loc.DriverID, loc.Latitude, loc.Longitude)
	if err != nil {
		return fmt.Errorf("insert coordinates: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *LocationRepository) UpdateLocation(ctx context.Context, loc domain.LocationUpdate) (domain.LocationResult, error) {
	if loc.RecordedAt.IsZero() {
		loc.RecordedAt = time.Now().UTC()
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.LocationResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var coordID string
	err = tx.QueryRow(ctx, `
		UPDATE coordinates
		SET latitude = $2,
		    longitude = $3,
		    updated_at = now()
		WHERE entity_id = $1 AND entity_type = 'driver' AND is_current = true
		RETURNING id
	`, loc.DriverID, loc.Latitude, loc.Longitude).Scan(&coordID)

	if err == pgx.ErrNoRows {
		coordID, err = newUUID()
		if err != nil {
			return domain.LocationResult{}, fmt.Errorf("generate uuid: %w", err)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO coordinates (id, entity_id, entity_type, address, latitude, longitude, is_current, created_at, updated_at)
			VALUES ($1, $2, 'driver', '', $3, $4, true, now(), now())
		`, coordID, loc.DriverID, loc.Latitude, loc.Longitude)
		if err != nil {
			return domain.LocationResult{}, fmt.Errorf("insert coordinates: %w", err)
		}
	} else if err != nil {
		return domain.LocationResult{}, fmt.Errorf("update coordinates: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO location_history (
			id, driver_id, latitude, longitude, accuracy_meters,
			speed_kmh, heading_degrees, recorded_at
		)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7)
	`, loc.DriverID, loc.Latitude, loc.Longitude, loc.AccuracyMeters, loc.SpeedKmh, loc.HeadingDegrees, loc.RecordedAt)
	if err != nil {
		return domain.LocationResult{}, fmt.Errorf("insert location_history: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.LocationResult{}, fmt.Errorf("commit tx: %w", err)
	}

	return domain.LocationResult{
		CoordinateID: coordID,
		UpdatedAt:    loc.RecordedAt,
	}, nil
}

// newUUID generates a random RFC 4122-compliant UUID v4
func newUUID() (string, error) {
	u := make([]byte, 16)
	if _, err := rand.Read(u); err != nil {
		return "", fmt.Errorf("rand.Read: %w", err)
	}

	u[6] = (u[6] & 0x0f) | 0x40
	u[8] = (u[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:]), nil
}
