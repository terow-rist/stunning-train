package service

import (
	"context"
	"ride-hail/internal/domain/geo"
	"ride-hail/internal/general/contracts"
	"ride-hail/internal/ports"
	"time"
)

// UpdateLocation upserts a new "current" coordinate, and archives a LocationHistory record.
func (service *driverLocationService) UpdateLocation(ctx context.Context, in ports.UpdateLocationInput) (ports.UpdateLocationResult, error) {
	var out ports.UpdateLocationResult
	var rideIDPtr *string
	corrID := generateCorrelationID()

	err := service.uow.WithinTx(ctx, func(ctx context.Context) error {
		// ensure that driver exists
		if _, err := service.drivers.GetByID(ctx, in.DriverID); err != nil {
			return err
		}

		// fetch the ride
		if r, err := service.rides.GetActiveForDriver(ctx, in.DriverID); err == nil && r != nil {
			rid := r.ID
			rideIDPtr = &rid
		}

		// simple rate limit: skip writes if the last update is < 3s ago
		if cur, err := service.coords.GetCurrentForDriver(ctx, in.DriverID); err == nil && cur != nil {
			if time.Since(cur.UpdatedAt) < 3*time.Second {
				out.CoordinateID = cur.ID
				out.UpdatedAt = cur.UpdatedAt
				return nil
			}
		}

		// upsert a fresh "current" coordinate
		coord := geo.Coordinate{
			EntityID:   in.DriverID,
			EntityType: geo.EntityTypeDriver,
			Address:    "N/A",
			Latitude:   in.Latitude,
			Longitude:  in.Longitude,
			IsCurrent:  true,
		}
		coordID, updatedAt, err := service.coords.UpsertForDriver(ctx, in.DriverID, coord, true)
		if err != nil {
			return err
		}

		// prepare output
		out.CoordinateID = coordID
		out.UpdatedAt = updatedAt

		// archive new record to location_history
		lh, err := geo.NewLocationHistory(
			coordID,
			in.DriverID,
			rideIDPtr,
			in.Latitude,
			in.Longitude,
			in.AccuracyMeters,
			in.SpeedKmh,
			in.HeadingDegrees,
			time.Now().UTC(),
		)
		if err != nil {
			return err
		}
		return service.locHistory.Archive(ctx, lh)
	})
	if err != nil {
		service.logger.Error(ctx, "driver_location_update_failed", "Failed to update driver location", err, map[string]any{
			"driver_id":  in.DriverID,
			"latitude":   in.Latitude,
			"longitude":  in.Longitude,
			"request_id": corrID,
		})
		return ports.UpdateLocationResult{}, err
	}

	var speed float64
	if in.SpeedKmh != nil {
		speed = *in.SpeedKmh
	}
	var heading float64
	if in.HeadingDegrees != nil {
		heading = *in.HeadingDegrees
	}

	// prepare location update message for fanout broadcast
	locMsg := contracts.LocationUpdateMessage{
		DriverID: in.DriverID,
		Location: contracts.GeoPoint{
			Lat: in.Latitude,
			Lng: in.Longitude,
		},
		SpeedKMH:       speed,
		HeadingDegrees: heading,
		Timestamp:      time.Now().UTC(),
		Envelope: contracts.Envelope{
			Producer:      "driver-location-service",
			CorrelationID: corrID,
		},
	}
	if rideIDPtr != nil {
		locMsg.RideID = *rideIDPtr
	}

	// publish to fanout exchange (no routing key)
	if err = service.broadcastLocationUpdate(ctx, locMsg); err != nil {
		service.logger.Error(ctx, "location_update_publish_failed", "Failed to broadcast location update to RabbitMQ", err, map[string]any{
			"driver_id":  in.DriverID,
			"ride_id":    locMsg.RideID,
			"request_id": corrID,
		})
	}

	// log successful location update
	service.logger.Info(ctx, "driver_location_updated", "Driver location updated and broadcast", map[string]any{
		"driver_id":     in.DriverID,
		"coordinate_id": out.CoordinateID,
		"updated_at":    out.UpdatedAt,
		"ride_id":       locMsg.RideID,
		"lat":           in.Latitude,
		"lng":           in.Longitude,
		"speed_kmh":     speed,
		"heading_deg":   heading,
		"timestamp":     time.Now().UTC(),
		"request_id":    corrID,
	})

	return out, err
}
