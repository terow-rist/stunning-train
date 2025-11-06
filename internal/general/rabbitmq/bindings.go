package rabbitmq

import (
	"fmt"
	"ride-hail/internal/general/contracts"

	amqp "github.com/rabbitmq/amqp091-go"
)

func declareTopology(ch *amqp.Channel) error {
	// 1. Exchanges
	exchanges := []struct {
		name string
		kind string
	}{
		{contracts.ExchangeRideTopic, "topic"},
		{contracts.ExchangeDriverTopic, "topic"},
		{contracts.ExchangeLocationFanout, "fanout"},
	}

	for _, ex := range exchanges {
		if err := ch.ExchangeDeclare(ex.name, ex.kind, true, false, false, false, nil); err != nil {
			return fmt.Errorf("declare exchange %s: %w", ex.name, err)
		}
	}

	// 2. Queues
	queues := []string{
		contracts.QueueRideRequests,
		contracts.QueueRideStatus,
		contracts.QueueDriverMatching,
		contracts.QueueDriverResponses,
		contracts.QueueDriverStatus,
		contracts.QueueLocationUpdatesRide,
	}

	for _, q := range queues {
		if _, err := ch.QueueDeclare(q, true, false, false, false, nil); err != nil {
			return fmt.Errorf("declare queue %s: %w", q, err)
		}
	}

	// 3. Bindings
	bindings := []struct {
		queue      string
		exchange   string
		routingKey string
	}{
		{contracts.QueueRideRequests, contracts.ExchangeRideTopic, contracts.RouteRideRequestPrefix + "*"},
		{contracts.QueueRideStatus, contracts.ExchangeRideTopic, contracts.RouteRideStatusPrefix + "*"},
		{contracts.QueueDriverMatching, contracts.ExchangeRideTopic, contracts.RouteRideRequestPrefix + "*"},
		{contracts.QueueDriverResponses, contracts.ExchangeDriverTopic, contracts.RouteDriverRespPrefix + "*"},
		{contracts.QueueDriverStatus, contracts.ExchangeDriverTopic, contracts.RouteDriverStatusPrefix + "*"},
		{contracts.QueueLocationUpdatesRide, contracts.ExchangeLocationFanout, ""},
	}

	for _, b := range bindings {
		if err := ch.QueueBind(b.queue, b.routingKey, b.exchange, false, nil); err != nil {
			return fmt.Errorf("bind queue %s to %s: %w", b.queue, b.exchange, err)
		}
	}

	return nil
}
