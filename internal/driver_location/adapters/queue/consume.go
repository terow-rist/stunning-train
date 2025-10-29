package queue

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"log/slog"
// 	"ride-hail/internal/common/log"
// 	"ride-hail/internal/driver_location/adapters/repository"
// 	"ride-hail/internal/driver_location/domain"

// 	amqp "github.com/rabbitmq/amqp091-go"
// )

// type DriverConsumer struct {
// 	rmq          interface{ Channel() (*amqp.Channel, error) }
// 	driverRepo   *repository.DriverRepository
// 	locationRepo *repository.LocationRepository
// 	logger       *slog.Logger
// }

// func NewDriverConsumer(rmq interface{ Channel() (*amqp.Channel, error) }, driverRepo *repository.DriverRepository, locationRepo *repository.LocationRepository, logger *slog.Logger) *DriverConsumer {
// 	return &DriverConsumer{
// 		rmq:          rmq,
// 		driverRepo:   driverRepo,
// 		locationRepo: locationRepo,
// 		logger:       logger,
// 	}
// }

// func (c *DriverConsumer) Start(ctx context.Context) {
// 	ch, err := c.rmq.Channel()
// 	if err != nil {
// 		log.Error(ctx, c.logger, "queue_start_fail", "Failed to open RMQ channel", err)
// 		return
// 	}

// 	msgs, err := ch.Consume("driver_matching", "", true, false, false, false, nil)
// 	if err != nil {
// 		log.Error(ctx, c.logger, "queue_consume_fail", "Failed to start consuming driver_matching", err)
// 		return
// 	}

// 	log.Info(ctx, c.logger, "consumer_started", "Listening for ride requests on driver_matching queue")

// 	for {
// 		select {
// 		case <-ctx.Done():
// 			log.Info(ctx, c.logger, "consumer_stop", "Stopping consumer gracefully")
// 			return
// 		case msg := <-msgs:
// 			var rideReq domain.RideRequest
// 			if err := json.Unmarshal(msg.Body, &rideReq); err != nil {
// 				log.Error(ctx, c.logger, "msg_unmarshal_fail", "Failed to parse ride request message", err)
// 				continue
// 			}

// 			log.Info(ctx, c.logger, "ride_request_received", "Received new ride request")

// 			drivers, err := c.driverRepo.FindNearbyDrivers(ctx, rideReq.Pickup.Lat, rideReq.Pickup.Lng, rideReq.RideType, 5)
// 			if err != nil {
// 				log.Error(ctx, c.logger, "driver_query_fail", "Failed to query nearby drivers", err)
// 				continue
// 			}

// 			for _, d := range drivers {
// 				fmtMsg := fmt.Sprintf("Driver candidate found: ID=%s, Rating=%.1f, Distance<5km", d.ID, d.Rating)
// 				log.Info(ctx, c.logger, "driver_candidate", fmtMsg)
// 			}
// 		}
// 	}
// }
