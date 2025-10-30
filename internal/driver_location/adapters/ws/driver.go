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

	// ---------------- AUTH PHASE ----------------
	conn.SetReadDeadline(time.Now().Add(5 * time.Second)) // 5-second window for first message
	_, data, err := conn.ReadMessage()
	if err != nil {
		h.logger.Warn("ws_auth_timeout_or_fail", "driver_id", driverID, "error", err)
		conn.WriteJSON(domain.ServerMessage{Type: "error", Message: "auth timeout or failed"})
		return
	}

	var authMsg domain.AuthMessage
	if err := json.Unmarshal(data, &authMsg); err != nil {
		h.logger.Warn("ws_auth_unmarshal_fail", "driver_id", driverID, "error", err)
		conn.WriteJSON(domain.ServerMessage{Type: "error", Message: "invalid auth message"})
		return
	}
	if authMsg.Type != "auth" || !strings.HasPrefix(authMsg.Token, "Bearer ") {
		conn.WriteJSON(domain.ServerMessage{Type: "error", Message: "bad auth format"})
		return
	}
	tokenStr := strings.TrimPrefix(authMsg.Token, "Bearer ")
	claims, err := auth.VerifyDriverJWT(tokenStr)
	if err != nil {
		conn.WriteJSON(domain.ServerMessage{Type: "error", Message: "invalid token"})
		return
	}
	if claims.DriverID != driverID {
		conn.WriteJSON(domain.ServerMessage{Type: "error", Message: "token-driver mismatch"})
		return
	}

	h.logger.Info("ws_auth_success", "driver_id", driverID)
	h.hub.Add(driverID, conn)
	defer h.hub.Remove(driverID)
	conn.WriteJSON(domain.ServerMessage{Type: "info", Message: "authenticated"})

	// ---------------- KEEP-ALIVE PHASE ----------------
	const (
		pingPeriod = 30 * time.Second
		pongWait   = 60 * time.Second
	)
	pingTicker := time.NewTicker(pingPeriod)
	defer pingTicker.Stop()

	// Every received pong extends read deadline
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	// initial read deadline
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))

	for {
		select {
		case <-pingTicker.C:
			// send ping
			if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second)); err != nil {
				h.logger.Warn("ws_ping_fail", "driver_id", driverID, "error", err)
				return
			}
		default:
			// read incoming message (blocks until message or read-deadline timeout)
			if _, msg, err := conn.ReadMessage(); err != nil {
				h.logger.Warn("ws_read_or_pong_timeout", "driver_id", driverID, "error", err)
				return
			} else {
				h.logger.Info("ws_msg", "driver_id", driverID, "msg", string(msg))
			}
		}
	}
}
