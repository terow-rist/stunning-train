package domain

import "time"

type Driver struct {
	ID          string
	License     string
	VehicleType string
	Latitude    float64
	Longitude   float64
	Status      string
	Rating      float64
	IsVerified  bool
	UpdatedAt   time.Time
}

type LocationUpdate struct {
	DriverID       string
	Latitude       float64
	Longitude      float64
	AccuracyMeters float64
	SpeedKmh       float64
	HeadingDegrees float64
	RecordedAt     time.Time
}

type LocationResult struct {
	CoordinateID string    `json:"coordinate_id"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type RideRequest struct {
	RideID        string  `json:"ride_id"`
	RideNumber    string  `json:"ride_number"`
	RideType      string  `json:"ride_type"`
	EstimatedFare float64 `json:"estimated_fare"`
	Pickup        struct {
		Lat     float64 `json:"lat"`
		Lng     float64 `json:"lng"`
		Address string  `json:"address"`
	} `json:"pickup_location"`
	Destination struct {
		Lat     float64 `json:"lat"`
		Lng     float64 `json:"lng"`
		Address string  `json:"address"`
	} `json:"destination_location"`
	CorrelationID string `json:"correlation_id"`
}

type SessionSummary struct {
	DurationHours  float64 `json:"duration_hours"`
	RidesCompleted int     `json:"rides_completed"`
	Earnings       float64 `json:"earnings"`
}

type AuthMessage struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

type ServerMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
