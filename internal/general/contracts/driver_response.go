package contracts

import "time"

// DriverMatchResponse is published by Driver & Location Service.
// Routing key: "driver.response.{ride_id}" on ExchangeDriverTopic.
type DriverMatchResponse struct {
	RideID                  string       `json:"ride_id"`
	DriverID                string       `json:"driver_id"`
	Accepted                bool         `json:"accepted"`
	EstimatedArrivalMinutes int          `json:"estimated_arrival_minutes,omitempty"`
	DriverLocation          *GeoPoint    `json:"driver_location,omitempty"`
	DriverInfo              *DriverBrief `json:"driver_info,omitempty"`
	EstimatedArrival        *time.Time   `json:"estimated_arrival,omitempty"`
	Envelope
}
