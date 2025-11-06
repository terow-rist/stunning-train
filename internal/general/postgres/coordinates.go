package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"ride-hail/internal/domain/geo"
	"ride-hail/internal/ports"
)

// CoordinatesRepo persists coordinate data using pgx and plain SQL.
type CoordinatesRepo struct {
	locHistory ports.LocationHistoryRepository
}

// NewCoordinatesRepo constructs a new CoordinatesRepo.
func NewCoordinatesRepo(locHistory ports.LocationHistoryRepository) ports.CoordinatesRepository {
	return &CoordinatesRepo{
		locHistory: locHistory,
	}
}

// SaveDriverLocation creates a new coordinate for driver with location data and archives to history
func (repo *CoordinatesRepo) SaveDriverLocation(
	ctx context.Context,
	driverID string,
	latitude, longitude, accuracyMeters, speedKmh, headingDegrees float64,
	address string,
) (*geo.Coordinate, error) {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Validate inputs
	if driverID == "" {
		return nil, errors.New("driverID cannot be empty")
	}
	if latitude < -90 || latitude > 90 {
		return nil, geo.ErrInvalidLatitude
	}
	if longitude < -180 || longitude > 180 {
		return nil, geo.ErrInvalidLongitude
	}
	if address == "" {
		address = "Current Location"
	}

	var coord geo.Coordinate

	// Execute operations in sequence (since we're already in a transaction via UnitOfWork)
	// 1. Mark previous current coordinates as not current
	_, err = tx.Exec(ctx, `
		UPDATE coordinates 
		SET is_current = false, updated_at = now()
		WHERE entity_id = $1 
		AND entity_type = $2 
		AND is_current = true
	`, driverID, geo.EntityTypeDriver.String())
	if err != nil {
		return nil, fmt.Errorf("failed to update previous coordinates: %w", err)
	}

	// 2. Insert new current coordinate
	err = tx.QueryRow(ctx, `
		INSERT INTO coordinates (
			entity_id, entity_type, address,
			latitude, longitude,
			fare_amount, distance_km, duration_minutes, is_current,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now(), now())
		RETURNING id, created_at, updated_at
	`,
		driverID,
		geo.EntityTypeDriver.String(),
		address,
		latitude,
		longitude,
		0.0,  // fare_amount
		0.0,  // distance_km
		0,    // duration_minutes
		true, // is_current
	).Scan(&coord.ID, &coord.CreatedAt, &coord.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert coordinate: %w", err)
	}

	// Fill the rest of coordinate data
	coord.EntityID = driverID
	coord.EntityType = geo.EntityTypeDriver
	coord.Address = address
	coord.Latitude = latitude
	coord.Longitude = longitude
	coord.FareAmount = 0
	coord.DistanceKM = 0
	coord.DurationMinutes = 0
	coord.IsCurrent = true

	// 3. Archive to location_history (best effort - if it fails, we still return the coordinate)
	if repo.locHistory != nil {
		lh, err := geo.NewLocationHistory(
			coord.ID,
			driverID,
			nil, // ride_id - will be filled later if there's an active ride
			latitude,
			longitude,
			&accuracyMeters,
			&speedKmh,
			&headingDegrees,
			time.Now().UTC(),
		)
		if err == nil {
			// Use a separate context to avoid transaction issues with archive
			archiveCtx := context.Background()
			if archiveErr := repo.locHistory.Archive(archiveCtx, lh); archiveErr != nil {
				// Log the error but don't fail the main operation
				fmt.Printf("Warning: failed to archive location history: %v\n", archiveErr)
			}
		}
	}

	return &coord, nil
}

// UpsertForDriver is a wrapper to upsert the current coordinate for a driver.
func (repo *CoordinatesRepo) UpsertForDriver(ctx context.Context, driverID string, coord geo.Coordinate, makeCurrent bool) (string, time.Time, error) {
	return repo.upsertNewCoordinate(ctx, driverID, geo.EntityTypeDriver, &coord, makeCurrent)
}

