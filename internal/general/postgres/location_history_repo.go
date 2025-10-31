// internal/adapters/postgres/location_history_repo.go
package postgres

import (
	"context"
	"ride-hail/internal/domain/geo"
	"ride-hail/internal/ports"
)

// LocationHistoryRepo persists location history rows using pgx and plain SQL.
type LocationHistoryRepo struct{}

// NewLocationHistoryRepo constructs a new LocationHistoryRepo.
func NewLocationHistoryRepo() ports.LocationHistoryRepository {
	return &LocationHistoryRepo{}
}

// Archive inserts a single location_history record.
func (repo *LocationHistoryRepo) Archive(ctx context.Context, record *geo.LocationHistory) error {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return err
	}

	// validate domain invariants
	if err := record.Validate(); err != nil {
		return err
	}

	// insert a new entry
	var insertedID string
	err = tx.QueryRow(ctx, `
		INSERT INTO location_history (
			coordinate_id, driver_id, latitude, longitude,
			accuracy_meters, speed_kmh, heading_degrees,
			recorded_at, ride_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE($8, now()), $9)
		RETURNING id
	`,
		record.CoordinateID,
		record.DriverID,
		record.Latitude,
		record.Longitude,
		record.AccuracyMeters,
		record.SpeedKMH,
		record.HeadingDegrees,
		record.RecordedAt,
		record.RideID,
	).Scan(&insertedID)
	if err != nil {
		return err
	}

	record.ID = geo.ID(insertedID)

	return nil
}
