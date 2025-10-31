package driver

import (
	"errors"
	"strings"
)

// DriverStatus is a driver status as stored in the `driver_status` table.
type DriverStatus string

const (
	DriverStatusOffline   DriverStatus = "OFFLINE"
	DriverStatusAvailable DriverStatus = "AVAILABLE"
	DriverStatusBusy      DriverStatus = "BUSY"
	DriverStatusEnRoute   DriverStatus = "EN_ROUTE"
)

var ErrInvalidDriverStatus = errors.New("invalid driver status")

// ParseDriverStatus normalizes (uppercases+trims) and validates a driver status string.
func ParseDriverStatus(in string) (DriverStatus, error) {
	status := DriverStatus(strings.ToUpper(strings.TrimSpace(in)))
	if status.Valid() {
		return status, nil
	}
	return "", ErrInvalidDriverStatus
}

// Valid reports whether the driver status is one of the allowed driver status constants.
func (status DriverStatus) Valid() bool {
	switch status {
	case DriverStatusOffline, DriverStatusAvailable, DriverStatusBusy, DriverStatusEnRoute:
		return true
	default:
		return false
	}
}

// Terminal indicates if the driver is in a terminal/non-working state.
func (status DriverStatus) Terminal() bool {
	return status == DriverStatusOffline
}

// String returns the string representation of the DriverStatus.
func (status DriverStatus) String() string {
	return string(status)
}
