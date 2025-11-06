package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"ride-hail/internal/domain/ride"
	"ride-hail/internal/general/contracts"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RunBackgroundConsumers starts background consumers for the ride service
// that are responsible for updating the ride status and notifying the passenger via WebSocket.
func (service *rideService) RunBackgroundConsumers(ctx context.Context) {
	service.startRideProgressConsumer(ctx)
	service.startLocationUpdatesConsumer(ctx)
}

// startRideProgressConsumer starts a background consumer for driver status messages.
func (service *rideService) startRideProgressConsumer(ctx context.Context) {
	go func() {
		// create a cancellable context for the consume function
		consumeCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		// consume from the predeclared queue and filter by ride_id
		err := service.rabbitmq.Consume(
			consumeCtx,
			contracts.QueueDriverStatus, // queue to consume from
			"ride-progress",             // consumer tag (unique per ride)
			20,                          // prefetch count
			func(_ context.Context, d amqp.Delivery) error {
				// decode the driver status message
				var msg contracts.DriverStatusMessage
				if err := json.Unmarshal(d.Body, &msg); err != nil {
					service.logger.Error(ctx, "driver_status_decode_failed",
						"Failed to decode driver status message", err,
						map[string]any{"size": len(d.Body)})
					return fmt.Errorf("decode: %w", err)
				}

				// if the ride id is empty, ignore the message
				if msg.RideID == "" {
					return nil
				}

				// switch on the status of the driver
				correlationID := msg.CorrelationID
				switch msg.Status {
				case "ARRIVED":
					return service.setRideProgress(ctx, msg.RideID, ride.StatusArrived, correlationID)
				case "IN_PROGRESS":
					return service.setRideProgress(ctx, msg.RideID, ride.StatusInProgress, correlationID)
				case "COMPLETED":
					return service.setRideProgress(ctx, msg.RideID, ride.StatusCompleted, correlationID)
				default:
					// unknown status - just ack & ignore to avoid poison loops
					return nil
				}
			},
		)
		if err != nil && !errors.Is(err, context.Canceled) {
			service.logger.Error(ctx, "ride_progress_consume_failed",
				"Failed to consume driver status messages", err,
				map[string]any{"queue": contracts.QueueDriverStatus})
		}
	}()
}

// setRideProgress persists the new status (idempotent), publishes ride.status.{STATUS}, and notifies the passenger via WebSocket.
func (service *rideService) setRideProgress(
	ctx context.Context,
	rideID string,
	next ride.Status,
	correlationID string,
) error {
	return service.uow.WithinTx(ctx, func(ctx context.Context) error {
		// get the ride by id
		rd, err := service.rideRepo.GetByID(ctx, rideID)
		if err != nil {
			return err
		}

		// apply new state only if it is allowed
		current := rd.Status
		if !next.Valid() || !current.CanTransitionTo(next) {
			return nil
		}
		rd.Status = next

		// update the status of the ride to the next status
		if err := service.rideRepo.UpdateStatus(ctx, rideID, next, time.Now().UTC()); err != nil {
			return err
		}

		// build the ride status message
		statusMsg := contracts.RideStatusMessage{
			RideID:    rideID,
			Status:    rd.Status.String(),
			Timestamp: time.Now().UTC(),
			Envelope: contracts.Envelope{
				CorrelationID: correlationID,
				Producer:      "ride-service",
			},
		}

		// publish the ride status message to the ride topic exchange
		if err := service.publishRideStatus(ctx, statusMsg); err != nil {
			return err
		}

		// build the websocket message
		wsMsg := contracts.WSPassengerRideStatus{
			Type:       "ride_status_update",
			RideID:     rd.ID,
			RideNumber: rd.RideNumber,
			Status:     rd.Status.String(),
			Envelope: contracts.Envelope{
				CorrelationID: "", // optional here; fill if you propagate IDs
				Producer:      "ride-service",
				SentAt:        time.Now().UTC(),
			},
		}

		// notify the passenger about the status update
		if err := service.websocket.NotifyPassengerRideStatus(ctx, rd.PassengerID, wsMsg); err != nil {
			service.logger.Error(ctx, "ws_notify_passenger_failed",
				"Failed to push ride status to passenger", err,
				map[string]any{"ride_id": rd.ID, "passenger_id": rd.PassengerID})
		}

		return nil
	})
}

func (service *rideService) startLocationUpdatesConsumer(ctx context.Context) {
	go func() {
		service.logger.Info(ctx, "location_consumer_starting", "Starting location updates consumer",
			map[string]any{"queue": "location_updates_ride"})

		err := service.rabbitmq.Consume(
			ctx,
			"location_updates_ride",
			"ride-service-locations",
			50,
			func(ctx context.Context, d amqp.Delivery) error {
				var locMsg contracts.LocationUpdateMessage
				if err := json.Unmarshal(d.Body, &locMsg); err != nil {
					service.logger.Error(ctx, "location_decode_failed",
						"Failed to decode location update", err,
						map[string]any{"body_size": len(d.Body)})
					return err
				}

				service.logger.Info(ctx, "location_update_received", "Received location update",
					map[string]any{
						"driver_id":   locMsg.DriverID,
						"ride_id":     locMsg.RideID,
						"lat":         locMsg.Location.Lat,
						"lng":         locMsg.Location.Lng,
						"has_ride_id": locMsg.RideID != "",
					})

				// Используем UnitOfWork для создания транзакции
				err := service.uow.WithinTx(ctx, func(txCtx context.Context) error {
					return service.processLocationUpdate(txCtx, locMsg)
				})
				if err != nil {
					service.logger.Error(ctx, "location_processing_failed",
						"Failed to process location update", err,
						map[string]any{"driver_id": locMsg.DriverID})
				}

				return nil
			},
		)

		if err != nil && err != context.Canceled {
			service.logger.Error(ctx, "location_consumer_failed",
				"Location updates consumer stopped", err, nil)
		} else {
			service.logger.Info(ctx, "location_consumer_stopped",
				"Location updates consumer stopped normally", nil)
		}
	}()
}

