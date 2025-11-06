package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// newConsumerChannel returns a fresh channel with prefetch (QoS) applied.
func (client *Client) newConsumerChannel(prefetch int) (*amqp.Channel, error) {
	client.mu.RLock()
	conn := client.conn
	client.mu.RUnlock()

	// quick fail if no connection
	if conn == nil || conn.IsClosed() {
		return nil, errors.New("rabbitmq: connection is not ready")
	}

	// open a new channel
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: open channel: %w", err)
	}

	// set prefetch if requested
	if prefetch < 0 {
		prefetch = 1
	}
	if prefetch > 0 {
		if err := ch.Qos(prefetch, 0, false); err != nil {
			_ = ch.Close()
			return nil, fmt.Errorf("rabbitmq: set QoS (prefetch=%d): %w", prefetch, err)
		}
	}

	return ch, nil
}

// Consume starts consuming messages from a queue with manual acks.
func (client *Client) Consume(
	ctx context.Context,
	queue string,
	consumerTag string,
	prefetch int,
	handler func(context.Context, amqp.Delivery) error,
) error {
	// open a fresh channel for this consumer, apply QoS if prefetch > 0
	ch, err := client.newConsumerChannel(prefetch)
	if err != nil {
		return err
	}
	defer ch.Close()

	deliveries, err := ch.Consume(
		queue,
		consumerTag,
		false, // autoAck
		false, // exclusive
		false, // noLocal (ignored by RabbitMQ)
		false, // noWait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("rabbitmq: consume(%s): %w", queue, err)
	}

	chClosed := ch.NotifyClose(make(chan *amqp.Error, 1))

	for {
		select {
		case <-ctx.Done():
			if consumerTag != "" {
				_ = ch.Cancel(consumerTag, false)
			}
			return nil

		case cerr := <-chClosed:
			if cerr != nil {
				return fmt.Errorf("rabbitmq: channel closed while consuming %s: %w", queue, cerr)
			}
			return nil

		case d, ok := <-deliveries:
			if !ok {
				// deliveries stream ended
				return nil
			}

			hCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			err := handler(hCtx, d)
			cancel()

			if err != nil {
				_ = d.Nack(false, false) // drop poison message
				continue
			}
			_ = d.Ack(false)
		}
	}
}
