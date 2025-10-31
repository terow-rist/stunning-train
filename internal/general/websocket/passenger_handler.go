package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ride-hail/internal/general/contracts"

	"github.com/gorilla/websocket"
)

// handlePassengerRideRequest validates and forwards a passenger's ride request.
func (ws *WebSocket) handlePassengerRideRequest(
	ctx context.Context,
	conn *websocket.Conn,
	passengerID string,
	raw json.RawMessage,
) error {
	// Minimal client-side sanity check: ensure we received some JSON object.
	if len(raw) == 0 || (len(raw) >= 2 && string(raw) == "null") {
		_ = ws.wsWriteMessage(conn, websocket.TextMessage, []byte(`{"type":"error","error":"empty ride request"}`))
		return fmt.Errorf("empty ride request payload")
	}

	// Wrap the raw payload with routing metadata for the ride service.
	// We avoid over-structuring per project scope—ride service performs full validation.
	msg := struct {
		Type        string          `json:"type"`         // always "ride_request"
		PassengerID string          `json:"passenger_id"` // source user
		Data        json.RawMessage `json:"data"`         // pass-through payload
		RequestedAt time.Time       `json:"requested_at"` // UTC for internal consistency
	}{
		Type:        "ride_request",
		PassengerID: passengerID,
		Data:        raw,
		RequestedAt: time.Now().UTC(),
	}

	body, err := json.Marshal(msg)
	if err != nil {
		_ = ws.wsWriteMessage(conn, websocket.TextMessage, []byte(`{"type":"error","error":"encode failed"}`))
		return fmt.Errorf("marshal ride_request: %w", err)
	}

	// Publish to the ride topic; routing key carries the passenger id.
	// Adjust the exchange constant to your existing one (e.g., rabbitmq.ExchangeRideTopic).
	rk := "ride.request." + passengerID
	if err := ws.pub.Publish(contracts.ExchangeRideTopic, rk, body); err != nil {
		_ = ws.wsWriteMessage(conn, websocket.TextMessage, []byte(`{"type":"error","error":"failed to publish ride request"}`))
		return fmt.Errorf("publish ride_request to %s: %w", rk, err)
	}

	// Best-effort ACK back to the passenger
	_ = ws.wsWriteMessage(conn, websocket.TextMessage, []byte(`{"type":"ride_request_ack","status":"ok"}`))
	return nil
}

