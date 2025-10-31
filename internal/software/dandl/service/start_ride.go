package service

import (
	"context"
	"fmt"
	"ride-hail/internal/domain/driver"
	"ride-hail/internal/domain/ride"
	"ride-hail/internal/general/contracts"
	"ride-hail/internal/ports"
	"time"
)

// StartRide transitions the ride to IN_PROGRESS and marks the driver BUSY.
func (service *driverLocationService) StartRide(ctx context.Context, in ports.StartRideInput) (ports.StartRideResult, error) {
	var out ports.StartRideResult
	corrID := generateCorrelationID()

	err := service.uow.WithinTx(ctx, func(ctx context.Context) error {
		// ensure that driver exists
		if _, err := service.drivers.GetByID(ctx, in.DriverID); err != nil {
			return err
		}

		// fetch the ride and validate ownership
		r, err := service.rides.GetByID(ctx, in.RideID)
		if err != nil {
			return err
		}

		// ensure the caller is the assigned driver.
		if r.DriverID == nil || *r.DriverID != in.DriverID {
			return fmt.Errorf("ride %s is not assigned to driver %s", in.RideID, in.DriverID)
		}

		// transition the ride status to IN_PROGRESS
		if err = r.Start(); err != nil {
			return err
		}

		// update ride status -> IN_PROGRESS
		if err := service.rides.UpdateStatus(ctx, in.RideID, ride.StatusInProgress, *r.StartedAt); err != nil {
			return err
		}

		// update driver status -> BUSY
		if err := service.drivers.UpdateStatus(ctx, in.DriverID, driver.DriverStatusBusy); err != nil {
			return err
		}

		// prepare output
		out.RideID = in.RideID
		out.Status = driver.DriverStatusBusy.String()
		out.StartedAt = *r.StartedAt
		out.Message = "Ride started successfully"

		return nil
	})
	if err != nil {
		service.logger.Error(ctx, "driver_start_ride_failed", "Failed to start ride", err, map[string]any{
			"driver_id":  in.DriverID,
			"ride_id":    in.RideID,
			"request_id": corrID,
		})
		return ports.StartRideResult{}, err
	}

	// prepare driver status update message (BUSY)
	statusMsg := contracts.DriverStatusMessage{
		DriverID:  in.DriverID,
		Status:    driver.DriverStatusBusy.String(),
		RideID:    in.RideID,
		Timestamp: time.Now().UTC(),
		Envelope: contracts.Envelope{
			Producer:      "driver-location-service",
			CorrelationID: corrID,
		},
	}

	// publish driver status update (BUSY)
	if err = service.publishDriverStatus(ctx, statusMsg); err != nil {
		service.logger.Error(ctx, "driver_status_publish_failed", "Failed to publish driver status to RabbitMQ", err, map[string]any{
			"driver_id":  in.DriverID,
			"ride_id":    in.RideID,
			"status":     statusMsg.Status,
			"request_id": corrID,
		})
	}

	// log successful start of the ride
	service.logger.Info(ctx, "driver_started_ride", "Driver started ride", map[string]any{
		"driver_id":  in.DriverID,
		"ride_id":    in.RideID,
		"status":     out.Status,
		"started_at": out.StartedAt,
		"request_id": corrID,
	})

	return out, err
}
