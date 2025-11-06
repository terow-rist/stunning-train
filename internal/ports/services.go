package ports

import (
	"context"
	"ride-hail/internal/domain/ride"
	"time"
)

// ----- DTOs for Ride Service -----

// CreateRideInput is the validated input required to create a ride.
type CreateRideInput struct {
	PassengerID          string
	PickupLatitude       float64
	PickupLongitude      float64
	PickupAddress        string
	DestinationLatitude  float64
	DestinationLongitude float64
	DestinationAddress   string
	VehicleType          ride.VehicleType
}

// CreateRideResult is returned by RideService.CreateRide() function.
type CreateRideResult struct {
	RideID                   string  `json:"ride_id"`
	RideNumber               string  `json:"ride_number"`
	Status                   string  `json:"status"`
	EstimatedFare            float64 `json:"estimated_fare"`
	EstimatedDurationMinutes int     `json:"estimated_duration_minutes"`
	EstimatedDistanceKM      float64 `json:"estimated_distance_km"`
}

type CancelRideResult struct {
	RideID      string `json:"ride_id"`
	Status      string `json:"status"`
	CancelledAt string `json:"cancelled_at"`
	Message     string `json:"message"`
}

// ----- Ride Service Interface -----

// RideService exposes the boundary for the ride service.
type RideService interface {
	CreateRide(ctx context.Context, in CreateRideInput) (CreateRideResult, error)
	CancelRide(ctx context.Context, rideID, reason string) (CancelRideResult, error)
	RunBackgroundConsumers(ctx context.Context)
}

// ---------------------------------------------------------------------------------------------------------------

// ----- DTOs for Driver & Location Service -----

// GoOnlineInput is the validated input for POST /drivers/{driver_id}/online.
type GoOnlineInput struct {
	DriverID  string  // from path
	Latitude  float64 // from body
	Longitude float64 // from body
}

// GoOnlineResult matches the API response for going online.
type GoOnlineResult struct {
	Status    string `json:"status"`     // "AVAILABLE"
	SessionID string `json:"session_id"` // driver session identifier
	Message   string `json:"message"`    // friendly confirmation
}

// GoOfflineInput is the validated input for POST /drivers/{driver_id}/offline.
type GoOfflineInput struct {
	DriverID string // from path
}

// SessionSummary summarizes an ended online session.
type SessionSummary struct {
	DurationHours  float64 `json:"duration_hours"`
	RidesCompleted int     `json:"rides_completed"`
	Earnings       float64 `json:"earnings"`
}

// GoOfflineResult matches the API response for going offline.
type GoOfflineResult struct {
	Status         string         `json:"status"`     // "OFFLINE"
	SessionID      string         `json:"session_id"` // the same session id
	SessionSummary SessionSummary `json:"session_summary"`
	Message        string         `json:"message"`
}

// UpdateLocationInput is the validated input for POST /drivers/{driver_id}/location.
type UpdateLocationInput struct {
	DriverID       string   // from path
	Latitude       float64  // from body
	Longitude      float64  // from body
	AccuracyMeters *float64 // optional
	SpeedKmh       *float64 // optional
	HeadingDegrees *float64 // optional
}