// handleRideResponse handles a driver's accept/reject reply coming over WebSocket
// and publishes it to RabbitMQ on driver_topic with routing key "driver.response.{ride_id}".
func (ws *WebSocket) handleRideResponse(conn *websocket.Conn, driverID string, raw json.RawMessage) error {
	ctx := context.Background()

	type inbound struct {
		Type                    string `json:"type"`
		OfferID                 string `json:"offer_id,omitempty"`
		RideID                  string `json:"ride_id"`
		Decision                string `json:"decision,omitempty"`
		Accepted                *bool  `json:"accepted,omitempty"`
		EstimatedArrivalMinutes *int   `json:"estimated_arrival_minutes,omitempty"`
		CurrentLocation         *struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"current_location,omitempty"`
	}

	var in inbound
	if err := json.Unmarshal(raw, &in); err != nil {
		ws.logger.Error(ctx, "ws_bad_payload", "Failed to decode ride_response payload", err, map[string]any{
			"driver_id": driverID,
			"raw_data":  string(raw), // Добавим для отладки
		})
		errorMsg := []byte(`{"type":"error","error":"bad ride_response payload"}`)
		_ = ws.wsWriteMessage(conn, websocket.TextMessage, errorMsg)
		return err
	}

	// Проверка обязательных полей
	if in.RideID == "" {
		ws.logger.Error(ctx, "ws_validation_error", "ride_response missing ride_id", nil, map[string]any{
			"driver_id": driverID,
		})
		errorMsg := []byte(`{"type":"error","error":"missing ride_id"}`)
		_ = ws.wsWriteMessage(conn, websocket.TextMessage, errorMsg)
		return fmt.Errorf("missing ride_id")
	}

	// Определяем accepted
	accepted := false
	if in.Accepted != nil {
		accepted = *in.Accepted
	} else if in.Decision != "" {
		accepted = (strings.ToLower(in.Decision) == "accept")
	}

	if accepted {
		// Сохраняем текущую локацию если она есть
		if in.CurrentLocation != nil {
			location := &LocationData{
				Latitude:  in.CurrentLocation.Latitude,
				Longitude: in.CurrentLocation.Longitude,
			}
			ws.UpdateLastLocation(driverID, location)
		}

		// Запускаем автоматическую отправку локаций
		ws.StartLocationTracking(driverID, conn)

		ws.logger.Info(ctx, "auto_location_tracking_started",
			"Started automatic location tracking after ride acceptance",
			map[string]any{
				"driver_id": driverID,
				"ride_id":   in.RideID,
			})
	}

	// Создаем сообщение для RabbitMQ
	message := map[string]interface{}{
		"ride_id":   in.RideID,
		"driver_id": driverID,
		"accepted":  accepted,
		"offer_id":  in.OfferID,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	// Добавляем опциональные поля
	if in.EstimatedArrivalMinutes != nil {
		message["estimated_arrival_minutes"] = *in.EstimatedArrivalMinutes
	}
	if in.CurrentLocation != nil {
		message["driver_location"] = map[string]float64{
			"latitude":  in.CurrentLocation.Latitude,
			"longitude": in.CurrentLocation.Longitude,
		}
	}

	// Публикуем в RabbitMQ
	body, err := json.Marshal(message)
	if err != nil {
		ws.logger.Error(ctx, "driver_response_encode_failed", "Failed to marshal driver response", err, map[string]any{
			"driver_id": driverID,
			"ride_id":   in.RideID,
		})
		errorMsg := []byte(`{"type":"error","error":"encode failed"}`)
		_ = ws.wsWriteMessage(conn, websocket.TextMessage, errorMsg)
		return err
	}

	routingKey := "driver.response." + in.RideID
	if err := ws.pub.Publish("driver_topic", routingKey, body); err != nil {
		ws.logger.Error(ctx, "driver_response_publish_failed", "Failed to publish driver response", err, map[string]any{
			"driver_id":   driverID,
			"ride_id":     in.RideID,
			"routing_key": routingKey,
		})
		errorMsg := []byte(`{"type":"error","error":"publish failed"}`)
		_ = ws.wsWriteMessage(conn, websocket.TextMessage, errorMsg)
		return err
	}

	ws.logger.Info(ctx, "driver_response_published", "Published driver match response to RabbitMQ", map[string]any{
		"routing_key": routingKey,
		"ride_id":     in.RideID,
		"driver_id":   driverID,
		"accepted":    accepted,
	})

	// Отправляем подтверждение водителю
	ackMsg := map[string]interface{}{
		"type":      "ride_response_ack",
		"ride_id":   in.RideID,
		"accepted":  accepted,
		"published": true,
		"sent_at":   time.Now().UTC().Format(time.RFC3339),
	}
	ackBytes, _ := json.Marshal(ackMsg)
	_ = ws.wsWriteMessage(conn, websocket.TextMessage, ackBytes)

	return nil
}

// handlePassengerRideCancel validates and forwards a passenger's ride cancellation.
func (ws *WebSocket) handlePassengerRideCancel(
	ctx context.Context,
	conn *websocket.Conn,
	passengerID string,
	raw json.RawMessage,
) error {
	// minimal sanity check
	if len(raw) == 0 || (len(raw) >= 2 && string(raw) == "null") {
		_ = ws.wsWriteMessage(conn, websocket.TextMessage, []byte(`{"type":"error","error":"empty cancel payload"}`))
		return fmt.Errorf("empty ride cancel payload")
	}

	// wrap raw payload with routing metadata
	msg := struct {
		Type        string          `json:"type"`         // "ride_cancel"
		PassengerID string          `json:"passenger_id"` // who cancels
		Data        json.RawMessage `json:"data"`         // pass-through payload (e.g., {"ride_id":"...","reason":"..."} )
		CancelledAt time.Time       `json:"cancelled_at"` // UTC for internal consistency
	}{
		Type:        "ride_cancel",
		PassengerID: passengerID,
		Data:        raw,
		CancelledAt: time.Now().UTC(),
	}

	body, err := json.Marshal(msg)
	if err != nil {
		_ = ws.wsWriteMessage(conn, websocket.TextMessage, []byte(`{"type":"error","error":"encode failed"}`))
		return fmt.Errorf("marshal ride_cancel: %w", err)
	}

	// publish to ride topic with passenger-specific routing key
	rk := "ride.cancel." + passengerID
	if err := ws.pub.Publish(contracts.ExchangeRideTopic, rk, body); err != nil {
		_ = ws.wsWriteMessage(conn, websocket.TextMessage, []byte(`{"type":"error","error":"failed to publish cancel"}`))
		return fmt.Errorf("publish ride_cancel to %s: %w", rk, err)
	}

	// best-effort ACK to client
	_ = ws.wsWriteMessage(conn, websocket.TextMessage, []byte(`{"type":"ride_cancel_ack","status":"ok"}`))
	return nil
}

// NotifyPassengerRideStatus sends a JSON status packet to a connected passenger.
func (ws *WebSocket) NotifyPassengerRideStatus(ctx context.Context, passengerID string, msg contracts.WSPassengerRideStatus) error {
	v, ok := ws.passengers.Load(passengerID)
	if !ok {
		return fmt.Errorf("passenger %s not connected", passengerID)
	}
	conn, _ := v.(*websocket.Conn)
	if conn == nil {
		return fmt.Errorf("no connection for passenger %s", passengerID)
	}

	if err := ws.writeJSON(conn, msg); err != nil {
		ws.logger.Error(ctx, "ws_write_failed", "Failed to push ride status to passenger", err, map[string]any{
			"passenger_id": passengerID,
		})
		return err
	}

	return nil
}

func (ws *WebSocket) NotifyPassengerLocationUpdate(ctx context.Context, passengerID string, msg contracts.WSPassengerLocationUpdate) error {
	v, ok := ws.passengers.Load(passengerID)
	if !ok {
		return fmt.Errorf("passenger %s not connected", passengerID)
	}
	conn, _ := v.(*websocket.Conn)
	if conn == nil {
		return fmt.Errorf("no connection for passenger %s", passengerID)
	}

	if err := ws.writeJSON(conn, msg); err != nil {
		ws.logger.Error(ctx, "ws_location_send_failed",
			"Failed to send location update to passenger", err,
			map[string]any{"passenger_id": passengerID})
		return err
	}

	return nil
}
