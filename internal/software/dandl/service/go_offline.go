package service

import (
	"context"
	"ride-hail/internal/domain/driver"
	"ride-hail/internal/general/contracts"
	"ride-hail/internal/ports"
	"time"
)

// GoOffline marks the driver OFFLINE and finalizes the active session with a summary.
func (service *driverLocationService) GoOffline(ctx context.Context, in ports.GoOfflineInput) (ports.GoOfflineResult, error) {
	var out ports.GoOfflineResult
	corrID := generateCorrelationID()

	err := service.uow.WithinTx(ctx, func(ctx context.Context) error {
		// ensure that driver exists
		if _, err := service.drivers.GetByID(ctx, in.DriverID); err != nil {
			return err
		}

		// find the active session for this driver
		activeSession, err := service.sessions.GetActiveForDriver(ctx, in.DriverID)
		if err != nil {
			return err
		}

		// build session summary
		now := time.Now().UTC()
		summary := driver.DriverSession{
			ID:            activeSession.ID,
			DriverID:      activeSession.DriverID,
			StartedAt:     activeSession.StartedAt,
			EndedAt:       &now,
			TotalRides:    activeSession.TotalRides,
			TotalEarnings: activeSession.TotalEarnings,
		}

		// end the active session with the summary
		if err := service.sessions.End(ctx, activeSession.ID, summary); err != nil {
			return err
		}

		// update driver status to OFFLINE
		if err := service.drivers.UpdateStatus(ctx, in.DriverID, driver.DriverStatusOffline); err != nil {
			return err
		}

		// prepare output
		out = ports.GoOfflineResult{
			Status:    string(driver.DriverStatusOffline),
			SessionID: activeSession.ID,
			SessionSummary: ports.SessionSummary{
				DurationHours:  now.Sub(activeSession.StartedAt).Hours(),
				RidesCompleted: activeSession.TotalRides,
				Earnings:       activeSession.TotalEarnings,
			},
			Message: "You are now offline",
		}
		return nil
	})
	if err != nil {
		service.logger.Error(ctx, "driver_go_offline_failed", "Failed to bring driver offline", err, map[string]any{
			"driver_id":  in.DriverID,
			"request_id": corrID,
		})
		return ports.GoOfflineResult{}, err
	}

	// prepare driver status update message (OFFLINE)
	statusMsg := contracts.DriverStatusMessage{
		DriverID:  in.DriverID,
		Status:    driver.DriverStatusOffline.String(),
		Timestamp: time.Now().UTC(),
		Envelope: contracts.Envelope{
			Producer:      "driver-location-service",
			CorrelationID: corrID,
		},
	}

	// publish driver status update (OFFLINE)
	if err = service.publishDriverStatus(ctx, statusMsg); err != nil {
		service.logger.Error(ctx, "driver_status_publish_failed", "Failed to publish driver status to RabbitMQ", err, map[string]any{
			"driver_id":  in.DriverID,
			"status":     statusMsg.Status,
			"request_id": corrID,
		})
	}

	// log succesful status update
	service.logger.Info(ctx, "driver_offline", "Driver successfully went offline", map[string]any{
		"driver_id":  in.DriverID,
		"session_id": out.SessionID,
		"status":     out.Status,
		"timestamp":  time.Now().UTC(),
		"request_id": corrID,
	})

	return out, err
}
