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

// superviseRideMatch starts a 2-minute window and listens on the predeclared QueueDriverResponses for the first "accept" of this ride_id.
func (service *rideService) superviseRideMatch(ctx context.Context, rideID, correlationID string) {
	// create a timer for the 2-minute window
	timer := time.NewTimer(5 * time.Minute)
	defer timer.Stop()

	// create a channel for the winner driver id and an error channel
	type winner struct {
		driverID string
	}
	winCh := make(chan winner, 1)
	errCh := make(chan error, 1)

	// create a cancellable context for the consume function
	consumeCtx, cancel := context.WithCancel(context.Background())

	// start a goroutine to consume from the predeclared queue and filter by ride_id
	go func() {
		defer close(errCh)

		// consume from the predeclared queue and filter by ride_id
		err := service.rabbitmq.Consume(
			consumeCtx,                     // context for the consume function
			contracts.QueueDriverResponses, // queue to consume from
			"ride-match-"+rideID,           // consumer tag (unique per ride)
			10,                             // prefetch count
			func(_ context.Context, d amqp.Delivery) error {
				// decode the driver response message
				var msg contracts.DriverMatchResponse
				if err := json.Unmarshal(d.Body, &msg); err != nil {
					service.logger.Error(ctx,
						"driver_response_decode_failed",
						"Failed to decode driver response",
						err,
						map[string]any{"size": len(d.Body)},
					)
					return fmt.Errorf("decode: %w", err)
				}

				// if the ride id and the driver id are the same and the driver accepted the ride, send the driver id to the winner channel
				if msg.RideID == rideID && msg.Accepted {
					select {
					case winCh <- winner{driverID: msg.DriverID}:
					default:
					}
					// if the ride id and the driver id are the same and the driver accepted the ride, return nil
					return nil
				}

				// if the ride id and the driver id are not the same or the driver did not accept the ride, return nil
				return nil
			},
		)
		if err != nil && !errors.Is(err, context.Canceled) {
			errCh <- err
		}
	}()

	// select the winner driver id or the timeout
	select {
	case <-timer.C:
		cancel()
		service.logger.Info(ctx,
			"match_timeout",
			"No driver accepted within 2 minutes",
			map[string]any{"ride_id": rideID, "request_id": correlationID},
		)
		if err := service.cancelOnNoMatch(ctx, rideID, correlationID); err != nil {
			service.logger.Error(ctx,
				"match_timeout_cancel_failed",
				"Failed to cancel ride on timeout",
				err,
				map[string]any{"ride_id": rideID, "request_id": correlationID},
			)
		}
		return

	case w := <-winCh:
		cancel()
		if w.driverID == "" {
			if err := service.cancelOnNoMatch(ctx, rideID, correlationID); err != nil {
				service.logger.Error(ctx,
					"match_winner_empty_driver_cancel_failed",
					"Failed to cancel ride after empty winner",
					err,
					map[string]any{"ride_id": rideID, "request_id": correlationID},
				)
			}
			return
		}
		if err := service.markMatched(ctx, rideID, w.driverID, correlationID); err != nil {
			service.logger.Error(ctx,
				"mark_matched_failed",
				"Failed to persist MATCHED after driver accept",
				err,
				map[string]any{"ride_id": rideID, "driver_id": w.driverID, "request_id": correlationID},
			)
			_ = service.cancelOnNoMatch(ctx, rideID, correlationID)
		}
		return

	case err := <-errCh:
		cancel()
		service.logger.Error(ctx,
			"driver_response_consume_failed",
			"Failed to consume driver responses",
			err,
			map[string]any{"ride_id": rideID, "request_id": correlationID, "queue": contracts.QueueDriverResponses},
		)

		// cancel the ride if the consume function failed
		if err := service.cancelOnNoMatch(ctx, rideID, correlationID); err != nil {
			service.logger.Error(ctx,
				"consume_error_cancel_failed",
				"Failed to cancel ride after consume error",
				err,
				map[string]any{"ride_id": rideID, "request_id": correlationID},
			)
		}
		return
	}
}

