package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"ride-hail/internal/common/contextx"
	"ride-hail/internal/common/log"
	"ride-hail/internal/driver_location/domain"
	"time"
)

type goOnlineRequest struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type goOnlineResponse struct {
	Status    string `json:"status"`
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

type updateLocationRequest struct {
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	AccuracyMeters float64 `json:"accuracy_meters"`
	SpeedKmh       float64 `json:"speed_kmh"`
	HeadingDegrees float64 `json:"heading_degrees"`
}

type updateLocationResponse struct {
	CoordinateID string    `json:"coordinate_id"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// -------------------- ERROR HANDLING --------------------

func (h *Handler) handleAppError(ctx context.Context, w http.ResponseWriter, err error, driverID string) {
	switch {
	case errors.Is(err, domain.ErrInvalidCoordinates):
		writeJSONError(ctx, w, http.StatusBadRequest, "invalid coordinates")
	case errors.Is(err, domain.ErrInvalidDriverID):
		writeJSONError(ctx, w, http.StatusBadRequest, "invalid driver ID")
	case errors.Is(err, domain.ErrPublishFailed):
		log.Error(ctx, h.logger, "publish_fail driver", driverID, err)
		writeJSONError(ctx, w, http.StatusInternalServerError, "status publish failed")
	case errors.Is(err, domain.ErrWebSocketSend):
		log.Warn(ctx, h.logger, "ws_send_fail driver", driverID, err)
		writeJSONError(ctx, w, http.StatusAccepted, "status updated but ws notification failed")
	case errors.Is(err, domain.ErrAlreadyOnline):
		writeJSONError(ctx, w, http.StatusConflict, "driver already online")
	case errors.Is(err, domain.ErrAlreadyOffline):
		writeJSONError(ctx, w, http.StatusConflict, "driver already offline")
	default:
		log.Error(ctx, h.logger, "internal_error driver", driverID, err)
		writeJSONError(ctx, w, http.StatusInternalServerError, "internal server error")
	}
}

// -------------------- RESPONSE HELPERS --------------------

func writeJSONError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := map[string]any{
		"error":      message,
		"code":       status,
		"request_id": contextx.GetRequestID(ctx),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func writeJSONInfo(ctx context.Context, w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
