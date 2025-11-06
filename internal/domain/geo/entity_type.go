package geo

import (
	"errors"
	"strings"
)

// EntityType indicates the owner of a coordinate record (driver or passenger).
type EntityType string

const (
	EntityTypeDriver    EntityType = "driver"
	EntityTypePassenger EntityType = "passenger"
)

var ErrInvalidEntityType = errors.New("invalid entity type")

// ParseEntityType normalizes (lowercases+trims) and validates an entity type string.
func ParseEntityType(input string) (EntityType, error) {
	entityType := EntityType(strings.ToLower(strings.TrimSpace(input)))
	if entityType.Valid() {
		return entityType, nil
	}
	return "", ErrInvalidEntityType
}

// Valid reports whether entityType is one of the allowed entity type constants.
func (entityType EntityType) Valid() bool {
	switch entityType {
	case EntityTypeDriver, EntityTypePassenger:
		return true
	default:
		return false
	}
}

// String returns the string representation of the EntityType.
func (entityType EntityType) String() string {
	return string(entityType)
}

func (entityType EntityType) IsDriver() bool    { return entityType == EntityTypeDriver }
func (entityType EntityType) IsPassenger() bool { return entityType == EntityTypePassenger }