// UpsertForPassenger is a wrapper to upsert the current coordinate for a passenger.
func (repo *CoordinatesRepo) UpsertForPassenger(ctx context.Context, passengerID string, coord geo.Coordinate, makeCurrent bool) (string, time.Time, error) {
	return repo.upsertNewCoordinate(ctx, passengerID, geo.EntityTypePassenger, &coord, makeCurrent)
}

// upsertNewCoordinate is a helper to upsert the current coordinate for a given entity.
func (repo *CoordinatesRepo) upsertNewCoordinate(
	ctx context.Context,
	entityID string,
	entityType geo.EntityType,
	coord *geo.Coordinate,
	makeCurrent bool,
) (coordinateID string, updatedAt time.Time, err error) {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return "", time.Time{}, err
	}

	// validate entity type
	if !entityType.Valid() {
		return "", time.Time{}, errors.New("invalid entity type")
	}

	// ensure coord mirrors the input identity
	coord.EntityID = entityID
	coord.EntityType = entityType

	// validate new coord
	if err := coord.Validate(); err != nil {
		return "", time.Time{}, err
	}

	// when making this the current point, flip any previous current for the same entity
	if makeCurrent {
		if _, err := tx.Exec(ctx, `
			UPDATE coordinates
			SET is_current = false, updated_at = now()
			WHERE entity_id = $1
			  AND entity_type = $2
			  AND is_current = true
		`, entityID, entityType.String()); err != nil {
			return "", time.Time{}, err
		}
	}

	// insert new coordinate with desired currentness
	err = tx.QueryRow(ctx, `
		INSERT INTO coordinates (
			entity_id, entity_type, address,
			latitude, longitude,
			fare_amount, distance_km, duration_minutes, is_current
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at
	`,
		entityID,
		entityType.String(),
		coord.Address,
		coord.Latitude,
		coord.Longitude,
		coord.FareAmount,
		coord.DistanceKM,
		coord.DurationMinutes,
		makeCurrent,
	).Scan(&coord.ID, &coord.CreatedAt, &coord.UpdatedAt)
	if err != nil {
		return "", time.Time{}, err
	}

	coord.IsCurrent = makeCurrent

	return string(coord.ID), coord.UpdatedAt, nil
}

// GetCurrentForDriver retrieves the current coordinate for a driver.
func (repo *CoordinatesRepo) GetCurrentForDriver(ctx context.Context, driverID string) (*geo.Coordinate, error) {
	return repo.getCurrent(ctx, driverID, geo.EntityTypeDriver)
}

// GetCurrentForPassenger retrieves the current coordinate for a passenger.
func (repo *CoordinatesRepo) GetCurrentForPassenger(ctx context.Context, passengerID string) (*geo.Coordinate, error) {
	return repo.getCurrent(ctx, passengerID, geo.EntityTypePassenger)
}

// getCurrent is a helper to retrieve the current coordinate for a given entity.
func (repo *CoordinatesRepo) getCurrent(
	ctx context.Context,
	entityID string,
	entityType geo.EntityType,
) (*geo.Coordinate, error) {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var out geo.Coordinate
	var et string

	err = tx.QueryRow(ctx, `
		SELECT
			id, created_at, updated_at,
			entity_id, entity_type,
			address, latitude, longitude,
			fare_amount, distance_km, duration_minutes, is_current
		FROM coordinates
		WHERE entity_id = $1
		  AND entity_type = $2
		  AND is_current = true
		LIMIT 1
	`, entityID, entityType.String()).Scan(
		&out.ID, &out.CreatedAt, &out.UpdatedAt,
		&out.EntityID, &et,
		&out.Address, &out.Latitude, &out.Longitude,
		&out.FareAmount, &out.DistanceKM, &out.DurationMinutes, &out.IsCurrent,
	)
	if err != nil {
		return nil, err
	}

	out.EntityType, _ = geo.ParseEntityType(et)

	return &out, nil
}
