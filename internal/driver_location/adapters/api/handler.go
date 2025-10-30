package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"log/slog"
	"ride-hail/internal/common/auth"
	"ride-hail/internal/common/contextx"
	"ride-hail/internal/common/log"
	"ride-hail/internal/driver_location/app"
)

type Handler struct {
	appService *app.AppService
	logger     *slog.Logger
}

func NewHandler(appService *app.AppService, lg *slog.Logger) *Handler {
	return &Handler{appService: appService, logger: lg}
}

func (h *Handler) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/drivers/", h.driversPrefixHandler)
	return mux
}

func (h *Handler) driversPrefixHandler(w http.ResponseWriter, r *http.Request) {
	ctx := contextx.WithNewRequestID(r.Context())

	p := strings.TrimPrefix(r.URL.Path, "/drivers/")
	parts := strings.Split(p, "/")
	if len(parts) < 2 {
		writeJSONError(ctx, w, http.StatusNotFound, "endpoint not found")
		return
	}

	driverID := parts[0]
	action := parts[1]

	switch {
	case r.Method == http.MethodPost && action == "online":
		h.handleGoOnline(ctx, w, r, driverID)
	case r.Method == http.MethodPost && action == "offline":
		h.handleGoOffline(ctx, w, r, driverID)
	case r.Method == http.MethodPost && action == "location":
		h.handleUpdateLocation(ctx, w, r, driverID)
	default:
		writeJSONError(ctx, w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// -------------------- DRIVER ACTIONS --------------------

func (h *Handler) handleGoOnline(ctx context.Context, w http.ResponseWriter, r *http.Request, driverID string) {
	ctx = contextx.WithRequestID(ctx, contextx.GetRequestID(ctx))
	start := time.Now()

	// --- Auth ---
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		writeJSONError(ctx, w, http.StatusUnauthorized, "missing bearer token")
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := auth.VerifyDriverJWT(token)
	if err != nil {
		writeJSONError(ctx, w, http.StatusUnauthorized, "invalid token")
		return
	}
	if claims.DriverID != driverID {
		writeJSONError(ctx, w, http.StatusForbidden, "forbidden: token does not match driver ID")
		return
	}

	var req goOnlineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error(ctx, h.logger, "invalid_body", "Unable to decode request body", err)
		writeJSONError(ctx, w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	sessionID, err := h.appService.GoOnline(ctx, driverID, req.Latitude, req.Longitude)
	if err != nil {
		h.handleAppError(ctx, w, err, driverID)
		return
	}

	resp := goOnlineResponse{
		Status:    "AVAILABLE",
		SessionID: sessionID,
		Message:   "You are now online and ready to accept rides",
	}
	writeJSONInfo(ctx, w, http.StatusOK, resp)

	log.Info(ctx, h.logger, "driver_online",
		fmt.Sprintf("driver=%s duration_ms=%d", driverID, time.Since(start).Milliseconds()))
}
func (h *Handler) handleGoOffline(ctx context.Context, w http.ResponseWriter, r *http.Request, driverID string) {
	ctx = contextx.WithRequestID(ctx, contextx.GetRequestID(ctx))
	start := time.Now()

	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		writeJSONError(ctx, w, http.StatusUnauthorized, "missing bearer token")
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := auth.VerifyDriverJWT(token)
	if err != nil {
		writeJSONError(ctx, w, http.StatusUnauthorized, "invalid token")
		return
	}
	if claims.DriverID != driverID {
		writeJSONError(ctx, w, http.StatusForbidden, "forbidden: token does not match driver ID")
		return
	}

	sessionID, summary, err := h.appService.GoOffline(ctx, driverID)
	if err != nil {
		h.handleAppError(ctx, w, err, driverID)
		return
	}

	resp := map[string]any{
		"status":          "OFFLINE",
		"session_id":      sessionID,
		"session_summary": summary,
		"message":         "You are now offline",
	}
	writeJSONInfo(ctx, w, http.StatusOK, resp)

	log.Info(ctx, h.logger, "driver_offline",
		fmt.Sprintf("driver=%s duration_ms=%d", driverID, time.Since(start).Milliseconds()))
}

func (h *Handler) handleUpdateLocation(ctx context.Context, w http.ResponseWriter, r *http.Request, driverID string) {
	ctx = contextx.WithRequestID(ctx, contextx.GetRequestID(ctx))
	start := time.Now()

	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		writeJSONError(ctx, w, http.StatusUnauthorized, "missing bearer token")
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := auth.VerifyDriverJWT(token)
	if err != nil {
		writeJSONError(ctx, w, http.StatusUnauthorized, "invalid token")
		return
	}
	if claims.DriverID != driverID {
		writeJSONError(ctx, w, http.StatusForbidden, "forbidden: token does not match driver ID")
		return
	}

	var req updateLocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error(ctx, h.logger, "invalid_body", "Unable to decode location body", err)
		writeJSONError(ctx, w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	result, err := h.appService.UpdateLocation(ctx, driverID, req.Latitude, req.Longitude, req.AccuracyMeters, req.SpeedKmh, req.HeadingDegrees)
	if err != nil {
		h.handleAppError(ctx, w, err, driverID)
		return
	}

	writeJSONInfo(ctx, w, http.StatusOK, result)

	log.Info(ctx, h.logger, "location_update",
		fmt.Sprintf("driver=%s duration_ms=%d lat=%.6f lng=%.6f", driverID, time.Since(start).Milliseconds(), req.Latitude, req.Longitude))
}
