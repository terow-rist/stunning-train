package app

import (
	"context"
	"fmt"
	"ride-hail/internal/driver_location/adapters/repository"
	"ride-hail/internal/driver_location/domain"
)

// AppService encapsulates business logic
type AppService struct {
	driverRepo   domain.DriverRepository
	locationRepo domain.LocationRepository
	publisher    domain.Publisher
}

func NewAppService(d domain.DriverRepository, l domain.LocationRepository, p domain.Publisher) *AppService {
	return &AppService{
		driverRepo:   d,
		locationRepo: l,
		publisher:    p,
	}
}

// GoOnline implements the "Driver goes online" use case.
func (a *AppService) GoOnline(ctx context.Context, driverID string, lat, lng float64) (string, error) {
	sessionID, err := a.driverRepo.StartSession(ctx, driverID)
	if err != nil {
		return "", fmt.Errorf("start session: %w", err)
	}

	if err := a.driverRepo.UpdateStatus(ctx, driverID, "AVAILABLE"); err != nil {
		return "", fmt.Errorf("update status: %w", err)
	}

	if err := a.locationRepo.SaveLocation(ctx, repository.LocationUpdate{
		DriverID:  driverID,
		Latitude:  lat,
		Longitude: lng,
	}); err != nil {
		return "", fmt.Errorf("save location: %w", err)
	}

	_ = a.publisher.PublishStatus(ctx, driverID, "AVAILABLE", sessionID)

	return sessionID, nil
}
