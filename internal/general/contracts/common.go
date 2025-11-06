package contracts

import "time"

// Envelope adds cross-cutting headers all messages may carry.
type Envelope struct {
	CorrelationID string    `json:"correlation_id,omitempty"` // Correlation for tracing across services
	Producer      string    `json:"producer,omitempty"`       // Producer service name, e.g. "ride-service"
	SentAt        time.Time `json:"sent_at,omitempty"`        // ISO-8601 send time (UTC)
}

type GeoPoint struct {
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Address string  `json:"address,omitempty"`
}

type VehicleInfo struct {
	Make  string `json:"make,omitempty"`
	Model string `json:"model,omitempty"`
	Color string `json:"color,omitempty"`
	Plate string `json:"plate,omitempty"`
}

type DriverBrief struct {
	DriverID string       `json:"driver_id"`
	Name     string       `json:"name,omitempty"`
	Rating   float64      `json:"rating,omitempty"`
	Vehicle  *VehicleInfo `json:"vehicle,omitempty"`
}
