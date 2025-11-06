package service

import (
	"context"
	"fmt"
	"ride-hail/internal/domain/geo"
	"ride-hail/internal/domain/ride"
	"ride-hail/internal/general/contracts"
	"ride-hail/internal/ports"
	"time"
)

// CreateRide creates a new ride request in REQUESTED state, persists pickup & destination coordinates.
func (service *rideService) CreateRide(ctx context.Context, in ports.CreateRideInput) (ports.CreateRideResult, error) {
	var (
		rideID             string
		rideStatus         string
		pickupCoordinateID string
		destCoordinateID   string
		rideNumber         = generateRideNumber()
		correlationID      = generateCorrelationID()
	)

	// compute the distance
	dst := ride.HaversineKM(in.PickupLatitude, in.PickupLongitude, in.DestinationLatitude, in.DestinationLongitude)

	// estimate the approximate duration of a ride in minutes
	min := ride.EstimateDurationMinutes(dst)

	// estimate the fare
	estimatedFare := ride.ComputeFinalFare(in.VehicleType, dst, min)

	// compute the priority given the distance and the vehicle type
	priority := ride.ComputePriority(in.VehicleType, dst)

	// carry out all the necessary database transactions
	err := service.uow.WithinTx(ctx, func(txCtx context.Context) error {
		// build and persist pickup coordinate (entity = passenger)
		pickup, err := geo.NewCoordinate(
			in.PassengerID,
			geo.EntityTypePassenger,
			in.PickupAddress,
			in.PickupLatitude,
			in.PickupLongitude,
		)
		if err != nil {
			return err
		}

		// upsert current pickup coordinate for passenger
		id, _, err := service.coordsRepo.UpsertForPassenger(txCtx, in.PassengerID, *pickup, true)
		if err != nil {
			return err
		}
		pickupCoordinateID = id

		// build and persist destination coordinate (entity = passenger)
		dest, err := geo.NewCoordinate(
			in.PassengerID,
			geo.EntityTypePassenger,
			in.DestinationAddress,
			in.DestinationLatitude,
			in.DestinationLongitude,
		)
		if err != nil {
			return err
		}

		// upsert current destination coordinate for passenger
		id, _, err = service.coordsRepo.UpsertForPassenger(txCtx, in.PassengerID, *dest, false)
		if err != nil {
			return err
		}
		destCoordinateID = id

		// build and persist the ride (REQUESTED)
		r, err := ride.NewRide(
			rideNumber,
			in.PassengerID,
			in.VehicleType,
			priority,
			pickupCoordinateID,
			destCoordinateID,
		)
		if err != nil {
			return err
		}

		// create ride record
		if err := service.rideRepo.CreateRide(txCtx, r); err != nil {
			return err
		}
		rideID = r.ID
		rideStatus = r.Status.String()

		return nil
	})
	if err != nil {
		service.logger.Error(ctx, "ride_create_failed", "Failed to create ride", err, map[string]any{
			"passenger_id": in.PassengerID,
			"ride_number":  rideNumber,
			"request_id":   correlationID,
		})
		return ports.CreateRideResult{}, err
	}

	// prepare ride request message
	reqMsg := contracts.RideMatchRequest{
		RideID:     rideID,
		RideNumber: rideNumber,
		PickupLocation: contracts.GeoPoint{
			Lat:     in.PickupLatitude,
			Lng:     in.PickupLongitude,
			Address: in.PickupAddress,
		},
		Destination: contracts.GeoPoint{
			Lat:     in.DestinationLatitude,
			Lng:     in.DestinationLongitude,
			Address: in.DestinationAddress,
		},
		RideType:       in.VehicleType.String(),
		EstimatedFare:  estimatedFare,
		MaxDistanceKM:  dst,
		TimeoutSeconds: 30,
		Envelope: contracts.Envelope{
			CorrelationID: correlationID, // e.g., from context/req header
			Producer:      "ride-service",
			SentAt:        time.Now().UTC(),
		},
	}

	// publish ride request for matching
	if err := service.publishRideRequest(ctx, in.VehicleType, reqMsg); err != nil {
		service.logger.Error(ctx, "ride_request_publish_failed", "Failed to publish ride request to RabbitMQ", err, map[string]any{
			"ride_id":    rideID,
			"request_id": correlationID,
		})
	}

	go service.superviseRideMatch(context.Background(), rideID, correlationID)

	// prepare ride status message
	statusMsg := contracts.RideStatusMessage{
		RideID:    rideID,
		Status:    rideStatus,
		Timestamp: time.Now().UTC(),
		Envelope: contracts.Envelope{
			CorrelationID: correlationID,
			Producer:      "ride-service",
		},
	}

	// publish initial ride status (REQUESTED)
	if err := service.publishRideStatus(ctx, statusMsg); err != nil {
		service.logger.Error(ctx, "ride_status_publish_failed", "Failed to publish ride status to RabbitMQ", err, map[string]any{
			"ride_id":    rideID,
			"request_id": correlationID,
		})
	}

	// log succesful creation
	service.logger.Info(ctx, "ride_created", fmt.Sprintf("Ride %s created", rideID), map[string]any{
		"ride_id":      rideID,
		"ride_number":  rideNumber,
		"passenger_id": in.PassengerID,
		"timestamp":    time.Now().UTC(),
		"request_id":   correlationID,
	})

	return ports.CreateRideResult{
		RideID:                   rideID,
		RideNumber:               rideNumber,
		Status:                   rideStatus,
		EstimatedFare:            estimatedFare,
		EstimatedDurationMinutes: min,
		EstimatedDistanceKM:      dst,
	}, nil
}
