package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"ride-hail/internal/common/contextx"
	"ride-hail/internal/common/log"
	"ride-hail/internal/driver_location/adapters/queue"
	"ride-hail/internal/driver_location/adapters/repository"
	"strings"
	"time"
)

type Handler struct {
	driverRepo   *repository.DriverRepository
	locationRepo *repository.LocationRepository
	publisher    *queue.DriverPublisher
	logger       *slog.Logger
}

// NewHandler constructs API handler
func NewHandler(d *repository.DriverRepository, l *repository.LocationRepository, p *queue.DriverPublisher, lg *slog.Logger) *Handler {
	return &Handler{
		driverRepo:   d,
		locationRepo: l,
		publisher:    p,
		logger:       lg,
	}
}

type goOnlineRequest struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type goOnlineResponse struct {
	Status    string `json:"status"`
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

func (h *Handler) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/drivers/", h.driversPrefixHandler)
	return mux
}

// driversPrefixHandler handles routes under /drivers/
func (h *Handler) driversPrefixHandler(w http.ResponseWriter, r *http.Request) {
	// expected path: /drivers/{driver_id}/online
	ctx := r.Context()
	ctx = contextx.WithNewRequestID(ctx)

	// trim and split path
	p := strings.TrimPrefix(r.URL.Path, "/drivers/")
	// p now "{driver_id}/online" or similar
	parts := strings.Split(p, "/")
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}
	driverID := parts[0]
	action := parts[1]

	switch r.Method {
	case http.MethodPost:
		if action == "online" {
			h.handleGoOnline(ctx, w, r, driverID)
			return
		}
		// other POST actions could be handled here
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleGoOnline(ctx context.Context, w http.ResponseWriter, r *http.Request, driverID string) {
	ctx = contextx.WithRequestID(ctx, contextx.GetRequestID(ctx)) // keep request id
	start := time.Now()

	var req goOnlineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error(ctx, h.logger, "invalid_body", "Unable to decode request body", err)
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if !validCoords(req.Latitude, req.Longitude) {
		http.Error(w, "invalid coordinates", http.StatusBadRequest)
		return
	}

	// Start session (transaction inside)
	sessionID, err := h.driverRepo.StartSession(ctx, driverID)
	if err != nil {
		log.Error(ctx, h.logger, "start_session_fail", "Failed to start driver session", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Update status
	if err := h.driverRepo.UpdateStatus(ctx, driverID, "AVAILABLE"); err != nil {
		log.Error(ctx, h.logger, "update_status_fail", "Failed to update driver status", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Save location (transactional)
	if err := h.locationRepo.SaveLocation(ctx, repository.LocationUpdate{
		DriverID:  driverID,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
	}); err != nil {
		log.Error(ctx, h.logger, "save_location_fail", "Failed to save driver location", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Publish status to RabbitMQ (best effort)
	if err := h.publisher.PublishStatus(ctx, driverID, "AVAILABLE", sessionID); err != nil {
		// log but do not fail client
		log.Error(ctx, h.logger, "publish_status_fail", "Failed to publish status to RMQ", err)
	}

	resp := goOnlineResponse{
		Status:    "AVAILABLE",
		SessionID: sessionID,
		Message:   "You are now online and ready to accept rides",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// log encode error
		log.Error(ctx, h.logger, "encode_response_fail", "Failed to encode response", err)
	}

	log.Info(ctx, h.logger, "driver_online", fmt.Sprintf("driver=%s duration_ms=%d", driverID, time.Since(start).Milliseconds()))
}

func validCoords(lat, lng float64) bool {
	return !(lat < -90 || lat > 90 || lng < -180 || lng > 180)
}

// helper to keep request id in context (simple wrapper)
func (c *contextKey) String() string { return "ctx" } // NOOP to avoid unused warnings
type contextKey struct{}
