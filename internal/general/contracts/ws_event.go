package contracts

import "time"

// WSPassengerRideStatus mirrors messages sent over passenger WebSocket.
type WSPassengerRideStatus struct {
	Type       string       `json:"type"` // "ride_status_update"
	RideID     string       `json:"ride_id"`
	RideNumber string       `json:"ride_number,omitempty"`
	Status     string       `json:"status"`
	DriverInfo *DriverBrief `json:"driver_info,omitempty"`
	Envelope                // allows correlation_id reuse
}

// WSDriverRideOffer mirrors "ride_offer" to drivers.
type WSDriverRideOffer struct {
	Type               string   `json:"type"` // "ride_offer"
	OfferID            string   `json:"offer_id"`
	RideID             string   `json:"ride_id"`
	RideNumber         string   `json:"ride_number,omitempty"`
	Pickup             GeoPoint `json:"pickup_location"`
	Destination        GeoPoint `json:"destination_location"`
	EstimatedFare      float64  `json:"estimated_fare,omitempty"`
	DriverEarnings     float64  `json:"driver_earnings,omitempty"`
	DistanceToPickupKm float64  `json:"distance_to_pickup_km,omitempty"`
	EstimatedRideMin   int      `json:"estimated_ride_duration_minutes,omitempty"`
	ExpiresAt          string   `json:"expires_at,omitempty"` // ISO-8601
	Envelope
}

// В internal/general/contracts/ws_event.go
type WSPassengerLocationUpdate struct {
	Type           string    `json:"type"` // "driver_location_update"
	RideID         string    `json:"ride_id"`
	Location       GeoPoint  `json:"location"`
	SpeedKMH       float64   `json:"speed_kmh,omitempty"`
	HeadingDegrees float64   `json:"heading_degrees,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
	Envelope
}
