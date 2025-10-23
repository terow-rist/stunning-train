package contextx

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"
)

type ctxKey string

const (
	requestIDKey = "request_id"
	rideIDKey    = "ride_id"
)

func WithNewRequestID(ctx context.Context) context.Context {
	return context.WithValue(ctx, requestIDKey, newRequestID())
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func GetRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}

	return ""
}

func WithRideID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, rideIDKey, id)
}

func GetRideID(ctx context.Context) string {
	if v, ok := ctx.Value(rideIDKey).(string); ok {
		return v
	}
	return ""
}

func newRequestID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("req_%d", time.Now().UnixNano())
	}
	return "req_" + hex.EncodeToString(b)
}

