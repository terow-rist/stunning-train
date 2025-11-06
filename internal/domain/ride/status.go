package ride

import (
	"errors"
	"strings"
)

// Status is a ride status as stored in the `ride_status` table.
type Status string

const (
	StatusRequested  Status = "REQUESTED"
	StatusMatched    Status = "MATCHED"
	StatusEnRoute    Status = "EN_ROUTE"
	StatusArrived    Status = "ARRIVED"
	StatusInProgress Status = "IN_PROGRESS"
	StatusCompleted  Status = "COMPLETED"
	StatusCancelled  Status = "CANCELLED"
)

var ErrInvalidStatus = errors.New("invalid ride status")

// ParseStatus normalizes (uppercases+trims) and validates a status string.
func ParseStatus(in string) (Status, error) {
	status := Status(strings.ToUpper(strings.TrimSpace(in)))
	if status.Valid() {
		return status, nil
	}
	return "", ErrInvalidStatus
}

// Valid reports whether status is one of the allowed ride status constants.
func (status Status) Valid() bool {
	switch status {
	case StatusRequested, StatusMatched, StatusEnRoute, StatusArrived, StatusInProgress, StatusCompleted, StatusCancelled:
		return true
	default:
		return false
	}
}

// String returns the string representation of the Status.
func (status Status) String() string {
	return string(status)
}

// CanTransitionTo specifies if the status can transition to the next status.
func (status Status) CanTransitionTo(next Status) bool {
	switch status {
	case StatusRequested:
		return next == StatusMatched || next == StatusCancelled

	case StatusMatched:
		return next == StatusArrived || next == StatusCancelled

	case StatusArrived:
		return next == StatusInProgress || next == StatusCancelled

	case StatusInProgress:
		return next == StatusCompleted || next == StatusCancelled

	case StatusCompleted, StatusCancelled:
		return false

	default:
		return false
	}
}

// Terminal indicates if the status is in a terminal/completed state.
func (status Status) Terminal() bool {
	return status == StatusCompleted || status == StatusCancelled
}
