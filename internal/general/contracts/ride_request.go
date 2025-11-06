package contracts

// RideMatchRequest is published by Ride Service to request matching.
// Routing key: "ride.request.{ride_type}" on ExchangeRideTopic.
type RideMatchRequest struct {
	RideID         string   `json:"ride_id"`     // UUID
	RideNumber     string   `json:"ride_number"` // e.g., RIDE_20241216_001
	PickupLocation GeoPoint `json:"pickup_location"`
	Destination    GeoPoint `json:"destination_location"`
	RideType       string   `json:"ride_type"` // ECONOMY|PREMIUM|XL
	EstimatedFare  float64  `json:"estimated_fare,omitempty"`
	MaxDistanceKM  float64  `json:"max_distance_km,omitempty"` // e.g., 5.0
	TimeoutSeconds int      `json:"timeout_seconds,omitempty"` // e.g., 30
	Envelope
}
