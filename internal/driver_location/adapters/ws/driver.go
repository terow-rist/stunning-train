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

	h.logger.Info("ws_wait_auth", "driver_id", driverID)
	_, data, err := conn.ReadMessage()
	if err != nil {
		h.logger.Warn("ws_auth_read_fail", "driver_id", driverID, "error", err)
		return
	}

	var authMsg domain.AuthMessage
	if err := json.Unmarshal(data, &authMsg); err != nil {
		h.logger.Warn("ws_auth_unmarshal_fail", "driver_id", driverID, "error", err)
		conn.WriteJSON(domain.ServerMessage{Type: "error", Message: "invalid auth message"})
		return
	}
	h.logger.Info("ws_auth_parsed", "driver_id", driverID, "type", authMsg.Type, "token", authMsg.Token)

	if authMsg.Type != "auth" || !strings.HasPrefix(authMsg.Token, "Bearer ") {
		h.logger.Warn("ws_auth_bad_format", "driver_id", driverID, "token", authMsg.Token)
		conn.WriteJSON(domain.ServerMessage{Type: "error", Message: "bad auth format"})
		return
	}

	tokenStr := strings.TrimPrefix(authMsg.Token, "Bearer ")
	h.logger.Info("ws_auth_verifying", "driver_id", driverID)
	claims, err := auth.VerifyDriverJWT(tokenStr)
	if err != nil {
		h.logger.Warn("ws_auth_token_invalid", "driver_id", driverID, "error", err)
		conn.WriteJSON(domain.ServerMessage{Type: "error", Message: "invalid token"})
		return
	}

	if claims.DriverID != driverID {
		h.logger.Warn("ws_auth_id_mismatch", "driver_id", driverID, "claims_driver_id", claims.DriverID)
		conn.WriteJSON(domain.ServerMessage{Type: "error", Message: "token-driver mismatch"})
		return
	}

	h.logger.Info("ws_auth_success", "driver_id", driverID)
	h.hub.Add(driverID, conn)
	conn.WriteJSON(domain.ServerMessage{Type: "info", Message: "authenticated"})

	conn.SetPongHandler(func(appData string) error { return nil })
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second)); err != nil {
				h.logger.Warn("ws_ping_fail", "driver_id", driverID, "error", err)
				return
			}
			h.logger.Debug("ws_ping_sent", "driver_id", driverID)
		default:
			_, msg, err := conn.ReadMessage()
			if err != nil {
				h.logger.Warn("ws_read_fail", "driver_id", driverID, "error", err)
				h.hub.Remove(driverID)
				return
			}
			h.logger.Info("ws_msg", "driver_id", driverID, "msg", string(msg))
		}
	}
}
