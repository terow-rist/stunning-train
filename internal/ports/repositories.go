package ports

import (
	"context"
	"time"

	"ride-hail/internal/domain/driver"
	"ride-hail/internal/domain/geo"
	"ride-hail/internal/domain/ride"
	"ride-hail/internal/domain/user"
)

// UnitOfWork interface is used to manage transactions across multiple repository operations.
type UnitOfWork interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// UserRepository defines the methods for managing user data.
type UserRepository interface {
	CreateUser(ctx context.Context, u *user.User) error
	GetByID(ctx context.Context, id string) (*user.User, error)
}

// CoordinatesRepository defines methods for managing coordinates (driver & passenger).
type CoordinatesRepository interface {
	UpsertForDriver(ctx context.Context, driverID string, coord geo.Coordinate, makeCurrent bool) (string, time.Time, error)
	UpsertForPassenger(ctx context.Context, passengerID string, coord geo.Coordinate, makeCurrent bool) (string, time.Time, error)
	GetCurrentForDriver(ctx context.Context, driverID string) (*geo.Coordinate, error)
	GetCurrentForPassenger(ctx context.Context, passengerID string) (*geo.Coordinate, error)
	SaveDriverLocation(ctx context.Context, driverID string, latitude, longitude, accuracyMeters, speedKmh, headingDegrees float64, address string) (*geo.Coordinate, error)
}

// RideRepository defines the methods for managing ride data.
type RideRepository interface {
	CreateRide(ctx context.Context, r *ride.Ride) error
	GetByID(ctx context.Context, id string) (*ride.Ride, error)
	GetActiveForDriver(ctx context.Context, driverID string) (*ride.Ride, error)
	GetRidesByDriver(ctx context.Context, driverID string, limit int) ([]*ride.Ride, error)
	UpdateStatus(ctx context.Context, id string, status ride.Status, ts time.Time) error
	AssignDriver(ctx context.Context, rideID, driverID string, matchedAt time.Time) error
	Complete(ctx context.Context, rideID string, finalFare float64, completedAt time.Time) error
	Cancel(ctx context.Context, rideID, reason string, cancelledAt time.Time) error
	CountActive(ctx context.Context) (int, error)
	CountCreatedBetween(ctx context.Context, start, end time.Time) (int, error)
	CancellationRateBetween(ctx context.Context, start, end time.Time) (float64, error)
	SumFinalFareCompletedBetween(ctx context.Context, start, end time.Time) (float64, error)
	AvgWaitMinutesBetween(ctx context.Context, start, end time.Time) (float64, error)
	AvgRideDurationMinutesBetween(ctx context.Context, start, end time.Time) (float64, error)
	HydrateActiveRows(ctx context.Context, offset, limit int) ([]ActiveRideRow, error)
}

// RideEventRepository defines the methods for managing ride event data.
type RideEventRepository interface {
	Append(ctx context.Context, e *ride.Event) error
}

// DriverRepository defines the methods for managing driver data.
type DriverRepository interface {
	CreateDriver(ctx context.Context, driverObj *driver.Driver) error
	GetByID(ctx context.Context, driverID string) (*driver.Driver, error)
	UpdateStatus(ctx context.Context, driverID string, status driver.DriverStatus) error
	FindNearbyAvailable(ctx context.Context, lat, lng float64, vehicle ride.VehicleType, radiusKm float64, limit int) ([]driver.Driver, error)
	IncrementCountersOnComplete(ctx context.Context, driverID string, earnings float64) error
	CountByStatus(ctx context.Context, status driver.DriverStatus) (int, error)
	CountByVehicleType(ctx context.Context, vehicle ride.VehicleType) (int, error)
	Hotspots(ctx context.Context, limit int) ([]Hotspot, error)
}

// DriverSessionRepository defines the methods for managing driver session data.
type DriverSessionRepository interface {
	Start(ctx context.Context, driverID string) (sessionID string, err error)
	End(ctx context.Context, sessionID string, summary driver.DriverSession) error
	GetActiveForDriver(ctx context.Context, driverID string) (*driver.DriverSession, error)
	IncrementCounters(ctx context.Context, sessionID string, totalRides int, totalEarnings float64) error
}

// LocationHistoryRepository defines the methods for archiving location history data.
type LocationHistoryRepository interface {
	Archive(ctx context.Context, record *geo.LocationHistory) error
}
