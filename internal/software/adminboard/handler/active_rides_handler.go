package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// --- Handler: GET /admin/rides/active?page=X&page_size=Y ---

func (handler *AdminHTTPHandler) handleActiveRides(w http.ResponseWriter, r *http.Request) {
	// generate a context with request ID
	ctx := handler.withReqID(r.Context(), r)

	// get the query parameters
	query := r.URL.Query()
	page := query.Get("page")
	pageSize := query.Get("page_size")

	// bound service call
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// get the active rides
	activeRides, err := handler.svc.GetActiveRides(ctxWithTimeout, page, pageSize)
	if err != nil {
		// distinguish DB failures from validation errors
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			handler.httpError(ctxWithTimeout, w, http.StatusInternalServerError, "database error", err)
			return
		}
		handler.httpError(ctxWithTimeout, w, http.StatusInternalServerError, "failed to fetch active rides", err)
		return
	}

	// return the active rides
	handler.jsonResponse(ctxWithTimeout, w, http.StatusOK, activeRides)
}
