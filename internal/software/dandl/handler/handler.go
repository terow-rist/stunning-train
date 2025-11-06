package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"ride-hail/internal/domain/user"
	"ride-hail/internal/general/jwt"
	"ride-hail/internal/general/logger"
	"ride-hail/internal/general/websocket"
	"ride-hail/internal/ports"
)

// DriverHTTPHandler adapts HTTP requests to the DriverLocationService.
type DriverHTTPHandler struct {
	svc       ports.DriverLocationService
	logger    *logger.Logger
	auth      *jwt.Manager
	websocket *websocket.WebSocket
}

// NewDriverHTTPHandler wires an HTTP handler around the DriverLocationService.
func NewDriverHTTPHandler(
	svc ports.DriverLocationService,
	logger *logger.Logger,
	auth *jwt.Manager,
	ws *websocket.WebSocket,
) *DriverHTTPHandler {
	return &DriverHTTPHandler{svc: svc, logger: logger, auth: auth, websocket: ws}
}

// RegisterRoutes mounts driver/location endpoints on the provided mux.
func (handler *DriverHTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /drivers/{driver_id}/online",
		jwt.AuthMiddlewareFunc(handler.auth, user.RoleDriver)(handler.handleGoOnline),
	)
	mux.HandleFunc("POST /drivers/{driver_id}/offline",
		jwt.AuthMiddlewareFunc(handler.auth, user.RoleDriver)(handler.handleGoOffline),
	)
	// ДОБАВИТЬ middleware к location endpoint
	mux.HandleFunc("POST /drivers/{driver_id}/location",
		jwt.AuthMiddlewareFunc(handler.auth, user.RoleDriver)(handler.handleUpdateLocation),
	)
	mux.HandleFunc("POST /drivers/{driver_id}/start",
		jwt.AuthMiddlewareFunc(handler.auth, user.RoleDriver)(handler.handleStartRide),
	)
	mux.HandleFunc("POST /drivers/{driver_id}/complete",
		jwt.AuthMiddlewareFunc(handler.auth, user.RoleDriver)(handler.handleCompleteRide),
	)

	// WebSocket без middleware
	mux.HandleFunc("GET /ws/driver/{driver_id}", handler.websocket.ConnectDriver)

	mux.HandleFunc("GET /drivers/health", handler.handleHealth)
	mux.HandleFunc("POST /tokens", handler.handleCreateToken)
}

// ----- general helpers -----

type TokenRequest struct {
	UserID string    `json:"user_id"`
	Role   user.Role `json:"role"`
}

type TokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	UserID    string    `json:"user_id"`
	Role      user.Role `json:"role"`
}

// handleCreateToken generates JWT tokens for testing
func (handler *DriverHTTPHandler) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	ctx := handler.withReqID(r.Context(), r)

	if r.Method != "POST" {
		handler.httpError(ctx, w, http.StatusMethodNotAllowed, "Method not allowed", nil)
		return
	}

	var req TokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handler.httpError(ctx, w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate required fields
	if strings.TrimSpace(req.UserID) == "" {
		handler.httpError(ctx, w, http.StatusBadRequest, "user_id is required", nil)
		return
	}

	// Generate token
	tokenString, claims, err := handler.auth.IssueUserToken(req.UserID, req.Role)
	if err != nil {
		handler.httpError(ctx, w, http.StatusInternalServerError, "Failed to generate token", err)
		return
	}

	response := TokenResponse{
		Token:     tokenString,
		ExpiresAt: claims.ExpiresAt.Time,
		UserID:    req.UserID,
		Role:      req.Role,
	}

	handler.logger.Info(ctx, "token_generated", "JWT token generated successfully",
		map[string]any{"user_id": req.UserID, "role": req.Role.String()})

	handler.jsonResponse(ctx, w, http.StatusCreated, response)
}

// jsonResponse takes any type of data and encode it to HTTP response.
func (handler *DriverHTTPHandler) jsonResponse(ctx context.Context, w http.ResponseWriter, status int, data any) {
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
func (handler *DriverHTTPHandler) httpError(ctx context.Context, w http.ResponseWriter, status int, msg string, err error) {
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
func (handler *DriverHTTPHandler) withReqID(ctx context.Context, r *http.Request) context.Context {
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
