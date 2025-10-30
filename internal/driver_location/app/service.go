package app

import (
	"context"
	"fmt"
	"math"

	"ride-hail/internal/driver_location/domain"
)

// AppService orchestrates the core driver-location use cases.
type AppService struct {
	driverRepo   domain.DriverRepository
	locationRepo domain.LocationRepository
	publisher    domain.Publisher
	wsPort       domain.WebSocketPort
}

func NewAppService(
	dr domain.DriverRepository,
	lr domain.LocationRepository,
	pub domain.Publisher,
	ws domain.WebSocketPort,
) *AppService {
	return &AppService{
		driverRepo:   dr,
		locationRepo: lr,
		publisher:    pub,
		wsPort:       ws,
	}
}

// GoOnline transitions a driver into AVAILABLE status,
// starts a new session, saves location, and notifies systems.
func (a *AppService) GoOnline(ctx context.Context, driverID string, lat, lng float64) (string, error) {
	if driverID == "" {
		return "", domain.ErrInvalidDriverID
	}
	if math.IsNaN(lat) || math.IsNaN(lng) {
		return "", domain.ErrInvalidCoordinates
	}
	if math.Abs(lat) > 90 || math.Abs(lng) > 180 {
		return "", domain.ErrInvalidCoordinates
	}
	if lat == 0 || lng == 0 {
		return "", domain.ErrInvalidCoordinates
	}
	sessionID, err := a.driverRepo.StartSession(ctx, driverID)
	if err != nil {
		return "", fmt.Errorf("start session: %w", err)
	}

	if err := a.driverRepo.UpdateStatus(ctx, driverID, "AVAILABLE"); err != nil {
		return "", fmt.Errorf("update status: %w", err)
	}

	loc := domain.LocationUpdate{
		DriverID:  driverID,
		Latitude:  lat,
		Longitude: lng,
	}
	if err := a.locationRepo.SaveLocation(ctx, loc); err != nil {
		return "", fmt.Errorf("save location: %w", err)
	}

	if err := a.publisher.PublishStatus(ctx, driverID, "AVAILABLE", sessionID); err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrPublishFailed, err)
	}

	if a.wsPort != nil {
		msg := map[string]any{
			"type":    "status_update",
			"status":  "AVAILABLE",
			"message": "You are now online and ready to accept rides",
		}

		if err := a.wsPort.SendToDriver(ctx, driverID, msg); err != nil {
			return sessionID, fmt.Errorf("%w: %v", domain.ErrWebSocketSend, err)
		}
	}

	return sessionID, nil
}
