package contracts

import "time"

// LocationUpdateMessage is broadcast by Driver & Location Service.
// Exchange: ExchangeLocationFanout (fanout, no routing key).
type LocationUpdateMessage struct {
	DriverID       string    `json:"driver_id"`
	RideID         string    `json:"ride_id,omitempty"`
	Location       GeoPoint  `json:"location"`
	SpeedKMH       float64   `json:"speed_kmh,omitempty"`
	HeadingDegrees float64   `json:"heading_degrees,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
	Envelope
}
