package contracts

import "time"

// DriverStatusMessage is published by Driver & Location Service.
// Routing key: "driver.status.{driver_id}" on ExchangeDriverTopic.
type DriverStatusMessage struct {
	DriverID  string    `json:"driver_id"`
	Status    string    `json:"status"` // OFFLINE|AVAILABLE|BUSY|EN_ROUTE
	RideID    string    `json:"ride_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Envelope
}