// UpdateLocationResult matches the API response for a location update.
type UpdateLocationResult struct {
	CoordinateID string    `json:"coordinate_id"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// StartRideInput is the validated input for POST /drivers/{driver_id}/start.
type StartRideInput struct {
	DriverID       string   // from path
	RideID         string   // from body
	DriverLocation GeoPoint `json:"driver_location"` // {"latitude","longitude"}
}

// StartRideResult matches the API response for starting a ride.
type StartRideResult struct {
	RideID    string    `json:"ride_id"`
	Status    string    `json:"status"` // typically "BUSY"
	StartedAt time.Time `json:"started_at"`
	Message   string    `json:"message"`
}

// CompleteRideInput is the validated input for POST /drivers/{driver_id}/complete.
type CompleteRideInput struct {
	DriverID              string   // from path
	RideID                string   // from body
	FinalLocation         GeoPoint `json:"final_location"` // {"latitude","longitude"}
	ActualDistanceKM      float64  `json:"actual_distance_km"`
	ActualDurationMinutes int      `json:"actual_duration_minutes"`
}

// CompleteRideResult matches the API response for completing a ride.
type CompleteRideResult struct {
	RideID         string    `json:"ride_id"`
	Status         string    `json:"status"` // typically "AVAILABLE"
	CompletedAt    time.Time `json:"completed_at"`
	DriverEarnings float64   `json:"driver_earnings"`
	Message        string    `json:"message"`
}

// ----- Driver & Location Service Interface -----

// DriverLocationService defines the methods for managing driver location data.
type DriverLocationService interface {
	GoOnline(ctx context.Context, in GoOnlineInput) (GoOnlineResult, error)
	GoOffline(ctx context.Context, in GoOfflineInput) (GoOfflineResult, error)
	UpdateLocation(ctx context.Context, in UpdateLocationInput) (UpdateLocationResult, error)
	StartRide(ctx context.Context, in StartRideInput) (StartRideResult, error)
	CompleteRide(ctx context.Context, in CompleteRideInput) (CompleteRideResult, error)
	StartBackgroundConsumer(ctx context.Context)
}

// ---------------------------------------------------------------------------------------------------------------

// ----- DTOs for Admin Dashboard -----

// OverviewMetrics groups all numeric KPIs for the overview.
type OverviewMetrics struct {
	ActiveRides                int     `json:"active_rides"`
	AvailableDrivers           int     `json:"available_drivers"`
	BusyDrivers                int     `json:"busy_drivers"`
	TotalRidesToday            int     `json:"total_rides_today"`
	TotalRevenueToday          float64 `json:"total_revenue_today"`
	AverageWaitTimeMinutes     float64 `json:"average_wait_time_minutes"`
	AverageRideDurationMinutes float64 `json:"average_ride_duration_minutes"`
	CancellationRate           float64 `json:"cancellation_rate"`
}

// DriverDistribution shows driver counts by vehicle type.
type DriverDistribution struct {
	Economy int `json:"ECONOMY"`
	Premium int `json:"PREMIUM"`
	XL      int `json:"XL"`
}

// Hotspot is a single hotspot entry for the admin overview.
type Hotspot struct {
	Location       string `json:"location"`
	ActiveRides    int    `json:"active_rides"`
	WaitingDrivers int    `json:"waiting_drivers"`
}

// SystemOverviewResult is the top-level response DTO for GET /admin/overview endpoint.
type SystemOverviewResult struct {
	Timestamp          time.Time          `json:"timestamp"`
	Metrics            OverviewMetrics    `json:"metrics"`
	DriverDistribution DriverDistribution `json:"driver_distribution"`
	Hotspots           []Hotspot          `json:"hotspots"`
}

// GeoPoint represents a simple latitude/longitude pair.
type GeoPoint struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// ActiveRideRow represents a single active ride row in the admin overview.
type ActiveRideRow struct {
	RideID                string    `json:"ride_id"`
	RideNumber            string    `json:"ride_number"`
	Status                string    `json:"status"`
	PassengerID           string    `json:"passenger_id"`
	DriverID              string    `json:"driver_id"`
	PickupAddress         string    `json:"pickup_address"`
	DestinationAddress    string    `json:"destination_address"`
	StartedAt             time.Time `json:"started_at"`
	EstimatedCompletion   time.Time `json:"estimated_completion"`
	CurrentDriverLocation GeoPoint  `json:"current_driver_location"`
	DistanceCompletedKM   float64   `json:"distance_completed_km"`
	DistanceRemainingKM   float64   `json:"distance_remaining_km"`
}

// ActiveRidesResult  is the top-level response DTO for GET /admin/rides/active endpoint.
type ActiveRidesResult struct {
	Rides      []ActiveRideRow `json:"rides"`
	TotalCount int             `json:"total_count"`
	Page       int             `json:"page"`
	PageSize   int             `json:"page_size"`
}

// ----- Admin Service Interface -----

// AdminService exposes monitoring and analytics operations for administrators.
type AdminService interface {
	GetSystemOverview(ctx context.Context) (SystemOverviewResult, error)
	GetActiveRides(ctx context.Context, page, pageSize string) (ActiveRidesResult, error)
}
