package queue

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"ride-hail/internal/common/rabbitmq"
	"ride-hail/internal/ride/domain"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RMQPublisher struct {
	rmq *rabbitmq.ManagerMQ
	log *slog.Logger
}

func NewPublisher(r *rabbitmq.ManagerMQ, logger *slog.Logger) domain.Publisher {
	return &RMQPublisher{
		rmq: r,
		log: logger.With("component", "rmq-publisher"),
	}
}

func (p *RMQPublisher) PublishRideRequest(ctx context.Context, payload any, rideType, corrID string) error {
	ch, err := p.rmq.Channel()
	if err != nil {
		return err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
}

func (p *RMQPublisher) PublishRideStatus(ctx context.Context, payload any, status, corrID string) error {
}
