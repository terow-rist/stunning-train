package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type rmqChanneler interface {
	Channel() (*amqp.Channel, error)
}

type DriverPublisher struct {
	rmq    rmqChanneler
	logger *slog.Logger
}

func NewDriverPublisher(rmq rmqChanneler, logger *slog.Logger) *DriverPublisher {
	return &DriverPublisher{rmq: rmq, logger: logger}
}

// PublishStatus publishes a driver.status.{driver_id} message to driver_topic exchange.
func (p *DriverPublisher) PublishStatus(ctx context.Context, driverID, status, sessionID string) error {
	ch, err := p.rmq.Channel()
	if err != nil {
		return fmt.Errorf("channel: %w", err)
	}
	// defer ch.Close()

	msg := map[string]any{
		"driver_id":  driverID,
		"status":     status,
		"session_id": sessionID,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	routingKey := fmt.Sprintf("driver.status.%s", driverID)
	if err := ch.PublishWithContext(ctx,
		"driver_topic",
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	); err != nil {
		return fmt.Errorf("publish: %w", err)
	}

	p.logger.Info("driver_status_published", "action", "publish_status", "driver_id", driverID, "status", status)
	return nil
}
