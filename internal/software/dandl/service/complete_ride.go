package service

import (
	"context"
	"fmt"
	"time"

	"ride-hail/internal/domain/driver"
	"ride-hail/internal/domain/ride"
	"ride-hail/internal/general/contracts"
	"ride-hail/internal/ports"
)

// CompleteRide marks the ride COMPLETED, makes the driver AVAILABLE, and increments driver earnings/counters.
func (service *driverLocationService) CompleteRide(ctx context.Context, in ports.CompleteRideInput) (ports.CompleteRideResult, error) {
	var out ports.CompleteRideResult
	corrID := generateCorrelationID()

	err := service.uow.WithinTx(ctx, func(ctx context.Context) error {
		// ensure that driver exists
		if _, err := service.drivers.GetByID(ctx, in.DriverID); err != nil {
			return err
		}

		// fetch the ride and validate ownership + state
		r, err := service.rides.GetByID(ctx, in.RideID)
		if err != nil {
			return err
		}
		if r == nil || r.DriverID == nil || *r.DriverID != in.DriverID {
			return fmt.Errorf("ride is not assigned to the driver")
		}

		// compute final fare using the project pricing model
		finalFare := ride.ComputeFinalFare(r.VehicleType, in.ActualDistanceKM, in.ActualDurationMinutes)

		// transition the ride status to COMPLETED
		if err = r.Complete(finalFare); err != nil {
			return err
		}

		// persist ride completion
		if err := service.rides.Complete(ctx, in.RideID, *r.FinalFare, *r.CompletedAt); err != nil {
			return err
		}

		// update driver status -> AVAILABLE
		if err := service.drivers.UpdateStatus(ctx, in.DriverID, driver.DriverStatusAvailable); err != nil {
			return err
		}

		// increment driver's aggregate counters (rides/earnings)
		if err := service.drivers.IncrementCountersOnComplete(ctx, in.DriverID, finalFare); err != nil {
			return err
		}

		// find the active session for this driver
		activeSession, err := service.sessions.GetActiveForDriver(ctx, in.DriverID)
		if err != nil {
			return err
		}

		// increment the session metrics
		if err := activeSession.AddRide(finalFare); err != nil {
			return err
		}

		// update the active driver session counters
		if err := service.sessions.IncrementCounters(ctx, activeSession.ID, activeSession.TotalRides, activeSession.TotalEarnings); err != nil {
			return err
		}

		// prepare output
		out.RideID = in.RideID
		out.Status = driver.DriverStatusAvailable.String()
		out.CompletedAt = *r.CompletedAt
		out.DriverEarnings = finalFare
		out.Message = "Ride completed successfully"

		return nil
	})
	if err != nil {
		service.logger.Error(ctx, "driver_complete_ride_failed", "Failed to complete ride", err, map[string]any{
			"driver_id":  in.DriverID,
			"ride_id":    in.RideID,
			"request_id": corrID,
		})
		return ports.CompleteRideResult{}, err
	}

	// prepare driver status update message (AVAILABLE)
	statusMsg := contracts.DriverStatusMessage{
		DriverID:  in.DriverID,
		Status:    driver.DriverStatusAvailable.String(),
		RideID:    in.RideID,
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
			"ride_id":    in.RideID,
			"status":     statusMsg.Status,
			"request_id": corrID,
		})
	}

	// log successful ride completion
	service.logger.Info(ctx, "ride_completed", "Ride completed; driver available", map[string]any{
		"driver_id":       in.DriverID,
		"ride_id":         in.RideID,
		"status":          out.Status,
		"completed_at":    out.CompletedAt,
		"driver_earnings": out.DriverEarnings,
		"request_id":      corrID,
	})
	if service.websocket != nil {
		service.websocket.StopLocationTracking(in.DriverID)
	}

	service.logger.Info(ctx, "ride_completed_location_stopped",
		"Ride completed and location tracking stopped",
		map[string]any{
			"ride_id":   in.RideID,
			"driver_id": in.DriverID,
		})

	return out, err
}
