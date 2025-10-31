package service

import (
	"context"
	"ride-hail/internal/domain/driver"
	"ride-hail/internal/domain/geo"
	"ride-hail/internal/general/contracts"
	"ride-hail/internal/ports"
	"time"
)

// GoOnline sets the driver AVAILABLE, starts a session, and records the current location.
func (service *driverLocationService) GoOnline(ctx context.Context, in ports.GoOnlineInput) (ports.GoOnlineResult, error) {
	var out ports.GoOnlineResult
	corrID := generateCorrelationID()

	err := service.uow.WithinTx(ctx, func(ctx context.Context) error {
		// ensure that driver exists
		if _, err := service.drivers.GetByID(ctx, in.DriverID); err != nil {
			return err
		}

		// update status to AVAILABLE
		if err := service.drivers.UpdateStatus(ctx, in.DriverID, driver.DriverStatusAvailable); err != nil {
			return err
		}

		// start a new driver session
		sessionID, err := service.sessions.Start(ctx, in.DriverID)
		if err != nil {
			return err
		}

		// upsert current coordinates and mark them as current
		coord := geo.Coordinate{
			EntityID:   in.DriverID,
			EntityType: geo.EntityTypeDriver,
			Address:    "N/A",
			Latitude:   in.Latitude,
			Longitude:  in.Longitude,
			IsCurrent:  true,
		}
		if _, _, err := service.coords.UpsertForDriver(ctx, in.DriverID, coord, true); err != nil {
			return err
		}

		// prepare output
		out = ports.GoOnlineResult{
			Status:    driver.DriverStatusAvailable.String(),
			SessionID: sessionID,
			Message:   "You are now online and ready to accept rides",
		}
		return nil
	})
	if err != nil {
		service.logger.Error(ctx, "driver_go_online_failed", "Failed to bring driver online", err, map[string]any{
			"driver_id":  in.DriverID,
			"latitude":   in.Latitude,
			"longitude":  in.Longitude,
			"request_id": corrID,
		})
		return ports.GoOnlineResult{}, err
	}

	// prepare driver status update message (AVAILABLE)
	statusMsg := contracts.DriverStatusMessage{
		DriverID:  in.DriverID,
		Status:    driver.DriverStatusAvailable.String(),
		Timestamp: time.Now().UTC(),
		Envelope: contracts.Envelope{
			Producer:      "driver-location-service",
			CorrelationID: corrID,
		},
	}

	// publish driver status update (AVAILABLE)
	if err = service.publishDriverStatus(ctx, statusMsg); err != nil {
		service.logger.Error(ctx, "driver_status_publish_failed", "Failed to publish driver status to RabbitMQ", err, map[string]any{
			"driver_id":  in.DriverID,
			"status":     statusMsg.Status,
			"request_id": corrID,
		})
	}

	// log succesful status update
	service.logger.Info(ctx, "driver_online", "Driver successfully went online", map[string]any{
		"driver_id":  in.DriverID,
		"session_id": out.SessionID,
		"status":     out.Status,
		"timestamp":  time.Now().UTC(),
		"request_id": corrID,
	})

	return out, err
}
