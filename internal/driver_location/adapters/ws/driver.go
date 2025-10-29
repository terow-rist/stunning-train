package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"ride-hail/internal/common/ws"
	"ride-hail/internal/driver_location/domain"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type WSHandler struct {
	logger   *slog.Logger
	hub      *ws.Hub
	upgrader websocket.Upgrader
}

func NewWSHandler(logger *slog.Logger, hub *ws.Hub) *WSHandler {
	return &WSHandler{
		logger: logger,
		hub:    hub,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (h *WSHandler) HandleDriverWS(w http.ResponseWriter, r *http.Request) {
	// URL: /ws/drivers/{driver_id}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/ws/drivers/"), "/")
	if len(parts) < 1 || parts[0] == "" {
		http.Error(w, "missing driver id", http.StatusBadRequest)
		return
	}
	driverID := parts[0]

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("ws_upgrade_fail", "error", err)
		return
	}
	defer conn.Close()

	h.logger.Info("ws_connected", "driver_id", driverID)

	authCh := make(chan bool, 1)
	go h.waitForAuth(conn, driverID, authCh)

	select {
	case ok := <-authCh:
		if !ok {
			h.logger.Warn("ws_auth_fail", "driver_id", driverID)
			conn.WriteJSON(domain.ServerMessage{Type: "error", Message: "auth failed"})
			return
		}
		h.logger.Info("ws_auth_success", "driver_id", driverID)
		conn.WriteJSON(domain.ServerMessage{Type: "info", Message: "authenticated"})
	case <-time.After(5 * time.Second):
		conn.WriteJSON(domain.ServerMessage{Type: "error", Message: "auth timeout"})
		return
	}

	h.hub.Add(driverID, conn)
	defer h.hub.Remove(driverID)

	// start keepalive
	conn.SetPongHandler(func(appData string) error {
		return nil
	})

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second)); err != nil {
				h.logger.Warn("ws_ping_fail", "driver_id", driverID, "error", err)
				return
			}
		default:
			// echo simple received messages
			_, msg, err := conn.ReadMessage()
			if err != nil {
				h.logger.Warn("ws_read_fail", "driver_id", driverID, "error", err)
				return
			}
			h.logger.Info("ws_msg", "driver_id", driverID, "msg", string(msg))
		}
	}
}

func (h *WSHandler) waitForAuth(conn *websocket.Conn, driverID string, result chan<- bool) {
	defer close(result)

	// wait for the first message
	_, data, err := conn.ReadMessage()
	if err != nil {
		result <- false
		return
	}

	var auth domain.AuthMessage
	if err := json.Unmarshal(data, &auth); err != nil {
		result <- false
		return
	}

	if auth.Type != "auth" || !strings.HasPrefix(auth.Token, "Bearer ") {
		result <- false
		return
	}

	// TODO: later verify JWT here
	result <- true
}
