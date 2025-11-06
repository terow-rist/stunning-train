package postgres

import (
	"context"
	"ride-hail/internal/domain/driver"
	"ride-hail/internal/domain/ride"
)

// CountByStatus returns the total number of drivers with the given status.
func (repo *DriverRepo) CountByStatus(ctx context.Context, status driver.DriverStatus) (int, error) {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return 0, err
	}

	// validate driver status input
	if !status.Valid() {
		return 0, driver.ErrInvalidDriverStatus
	}

	var count int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM drivers
		WHERE status = $1
	`, status.String()).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// CountByVehicleType returns the total number of drivers with the given vehicle type.
func (repo *DriverRepo) CountByVehicleType(ctx context.Context, vehicle ride.VehicleType) (int, error) {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return 0, err
	}

	// validate vehicle type input
	if !vehicle.Valid() {
		return 0, ride.ErrInvalidVehicleType
	}

	var count int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM drivers
		WHERE vehicle_type = $1
	`, vehicle.String()).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}
