package ride

import (
	"encoding/json"
	"errors"
	"maps"
	"strings"
	"time"
)

// Event is the domain entity corresponding to the `ride_events` table.
type Event struct {
	// Identity & audit
	ID        string
	CreatedAt time.Time

	// Foreign keys
	RideID string

	// Core payload
	Type EventType
	Data map[string]any
}

var (
	ErrRideIDRequired = errors.New("ride id is required")
	ErrEventDataNil   = errors.New("event data must not be nil")
)

// NewEvent constructs a new domain Event.
func NewEvent(rideID string, eventType EventType, eventData map[string]any) (*Event, error) {
	if rideID = strings.TrimSpace(rideID); rideID == "" {
		return nil, ErrRideIDRequired
	}
	if !eventType.Valid() {
		return nil, ErrInvalidEventType
	}
	if eventData == nil {
		return nil, ErrEventDataNil
	}

	return &Event{
		RideID:    rideID,
		Type:      eventType,
		Data:      cloneMap(eventData),
		CreatedAt: time.Now().UTC(),
	}, nil
}

// Validate performs basic invariants checks mirroring DB constraints.
func (event *Event) Validate() error {
	if event.RideID == "" {
		return ErrRideIDRequired
	}
	if !event.Type.Valid() {
		return ErrInvalidEventType
	}
	if event.Data == nil {
		return ErrEventDataNil
	}
	return nil
}

// DataJSON returns event.Data encoded as JSON.
func (event *Event) DataJSON() ([]byte, error) {
	if event.Data == nil {
		return nil, ErrEventDataNil
	}
	return json.Marshal(event.Data)
}

// WithField sets/overwrites a single key in Data.
func (event *Event) WithField(key string, value any) {
	if event.Data == nil {
		event.Data = make(map[string]any)
	}
	event.Data[key] = value
}

// cloneMap makes a shallow defensive copy of a map[string]any.
func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}

	dst := make(map[string]any, len(src))
	maps.Copy(dst, src)
	return dst
}
