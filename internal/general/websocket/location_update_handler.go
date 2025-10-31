package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ride-hail/internal/domain/ride"
	"ride-hail/internal/shared/contracts"

	"github.com/gorilla/websocket"
)

func (ws *WebSocket) handleLocationUpdate(ctx context.Context, conn *websocket.Conn, driverID string, data json.RawMessage, lastLocAt *time.Time) error {
	// Проверяем rate limiting (макс 1 раз в 3 секунды)
	now := time.Now()
	if now.Sub(*lastLocAt) < 3*time.Second {
		ws.logger.Debug(ctx, "location_update_throttled", "Location update throttled", map[string]any{
			"driver_id": driverID,
			"interval":  now.Sub(*lastLocAt).String(),
		})
		return nil // Игнорируем слишком частые обновления
	}
	*lastLocAt = now

	// Парсим данные местоположения
	var locationData struct {
		Latitude       float64 `json:"latitude"`
		Longitude      float64 `json:"longitude"`
		AccuracyMeters float64 `json:"accuracy_meters,omitempty"`
		SpeedKmh       float64 `json:"speed_kmh,omitempty"`
		HeadingDegrees float64 `json:"heading_degrees,omitempty"`
		Address        string  `json:"address,omitempty"`
	}

	if err := json.Unmarshal(data, &locationData); err != nil {
		ws.logger.Error(ctx, "location_update_parse_failed", "Failed to parse location data", err, map[string]any{
			"driver_id": driverID,
			"raw_data":  string(data),
		})
		errorMsg := []byte(`{"type":"error","error":"invalid location data"}`)
		_ = ws.wsWriteMessage(conn, websocket.TextMessage, errorMsg)
		return err
	}

	// Валидация координат
	if locationData.Latitude < -90 || locationData.Latitude > 90 ||
		locationData.Longitude < -180 || locationData.Longitude > 180 {
		ws.logger.Error(ctx, "location_update_invalid_coords", "Invalid coordinates received", nil, map[string]any{
			"driver_id": driverID,
			"latitude":  locationData.Latitude,
			"longitude": locationData.Longitude,
		})
		errorMsg := []byte(`{"type":"error","error":"invalid coordinates"}`)
		_ = ws.wsWriteMessage(conn, websocket.TextMessage, errorMsg)
		return fmt.Errorf("invalid coordinates")
	}
	ws.UpdateLastLocation(driverID, &LocationData{
		Latitude:       locationData.Latitude,
		Longitude:      locationData.Longitude,
		AccuracyMeters: locationData.AccuracyMeters,
		SpeedKmh:       locationData.SpeedKmh,
		HeadingDegrees: locationData.HeadingDegrees,
	})
	// Сохраняем в базу
	_, err := ws.coordinatesRepo.SaveDriverLocation(ctx, driverID, locationData.Latitude, locationData.Longitude,
		locationData.AccuracyMeters, locationData.SpeedKmh, locationData.HeadingDegrees, locationData.Address)
	if err != nil {
		ws.logger.Error(ctx, "location_update_db_failed", "Failed to save location to database", err, map[string]any{
			"driver_id": driverID,
		})
		errorMsg := []byte(`{"type":"error","error":"failed to save location"}`)
		_ = ws.wsWriteMessage(conn, websocket.TextMessage, errorMsg)
		return err
	}

	// Публикуем в RabbitMQ используя правильный контракт
	locMsg := contracts.LocationUpdateMessage{
		DriverID: driverID,
		Location: contracts.GeoPoint{
			Lat:     locationData.Latitude,
			Lng:     locationData.Longitude,
			Address: locationData.Address,
		},
		SpeedKMH:       locationData.SpeedKmh,
		HeadingDegrees: locationData.HeadingDegrees,
		Timestamp:      time.Now().UTC(),
		Envelope: contracts.Envelope{
			Producer: "driver-location-service",
			SentAt:   time.Now().UTC(),
		},
	}

	// Попробуем найти активную поездку для этого водителя
	if activeRide, err := ws.getActiveRideForDriver(ctx, driverID); err == nil && activeRide != nil {
		locMsg.RideID = activeRide.ID
		ws.logger.Info(ctx, "active_ride_found", "Found active ride for location update", map[string]any{
			"driver_id": driverID,
			"ride_id":   activeRide.ID,
			"status":    activeRide.Status.String(),
		})
	} else if err != nil {
		ws.logger.Error(ctx, "active_ride_lookup_failed", "Failed to lookup active ride", err, map[string]any{
			"driver_id": driverID,
		})
	} else {
		ws.logger.Debug(ctx, "no_active_ride", "No active ride found for driver", map[string]any{
			"driver_id": driverID,
		})
	}

	messageBytes, err := json.Marshal(locMsg)
	if err != nil {
		ws.logger.Error(ctx, "location_update_marshal_failed", "Failed to marshal location message", err, map[string]any{
			"driver_id": driverID,
		})
		return err
	}

	// Публикация в fanout exchange (все подписчики получат)
	if err := ws.pub.Publish(contracts.ExchangeLocationFanout, "", messageBytes); err != nil {
		ws.logger.Error(ctx, "location_update_publish_failed", "Failed to publish location update", err, map[string]any{
			"driver_id": driverID,
		})
		return err
	}

	ws.logger.Info(ctx, "location_update_published", "Location update published to RabbitMQ", map[string]any{
		"driver_id":       driverID,
		"latitude":        locationData.Latitude,
		"longitude":       locationData.Longitude,
		"speed_kmh":       locationData.SpeedKmh,
		"heading_degrees": locationData.HeadingDegrees,
		"ride_id":         locMsg.RideID,
		"exchange":        contracts.ExchangeLocationFanout,
	})

	// Отправляем подтверждение водителю
	ackMsg := map[string]interface{}{
		"type":    "location_update_ack",
		"status":  "success",
		"message": "Location updated and broadcasted",
	}
	ackBytes, _ := json.Marshal(ackMsg)
	_ = ws.wsWriteMessage(conn, websocket.TextMessage, ackBytes)

	return nil
}

// getActiveRideForDriver возвращает активную поездку для водителя
func (ws *WebSocket) getActiveRideForDriver(ctx context.Context, driverID string) (*ride.Ride, error) {
	if ws.ridesRepo == nil {
		return nil, fmt.Errorf("rides repository not available")
	}

	activeRide, err := ws.ridesRepo.GetActiveForDriver(ctx, driverID)
	if err != nil {
		return nil, err
	}

	return activeRide, nil
}
