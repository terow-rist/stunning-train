package app

import (
	"context"
	"ride-hail/internal/driver_location/domain"
)

type AppService struct {
	driverRepo   domain.DriverRepository
	locationRepo domain.LocationRepository
	publisher    domain.Publisher
	wsPort       domain.WebSocketPort // interface abstraction
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

func (a *AppService) GoOnline(ctx context.Context, driverID string, lat, lng float64) (string, error) {
	// 1️⃣ Start session
	sessionID, err := a.driverRepo.StartSession(ctx, driverID)
	if err != nil {
		return "", err
	}

	// 2️⃣ Update driver status
	if err := a.driverRepo.UpdateStatus(ctx, driverID, "AVAILABLE"); err != nil {
		return "", err
	}

	// 3️⃣ Save location
	if err := a.locationRepo.SaveLocation(ctx, domain.LocationUpdate{
		DriverID:  driverID,
		Latitude:  lat,
		Longitude: lng,
	}); err != nil {
		return "", err
	}

	// 4️⃣ Publish status to RabbitMQ (best effort)
	_ = a.publisher.PublishStatus(ctx, driverID, "AVAILABLE", sessionID)

	// 5️⃣ Notify WebSocket adapter if available
	if a.wsPort != nil {
		msg := map[string]any{
			"type":    "status_update",
			"status":  "AVAILABLE",
			"message": "You are now online and ready to accept rides",
		}
		_ = a.wsPort.SendToDriver(ctx, driverID, msg)
	}

	return sessionID, nil
}
