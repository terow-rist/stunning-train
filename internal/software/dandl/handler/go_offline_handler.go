package handler

import (
	"context"
	"errors"
	"net/http"
	"ride-hail/internal/general/jwt"
	"ride-hail/internal/ports"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// ----- Handler: POST /drivers/{driver_id}/offline -----

func (handler *DriverHTTPHandler) handleGoOffline(w http.ResponseWriter, r *http.Request) {
	// generate a context with request ID
	ctx := handler.withReqID(r.Context(), r)

	// check the content type
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		handler.httpError(ctx, w, http.StatusUnsupportedMediaType, "Content-Type must be application/json", nil)
		return
	}

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

	// map to service DTO defined in ports
	in := ports.GoOfflineInput{
		DriverID: driverID,
	}

	// bound service call
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// change the status to offline and obtain the result
	res, err := handler.svc.GoOffline(ctxWithTimeout, in)
	if err != nil {
		// distinguish DB failures from validation errors
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			handler.httpError(ctxWithTimeout, w, http.StatusInternalServerError, "database error", err)
			return
		}
		handler.httpError(ctxWithTimeout, w, http.StatusInternalServerError, "failed to go offline", err)
		return
	}

	// build response according to the spec
	handler.jsonResponse(ctxWithTimeout, w, http.StatusOK, res)
}
