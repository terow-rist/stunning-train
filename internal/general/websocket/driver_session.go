package websocket

import (
	"sync"

	"github.com/gorilla/websocket"
)

// DriverSession управляет сессией водителя и автоматической отправкой локаций
type DriverSession struct {
	DriverID     string
	Conn         *websocket.Conn
	IsTracking   bool
	StopTracking chan struct{}
	LastLocation *LocationData
	mu           sync.RWMutex
}

type LocationData struct {
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	AccuracyMeters float64 `json:"accuracy_meters"`
	SpeedKmh       float64 `json:"speed_kmh"`
	HeadingDegrees float64 `json:"heading_degrees"`
}

// UpdateLastLocation безопасно обновляет последнюю локацию
func (ds *DriverSession) UpdateLastLocation(location *LocationData) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.LastLocation = location
}

// GetLastLocation безопасно получает последнюю локацию
func (ds *DriverSession) GetLastLocation() *LocationData {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.LastLocation
}
