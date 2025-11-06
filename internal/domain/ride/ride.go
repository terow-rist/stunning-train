package ride

import (
	"errors"
	"math"
	"strings"
	"time"
)

// Ride is the domain entity corresponding to the `rides` table.
type Ride struct {
	// Identity & audit
	ID         string
	RideNumber string
	CreatedAt  time.Time
	UpdatedAt  time.Time

	// Actors
	PassengerID string
	DriverID    *string // nil until matched

	// Core state
	VehicleType VehicleType
	Status      Status
	Priority    int

	// Lifecycle timestamps
	RequestedAt time.Time
	MatchedAt   *time.Time
	ArrivedAt   *time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	CancelledAt *time.Time

	// Additional info
	CancellationReason *string
	EstimatedFare      *float64
	FinalFare          *float64

	// Coordinates
	PickupCoordinateID      *string
	DestinationCoordinateID *string
}

var (
	ErrPassengerRequired       = errors.New("passenger id is required")
	ErrPriorityOutOfRange      = errors.New("priority must be between 1 and 10")
	ErrRideNumberRequired      = errors.New("ride number is required")
	ErrInvalidStatusTransition = errors.New("invalid ride status transition")
	ErrAlreadyAssigned         = errors.New("driver already assigned")
	ErrNoDriverAssigned        = errors.New("no driver assigned")
	ErrDriverRequired          = errors.New("driver id is required")
)

// New creates a new ride in REQUESTED state.
func NewRide(rideNumber, passengerID string, vt VehicleType, priority int, pickupCoordinateID, destinationCoordinateID string) (*Ride, error) {
	if rideNumber = strings.TrimSpace(rideNumber); rideNumber == "" {
		return nil, ErrRideNumberRequired
	}
	if passengerID = strings.TrimSpace(passengerID); passengerID == "" {
		return nil, ErrPassengerRequired
	}
	if !vt.Valid() {
		return nil, ErrInvalidVehicleType
	}
	if priority < 1 || priority > 10 {
		return nil, ErrPriorityOutOfRange
	}

	now := time.Now().UTC()
	ride := &Ride{
		RideNumber:  rideNumber,
		CreatedAt:   now,
		UpdatedAt:   now,
		PassengerID: passengerID,
		VehicleType: vt,
		Status:      StatusRequested,
		Priority:    priority,
		RequestedAt: now,
	}

	// coordinates are optional in the DB.
	if pickupCoordinateID != "" {
		ride.PickupCoordinateID = &pickupCoordinateID
	}
	if destinationCoordinateID != "" {
		ride.DestinationCoordinateID = &destinationCoordinateID
	}

	return ride, nil
}

// AssignDriver sets the driver and moves REQUESTED -> MATCHED.
func (ride *Ride) AssignDriver(driverID string) error {
	if driverID == "" {
		return ErrDriverRequired
	}
	if ride.DriverID != nil && *ride.DriverID != "" {
		return ErrAlreadyAssigned
	}
	if ride.Status != StatusRequested {
		return ErrInvalidStatusTransition
	}

	ride.DriverID = &driverID
	now := time.Now().UTC()
	ride.MatchedAt = &now
	ride.setStatus(StatusMatched)
	return nil
}

// MarkEnRoute transitions MATCHED -> EN_ROUTE.
func (ride *Ride) MarkEnRoute() error {
	if ride.DriverID == nil || *ride.DriverID == "" {
		return ErrNoDriverAssigned
	}
	if ride.Status != StatusMatched {
		return ErrInvalidStatusTransition
	}
	ride.setStatus(StatusEnRoute)
	return nil
}

// MarkArrived transitions EN_ROUTE -> ARRIVED.
func (ride *Ride) MarkArrived() error {
	if ride.DriverID == nil || *ride.DriverID == "" {
		return ErrNoDriverAssigned
	}
	if ride.Status != StatusEnRoute {
		return ErrInvalidStatusTransition
	}
	now := time.Now().UTC()
	ride.ArrivedAt = &now
	ride.setStatus(StatusArrived)
	return nil
}

