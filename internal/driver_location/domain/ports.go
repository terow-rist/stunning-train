package domain

import (
	"context"
	"ride-hail/internal/driver_location/adapters/repository"
)

type DriverRepository interface {
	StartSession(ctx context.Context, driverID string) (string, error)
	UpdateStatus(ctx context.Context, driverID, status string) error
}

type LocationRepository interface {
	SaveLocation(ctx context.Context, loc repository.LocationUpdate) error
}

type Publisher interface {
	PublishStatus(ctx context.Context, driverID, status, sessionID string) error
}
