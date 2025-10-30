package app

import (
	"context"
	"fmt"
	"math"
	"time"

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

func (a *AppService) GoOffline(ctx context.Context, driverID string) (string, domain.SessionSummary, error) {
	if driverID == "" {
		return "", domain.SessionSummary{}, domain.ErrInvalidDriverID
	}

	sessionID, summary, err := a.driverRepo.EndSession(ctx, driverID)
	if err != nil {
		return "", domain.SessionSummary{}, fmt.Errorf("end session: %w", err)
	}

	if err := a.driverRepo.UpdateStatus(ctx, driverID, "OFFLINE"); err != nil {
		return "", domain.SessionSummary{}, fmt.Errorf("update status: %w", err)
	}

	if err := a.publisher.PublishStatus(ctx, driverID, "OFFLINE", sessionID); err != nil {
		return "", domain.SessionSummary{}, fmt.Errorf("%w: %v", domain.ErrPublishFailed, err)
	}

	if a.wsPort != nil {
		msg := map[string]any{
			"type":    "status_update",
			"status":  "OFFLINE",
			"message": "You are now offline",
		}
		if err := a.wsPort.SendToDriver(ctx, driverID, msg); err != nil {
			return sessionID, summary, fmt.Errorf("%w: %v", domain.ErrWebSocketSend, err)
		}
	}

	return sessionID, summary, nil
}

func (a *AppService) UpdateLocation(ctx context.Context, driverID string, lat, lng, acc, speed, heading float64) (domain.LocationResult, error) {
	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return domain.LocationResult{}, domain.ErrInvalidCoordinates
	}

	active, err := a.driverRepo.HasActiveSession(ctx, driverID)
	if err != nil {
		return domain.LocationResult{}, fmt.Errorf("check active session: %w", err)
	}
	if !active {
		return domain.LocationResult{}, domain.ErrAlreadyOffline
	}

	loc := domain.LocationUpdate{
		DriverID:       driverID,
		Latitude:       lat,
		Longitude:      lng,
		AccuracyMeters: acc,
		SpeedKmh:       speed,
		HeadingDegrees: heading,
		RecordedAt:     time.Now().UTC(),
	}

	result, err := a.locationRepo.UpdateLocation(ctx, loc)
	if err != nil {
		return domain.LocationResult{}, fmt.Errorf("save location: %w", err)
	}

	if a.wsPort != nil {
		msg := map[string]any{
			"type":      "location_update",
			"latitude":  lat,
			"longitude": lng,
			"speed":     speed,
			"heading":   heading,
			"accuracy":  acc,
			"timestamp": loc.RecordedAt,
		}
		_ = a.wsPort.SendToDriver(ctx, driverID, msg)
	}

	return result, nil
}