// Выносим логику обработки в отдельную функцию
func (service *rideService) processLocationUpdate(ctx context.Context, locMsg contracts.LocationUpdateMessage) error {
	service.logger.Info(ctx, "location_update_processing", "Processing location update",
		map[string]any{
			"driver_id": locMsg.DriverID,
			"ride_id":   locMsg.RideID,
			"lat":       locMsg.Location.Lat,
			"lng":       locMsg.Location.Lng,
		})

	var rideObj *ride.Ride
	var err error

	// Сначала используем ride_id из сообщения, если он есть
	if locMsg.RideID != "" {
		rideObj, err = service.rideRepo.GetByID(ctx, locMsg.RideID)
		if err != nil {
			service.logger.Error(ctx, "get_ride_by_id_failed", "Failed to get ride by ID", err,
				map[string]any{"ride_id": locMsg.RideID})
			// Продолжаем поиск через активную поездку
		} else if rideObj != nil {
			service.logger.Info(ctx, "ride_found_by_id", "Found ride by ID",
				map[string]any{
					"ride_id": rideObj.ID,
					"status":  rideObj.Status.String(),
				})
		}
	}

	// Если не нашли по ride_id или ride_id пустой, ищем активную поездку
	if rideObj == nil {
		service.logger.Info(ctx, "looking_for_active_ride", "Looking for active ride for driver",
			map[string]any{"driver_id": locMsg.DriverID})

		rideObj, err = service.rideRepo.GetActiveForDriver(ctx, locMsg.DriverID)
		if err != nil {
			service.logger.Error(ctx, "get_active_ride_failed", "Failed to get active ride", err,
				map[string]any{"driver_id": locMsg.DriverID})
			return err
		}
	}

	// Если поездка не найдена - просто логируем и выходим
	if rideObj == nil {
		service.logger.Info(ctx, "no_active_ride", "No active ride found for driver - skipping location update",
			map[string]any{"driver_id": locMsg.DriverID})
		return nil
	}

	// Проверяем, что поездка в подходящем статусе для отправки локаций
	if !isRideStatusValidForLocationUpdates(rideObj.Status) {
		service.logger.Info(ctx, "ride_status_invalid_for_location", "Ride status not valid for location updates",
			map[string]any{
				"ride_id": rideObj.ID,
				"status":  rideObj.Status.String(),
			})
		return nil
	}

	service.logger.Info(ctx, "sending_location_to_passenger", "Sending location to passenger",
		map[string]any{
			"ride_id":      rideObj.ID,
			"passenger_id": rideObj.PassengerID,
			"status":       rideObj.Status.String(),
		})

	// Отправляем обновление пассажиру
	wsMsg := contracts.WSPassengerLocationUpdate{
		Type:   "driver_location_update",
		RideID: rideObj.ID,
		Location: contracts.GeoPoint{
			Lat: locMsg.Location.Lat,
			Lng: locMsg.Location.Lng,
		},
		SpeedKMH:       locMsg.SpeedKMH,
		HeadingDegrees: locMsg.HeadingDegrees,
		Timestamp:      locMsg.Timestamp,
		Envelope: contracts.Envelope{
			Producer: "ride-service",
			SentAt:   time.Now().UTC(),
		},
	}

	if err := service.websocket.NotifyPassengerLocationUpdate(ctx, rideObj.PassengerID, wsMsg); err != nil {
		service.logger.Error(ctx, "passenger_location_notify_failed", "Failed to notify passenger", err,
			map[string]any{
				"passenger_id": rideObj.PassengerID,
				"ride_id":      rideObj.ID,
			})
		return err
	}

	service.logger.Info(ctx, "location_update_sent", "Location update sent to passenger",
		map[string]any{
			"ride_id": rideObj.ID,
			"lat":     locMsg.Location.Lat,
			"lng":     locMsg.Location.Lng,
		})

	return nil
}

// isRideStatusValidForLocationUpdates проверяет, можно ли отправлять обновления локации для данного статуса
func isRideStatusValidForLocationUpdates(status ride.Status) bool {
	validStatuses := []ride.Status{
		ride.StatusMatched,
		ride.StatusEnRoute,
		ride.StatusArrived,
		ride.StatusInProgress,
	}

	for _, validStatus := range validStatuses {
		if status == validStatus {
			return true
		}
	}
	return false
}

// Вспомогательные функции
func getRideStatuses(rides []*ride.Ride) []string {
	statuses := make([]string, len(rides))
	for i, r := range rides {
		statuses[i] = r.Status.String()
	}
	return statuses
}

func getRideIDs(rides []*ride.Ride) []string {
	ids := make([]string, len(rides))
	for i, r := range rides {
		ids[i] = r.ID
	}
	return ids
}
