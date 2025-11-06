package geo

import (
	"errors"
	"strings"
	"time"
)

// Coordinate is the domain entity corresponding to the `coordinates` table.
type Coordinate struct {
	ID              string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	EntityID        string
	EntityType      EntityType
	Address         string
	Latitude        float64
	Longitude       float64
	FareAmount      float64 // 0 means "unset/unknown" unless explicitly set via SetTripMetrics
	DistanceKM      float64 // 0 means "unset/unknown" unless explicitly set via SetTripMetrics
	DurationMinutes int     // 0 means "unset/unknown" unless explicitly set via SetTripMetrics
	IsCurrent       bool
}

var (
	ErrEmptyEntityID    = errors.New("entity_id cannot be empty")
	ErrEmptyAddress     = errors.New("address cannot be empty")
	ErrInvalidLatitude  = errors.New("latitude must be between -90 and 90")
	ErrInvalidLongitude = errors.New("longitude must be between -180 and 180")
	ErrNegativeFare     = errors.New("fare_amount cannot be negative")
	ErrNegativeDistance = errors.New("distance_km cannot be negative")
	ErrNegativeDuration = errors.New("duration_minutes cannot be negative")
	ErrBadTimestamps    = errors.New("updated_at cannot be before created_at")
)

// NewCoordinate constructs a new Coordinate entity with IsCurrent=true.
func NewCoordinate(entityID string, entityType EntityType, address string, latitude, longitude float64) (*Coordinate, error) {
	now := time.Now().UTC()
	coordinate := &Coordinate{
		CreatedAt:  now,
		UpdatedAt:  now,
		EntityID:   strings.TrimSpace(entityID),
		EntityType: entityType,
		Address:    strings.TrimSpace(address),
		Latitude:   latitude,
		Longitude:  longitude,
		IsCurrent:  true,
	}
	if err := coordinate.Validate(); err != nil {
		return nil, err
	}

	return coordinate, nil
}

// Validate checks invariants of the Coordinate entity.
func (coordinate *Coordinate) Validate() error {
	if strings.TrimSpace(coordinate.EntityID) == "" {
		return ErrEmptyEntityID
	}
	if !coordinate.EntityType.Valid() {
		return ErrInvalidEntityType
	}
	if strings.TrimSpace(coordinate.Address) == "" {
		return ErrEmptyAddress
	}
	if coordinate.Latitude < -90 || coordinate.Latitude > 90 {
		return ErrInvalidLatitude
	}
	if coordinate.Longitude < -180 || coordinate.Longitude > 180 {
		return ErrInvalidLongitude
	}
	if coordinate.FareAmount < 0 {
		return ErrNegativeFare
	}
	if coordinate.DistanceKM < 0 {
		return ErrNegativeDistance
	}
	if coordinate.DurationMinutes < 0 {
		return ErrNegativeDuration
	}
	if !coordinate.CreatedAt.IsZero() && !coordinate.UpdatedAt.IsZero() && coordinate.UpdatedAt.Before(coordinate.CreatedAt) {
		return ErrBadTimestamps
	}
	return nil
}

// ----- Setters and helpers -----

// UpdateLocation updates address and lat/lng with range checks. Updates UpdatedAt timestamp.
func (coordinate *Coordinate) UpdateLocation(address string, latitude, longitude float64) error {
	address = strings.TrimSpace(address)
	if address == "" {
		return ErrEmptyAddress
	}
	if latitude < -90 || latitude > 90 {
		return ErrInvalidLatitude
	}
	if longitude < -180 || longitude > 180 {
		return ErrInvalidLongitude
	}
	coordinate.Address = address
	coordinate.Latitude = latitude
	coordinate.Longitude = longitude
	coordinate.touch()
	return nil
}

// MarkCurrent toggles the "is_current" flag. Updates UpdatedAt timestamp.
func (coordinate *Coordinate) MarkCurrent(isCurrent bool) {
	coordinate.IsCurrent = isCurrent
	coordinate.touch()
}

// touch sets UpdatedAt to now (UTC).
func (coordinate *Coordinate) touch() {
	coordinate.UpdatedAt = time.Now().UTC()
}
