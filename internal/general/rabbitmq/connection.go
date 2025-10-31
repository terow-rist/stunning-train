package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"time"

	"ride-hail/internal/general/config"
	"ride-hail/internal/general/logger"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Client is a resilient RabbitMQ connector with auto-reconnect and topology setup.
type Client struct {
	url    string
	logger *logger.Logger
	logCtx context.Context // context for logging (without cancel)

	mu      sync.RWMutex
	conn    *amqp.Connection
	pubChan *amqp.Channel

	pubMu       sync.Mutex
	pubConfirms chan amqp.Confirmation

	closed    chan struct{}
	reconnect chan struct{}
}

// ConnectRabbitMQ establishes connection and starts a background watcher that reconnects on failures.
func ConnectRabbitMQ(ctx context.Context, cfg *config.Config, logger *logger.Logger) (*Client, error) {
	// construct the AMQP URL
	url := fmt.Sprintf("amqp://%s:%s@%s:%d/", cfg.RabbitMQ.User, cfg.RabbitMQ.Password, cfg.RabbitMQ.Host, cfg.RabbitMQ.Port)

	// create the initial client structure
	client := &Client{
		url:       url,
		logger:    logger,
		logCtx:    context.WithoutCancel(ctx), // avoid ctx cancel on reconnects
		closed:    make(chan struct{}),
		reconnect: make(chan struct{}, 1),
	}

	// initial connect (single attempt; further retries happen in the watcher)
	if err := client.connectOnce(); err != nil {
		return nil, err
	}

	// background watcher for reconnects
	go client.watch()

	return client, nil
}

// Close gracefully stops the watcher and closes AMQP resources.
func (client *Client) Close() {
	select {
	case <-client.closed:
		// already closed
	default:
		close(client.closed)
	}

	// close connection and channel
	client.mu.Lock()
	if client.pubChan != nil {
		_ = client.pubChan.Close()
		client.pubChan = nil
	}
	if client.conn != nil {
		_ = client.conn.Close()
		client.conn = nil
	}
	client.mu.Unlock()

	// close the confirms channel so any waiters exit cleanly
	client.pubMu.Lock()
	if client.pubConfirms != nil {
		close(client.pubConfirms)
		client.pubConfirms = nil
	}
	client.pubMu.Unlock()
}

// --- internals ---

// connectOnce tries to connect and set up topology once.
func (client *Client) connectOnce() error {
	// establish connection with sane defaults
	conn, err := amqp.DialConfig(client.url, amqp.Config{
		Heartbeat: 10 * time.Second,                   // heartbeat interval
		Locale:    "en_US",                            // default locale
		Dial:      amqp.DefaultDial(30 * time.Second), // dial timeout
	})
	if err != nil {
		client.logger.Error(client.logCtx, "rabbitmq_dial_failed", "Failed to dial RabbitMQ", err, nil)
		return fmt.Errorf("rabbitmq dial failed: %w", err)
	}

	defer func() {
		if err != nil && conn != nil {
			_ = conn.Close()
		}
	}()

	// create a channel for publishing messages
	ch, err := conn.Channel()
	if err != nil {
		client.logger.Error(client.logCtx, "rabbitmq_open_channel_failed", "Failed to open RabbitMQ channel", err, nil)
		return fmt.Errorf("rabbitmq: failed to open channel: %w", err)
	}

	defer func() {
		if err != nil && ch != nil {
			_ = ch.Close()
		}
	}()

	// declare topology (exchanges, queues, bindings)
	if err = declareTopology(ch); err != nil {
		client.logger.Error(client.logCtx, "rabbitmq_declare_topology_failed", "Failed to declare RabbitMQ topology", err, nil)
		return fmt.Errorf("rabbitmq: failed to declare topology: %w", err)
	}

	// enable publisher confirms on the publishing channel
	if err = ch.Confirm(false); err != nil {
		client.logger.Error(client.logCtx, "rabbitmq_enable_confirms_failed", "Failed to enable publisher confirms", err, nil)
		return fmt.Errorf("rabbitmq: failed to enable confirms: %w", err)
	}

	// create the confirms channel
	client.pubMu.Lock()
	oldConfirms := client.pubConfirms
	client.pubConfirms = ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	client.pubMu.Unlock()

	// close the old confirms channel if it exists
	if oldConfirms != nil {
		close(oldConfirms)
	}

	// set up handler for unroutable messages (publish with mandatory=true)
	returns := ch.NotifyReturn(make(chan amqp.Return, 1))
	go func() {
		for r := range returns {
			// keep this simple and non-fatal; the publish call should also surface errors via confirms
			client.logger.Error(client.logCtx, "rabbitmq_returned",
				"Message was returned (unroutable)",
				fmt.Errorf("code=%d text=%s", r.ReplyCode, r.ReplyText),
				map[string]any{
					"exchange":   r.Exchange,
					"routingKey": r.RoutingKey,
					"size":       len(r.Body),
				},
			)
		}

		// the returns channel closed; likely due to channel shutdown/reconnect
		client.logger.Info(client.logCtx, "rabbitmq_return_stream_closed",
			"NotifyReturn channel closed; likely due to channel shutdown/reconnect", nil)
	}()

	// atomically install the new connection + publishing channel
	client.mu.Lock()

	// close/replace any previous publishing channel to avoid leaks
	if client.pubChan != nil && !client.pubChan.IsClosed() {
		_ = client.pubChan.Close()
	}
	client.conn = conn
	client.pubChan = ch

	client.mu.Unlock()

	// watch for connection/channel closures and trigger reconnect
	go func(conn *amqp.Connection, ch *amqp.Channel) {
		// either the connection or the publisher channel closing should trigger reconnect
		connClosed := conn.NotifyClose(make(chan *amqp.Error, 1))
		chClosed := ch.NotifyClose(make(chan *amqp.Error, 1))
		select {
		case <-client.closed:
			return
		case <-connClosed:
		case <-chClosed:
		}

		// try to enqueue a reconnect signal
		select {
		case client.reconnect <- struct{}{}:
		default:
			// already enqueued; no-op
		}
	}(conn, ch)

	client.logger.Info(client.logCtx, "rabbitmq_connected", "RabbitMQ connection established successfully", nil)

	return nil
}

// watch runs in background and attempts reconnects with exponential backoff.
func (client *Client) watch() {
	// reconnect loop with exponential backoff
	backoff := time.Second
	for {
		select {
		case <-client.closed:
			return
		case <-client.reconnect:
			// attempt reconnect until success or Close()
			for {
				select {
				case <-client.closed:
					return
				default:
				}

				err := client.connectOnce()

				if err == nil {
					// reset backoff on success
					backoff = time.Second
					client.logger.Info(client.logCtx, "rabbitmq_reconnected", "Reconnected to RabbitMQ and re-ensured topology", nil)
					break
				}

				// log retry attempt and sleep with backoff
				client.logger.Error(client.logCtx, "retry_attempted", "Failed to reconnect to RabbitMQ", err, nil)

				// cap the backoff
				time.Sleep(backoff)
				if backoff < 30*time.Second {
					backoff *= 2
					if backoff > 30*time.Second {
						backoff = 30 * time.Second
					}
				}
			}
		}
	}
}
