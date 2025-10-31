package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"ride-hail/internal/domain/ride"
	"ride-hail/internal/general/jwt"
	"ride-hail/internal/ports"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// --- Request DTO (HTTP boundary) ---

type createRideRequest struct {
	PassengerID          string  `json:"passenger_id"`
	PickupLatitude       float64 `json:"pickup_latitude"`
	PickupLongitude      float64 `json:"pickup_longitude"`
	PickupAddress        string  `json:"pickup_address"`
	DestinationLatitude  float64 `json:"destination_latitude"`
	DestinationLongitude float64 `json:"destination_longitude"`
	DestinationAddress   string  `json:"destination_address"`
	VehicleType          string  `json:"ride_type"` // ECONOMY | PREMIUM | XL
}

// ----- Handler: POST /rides -----

func (handler *RideHTTPHandler) handleCreateRide(w http.ResponseWriter, r *http.Request) {
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

	// decode strictly
	var req createRideRequest
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

	// obtain the JWT claims
	claims := jwt.RequireClaims(r)
	if claims == nil {
		handler.httpError(ctx, w, http.StatusUnauthorized, "missing auth claims", errors.New("no claims"))
		return
	}

	// fill or verify passenger_id
	sub := strings.TrimSpace(claims.Subject) // user id from token
	if strings.TrimSpace(req.PassengerID) == "" {
		req.PassengerID = sub
	} else if req.PassengerID != sub {
		handler.httpError(ctx, w, http.StatusForbidden, "passenger_id does not match token subject", errors.New("passenger/token mismatch"))
		return
	}

	// parse the vehicle type
	vt, err := ride.ParseVehicleType(req.VehicleType)
	if err != nil {
		handler.httpError(ctx, w, http.StatusBadRequest, "ride_type must be one of: ECONOMY, PREMIUM, XL", errors.New("invalid ride_type"))
		return
	}

	// map to service DTO defined in ports
	in := ports.CreateRideInput{
		PassengerID:          strings.TrimSpace(req.PassengerID),
		PickupLatitude:       req.PickupLatitude,
		PickupLongitude:      req.PickupLongitude,
		PickupAddress:        strings.TrimSpace(req.PickupAddress),
		DestinationLatitude:  req.DestinationLatitude,
		DestinationLongitude: req.DestinationLongitude,
		DestinationAddress:   strings.TrimSpace(req.DestinationAddress),
		VehicleType:          vt,
	}

	// bound service call
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// create the ride and obtain the result
	res, err := handler.svc.CreateRide(ctxWithTimeout, in)
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
	ctxWithTimeout = handler.logger.WithRideID(ctxWithTimeout, res.RideID)

	// build response according to the spec
	handler.jsonResponse(ctxWithTimeout, w, http.StatusCreated, res)
}
