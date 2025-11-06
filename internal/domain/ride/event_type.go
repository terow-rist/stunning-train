package ride

import (
	"errors"
	"strings"
)

// EventType corresponds to the values in the `ride_event_type` table.
type EventType string

const (
	EventRideRequested   EventType = "RIDE_REQUESTED"
	EventDriverMatched   EventType = "DRIVER_MATCHED"
	EventDriverArrived   EventType = "DRIVER_ARRIVED"
	EventRideStarted     EventType = "RIDE_STARTED"
	EventRideCompleted   EventType = "RIDE_COMPLETED"
	EventRideCancelled   EventType = "RIDE_CANCELLED"
	EventStatusChanged   EventType = "STATUS_CHANGED"
	EventLocationUpdated EventType = "LOCATION_UPDATED"
	EventFareAdjusted    EventType = "FARE_ADJUSTED"
)

var ErrInvalidEventType = errors.New("invalid ride event type")

// ParseEventType normalizes (uppercases+trims) and validates an event type string.
func ParseEventType(input string) (EventType, error) {
	eventType := EventType(strings.ToUpper(strings.TrimSpace(input)))
	if eventType.Valid() {
		return eventType, nil
	}
	return "", ErrInvalidEventType
}

// Valid reports whether eventType is one of the allowed event type constants.
func (eventType EventType) Valid() bool {
	switch eventType {
	case EventRideRequested,
		EventDriverMatched,
		EventDriverArrived,
		EventRideStarted,
		EventRideCompleted,
		EventRideCancelled,
		EventStatusChanged,
		EventLocationUpdated,
		EventFareAdjusted:
		return true
	default:
		return false
	}
}

// String returns the string representation of the EventType.
func (eventType EventType) String() string {
	return string(eventType)
}