// Start transitions ARRIVED -> IN_PROGRESS.
func (ride *Ride) Start() error {
	if ride.DriverID == nil || *ride.DriverID == "" {
		return ErrNoDriverAssigned
	}
	if ride.Status != StatusArrived {
		return ErrInvalidStatusTransition
	}
	now := time.Now().UTC()
	ride.StartedAt = &now
	ride.setStatus(StatusInProgress)
	return nil
}

// Complete transitions IN_PROGRESS -> COMPLETED.
func (ride *Ride) Complete(finalFare float64) error {
	if ride.Status != StatusInProgress {
		return ErrInvalidStatusTransition
	}
	now := time.Now().UTC()
	ride.CompletedAt = &now
	ride.FinalFare = &finalFare
	ride.setStatus(StatusCompleted)
	return nil
}

// Cancel transitions to CANCELLED (if not terminal).
func (ride *Ride) Cancel(reason string) error {
	if ride.Status.Terminal() {
		return ErrInvalidStatusTransition
	}
	now := time.Now().UTC()
	ride.CancelledAt = &now
	if rs := strings.TrimSpace(reason); rs != "" {
		ride.CancellationReason = &rs
	}
	ride.setStatus(StatusCancelled)
	return nil
}

// haversine distance in kilometers
func HaversineKM(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0 // Earth radius in km
	a1 := lat1 * math.Pi / 180
	a2 := lat2 * math.Pi / 180
	da := (lat2 - lat1) * math.Pi / 180
	db := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(da/2)*math.Sin(da/2) +
		math.Cos(a1)*math.Cos(a2)*math.Sin(db/2)*math.Sin(db/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

// duration estimate from distance with a simple average-city-speed heuristic.
func EstimateDurationMinutes(distanceKM float64) int {
	const avgSpeedKMH = 21.0 // ~5.2 km in 15 min (as in the spec example)
	minutes := (distanceKM / avgSpeedKMH) * 60.0

	// ceil to whole minutes
	m := int(math.Ceil(minutes))
	if m < 1 {
		return 1
	}

	return m
}

// ComputeFinalFare returns base + (distance_km * rate_per_km) + (duration_min * rate_per_min).
func ComputeFinalFare(vt VehicleType, distanceKM float64, durationMin int) float64 {
	type rates struct {
		base      float64
		perKM     float64
		perMinute float64
	}

	var rate rates
	switch vt {
	case VehicleEconomy:
		rate = rates{base: 500, perKM: 100, perMinute: 50}
	case VehiclePremium:
		rate = rates{base: 800, perKM: 120, perMinute: 60}
	case VehicleXL:
		rate = rates{base: 1000, perKM: 150, perMinute: 75}
	default:
		rate = rates{base: 500, perKM: 100, perMinute: 50} // default to ECONOMY if something slips through
	}

	if distanceKM < 0 {
		distanceKM = 0
	}
	if durationMin < 0 {
		durationMin = 0
	}

	return rate.base + rate.perKM*distanceKM + rate.perMinute*float64(durationMin)
}

// computePriority returns a priority in [1..10] using only vehicle type and trip distance (km).
func ComputePriority(vt VehicleType, tripDistanceKM float64) int {
	// base by vehicle type
	base := 3
	switch vt {
	case VehicleEconomy:
		base = 3
	case VehiclePremium:
		base = 5
	case VehicleXL:
		base = 7
	}

	// distance modifier (longer trips -> slightly higher priority)
	if tripDistanceKM < 0 {
		tripDistanceKM = 0
	}
	mod := 0
	switch {
	case tripDistanceKM >= 15:
		mod = 3
	case tripDistanceKM >= 8:
		mod = 2
	case tripDistanceKM >= 3:
		mod = 1
	default:
		mod = 0
	}

	p := base + mod
	return p
}

// ----- internal helpers -----

func (ride *Ride) setStatus(status Status) {
	ride.Status = status
	ride.touch()
}

func (ride *Ride) touch() {
	ride.UpdatedAt = time.Now().UTC()
}
