package auth

type DriverClaims struct {
	DriverID string `json:"driver_id"`
	Exp      int64  `json:"exp"` // optional, not enforced yet
}

// now are not used
type PassengerClaims struct {
	PassengerID string `json:"passenger_id"`
	Exp         int64  `json:"exp"` // optional, not enforced yet
}

type ServiceClaims struct {
	Service string `json:"service"`
	Exp     int64  `json:"exp"` // optional, not enforced yet
}
