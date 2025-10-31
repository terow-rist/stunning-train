package service

import (
	"context"
	"encoding/json"
	"time"

	"ride-hail/internal/domain/ride"
	"ride-hail/internal/general/contracts"

	amqp "github.com/rabbitmq/amqp091-go"
)

// StartBackgroundConsumer starts consuming ride requests from RabbitMQ
func (service *driverLocationService) StartBackgroundConsumer(ctx context.Context) {
	// Consumer for ride requests from ride_topic exchange
	go service.rabbitmq.Consume(ctx, "driver_matching", "driver-service-ride-requests", 10,
		func(ctx context.Context, d amqp.Delivery) error {
			service.logger.Info(ctx, "ride_request_received", "Processing ride request from MQ",
				map[string]any{"routing_key": d.RoutingKey, "body": string(d.Body)})

			var request contracts.RideMatchRequest
			if err := json.Unmarshal(d.Body, &request); err != nil {
				service.logger.Error(ctx, "mq_message_parse_failed", "Failed to parse ride request", err, nil)
				return err
			}
			err := service.uow.WithinTx(ctx, func(ctx context.Context) error {
				// Find nearby available drivers
				nearbyDrivers, err := service.drivers.FindNearbyAvailable(
					ctx,
					request.PickupLocation.Lat,
					request.PickupLocation.Lng,
					ride.VehicleType(request.RideType),
					5.0, // 5km radius
					10,  // max 10 drivers
				)
				if err != nil {
					service.logger.Error(ctx, "find_drivers_failed", "Failed to find nearby drivers", err,
						map[string]any{"ride_id": request.RideID})
					return err
				}

				service.logger.Info(ctx, "drivers_found", "Found nearby drivers for ride request",
					map[string]any{
						"ride_id":       request.RideID,
						"drivers_count": len(nearbyDrivers),
						"vehicle_type":  request.RideType,
					})

				// Send ride offers to drivers via WebSocket
				estimatedRideMin := service.estimateRideMinutes(
					request.PickupLocation.Lat, request.PickupLocation.Lng,
					request.Destination.Lat, request.Destination.Lng,
				)

				// compute the expiration time (30 seconds as per timeout_seconds in request)
				exp := time.Now().Add(time.Duration(request.TimeoutSeconds) * time.Second).UTC().Format(time.RFC3339)

				// send offers to top candidates
				for _, drv := range nearbyDrivers {
					// Check if driver is connected via WebSocket
					if !service.websocket.IsDriverConnected(drv.ID) {
						service.logger.Debug(ctx, "driver_not_connected", "Driver is not connected via WebSocket",
							map[string]any{"driver_id": drv.ID, "ride_id": request.RideID})
						continue
					}

					offerID := service.genOfferID()

					// calculate the distance from THIS driver to pickup (km)
					distKm, err := service.distanceFromDriverToPickup(ctx, drv.ID, request.PickupLocation.Lat, request.PickupLocation.Lng)
					if err != nil {
						service.logger.Error(ctx, "distance_calc_failed", "Failed to get distance driver->pickup", err,
							map[string]any{"ride_id": request.RideID, "driver_id": drv.ID})
						continue // skip this driver but keep others
					}

					// build the offer message
					offer := contracts.WSDriverRideOffer{
						Type:       "ride_offer",
						OfferID:    offerID,
						RideID:     request.RideID,
						RideNumber: request.RideNumber,
						Pickup: contracts.GeoPoint{
							Lat:     request.PickupLocation.Lat,
							Lng:     request.PickupLocation.Lng,
							Address: request.PickupLocation.Address,
						},
						Destination: contracts.GeoPoint{
							Lat:     request.Destination.Lat,
							Lng:     request.Destination.Lng,
							Address: request.Destination.Address,
						},
						EstimatedFare:      request.EstimatedFare,
						DriverEarnings:     request.EstimatedFare * 0.8, // 80% to driver
						DistanceToPickupKm: distKm,
						EstimatedRideMin:   estimatedRideMin,
						ExpiresAt:          exp,
						Envelope: contracts.Envelope{
							Producer:      "driver-location-service",
							CorrelationID: request.CorrelationID,
							SentAt:        time.Now().UTC(),
						},
					}

					// Use WebSocket to send offer to driver
					if err := service.websocket.SendToDriver(drv.ID, offer); err != nil {
						service.logger.Error(ctx, "ws_send_failed", "Failed to send ride offer to driver", err,
							map[string]any{"driver_id": drv.ID, "ride_id": request.RideID})
						continue // Continue with other drivers
					}

					service.logger.Info(ctx, "ride_offer_sent", "Ride offer sent to driver via WebSocket",
						map[string]any{
							"driver_id":   drv.ID,
							"ride_id":     request.RideID,
							"offer_id":    offerID,
							"distance_km": distKm,
							"expires_at":  exp,
						})
				}

				return nil
			})
			if err != nil {
				service.logger.Error(ctx, "trans fomr comsumer", "", err, nil)
			}

			return nil
		})

	service.logger.Info(ctx, "mq_consumer_started", "Driver service MQ consumer started",
		map[string]any{"queue": "driver_matching"})
}
