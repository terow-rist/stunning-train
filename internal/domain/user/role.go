package user

import (
	"errors"
	"strings"
)

// Role is a user role as stored in the `roles` table.
type Role string

const (
	RolePassenger Role = "PASSENGER"
	RoleDriver    Role = "DRIVER"
	RoleAdmin     Role = "ADMIN"
)

var ErrInvalidRole = errors.New("invalid role")

// ParseRole normalizes (uppercases+trims) and validates a role string.
func ParseRole(s string) (Role, error) {
	role := Role(strings.ToUpper(strings.TrimSpace(s)))
	if role.Valid() {
		return role, nil
	}
	return "", ErrInvalidRole
}

// Valid reports whether role is one of the allowed role constants.
func (role Role) Valid() bool {
	switch role {
	case RolePassenger, RoleDriver, RoleAdmin:
		return true
	default:
		return false
	}
}

// String returns the string representation of the Role.
func (role Role) String() string {
	return string(role)
}

// Convenience helpers.
func (role Role) IsPassenger() bool { return role == RolePassenger }
func (role Role) IsDriver() bool    { return role == RoleDriver }
func (role Role) IsAdmin() bool     { return role == RoleAdmin }
