package domain

import (
	"context"
)

type DriverRepository interface {
	StartSession(ctx context.Context, driverID string) (string, error)
	UpdateStatus(ctx context.Context, driverID, status string) error
}

type LocationRepository interface {
	SaveLocation(ctx context.Context, loc LocationUpdate) error
}

type Publisher interface {
	PublishStatus(ctx context.Context, driverID, status, sessionID string) error
}

type WebSocketPort interface {
	SendToDriver(ctx context.Context, driverID string, msg any) error
}
