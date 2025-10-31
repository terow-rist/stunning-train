// internal/adapters/postgres/driver_repo.go
package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"ride-hail/internal/domain/driver"
	"ride-hail/internal/domain/ride"
	"ride-hail/internal/ports"
)

// DriverRepo persists drivers using pgx and plain SQL.
type DriverRepo struct{}

// NewDriverRepo constructs a new DriverRepo.
func NewDriverRepo() ports.DriverRepository {
	return &DriverRepo{}
}

// Create inserts a new driver row. The referenced user must already exist in users(id).
func (repo *DriverRepo) CreateDriver(ctx context.Context, driverObj *driver.Driver) error {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return err
	}

	// insert driver record
	err = tx.QueryRow(ctx, `
		INSERT INTO drivers (id, license_number, vehicle_type, vehicle_attrs, status) 
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at, rating, total_rides, total_earnings, is_verified
	`,
		driverObj.ID,
		driverObj.LicenseNumber,
		driverObj.VehicleType.String(),
		driverObj.VehicleAttrs,    // automatically marshaled by pgx to jsonb
		driverObj.Status.String(), // typically start as 'OFFLINE' or 'AVAILABLE'
	).Scan(&driverObj.ID, &driverObj.CreatedAt, &driverObj.UpdatedAt, &driverObj.Rating, &driverObj.TotalRides, &driverObj.TotalEarnings, &driverObj.IsVerified)
	if err != nil {
		return err
	}

	return nil
}

// GetByID returns one driver by id.
func (repo *DriverRepo) GetByID(ctx context.Context, driverID string) (*driver.Driver, error) {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var out driver.Driver
	var vehicleType string
	var statusText string
	var vehicleAttrs []byte

	// query driver row
	err = tx.QueryRow(ctx, `
		SELECT
			id, created_at, updated_at,
			license_number, vehicle_type, vehicle_attrs,
			rating, total_rides, total_earnings,
			status, is_verified
		FROM drivers
		WHERE id = $1
	`, driverID).Scan(
		&out.ID, &out.CreatedAt, &out.UpdatedAt,
		&out.LicenseNumber, &vehicleType, &vehicleAttrs,
		&out.Rating, &out.TotalRides, &out.TotalEarnings,
		&statusText, &out.IsVerified,
	)
	if err != nil {
		return nil, err
	}

	out.VehicleType = ride.VehicleType(vehicleType)
	out.Status = driver.DriverStatus(statusText)

	if len(vehicleAttrs) > 0 {
		if err := json.Unmarshal(vehicleAttrs, &out.VehicleAttrs); err != nil {
			return nil, err
		}
	}

	return &out, nil
}

// UpdateStatus sets the driver status (idempotent if unchanged).
func (repo *DriverRepo) UpdateStatus(ctx context.Context, driverID string, status driver.DriverStatus) error {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return err
	}

	// lock the row and read current status to keep transitions explicit when needed
	var current string
	err = tx.QueryRow(ctx, `
		SELECT status
		FROM drivers
		WHERE id = $1
		FOR UPDATE
	`, driverID).Scan(&current)
	if err != nil {
		return err
	}

	// idempotent success
	if current == status.String() {
		return nil
	}

	// validate new status
	if !status.Valid() {
		return errors.New("invalid driver status")
	}

	// update state
	_, err = tx.Exec(ctx, `
		UPDATE drivers
		SET status = $1,
		    updated_at = now()
		WHERE id = $2
	`, status.String(), driverID)
	return err
}

// FindNearbyAvailable returns AVAILABLE drivers of the given vehicle type within radius, ordered by distance then rating.
func (repo *DriverRepo) FindNearbyAvailable(
	ctx context.Context,
	lat, lng float64,
	vehicle ride.VehicleType,
	radiusKm float64,
	limit int,
) ([]driver.Driver, error) {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// query available drivers within radius
	rows, err := tx.Query(ctx, `
		SELECT
			d.id, d.created_at, d.updated_at,
			d.license_number, d.vehicle_type, d.vehicle_attrs,
			d.rating, d.total_rides, d.total_earnings,
			d.status, d.is_verified
		FROM drivers d
		JOIN coordinates c
		  ON c.entity_id = d.id
		 AND c.entity_type = 'driver'
		 AND c.is_current = true
		WHERE d.status = 'AVAILABLE'
		  AND d.vehicle_type = $3
		  AND ST_DWithin(
				ST_MakePoint(c.longitude, c.latitude)::geography,
				ST_MakePoint($2, $1)::geography,
				$4 * 1000.0
			  )
		ORDER BY
		  ST_Distance(
			ST_MakePoint(c.longitude, c.latitude)::geography,
			ST_MakePoint($2, $1)::geography
		  ),
		  d.rating DESC
		LIMIT $5
	`, lat, lng, vehicle.String(), radiusKm, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var drivers []driver.Driver
	for rows.Next() {
		var (
			out          driver.Driver
			vehicleType  string
			statusText   string
			vehicleAttrs []byte
		)

		if err := rows.Scan(
			&out.ID, &out.CreatedAt, &out.UpdatedAt,
			&out.LicenseNumber, &vehicleType, &vehicleAttrs,
			&out.Rating, &out.TotalRides, &out.TotalEarnings,
			&statusText, &out.IsVerified,
		); err != nil {
			return nil, err
		}

		// map DB strings to domain enums
		out.VehicleType = ride.VehicleType(vehicleType)
		out.Status = driver.DriverStatus(statusText)

		// decode JSONB vehicle_attrs (nullable)
		if len(vehicleAttrs) > 0 {
			var attrs driver.Attrs
			if err := json.Unmarshal(vehicleAttrs, &attrs); err != nil {
				return nil, err
			}
			out.VehicleAttrs = attrs
		}

		drivers = append(drivers, out)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return drivers, nil
}

// IncrementCountersOnComplete increments total_rides by 1 and adds earnings to total_earnings.
func (repo *DriverRepo) IncrementCountersOnComplete(ctx context.Context, driverID string, earnings float64) error {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return err
	}

	// guard against negative inputs (mirrors domain & DB constraints)
	if earnings < 0 {
		return errors.New("earnings cannot be negative")
	}

	// update counters atomically
	_, err = tx.Exec(ctx, `
		UPDATE drivers
		SET total_rides = total_rides + 1,
		    total_earnings = total_earnings + $1,
		    updated_at = now()
		WHERE id = $2
	`, earnings, driverID)
	return err
}
