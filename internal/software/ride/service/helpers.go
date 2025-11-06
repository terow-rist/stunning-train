package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"ride-hail/internal/domain/ride"
	"ride-hail/internal/general/contracts"
	"strings"
	"time"
)

// generateRideNumber returns an ID like: RIDE_YYYYMMDD_HHMMSS_XXX
// where XXX is a monotonic millisecond fragment to reduce collisions.
func generateRideNumber() string {
	now := time.Now().UTC()
	return fmt.Sprintf("RIDE_%04d%02d%02d_%02d%02d%02d_%03d",
		now.Year(), int(now.Month()), now.Day(),
		now.Hour(), now.Minute(), now.Second(),
		now.Nanosecond()/1e6, // ms
	)
}

// generateCorrelationID creates a simple correlation ID for tracing requests.
func generateCorrelationID() string {
	var b [3]byte // 6 hex chars
	_, _ = rand.Read(b[:])
	ts := time.Now().UTC().Format("20060102T150405") // e.g., 20251028T184523
	return "req_" + ts + "_" + hex.EncodeToString(b[:])
}

// publishRideRequest sends a ride request to the ride topic exchange using routing key
// ride.request.{vehicleType}, e.g., ride.request.economy.
func (service *rideService) publishRideRequest(ctx context.Context, vt ride.VehicleType, msg contracts.RideMatchRequest) error {
	// construct routing key (e.g., "ride.request.economy")
	routingKey := contracts.RouteRideRequestPrefix + strings.ToLower(vt.String())

	// marshal and publish
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if err := service.pub.Publish(contracts.ExchangeRideTopic, routingKey, body); err != nil {
		return err
	}

	// log successful publication
	service.logger.Info(ctx, "ride_request_published", "Published ride request to RabbitMQ", map[string]any{
		"routing_key": routingKey,
	})

	return nil
}

// publishRideStatus sends a ride status update to the ride topic exchange using routing key
// ride.status.{status}, e.g., ride.status.requested
func (service *rideService) publishRideStatus(ctx context.Context, msg contracts.RideStatusMessage) error {
	// construct routing key (e.g., "ride.request.requested")
	routingKey := contracts.RouteRideStatusPrefix + strings.ToLower(msg.Status)

	// marshal and publish
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if err := service.pub.Publish(contracts.ExchangeRideTopic, routingKey, body); err != nil {
		return err
	}

	// log successful publication
	service.logger.Info(ctx, "ride_status_published", "Published ride status to RabbitMQ", map[string]any{
		"routing_key": routingKey,
	})

	return nil
}
