package ws

import (
	"log/slog"
	"sync"

	"github.com/gorilla/websocket"
)

// Hub stores all active WebSocket connections keyed by user/service ID.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]*websocket.Conn
	logger  *slog.Logger
}

func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients: make(map[string]*websocket.Conn),
		logger:  logger,
	}
}

// Add registers a new connection under a unique ID (driver, passenger, etc.).
func (h *Hub) Add(id string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if old, ok := h.clients[id]; ok {
		_ = old.Close()
	}
	h.clients[id] = conn
	h.logger.Info("ws_registered", "id", id)
}

// Remove deletes and closes a connection.
func (h *Hub) Remove(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conn, ok := h.clients[id]; ok {
		_ = conn.Close()
		delete(h.clients, id)
		h.logger.Info("ws_removed", "id", id)
	}
}

// Send transmits a JSON message to a connected user/service.
func (h *Hub) Send(id string, msg any) error {
	h.mu.RLock()
	conn, ok := h.clients[id]
	h.mu.RUnlock()
	if !ok {
		return nil // user not connected
	}
	return conn.WriteJSON(msg)
}

// ListConnected returns all connected IDs (for debugging/admin tools).
func (h *Hub) ListConnected() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	keys := make([]string, 0, len(h.clients))
	for k := range h.clients {
		keys = append(keys, k)
	}
	return keys
}
