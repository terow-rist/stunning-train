package ws

import (
	"context"

	"ride-hail/internal/common/ws"
	"ride-hail/internal/driver_location/domain"
)

var _ domain.WebSocketPort = (*Talker)(nil)

type Talker struct {
	hub *ws.Hub
}

func NewTalker(hub *ws.Hub) *Talker {
	return &Talker{hub: hub}
}

func (t *Talker) SendToDriver(ctx context.Context, driverID string, msg any) error {
	return t.hub.Send(driverID, msg)
}