// markMatched updates the ride status to matched and assigns the driver to the ride.
// markMatched updates the ride status to matched and assigns the driver to the ride.
func (service *rideService) markMatched(ctx context.Context, rideID, driverID, correlationID string) error {
	return service.uow.WithinTx(ctx, func(ctx context.Context) error {
		// get the ride by id
		rd, err := service.rideRepo.GetByID(ctx, rideID)
		if err != nil {
			return err
		}

		// if the ride is not in the requested status, do nothing
		if rd.Status != ride.StatusRequested {
			service.logger.Error(ctx, "ride_not_requested",
				"Ride is not in REQUESTED status, cannot assign driver", nil,
				map[string]any{
					"ride_id":        rideID,
					"current_status": rd.Status.String(),
					"driver_id":      driverID,
				})
			return nil
		}

		// ВАЖНО: Используем AssignDriver репозитория вместо UpdateStatus
		// AssignDriver устанавливает driver_id И статус MATCHED в одной транзакции
		if err := service.rideRepo.AssignDriver(ctx, rideID, driverID, time.Now().UTC()); err != nil {
			service.logger.Error(ctx, "assign_driver_failed",
				"Failed to assign driver to ride", err,
				map[string]any{
					"ride_id":   rideID,
					"driver_id": driverID,
				})
			return err
		}

		// Перезагружаем поездку чтобы получить обновленные данные
		rd, err = service.rideRepo.GetByID(ctx, rideID)
		if err != nil {
			return err
		}

		// Логируем успешное назначение
		service.logger.Info(ctx, "ride_matched_success",
			"Ride successfully matched with driver",
			map[string]any{
				"ride_id":          rideID,
				"driver_id":        driverID,
				"status":           rd.Status.String(),
				"has_driver_in_db": rd.DriverID != nil,
				"correlation_id":   correlationID,
			})

		// publish the matched status to the ride topic exchange
		statusMsg := contracts.RideStatusMessage{
			RideID:    rideID,
			Status:    ride.StatusMatched.String(),
			Timestamp: time.Now().UTC(),
			Envelope: contracts.Envelope{
				CorrelationID: correlationID,
				Producer:      "ride-service",
			},
		}
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
				CorrelationID: correlationID,
				Producer:      "ride-service",
				SentAt:        time.Now().UTC(),
			},
		}

		// notify the passenger about the matched status
		if err := service.websocket.NotifyPassengerRideStatus(ctx, rd.PassengerID, wsMsg); err != nil {
			service.logger.Error(ctx, "ws_notify_passenger_failed",
				"Failed to push ride status to passenger", err,
				map[string]any{
					"ride_id":      rd.ID,
					"passenger_id": rd.PassengerID,
					"request_id":   correlationID,
				})
		}

		return nil
	})
}

// cancelOnNoMatch cancels the ride and publishes the cancelled status to the ride topic exchange.
func (service *rideService) cancelOnNoMatch(ctx context.Context, rideID, correlationID string) error {
	return service.uow.WithinTx(ctx, func(ctx context.Context) error {
		// get the ride by id
		rd, err := service.rideRepo.GetByID(ctx, rideID)
		if err != nil {
			return err
		}

		// if the ride is not in the requested status, do nothing
		if rd.Status != ride.StatusRequested {
			return nil
		}

		// cancel the ride
		if err := rd.Cancel("NO_MATCH_TIMEOUT"); err != nil {
			return err
		}
		if err := service.rideRepo.Cancel(ctx, rideID, "Not a single match was found in the allotted time", time.Now().UTC()); err != nil {
			return err
		}

		// publish the cancelled status
		statusMsg := contracts.RideStatusMessage{
			RideID:    rideID,
			Status:    rd.Status.String(),
			Timestamp: time.Now().UTC(),
			Envelope: contracts.Envelope{
				CorrelationID: correlationID,
				Producer:      "ride-service",
			},
		}
		return service.publishRideStatus(ctx, statusMsg)
	})
}
