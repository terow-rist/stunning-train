package websocket

import (
	"context"
	"encoding/json"
	"ride-hail/internal/domain/driver"
	"ride-hail/internal/general/contracts"
	"time"

	"github.com/gorilla/websocket"
)

func (ws *WebSocket) handleDriverStatus(conn *websocket.Conn, driverID string, raw json.RawMessage) error {
	ctx := context.Background()

	// decode inbound payload
	var p struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		ws.logger.Error(ctx, "ws_bad_payload", "Failed to decode driver_status payload", err, map[string]any{
			"driver_id": driverID,
		})
		_ = ws.writeJSON(conn, map[string]any{
			"type":  "error",
			"error": "bad driver_status payload",
		})
		return err
	}

	// validate status against domain enum
	status, err := driver.ParseDriverStatus(p.Status)
	if err != nil {
		ws.logger.Error(ctx, "ws_validation_error", "Invalid driver status", err, map[string]any{
			"driver_id": driverID, "status": p.Status,
		})
		_ = ws.writeJSON(conn, map[string]any{
			"type":  "error",
			"error": "invalid driver status",
		})
		return err
	}

	// publish status update to RabbitMQ
	body, _ := json.Marshal(map[string]any{
		"type":       "driver_status",
		"driver_id":  driverID,
		"status":     status.String(),
		"created_at": time.Now().UTC(),
	})
	routingKey := "driver.status." + driverID
	if err := ws.pub.Publish(contracts.ExchangeDriverTopic, routingKey, body); err != nil {
		ws.logger.Error(ctx, "driver_status_publish_failed", "Failed to publish driver status", err, map[string]any{
			"driver_id": driverID, "status": status.String(), "routing_key": routingKey,
		})
		_ = ws.writeJSON(conn, map[string]any{
			"type":  "error",
			"error": "failed to publish driver status",
		})
		return err
	}

	ws.logger.Info(ctx, "driver_status_published", "Published driver status update", map[string]any{
		"driver_id":   driverID,
		"status":      status.String(),
		"routing_key": routingKey,
	})

	// best-effort ACK to the driver socket so UI can update immediately
	_ = ws.writeJSON(conn, map[string]any{
		"type":      "driver_status_ack",
		"status":    status.String(),
		"published": true,
		"sent_at":   time.Now().UTC(),
	})

	return nil
}
