package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"ride-hail/internal/domain/user"
	"ride-hail/internal/general/jwt"
	"ride-hail/internal/general/logger"
	"ride-hail/internal/ports"
	"strings"
)

// AdminHTTPHandler adapts HTTP requests to the AdminService.
type AdminHTTPHandler struct {
	svc    ports.AdminService
	logger *logger.Logger
	auth   *jwt.Manager
}

// NewAdminHTTPHandler wires an HTTP handler around the AdminService.
func NewAdminHTTPHandler(svc ports.AdminService, logger *logger.Logger, auth *jwt.Manager) *AdminHTTPHandler {
	return &AdminHTTPHandler{svc: svc, logger: logger, auth: auth}
}

// RegisterRoutes mounts admin endpoints on the provided mux.
func (handler *AdminHTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/overview",
		jwt.AuthMiddlewareFunc(handler.auth, user.RoleAdmin)(handler.handleOverview),
	)
	mux.HandleFunc("GET /admin/rides/active",
		jwt.AuthMiddlewareFunc(handler.auth, user.RoleAdmin)(handler.handleActiveRides),
	)
	mux.HandleFunc("GET /admin/health", handler.handleHealth)
}

// ----- general helpers -----

// jsonResponse takes any type of data and encode it to HTTP response.
func (handler *AdminHTTPHandler) jsonResponse(ctx context.Context, w http.ResponseWriter, status int, data any) {
	// encode to buffer first so we can control status on failure
	var buf []byte
	var err error

	if data != nil {
		buf, err = json.Marshal(data)
		if err != nil {
			handler.logger.Error(ctx, "response_encode_failed", "Failed to encode response", err, nil)
			http.Error(w, `{"error":"failed to encode response"}`, http.StatusInternalServerError)
			return
		}
	} else {
		buf = []byte("{}")
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf)
}

// httpError sends a JSON error response with a message.
func (handler *AdminHTTPHandler) httpError(ctx context.Context, w http.ResponseWriter, status int, msg string, err error) {
	action := "request_failed"
	if status >= 500 {
		action = "http_internal_error"
	} else if status == http.StatusBadRequest {
		action = "validation_failed"
	} else if status == http.StatusUnsupportedMediaType {
		action = "unsupported_media_type"
	}
	handler.logger.Error(ctx, action, msg, err, nil)

	type errBody struct {
		Error string `json:"error"`
	}
	handler.jsonResponse(ctx, w, status, errBody{Error: msg})
}

// withReqID extracts or generates a request ID and adds it to the context.
func (handler *AdminHTTPHandler) withReqID(ctx context.Context, r *http.Request) context.Context {
	reqID := r.Header.Get("X-Request-ID")
	if strings.TrimSpace(reqID) == "" {
		reqID = randID()
	}
	return handler.logger.WithRequestID(ctx, reqID)
}

// randID generates a random 24-char hex string suitable for request IDs.
func randID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
