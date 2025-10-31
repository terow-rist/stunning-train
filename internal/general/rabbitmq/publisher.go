package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// MQPublisher is a simple RabbitMQ publisher using the Client.
type MQPublisher struct {
	Client *Client
}

// NewMQPublisher constructs an MQPublisher using the provided RabbitMQ client.
func NewMQPublisher(client *Client) *MQPublisher {
	return &MQPublisher{Client: client}
}

// Publish sends a message to the specified RabbitMQ exchange and routing key.
func (publisher *MQPublisher) Publish(exchange, routingKey string, body []byte) error {
	return publisher.Client.PublishMessage(exchange, routingKey, body)
}

// PublishMessage publishes JSON messages with persistence and AMQP.
func (client *Client) PublishMessage(exchange, routingKey string, body []byte) error {
	client.mu.RLock()
	ch := client.pubChan
	conn := client.conn
	client.mu.RUnlock()

	// quick fail if no channel
	if conn == nil || conn.IsClosed() {
		return errors.New("rabbitmq: connection is not open")
	}
	if ch == nil || ch.IsClosed() {
		return errors.New("rabbitmq: publish channel is not open")
	}

	client.pubMu.Lock()
	defer client.pubMu.Unlock()
	confirms := client.pubConfirms

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := ch.PublishWithContext(ctx, exchange, routingKey, true /* mandatory */, false, /* immediate */
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         body,
		},
	); err != nil {
		return err
	}

	select {
	case c := <-confirms:
		if !c.Ack {
			return fmt.Errorf("rabbitmq: publish not acknowledged")
		}
	case <-ctx.Done():
		// keep the confirm stream aligned: try to consume exactly one confirm even if we return a timeout to the caller
		select {
		case c := <-confirms:
			// if we got a confirm now, return an error if it was a nack
			if !c.Ack {
				return fmt.Errorf("rabbitmq: publish not acknowledged after timeout")
			}
		case <-time.After(2 * time.Second):
			// give up trying to read from the confirms channel
		}

		// return the original context error
		return ctx.Err()
	}

	return nil
}
