package driver

import (
	"errors"
	"strings"
	"time"
)

// DriverSession is the domain entity corresponding to the `driver_sessions` table.
type DriverSession struct {
	ID            string
	DriverID      string
	StartedAt     time.Time
	EndedAt       *time.Time
	TotalRides    int
	TotalEarnings float64
}

var (
	ErrDriverIDRequired    = errors.New("driver id is required")
	ErrSessionAlreadyEnded = errors.New("session already ended")
)

// NewSession creates a new driver session starting "now".
func NewSession(driverID string) (*DriverSession, error) {
	if driverID = strings.TrimSpace(driverID); driverID == "" {
		return nil, ErrDriverIDRequired
	}

	now := time.Now().UTC()
	return &DriverSession{
		DriverID:      driverID,
		StartedAt:     now,
		TotalRides:    0,
		TotalEarnings: 0,
	}, nil
}

// AddRide increments session ride counters.
func (session *DriverSession) AddRide(earnings float64) error {
	if session.EndedAt != nil {
		return ErrSessionAlreadyEnded
	}
	if earnings < 0 {
		return ErrNegativeTotals
	}

	session.TotalRides++
	session.TotalEarnings += earnings
	return nil
}

// End marks the session ended "now". Returns an error on double end.
func (session *DriverSession) End() error {
	if session.EndedAt != nil {
		return ErrSessionAlreadyEnded
	}
	now := time.Now().UTC()
	session.EndedAt = &now
	return nil
}
