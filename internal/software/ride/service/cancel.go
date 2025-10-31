package service

import (
	"context"
	"fmt"
	"ride-hail/internal/domain/ride"
	"ride-hail/internal/general/contracts"
	"ride-hail/internal/ports"
	"strings"
	"time"
)

// CancelRide cancels a ride and publishes a CANCELLED status event.
func (service *rideService) CancelRide(ctx context.Context, rideID, reason string) (ports.CancelRideResult, error) {
	rideID = strings.TrimSpace(rideID)
	if rideID == "" {
		return ports.CancelRideResult{}, fmt.Errorf("rideservice: rideID is required to cancel ride")
	}
	corrID := generateCorrelationID()

	var (
		driverID   string
		cancelTime = time.Now().UTC()
	)

	err := service.uow.WithinTx(ctx, func(txCtx context.Context) error {
		// load the ride
		r, err := service.rideRepo.GetByID(txCtx, rideID)
		if err != nil {
			return err
		}
		if r.DriverID != nil {
			driverID = *r.DriverID
		}

		// cancel the ride
		if err := service.rideRepo.Cancel(txCtx, rideID, reason, cancelTime); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		service.logger.Error(ctx, "ride_cancel_failed", "Failed to cancel ride", err, map[string]any{
			"ride_id":    rideID,
			"reason":     reason,
			"request_id": corrID,
		})
		return ports.CancelRideResult{}, err
	}

	// fan-out: publish CANCELLED status (best-effort, outside tx)
	if err := service.publishRideStatus(ctx, contracts.RideStatusMessage{
		RideID:    rideID,
		Status:    ride.StatusCancelled.String(),
		Timestamp: time.Now().UTC(),
		DriverID:  driverID, // empty if none
		Envelope: contracts.Envelope{
			CorrelationID: corrID,
			Producer:      "ride-service",
		},
	}); err != nil {
		service.logger.Error(ctx, "ride_status_publish_failed", "Failed to publish CANCELLED status", err, map[string]any{
			"ride_id":    rideID,
			"request_id": corrID,
		})
	}

	service.logger.Info(ctx, "ride_cancelled",
		fmt.Sprintf("Ride %s cancelled", rideID),
		map[string]any{
			"ride_id":    rideID,
			"reason":     reason,
			"request_id": corrID,
		},
	)

	return ports.CancelRideResult{
		RideID:      rideID,
		Status:      "CANCELLED",
		CancelledAt: time.Now().UTC().Format(time.RFC3339),
		Message:     "Ride cancelled successfully",
	}, nil
}
