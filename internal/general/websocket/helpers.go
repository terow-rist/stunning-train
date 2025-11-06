package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// wsWriteClose sends a close control frame with the given code and reason.
func (ws *WebSocket) wsWriteClose(conn *websocket.Conn, code int, reason string) {
	mu := ws.lockOf(conn)
	mu.Lock()
	defer mu.Unlock()

	_ = conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
	_ = conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(code, reason),
		time.Now().Add(wsCloseAckWindow),
	)
	ws.writeLocks.Delete(conn)
}

// wsWriteMessage sets a short write deadline and writes a message.
func (ws *WebSocket) wsWriteMessage(conn *websocket.Conn, mt int, payload []byte) error {
	mu := ws.lockOf(conn)
	mu.Lock()
	defer mu.Unlock()
	_ = conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
	return conn.WriteMessage(mt, payload)
}

// lockOf returns the mutex for a specific connection
func (ws *WebSocket) lockOf(conn *websocket.Conn) *sync.Mutex {
	if v, ok := ws.writeLocks.Load(conn); ok {
		if mu, ok := v.(*sync.Mutex); ok && mu != nil {
			return mu
		}
	}
	mu := &sync.Mutex{}
	actual, _ := ws.writeLocks.LoadOrStore(conn, mu)
	return actual.(*sync.Mutex)
}

// writeJSON marshals v and writes a single TextMessage to the given connection.
func (ws *WebSocket) writeJSON(conn *websocket.Conn, v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}

	mu := ws.lockOf(conn)
	mu.Lock()
	defer mu.Unlock()

	_ = conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
	return conn.WriteMessage(websocket.TextMessage, payload)
}

func (ws *WebSocket) SendToDriver(driverID string, msg interface{}) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	conn, ok := ws.GetDriverConn(driverID)
	if !ok {
		return fmt.Errorf("driver %s not connected", driverID)
	}

	return ws.Send(conn, payload)
}

// IsDriverConnected checks if a driver is currently connected via WebSocket
func (ws *WebSocket) IsDriverConnected(driverID string) bool {
	conn, ok := ws.GetDriverConn(driverID)
	return ok && conn != nil
}

func (ws *WebSocket) RemoveDriverConn(driverID string) {
	ws.driverConns.Delete(driverID)
	ws.logger.Info(context.Background(), "driver_ws_removed", "Driver WebSocket connection removed",
		map[string]any{"driver_id": driverID})
}

// StartLocationTracking запускает автоматическую отправку локаций каждые 3 секунды
func (ws *WebSocket) StartLocationTracking(driverID string, conn *websocket.Conn) {
	// Проверяем, не запущен ли уже трекинг для этого водителя
	if existing, ok := ws.driverConns.Load(driverID); ok {
		if session, ok := existing.(*DriverSession); ok && session.IsTracking {
			ws.logger.Info(context.Background(), "location_tracking_already_started",
				"Location tracking already running for driver",
				map[string]any{"driver_id": driverID})
			return
		}
	}

	session := &DriverSession{
		DriverID:     driverID,
		Conn:         conn,
		IsTracking:   true,
		StopTracking: make(chan struct{}),
	}

	// Сохраняем сессию
	ws.driverConns.Store(driverID, session)

	// Запускаем горутину для автоматической отправки
	go ws.locationTrackingLoop(session)

	ws.logger.Info(context.Background(), "location_tracking_started",
		"Started automatic location tracking for driver",
		map[string]any{"driver_id": driverID})
}

// StopLocationTracking останавливает автоматическую отправку локаций
func (ws *WebSocket) StopLocationTracking(driverID string) {
	if existing, ok := ws.driverConns.Load(driverID); ok {
		if session, ok := existing.(*DriverSession); ok && session.IsTracking {
			close(session.StopTracking)
			session.IsTracking = false

			ws.logger.Info(context.Background(), "location_tracking_stopped",
				"Stopped automatic location tracking for driver",
				map[string]any{"driver_id": driverID})
		}
	}
}

// locationTrackingLoop цикл автоматической отправки локаций
func (ws *WebSocket) locationTrackingLoop(session *DriverSession) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	ws.logger.Info(context.Background(), "location_tracking_loop_started",
		"Location tracking loop started",
		map[string]any{"driver_id": session.DriverID})

	for {
		select {
		case <-ticker.C:
			// Отправляем текущую локацию если она есть
			if lastLocation := session.GetLastLocation(); lastLocation != nil {
				ws.sendLocationUpdate(session.Conn, session.DriverID, lastLocation)
			} else {
				ws.logger.Debug(context.Background(), "no_location_data",
					"No location data available for automatic tracking",
					map[string]any{"driver_id": session.DriverID})
			}
		case <-session.StopTracking:
			ws.logger.Info(context.Background(), "location_tracking_loop_exited",
				"Location tracking loop exited",
				map[string]any{"driver_id": session.DriverID})
			return
		}
	}
}

// sendLocationUpdate отправляет обновление локации через WebSocket
func (ws *WebSocket) sendLocationUpdate(conn *websocket.Conn, driverID string, location *LocationData) {
	locationMsg := map[string]interface{}{
		"type": "location_update",
		"data": location,
	}

	msgBytes, err := json.Marshal(locationMsg)
	if err != nil {
		ws.logger.Error(context.Background(), "location_update_marshal_failed",
			"Failed to marshal location update", err,
			map[string]any{"driver_id": driverID})
		return
	}

	if err := ws.wsWriteMessage(conn, websocket.TextMessage, msgBytes); err != nil {
		ws.logger.Error(context.Background(), "location_update_send_failed",
			"Failed to send location update to driver", err,
			map[string]any{"driver_id": driverID})
		// Если не удалось отправить, останавливаем трекинг
		ws.StopLocationTracking(driverID)
	}
}

// UpdateLastLocation обновляет последнюю известную локацию водителя
func (ws *WebSocket) UpdateLastLocation(driverID string, location *LocationData) {
	if existing, ok := ws.driverConns.Load(driverID); ok {
		if session, ok := existing.(*DriverSession); ok {
			session.UpdateLastLocation(location)
		}
	}
}
