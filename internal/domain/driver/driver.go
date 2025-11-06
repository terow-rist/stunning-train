package driver

import (
	"errors"
	"maps"
	"ride-hail/internal/domain/ride"
	"strings"
	"time"
)

// Attrs is a JSON-friendly bag for vehicle attributes (plate, make, model, color, year, etc.).
type Attrs map[string]any

// Driver is the domain entity corresponding to the `drivers` table.
type Driver struct {
	// Identity & audit
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time

	// Required business fields
	LicenseNumber string
	VehicleType   ride.VehicleType

	// Vehicle details (JSON)
	VehicleAttrs Attrs

	// KPIs
	Rating        float64
	TotalRides    int
	TotalEarnings float64

	// Operational state
	Status     DriverStatus
	IsVerified bool
}

var (
	ErrUserIDRequired      = errors.New("user id is required")
	ErrLicenseRequired     = errors.New("license number is required")
	ErrInvalidStatusSwitch = errors.New("invalid driver status transition")
	ErrInvalidRating       = errors.New("rating must be between 1.0 and 5.0")
	ErrNegativeTotals      = errors.New("totals cannot be negative")
)

// NewDriver creates a new Driver entity with sane defaults.
func NewDriver(userID, licenseNumber string, vehicleType ride.VehicleType, attrs Attrs) (*Driver, error) {
	if userID = strings.TrimSpace(userID); userID == "" {
		return nil, ErrUserIDRequired
	}
	if licenseNumber = strings.TrimSpace(licenseNumber); licenseNumber == "" {
		return nil, ErrLicenseRequired
	}
	if !vehicleType.Valid() {
		return nil, ride.ErrInvalidVehicleType
	}

	now := time.Now().UTC()
	return &Driver{
		ID:            userID,
		CreatedAt:     now,
		UpdatedAt:     now,
		LicenseNumber: licenseNumber,
		VehicleType:   vehicleType,
		VehicleAttrs:  cloneAttrs(attrs),
		Rating:        5.0,
		TotalRides:    0,
		TotalEarnings: 0,
		Status:        DriverStatusOffline,
		IsVerified:    false,
	}, nil
}

// ApplyEarnings increments counters after a ride settlement.
func (driver *Driver) ApplyEarnings(ridesDelta int, earningsDelta float64) error {
	if ridesDelta < 0 || earningsDelta < 0 {
		return ErrNegativeTotals
	}
	driver.TotalRides += ridesDelta
	driver.TotalEarnings += earningsDelta
	driver.touch()
	return nil
}

// ---- State transitions (minimal, explicit) ----

// MarkAvailable transitions OFFLINE/BUSY/EN_ROUTE -> AVAILABLE.
func (driver *Driver) MarkAvailable() error {
	switch driver.Status {
	case DriverStatusOffline, DriverStatusBusy, DriverStatusEnRoute:
		driver.setStatus(DriverStatusAvailable)
		return nil
	default:
		return ErrInvalidStatusSwitch
	}
}

// MarkBusy transitions AVAILABLE/EN_ROUTE -> BUSY.
func (driver *Driver) MarkBusy() error {
	switch driver.Status {
	case DriverStatusAvailable, DriverStatusEnRoute:
		driver.setStatus(DriverStatusBusy)
		return nil
	default:
		return ErrInvalidStatusSwitch
	}
}

// MarkEnRoute transitions AVAILABLE -> EN_ROUTE (after accepting a ride).
func (driver *Driver) MarkEnRoute() error {
	if driver.Status != DriverStatusAvailable {
		return ErrInvalidStatusSwitch
	}
	driver.setStatus(DriverStatusEnRoute)
	return nil
}

// GoOffline transitions AVAILABLE/BUSY/EN_ROUTE -> OFFLINE.
func (driver *Driver) GoOffline() error {
	switch driver.Status {
	case DriverStatusAvailable, DriverStatusBusy, DriverStatusEnRoute:
		driver.setStatus(DriverStatusOffline)
		return nil
	default:
		return ErrInvalidStatusSwitch
	}
}

// ---- internal helpers ----

func (driver *Driver) setStatus(status DriverStatus) {
	driver.Status = status
	driver.touch()
}

func (driver *Driver) touch() {
	driver.UpdatedAt = time.Now().UTC()
}

func cloneAttrs(in Attrs) Attrs {
	if in == nil {
		return nil
	}
	out := make(Attrs, len(in))
	maps.Copy(out, in)
	return out
}
