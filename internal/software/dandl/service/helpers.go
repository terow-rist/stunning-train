package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"ride-hail/internal/domain/ride"
	"ride-hail/internal/general/contracts"
	"ride-hail/internal/general/postgres"
	"time"
)

// generateCorrelationID creates a simple correlation ID for tracing requests.
func generateCorrelationID() string {
	var b [3]byte // 6 hex chars
	_, _ = rand.Read(b[:])
	ts := time.Now().UTC().Format("20060102T150405") // e.g., 20251028T184523
	return "req_" + ts + "_" + hex.EncodeToString(b[:])
}

// genOfferID returns a URL-safe, unique ID like "offer_20251030T031500Z_ab12cd34ef56".
func (service *driverLocationService) genOfferID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "offer_" + time.Now().UTC().Format("20060102T150405Z") + "_" + hex.EncodeToString(b[:])
}

// estimateRideMinutes returns an ETA (minutes) based on straight-line distance between pickup and destination.
// Uses a conservative urban average speed (km/h). You can tune this number in config later if needed.
func (service *driverLocationService) estimateRideMinutes(pickLat, pickLng, destLat, destLng float64) int {
	const avgCitySpeedKmh = 24.0 // ~24 km/h (adjust if you have a better source)
	dKm := ride.HaversineKM(pickLat, pickLng, destLat, destLng)
	if dKm <= 0 {
		return 5 // minimum ETA fallback
	}
	min := int(math.Ceil((dKm / avgCitySpeedKmh) * 60.0))
	if min < 5 {
		return 5
	}
	return min
}

// distanceFromDriverToPickup queries PostGIS for distance from driver's current coord to the pickup (in km).
func (service *driverLocationService) distanceFromDriverToPickup(ctx context.Context, driverID string, pickupLat, pickupLng float64) (float64, error) {
	var km float64
	err := service.uow.WithinTx(ctx, func(ctx context.Context) error {
		tx, err := postgres.MustTxFromContext(ctx)
		if err != nil {
			return err
		}
		row := tx.QueryRow(ctx, `
			SELECT
			  ST_Distance(
			    ST_MakePoint(c.longitude, c.latitude)::geography,
			    ST_MakePoint($1, $2)::geography
			  ) / 1000.0 AS km
			FROM coordinates c
			WHERE c.entity_id = $3
			  AND c.entity_type = 'driver'
			  AND c.is_current = true
			LIMIT 1
		`, pickupLng, pickupLat, driverID)
		if scanErr := row.Scan(&km); scanErr != nil {
			return scanErr
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if km < 0 || math.IsNaN(km) || math.IsInf(km, 0) {
		return 0, errors.New("invalid distance result")
	}
	return km, nil
}

// publishDriverResponse sends a driver match response to the driver_topic exchange
// using routing key "driver.response.{ride_id}" (topic).
func (service *driverLocationService) publishDriverResponse(ctx context.Context, msg contracts.DriverMatchResponse) error {
	// construct routing key (e.g., "driver.response.<ride_id>")
	routingKey := contracts.RouteDriverRespPrefix + msg.RideID

	// marshal and publish
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if err := service.pub.Publish(contracts.ExchangeDriverTopic, routingKey, body); err != nil {
		return err
	}

	// log successful publication
	service.logger.Info(ctx, "driver_response_published", "Published driver match response to RabbitMQ", map[string]any{
		"routing_key": routingKey,
		"ride_id":     msg.RideID,
		"driver_id":   msg.DriverID,
		"accepted":    msg.Accepted,
	})

	return nil
}

// publishDriverStatus sends a driver status update to the driver_topic exchange
// using routing key "driver.status.{driver_id}" (topic).
func (service *driverLocationService) publishDriverStatus(ctx context.Context, msg contracts.DriverStatusMessage) error {
	// construct routing key (e.g., "driver.status.<driver_id>")
	routingKey := contracts.RouteDriverStatusPrefix + msg.DriverID

	// marshal and publish
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if err := service.pub.Publish(contracts.ExchangeDriverTopic, routingKey, body); err != nil {
		return err
	}

	// log successful publication
	service.logger.Info(ctx, "driver_status_published", "Published driver status to RabbitMQ", map[string]any{
		"routing_key": routingKey,
		"driver_id":   msg.DriverID,
		"status":      msg.Status,
		"ride_id":     msg.RideID,
	})

	return nil
}

// broadcastLocationUpdate broadcasts a location update using the fanout exchange.
// Fanout ignores routing keys; pass an empty routing key.
func (service *driverLocationService) broadcastLocationUpdate(ctx context.Context, msg contracts.LocationUpdateMessage) error {
	// marshal and publish (fanout exchange -> routingKey must be "")
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if err := service.pub.Publish(contracts.ExchangeLocationFanout, "", body); err != nil {
		return err
	}

	// log successful publication
	service.logger.Info(ctx, "location_update_published", "Broadcasted location update to RabbitMQ", map[string]any{
		"driver_id": msg.DriverID,
		"ride_id":   msg.RideID,
		"lat":       msg.Location.Lat,
		"lng":       msg.Location.Lng,
	})

	return nil
}
