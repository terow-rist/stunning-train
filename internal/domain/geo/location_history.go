package geo

import (
	"errors"
	"math"
	"strings"
	"time"
)

// ID is a type alias for ID of location history (UUIDs in DB).
type ID string

// LocationHistory is the domain entity corresponding to the `location_history` table.
type LocationHistory struct {
	ID             ID
	CoordinateID   string
	DriverID       string
	Latitude       float64
	Longitude      float64
	AccuracyMeters *float64
	SpeedKMH       *float64
	HeadingDegrees *float64
	RecordedAt     time.Time
	RideID         *string
}

var (
	ErrMissingCoordinateID = errors.New("coordinate ID is missing")
	ErrMissingDriverID     = errors.New("driver ID is missing")
	ErrInvalidCoordinates  = errors.New("coordinates cannot be zero")
	ErrNegativeAccuracy    = errors.New("accuracy_meters cannot be negative")
	ErrNegativeSpeed       = errors.New("speed_kmh cannot be negative")
	ErrInvalidHeading      = errors.New("heading_degrees must be between 0 and 360")
	ErrRecordedAtZeroTime  = errors.New("recorded_at must be a valid timestamp")
)

// NewLocationHistory constructs a new LocationHistory record. Only latitude and longitude are strictly required while all other fields are optional.
func NewLocationHistory(
	coordinateID string,
	driverID string,
	rideID *string,
	latitude float64,
	longitude float64,
	accuracyMeters *float64,
	speedKMH *float64,
	headingDegrees *float64,
	recordedAt time.Time,
) (*LocationHistory, error) {
	location := &LocationHistory{
		CoordinateID:   strings.TrimSpace(coordinateID),
		DriverID:       strings.TrimSpace(driverID),
		Latitude:       latitude,
		Longitude:      longitude,
		AccuracyMeters: accuracyMeters,
		SpeedKMH:       speedKMH,
		HeadingDegrees: headingDegrees,
		RecordedAt:     recordedAt,
	}

	if rideID != nil {
		rID := strings.TrimSpace(*rideID)
		location.RideID = &rID
	}

	if location.RecordedAt.IsZero() {
		location.RecordedAt = time.Now().UTC()
	}

	if err := location.Validate(); err != nil {
		return nil, err
	}
	return location, nil
}

// Validate checks invariants of the LocationHistory entity.
func (location LocationHistory) Validate() error {
	// required foreign keys
	if location.CoordinateID == "" {
		return ErrMissingCoordinateID
	}
	if location.DriverID == "" {
		return ErrMissingDriverID
	}

	// required coordinates
	if location.Latitude == 0 && location.Longitude == 0 {
		return ErrInvalidCoordinates
	}
	if location.Latitude < -90 || location.Latitude > 90 || math.IsNaN(location.Latitude) {
		return ErrInvalidLatitude
	}
	if location.Longitude < -180 || location.Longitude > 180 || math.IsNaN(location.Longitude) {
		return ErrInvalidLongitude
	}

	// optional metrics
	if location.AccuracyMeters != nil {
		if *location.AccuracyMeters < 0 || math.IsNaN(*location.AccuracyMeters) {
			return ErrNegativeAccuracy
		}
	}
	if location.SpeedKMH != nil {
		if *location.SpeedKMH < 0 || math.IsNaN(*location.SpeedKMH) {
			return ErrNegativeSpeed
		}
	}
	if location.HeadingDegrees != nil {
		// allow exactly 0 and 360 (some SDKs report 360.0 instead of 0.0)
		if *location.HeadingDegrees < 0 || *location.HeadingDegrees > 360 || math.IsNaN(*location.HeadingDegrees) {
			return ErrInvalidHeading
		}
	}

	// timestamp is required
	if location.RecordedAt.IsZero() {
		return ErrRecordedAtZeroTime
	}
	return nil
}
