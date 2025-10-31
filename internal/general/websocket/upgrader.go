package websocket

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"ride-hail/internal/domain/user"
	"ride-hail/internal/ports"
	"ride-hail/internal/shared/jwt"
	"ride-hail/internal/shared/logger"
	"ride-hail/internal/shared/rabbitmq"

	"github.com/gorilla/websocket"
)

const (
	wsWriteTimeout   = 5 * time.Second
	wsCloseAckWindow = 2 * time.Second
	ctrlTimeout      = 5 * time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// WebSocket handles WebSocket connections with JWT auth.
type WebSocket struct {
	logger          *logger.Logger
	jwtMgr          *jwt.Manager
	pub             *rabbitmq.MQPublisher
	coordinatesRepo ports.CoordinatesRepository
	ridesRepo       ports.RideRepository
	uow             ports.UnitOfWork
	writeMu         sync.Mutex
	writeLocks      sync.Map
	passengers      sync.Map // key: passengerID(string) -> *websocket.Conn
	driverConns     sync.Map // key: driverID -> *websocket.Conn
}

// NewWebSocket creates a WebSocket handler with JWT auth.
func NewWebSocket(logger *logger.Logger, jwtMgr *jwt.Manager, pub *rabbitmq.MQPublisher, coordsRepo ports.CoordinatesRepository, rideRepo ports.RideRepository) *WebSocket {
	return &WebSocket{
		logger:          logger,
		jwtMgr:          jwtMgr,
		pub:             pub,
		coordinatesRepo: coordsRepo,
		ridesRepo:       rideRepo,
	}
}

// ConnectDriver handles WebSocket connections from drivers.
// ConnectDriver handles WebSocket connections from drivers with JWT auth.
// ConnectDriver handles WebSocket connections from drivers with JWT auth.
func (ws *WebSocket) ConnectDriver(w http.ResponseWriter, r *http.Request) {
	// 1) Upgrade HTTP -> WS
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		ws.logger.Error(r.Context(), "websocket_upgrade_failed", "Failed to upgrade to WebSocket", err, nil)
		return
	}
	// Teardown order (LIFO on return):
	defer conn.Close()               // 4) close the socket last
	defer ws.writeLocks.Delete(conn) // 3) forget per-connection mutex (idempotent)

	// 2) Set auth deadline
	conn.SetReadLimit(1 << 20) // 1 MiB
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		ws.logger.Error(r.Context(), "ws_set_deadline_failed", "Failed to set initial read deadline", err, nil)
		ws.sendAuthError(conn, "internal server error")
		return
	}

	// 3) Auth проверка
	msgType, firstFrame, err := conn.ReadMessage()
	if err != nil {
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			ws.logger.Error(r.Context(), "ws_auth_timeout", "Client disconnected before authentication", err, nil)
		} else {
			ws.logger.Error(r.Context(), "ws_auth_read_failed", "Failed to read auth message", err, nil)
		}
		ws.sendAuthError(conn, "authentication timeout: please send auth message within 5 seconds")
		return
	}

	if msgType != websocket.TextMessage {
		ws.logger.Error(r.Context(), "ws_auth_invalid_format", "Auth message must be text format", nil, nil)
		ws.sendAuthError(conn, "auth message must be in text format")
		return
	}

	res, err := jwt.ValidateWSAuth(firstFrame, ws.jwtMgr, user.RoleDriver)
	if err != nil {
		ws.logger.Error(r.Context(), "ws_auth_failed", "Invalid auth message or token", err, nil)
		ws.sendAuthError(conn, "authentication failed: invalid token")
		return
	}

	// 4) Path param must match the subject in claims
	if drvID := r.PathValue("driver_id"); drvID != "" && drvID != res.Claims.Subject {
		ws.logger.Error(r.Context(), "ws_auth_failed", "Driver ID mismatch", nil, map[string]any{
			"path_driver_id": drvID,
			"token_subject":  res.Claims.Subject,
		})
		ws.sendAuthError(conn, "driver ID mismatch")
		return
	}
	driverID := res.Claims.Subject

	// 5) Send authentication success message
	if err := ws.sendAuthSuccess(conn, driverID); err != nil {
		ws.logger.Error(r.Context(), "ws_auth_success_failed", "Failed to send auth success message", err, nil)
		return
	}

	ws.logger.Info(r.Context(), "ws_connected", "Driver WebSocket connected",
		map[string]any{"driver_id": driverID})

	// 6) Reset read deadline after auth
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(_ string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	// 7) Start ping loop (every 30s) using the **per-connection** writer lock
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			mu := ws.lockOf(conn)
			mu.Lock()
			_ = conn.SetWriteDeadline(time.Now().Add(ctrlTimeout))
			err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(ctrlTimeout))
			mu.Unlock()
			if err != nil {
				// Close socket to unblock reader; goroutine exits.
				_ = conn.Close()
				ws.logger.Error(r.Context(), "ws_ping_failed", "Failed to send ping", err, nil)
				return
			}
		}
	}()

	// 8) Register this driver for outbound ride offers; unregister on exit
	ws.RegisterDriverConn(driverID, conn)
	defer ws.RemoveDriverConn(driverID)

	// 9) Optional per-connection state (e.g., last location throttle marker)
	var lastLocAt time.Time

	// 10) Read loop: route messages
	for {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, payload, err := conn.ReadMessage()
		if err != nil {
			// Unexpected vs normal close differentiation
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				ws.logger.Error(r.Context(), "ws_unexpected_close", "Driver connection closed unexpectedly", err, map[string]any{
					"driver_id": driverID,
				})
				ws.wsWriteClose(conn, websocket.CloseInternalServerErr, "internal error")
			} else {
				ws.logger.Info(r.Context(), "ws_connection_closed", "Driver connection closed normally", map[string]any{
					"driver_id": driverID,
				})
				ws.wsWriteClose(conn, websocket.CloseNormalClosure, "bye")
			}
			break
		}

		// Minimal envelope
		var msg struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}

		if err := json.Unmarshal(payload, &msg); err != nil {
			_ = ws.wsWriteMessage(conn, websocket.TextMessage, []byte(`{"type":"error","error":"bad json"}`))
			continue
		}

		switch msg.Type {
		case "ride_response":
			if err := ws.handleRideResponse(conn, driverID, msg.Data); err != nil {
				ws.logger.Error(r.Context(), "driver_ws_message_failed", "driver.response publish failed", err, map[string]any{
					"driver_id": driverID,
				})
				_ = ws.wsWriteMessage(conn, websocket.TextMessage, []byte(`{"type":"error","error":"failed to publish response"}`))
			}

		case "driver_status":
			if err := ws.handleDriverStatus(conn, driverID, msg.Data); err != nil {
				ws.logger.Error(r.Context(), "driver_ws_message_failed", "driver.status publish failed", err, map[string]any{
					"driver_id": driverID,
				})
				_ = ws.wsWriteMessage(conn, websocket.TextMessage, []byte(`{"type":"error","error":"failed to publish driver status"}`))
			}

		case "location_update":
			ws.logger.Info(r.Context(), "location_message_received", "Raw location message received",
				map[string]any{
					"driver_id":       driverID,
					"msg_type":        msg.Type,
					"msg_data_length": len(msg.Data),
				})
			// Uses request context so logs/DB ops inherit request cancellation
			if err := ws.handleLocationUpdate(r.Context(), conn, driverID, msg.Data, &lastLocAt); err != nil {
				// sub-handler logs; no additional action needed
			}

		default:
			_ = ws.wsWriteMessage(conn, websocket.TextMessage, []byte(`{"type":"error","error":"unknown message type"}`))
		}
	}
}

