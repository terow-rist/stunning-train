package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// --- Handler: GET /admin/overview ---

func (handler *AdminHTTPHandler) handleOverview(w http.ResponseWriter, r *http.Request) {
	// generate a context with request ID
	ctx := handler.withReqID(r.Context(), r)

	// bound service call
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// get the system overview
	overview, err := handler.svc.GetSystemOverview(ctxWithTimeout)
	if err != nil {
		// distinguish DB failures from validation errors
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			handler.httpError(ctxWithTimeout, w, http.StatusInternalServerError, "database error", err)
			return
		}
		handler.httpError(ctxWithTimeout, w, http.StatusInternalServerError, "failed to fetch system overview", err)
		return
	}

	// return the system overview
	handler.jsonResponse(ctxWithTimeout, w, http.StatusOK, overview)
}
