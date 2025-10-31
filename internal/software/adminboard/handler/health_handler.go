package handler

import (
	"encoding/json"
	"net/http"
)

// ----- Handler: GET /rides/health -----

// handleHealth returns a minimal JSON health status payload.
func (handler *AdminHTTPHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)

	type resp struct {
		Status string `json:"status"`
	}
	_ = json.NewEncoder(w).Encode(resp{Status: "ok"})
}
