package domain

import (
	"fmt"
	"math"
	"strings"
)

type RideRequest struct {
	PassengerID          string  `json:"passenger_id"`
	PickupLatitude       float64 `json:"pickup_latitude"`
	PickupLongitude      float64 `json:"pickup_longitude"`
	PickupAddress        string  `json:"pickup_address"`
	DestinationLatitude  float64 `json:"destination_latitude"`
	DestinationLongitude float64 `json:"destination_longitude"`
	DestinationAddress   string  `json:"destination_address"`
	RideType             string  `json:"ride_type"`
}

func (r *RideRequest) Validate() error {
	// --- Basic presence checks ---
	if strings.TrimSpace(r.PassengerID) == "" {
		return fmt.Errorf("%w: passenger_id required", ErrInvalidRideRequest)
	}
	if strings.TrimSpace(r.PickupAddress) == "" {
		return fmt.Errorf("%w: pickup_address required", ErrInvalidRideRequest)
	}
	if strings.TrimSpace(r.DestinationAddress) == "" {
		return fmt.Errorf("%w: destination_address required", ErrInvalidRideRequest)
	}
	if strings.TrimSpace(r.RideType) == "" {
		return fmt.Errorf("%w: ride_type required", ErrInvalidRideRequest)
	}

	// --- Coordinate sanity checks ---
	if math.IsNaN(r.PickupLatitude) || math.IsNaN(r.PickupLongitude) ||
		math.IsNaN(r.DestinationLatitude) || math.IsNaN(r.DestinationLongitude) {
		return fmt.Errorf("%w: coordinate contains NaN", ErrInvalidRideRequest)
	}

	if math.IsInf(r.PickupLatitude, 0) || math.IsInf(r.PickupLongitude, 0) ||
		math.IsInf(r.DestinationLatitude, 0) || math.IsInf(r.DestinationLongitude, 0) {
		return fmt.Errorf("%w: coordinate contains infinite value", ErrInvalidRideRequest)
	}

	if r.PickupLatitude == 0 || r.PickupLongitude == 0 ||
		r.DestinationLatitude == 0 || r.DestinationLongitude == 0 {
		return fmt.Errorf("%w: coordinates cannot be zero", ErrInvalidRideRequest)
	}

	if r.PickupLatitude < -90 || r.PickupLatitude > 90 ||
		r.DestinationLatitude < -90 || r.DestinationLatitude > 90 {
		return fmt.Errorf("%w: latitude must be between -90 and 90", ErrInvalidRideRequest)
	}

	if r.PickupLongitude < -180 || r.PickupLongitude > 180 ||
		r.DestinationLongitude < -180 || r.DestinationLongitude > 180 {
		return fmt.Errorf("%w: longitude must be between -180 and 180", ErrInvalidRideRequest)
	}

	// --- Ride type validation ---
	r.RideType = strings.ToUpper(r.RideType)
	validTypes := map[string]bool{
		"ECONOMY": true,
		"PREMIUM": true,
		"XL":      true,
	}
	if !validTypes[r.RideType] {
		return fmt.Errorf("%w: invalid ride_type '%s'", ErrInvalidRideRequest, r.RideType)
	}

	return nil
}

func (r *RideRequest) EstimateFare() (fare float64, distKm float64, durMin int) {
	distKm = haversineKm(r.PickupLatitude, r.PickupLongitude,
		r.DestinationLatitude, r.DestinationLongitude)
	durMin = estimateDurationMin(distKm, 25)
	base := 500.0
	perKm := 100.0
	perMin := 50.0
	raw := base + perKm*distKm + perMin*float64(durMin)
	fare = math.Round(raw/10) * 10
	return
}

type RideResponse struct {
	RideID                   string  `json:"ride_id"`
	RideNumber               string  `json:"ride_number"`
	Status                   string  `json:"status"`
	EstimatedFare            float64 `json:"estimated_fare"`
	EstimatedDurationMinutes int     `json:"estimated_duration_minutes"`
	EstimatedDistanceKm      float64 `json:"estimated_distance_km"`
}

type AuthMessage struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

type ServerMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// --- geometry helpers ---
func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

func estimateDurationMin(distanceKm, avgSpeedKmh float64) int {
	if avgSpeedKmh <= 1 {
		avgSpeedKmh = 25
	}
	minutes := distanceKm / avgSpeedKmh * 60
	if minutes < 1 {
		minutes = 1
	}
	return int(math.Ceil(minutes))
}
