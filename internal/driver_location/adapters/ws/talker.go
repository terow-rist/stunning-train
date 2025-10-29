package ws

import (
	"context"

	"ride-hail/internal/common/ws"              // shared Hub
	"ride-hail/internal/driver_location/domain" // domain interface
)

// Ensure Talker implements the domain.WebSocketPort interface.
var _ domain.WebSocketPort = (*Talker)(nil)

// Talker is an outbound WebSocket adapter used by the core application layer
// to send messages to connected driver clients via the shared Hub.
type Talker struct {
	hub *ws.Hub
}

// NewTalker creates a new Talker bound to the shared Hub.
// It satisfies the domain.WebSocketPort interface.
func NewTalker(hub *ws.Hub) *Talker {
	return &Talker{hub: hub}
}

// SendToDriver sends a message to the given driver through the Hub.
// It serializes the message to JSON and writes it over the driver's WebSocket connection.
func (t *Talker) SendToDriver(ctx context.Context, driverID string, msg any) error {
	return t.hub.Send(driverID, msg)
}