// sendAuthError sends authentication error message to client
func (ws *WebSocket) sendAuthError(conn *websocket.Conn, message string) error {
	errorMsg := map[string]interface{}{
		"type":    "auth_error",
		"error":   message,
		"success": false,
	}
	msgBytes, err := json.Marshal(errorMsg)
	if err != nil {
		return err
	}
	return ws.wsWriteMessage(conn, websocket.TextMessage, msgBytes)
}

// sendAuthSuccess sends authentication success message to client
func (ws *WebSocket) sendAuthSuccess(conn *websocket.Conn, driverID string) error {
	successMsg := map[string]interface{}{
		"type":      "auth_success",
		"message":   "Authentication successful",
		"success":   true,
		"driver_id": driverID,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	msgBytes, err := json.Marshal(successMsg)
	if err != nil {
		return err
	}
	return ws.wsWriteMessage(conn, websocket.TextMessage, msgBytes)
}

// sendAuthSuccessPassenger sends authentication success message to passenger client
func (ws *WebSocket) sendAuthSuccessPassenger(conn *websocket.Conn, passengerID string) error {
	successMsg := map[string]interface{}{
		"type":         "auth_success",
		"message":      "Authentication successful",
		"success":      true,
		"passenger_id": passengerID,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
	}
	msgBytes, err := json.Marshal(successMsg)
	if err != nil {
		return err
	}
	return ws.wsWriteMessage(conn, websocket.TextMessage, msgBytes)
}

// ConnectPassenger handles WebSocket connections from passengers with JWT auth.
func (ws *WebSocket) ConnectPassenger(w http.ResponseWriter, r *http.Request) {
	// 1) Upgrade HTTP -> WS
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		ws.logger.Error(r.Context(), "websocket_upgrade_failed", "Failed to upgrade to WebSocket", err, nil)
		return
	}

	// Teardown order (LIFO on return):
	defer conn.Close()               // 4) close socket last
	defer ws.writeLocks.Delete(conn) // 3) forget per-conn writer lock (idempotent)

	// 2) Set auth deadline
	conn.SetReadLimit(1 << 20) // 1 MiB
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		ws.logger.Error(r.Context(), "ws_set_deadline_failed", "Failed to set initial read deadline", err, nil)
		ws.sendAuthError(conn, "internal server error")
		return
	}

	// 3) Auth проверка
	mt, first, err := conn.ReadMessage()
	if err != nil {
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			ws.logger.Error(r.Context(), "ws_auth_timeout", "Client disconnected before authentication", err, nil)
		} else {
			ws.logger.Error(r.Context(), "ws_auth_read_failed", "Failed to read auth message", err, nil)
		}
		ws.sendAuthError(conn, "authentication timeout: please send auth message within 10 seconds")
		return
	}

	if mt != websocket.TextMessage {
		ws.logger.Error(r.Context(), "ws_auth_invalid_format", "Auth message must be text format", nil, nil)
		ws.sendAuthError(conn, "auth message must be in text format")
		return
	}

	res, err := jwt.ValidateWSAuth(first, ws.jwtMgr, user.RolePassenger)
	if err != nil {
		ws.logger.Error(r.Context(), "ws_auth_failed", "Invalid auth message or token", err, nil)
		ws.sendAuthError(conn, "authentication failed: invalid token")
		return
	}

	if pid := r.PathValue("passenger_id"); pid != "" && pid != res.Claims.Subject {
		ws.logger.Error(r.Context(), "ws_auth_failed", "Passenger ID mismatch", nil, map[string]any{
			"path_passenger_id": pid,
			"token_subject":     res.Claims.Subject,
		})
		ws.sendAuthError(conn, "passenger ID mismatch")
		return
	}
	passengerID := res.Claims.Subject

	// 4) Send authentication success message
	if err := ws.sendAuthSuccessPassenger(conn, passengerID); err != nil {
		ws.logger.Error(r.Context(), "ws_auth_success_failed", "Failed to send auth success message", err, nil)
		return
	}

	ws.logger.Info(r.Context(), "ws_connected", "Passenger WebSocket connected",
		map[string]any{"passenger_id": passengerID})

	// 5) Reset read deadline after auth
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(_ string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	// 6) Start ping loop (every 30s) with per-connection writer lock
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			mu := ws.lockOf(conn)
			mu.Lock()
			_ = conn.SetWriteDeadline(time.Now().Add(ctrlTimeout))
			err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(ctrlTimeout))
			mu.Unlock()
			if err != nil {
				// Close socket to unblock reader; goroutine exits.
				_ = conn.Close()
				ws.logger.Error(r.Context(), "ws_ping_failed", "Failed to send ping", err, map[string]any{
					"passenger_id": passengerID,
				})
				return
			}
		}
	}()

	// 7) Register passenger connection for outbound notifications; unregister on exit
	ws.passengers.Store(passengerID, conn)
	defer ws.passengers.Delete(passengerID)

	// 8) Read loop: route messages from passenger
	for {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, payload, err := conn.ReadMessage()
		if err != nil {
			// Distinguish unexpected close (for logging)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				ws.logger.Error(r.Context(), "ws_unexpected_close", "Passenger connection closed unexpectedly", err, map[string]any{
					"passenger_id": passengerID,
				})
				ws.wsWriteClose(conn, websocket.CloseInternalServerErr, "internal error")
			} else {
				ws.logger.Info(r.Context(), "ws_connection_closed", "Passenger connection closed normally", map[string]any{
					"passenger_id": passengerID,
				})
				ws.wsWriteClose(conn, websocket.CloseNormalClosure, "bye")
			}
			break
		}

		// Minimal envelope
		var msg struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}

		if err := json.Unmarshal(payload, &msg); err != nil {
			_ = ws.wsWriteMessage(conn, websocket.TextMessage, []byte(`{"type":"error","error":"bad json"}`))
			continue
		}

		// Route by message type
		switch msg.Type {
		case "ride_request":
			if err := ws.handlePassengerRideRequest(r.Context(), conn, passengerID, msg.Data); err != nil {
				ws.logger.Error(r.Context(), "passenger_ws_message_failed", "ride.request publish/handle failed", err, map[string]any{
					"passenger_id": passengerID,
				})
				_ = ws.wsWriteMessage(conn, websocket.TextMessage, []byte(`{"type":"error","error":"failed to request ride"}`))
			}

		case "ride_cancel":
			if err := ws.handlePassengerRideCancel(r.Context(), conn, passengerID, msg.Data); err != nil {
				ws.logger.Error(r.Context(), "passenger_ws_message_failed", "ride.cancel publish/handle failed", err, map[string]any{
					"passenger_id": passengerID,
				})
				_ = ws.wsWriteMessage(conn, websocket.TextMessage, []byte(`{"type":"error","error":"failed to cancel ride"}`))
			}

		default:
			_ = ws.wsWriteMessage(conn, websocket.TextMessage, []byte(`{"type":"error","error":"unknown message type"}`))
		}
	}
}
