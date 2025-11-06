package ride

import (
	"errors"
	"strings"
)

// VehicleType is a vehicle type as stored in the `vehicle_type` table.
type VehicleType string

const (
	VehicleEconomy VehicleType = "ECONOMY"
	VehiclePremium VehicleType = "PREMIUM"
	VehicleXL      VehicleType = "XL"
)

var ErrInvalidVehicleType = errors.New("invalid vehicle type")

// ParseVehicleType normalizes (uppercases+trims) and validates a vehicle type string.
func ParseVehicleType(in string) (VehicleType, error) {
	vt := VehicleType(strings.ToUpper(strings.TrimSpace(in)))
	if vt.Valid() {
		return vt, nil
	}
	return "", ErrInvalidVehicleType
}

// Valid reports whether vehicleType is one of the allowed vehicle type constants.
func (vehicleType VehicleType) Valid() bool {
	switch vehicleType {
	case VehicleEconomy, VehiclePremium, VehicleXL:
		return true
	default:
		return false
	}
}

// String returns the string representation of the VehicleType.
func (vehicleType VehicleType) String() string {
	return string(vehicleType)
}
