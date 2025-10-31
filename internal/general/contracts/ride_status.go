package contracts

import "time"

// RideStatusMessage is published by Ride Service to show the status update.
// Routing key: "ride.status.{status}" on ExchangeRideTopic.
type RideStatusMessage struct {
	RideID    string    `json:"ride_id"`
	Status    string    `json:"status"` // REQUESTED|MATCHED|EN_ROUTE|ARRIVED|IN_PROGRESS|COMPLETED|CANCELLED
	Timestamp time.Time `json:"timestamp"`
	DriverID  string    `json:"driver_id,omitempty"`
	FinalFare *float64  `json:"final_fare,omitempty"`
	Envelope
}
