package postgres

import (
	"context"
	"ride-hail/internal/domain/ride"
	"ride-hail/internal/ports"
)

// RideEventRepo persists ride events using pgx and plain SQL.
type RideEventRepo struct{}

// NewRideEventRepo constructs a new RideEventRepo.
func NewRideEventRepo() ports.RideEventRepository {
	return &RideEventRepo{}
}

// Append inserts a new ride_events row.
func (repo *RideEventRepo) Append(ctx context.Context, event *ride.Event) error {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return err
	}

	// validate event before inserting
	if err := event.Validate(); err != nil {
		return err
	}

	// serialize event data to JSON
	data, err := event.DataJSON()
	if err != nil {
		return err
	}

	// insert ride event record
	err = tx.QueryRow(ctx, `
		INSERT INTO ride_events (ride_id, event_type, event_data)
		VALUES ($1, $2, $3::jsonb)
		RETURNING id, created_at
	`,
		event.RideID,
		event.Type.String(),
		string(data),
	).Scan(&event.ID, &event.CreatedAt)
	if err != nil {
		return err
	}

	return nil
}
