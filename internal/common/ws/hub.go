package ws

import (
	"log/slog"
	"sync"

	"github.com/gorilla/websocket"
)

// Hub stores all active WebSocket connections keyed by driverID.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]*websocket.Conn
	logger  *slog.Logger
}

// NewHub creates a new hub instance.
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients: make(map[string]*websocket.Conn),
		logger:  logger,
	}
}

// Add registers a new connection for driverID (replaces old if exists).
func (h *Hub) Add(driverID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if old, ok := h.clients[driverID]; ok {
		_ = old.Close()
	}
	h.clients[driverID] = conn
	h.logger.Info("ws_registered", "driver_id", driverID)
}

// Remove deletes a driver connection.
func (h *Hub) Remove(driverID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conn, ok := h.clients[driverID]; ok {
		_ = conn.Close()
		delete(h.clients, driverID)
		h.logger.Info("ws_removed", "driver_id", driverID)
	}
}

// Send sends a JSON message to a connected driver if exists.
func (h *Hub) Send(driverID string, msg any) error {
	h.mu.RLock()
	conn, ok := h.clients[driverID]
	h.mu.RUnlock()
	if !ok {
		return nil // driver not connected
	}
	return conn.WriteJSON(msg)
}
