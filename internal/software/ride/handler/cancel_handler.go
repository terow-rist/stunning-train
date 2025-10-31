package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// --- Request DTO (HTTP boundary) ---

type cancelRideRequest struct {
	Reason string `json:"reason"`
}

// --- Handler: POST /rides/{ride_id}/cancel ---

func (handler *RideHTTPHandler) handleCancelRide(w http.ResponseWriter, r *http.Request) {
	// generate a context with request ID
	ctx := handler.withReqID(r.Context(), r)

	// check the content type
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		handler.httpError(ctx, w, http.StatusUnsupportedMediaType, "Content-Type must be application/json", nil)
		return
	}

	// limit the body size
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10) // 256 KiB
	defer r.Body.Close()

	// fetch and check the ride id
	rideID := strings.TrimSpace(r.PathValue("ride_id"))
	if rideID == "" {
		handler.httpError(ctx, w, http.StatusBadRequest, "ride_id is required", errors.New("missing ride_id"))
		return
	}
	ctx = handler.logger.WithRideID(ctx, rideID)

	// decode strictly
	var req cancelRideRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			handler.httpError(ctx, w, http.StatusRequestEntityTooLarge, "request body too large", err)
			return
		}
		handler.httpError(ctx, w, http.StatusBadRequest, "invalid JSON: "+err.Error(), err)
		return
	}

	// bound service call
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// cancel the ride and obtain the result
	res, err := handler.svc.CancelRide(ctxWithTimeout, rideID, req.Reason)
	if err != nil {
		// distinguish DB failures from validation errors
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			handler.httpError(ctxWithTimeout, w, http.StatusInternalServerError, "database error", err)
			return
		}
		handler.httpError(ctxWithTimeout, w, http.StatusBadRequest, err.Error(), err)
		return
	}

	// build response according to the spec
	handler.jsonResponse(ctxWithTimeout, w, http.StatusOK, res)
}
