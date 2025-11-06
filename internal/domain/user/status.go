package user

import (
	"errors"
	"strings"
)

// Status is a user status as stored in the `user_status` table.
type Status string

const (
	StatusActive   Status = "ACTIVE"
	StatusInactive Status = "INACTIVE"
	StatusBanned   Status = "BANNED"
)

var ErrInvalidStatus = errors.New("invalid status")

// ParseStatus normalizes (uppercases+trims) and validates a status string.
func ParseStatus(in string) (Status, error) {
	status := Status(strings.ToUpper(strings.TrimSpace(in)))
	if status.Valid() {
		return status, nil
	}
	return "", ErrInvalidStatus
}

// Valid reports whether status is one of the allowed status constants.
func (status Status) Valid() bool {
	switch status {
	case StatusActive, StatusInactive, StatusBanned:
		return true
	default:
		return false
	}
}

// String returns the string representation of the Status.
func (status Status) String() string {
	return string(status)
}

// Convenience helpers.
func (status Status) IsActive() bool   { return status == StatusActive }
func (status Status) IsInactive() bool { return status == StatusInactive }
func (status Status) IsBanned() bool   { return status == StatusBanned }
