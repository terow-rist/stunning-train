package ws

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"log/slog"
	"ride-hail/internal/common/auth"
	"ride-hail/internal/common/ws"
	"ride-hail/internal/driver_location/domain"

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
	driverID := strings.TrimPrefix(r.URL.Path, "/ws/drivers/")
	if driverID == "" {
		http.Error(w, "missing driver id", http.StatusBadRequest)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("ws_upgrade_fail", "error", err)
		return
	}
	defer conn.Close()

	h.logger.Info("ws_connected", "driver_id", driverID)

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		h.logger.Warn("ws_auth_timeout_or_fail", "driver_id", driverID, "error", err)
		h.sendError(conn, driverID, "auth timeout or failed")
		return
	}

	var authMsg domain.AuthMessage
	if err := json.Unmarshal(data, &authMsg); err != nil {
		h.logger.Warn("ws_auth_unmarshal_fail", "driver_id", driverID, "error", err)
		h.sendError(conn, driverID, "invalid auth message")
		return
	}
	if authMsg.Type != "auth" || !strings.HasPrefix(authMsg.Token, "Bearer ") {
		h.sendError(conn, driverID, "bad auth format")
		return
	}
	tokenStr := strings.TrimPrefix(authMsg.Token, "Bearer ")
	claims, err := auth.VerifyDriverJWT(tokenStr)
	if err != nil {
		h.sendError(conn, driverID, "invalid token")
		return
	}
	if claims.DriverID != driverID {
		h.sendError(conn, driverID, "token-driver mismatch")
		return
	}

	h.logger.Info("ws_auth_success", "driver_id", driverID)
	h.hub.Add(driverID, conn)
	defer h.hub.Remove(driverID)
	h.sendInfo(conn, driverID, "authenticated")

	const (
		pingPeriod = 30 * time.Second
		pongWait   = 60 * time.Second
	)
	pingTicker := time.NewTicker(pingPeriod)
	defer pingTicker.Stop()

	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))

	for {
		select {
		case <-pingTicker.C:
			if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second)); err != nil {
				h.logger.Warn("ws_ping_fail", "driver_id", driverID, "error", err)
				return
			}
		default:
			if _, msg, err := conn.ReadMessage(); err != nil {
				h.logger.Warn("ws_read_or_pong_timeout", "driver_id", driverID, "error", err)
				return
			} else {
				h.logger.Info("ws_msg", "driver_id", driverID, "msg", string(msg))
			}
		}
	}
}

func (h *WSHandler) sendError(conn *websocket.Conn, driverID string, msg string) {
	serverMsg := domain.ServerMessage{
		Type:    "error",
		Message: msg,
	}
	if err := conn.WriteJSON(serverMsg); err != nil {
		h.logger.Error("ws_send_error_fail", "driver_id", driverID, "error", err)
	}
}

func (h *WSHandler) sendInfo(conn *websocket.Conn, driverID string, msg string) {
	serverMsg := domain.ServerMessage{
		Type:    "info",
		Message: msg,
	}
	if err := conn.WriteJSON(serverMsg); err != nil {
		h.logger.Error("ws_send_info_fail", "driver_id", driverID, "error", err)
	}
}
