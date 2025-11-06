package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"ride-hail/internal/general/jwt"
	"ride-hail/internal/ports"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// --- Request DTO (HTTP boundary) ---

type startRideRequest struct {
	RideID         string `json:"ride_id"`
	DriverLocation struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"driver_location"`
}

// ----- Handler: POST /drivers/{driver_id}/start -----

func (handler *DriverHTTPHandler) handleStartRide(w http.ResponseWriter, r *http.Request) {
	// generate a context with request ID
	ctx := handler.withReqID(r.Context(), r)

	// check the content type
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		handler.httpError(ctx, w, http.StatusUnsupportedMediaType, "Content-Type must be application/json", nil)
		return
	}

	// limit body size
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB
	defer r.Body.Close()

	// fetch and check the driver id
	driverID := strings.TrimSpace(r.PathValue("driver_id"))
	if driverID == "" {
		handler.httpError(ctx, w, http.StatusBadRequest, "missing driver_id in path", nil)
		return
	}

	// obtain the JWT claims
	claims := jwt.RequireClaims(r)
	if claims == nil {
		handler.httpError(ctx, w, http.StatusUnauthorized, "missing auth claims", errors.New("no claims"))
		return
	}

	// check if the user ID from the JWT token matches the driver ID from the request
	sub := strings.TrimSpace(claims.Subject) // user ID from token
	if sub == "" || sub != driverID {
		handler.httpError(ctx, w, http.StatusForbidden, "driver_id does not match token subject", errors.New("driver/token mismatch"))
		return
	}

	// decode strictly
	var req startRideRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			handler.httpError(ctx, w, http.StatusRequestEntityTooLarge, "request body too large", err)
			return
		}
		handler.httpError(ctx, w, http.StatusBadRequest, "invalid JSON body", err)
		return
	}

	// check the ride id
	if strings.TrimSpace(req.RideID) == "" {
		handler.httpError(ctx, w, http.StatusBadRequest, "ride_id is required", nil)
		return
	}

	// map to service DTO defined in ports
	in := ports.StartRideInput{
		DriverID: driverID,
		RideID:   req.RideID,
		DriverLocation: ports.GeoPoint{
			Latitude:  req.DriverLocation.Latitude,
			Longitude: req.DriverLocation.Longitude,
		},
	}

	// bound service call
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// start the ride and obtain the result
	res, err := handler.svc.StartRide(ctxWithTimeout, in)
	if err != nil {
		// distinguish DB failures from validation errors
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			handler.httpError(ctxWithTimeout, w, http.StatusInternalServerError, "database error", err)
			return
		}
		handler.httpError(ctxWithTimeout, w, http.StatusInternalServerError, "failed to start a ride", err)
		return
	}

	// build response according to the spec
	handler.jsonResponse(ctxWithTimeout, w, http.StatusOK, res)
}
