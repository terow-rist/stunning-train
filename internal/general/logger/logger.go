package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// ----- Public wire types -----

// ErrorObject is emitted only for error logs.
type ErrorObject struct {
	Msg   string `json:"msg"`
	Stack string `json:"stack"`
}

// LogEntry is the single-line JSON format written to stdout.
type LogEntry struct {
	Timestamp string       `json:"timestamp"`            // ISO 8601 format timestamp
	Level     string       `json:"level"`                // DEBUG | INFO | ERROR
	Service   string       `json:"service"`              // service name (e.g., ride-service)
	Action    string       `json:"action"`               // event name (e.g., ride_requested)
	Message   string       `json:"message"`              // human-readable description
	Hostname  string       `json:"hostname"`             // service hostname
	RequestID string       `json:"request_id,omitempty"` // correlation ID for tracing
	RideID    string       `json:"ride_id,omitempty"`    // ride identifier (when applicable)
	Details   any          `json:"details,omitempty"`    // optional: extra fields (map or struct)
	Error     *ErrorObject `json:"error,omitempty"`      // optional: error details
}

// ----- Logger -----

type Logger struct {
	service  string
	hostname string
	mu       sync.Mutex
}

// New creates a structured logger for the given service.
func New(service string) *Logger {
	hn, err := os.Hostname()
	if err != nil || strings.TrimSpace(hn) == "" {
		hn = "unknown-hostname"
	}

	if strings.TrimSpace(service) == "" {
		service = "unknown-service"
	}

	return &Logger{service: service, hostname: hn}
}

// emit marshals and prints a single JSON line to stdout.
func (l *Logger) emit(e LogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, err := json.Marshal(e)
	if err == nil {
		fmt.Println(string(b))
		return
	}

	// retry once without Details (common source of marshal errors)
	e.Details = nil
	if b, err := json.Marshal(e); err == nil {
		fmt.Println(string(b))
		return
	}

	// final structured fallback to stdout to keep logs JSON-shaped
	fallback := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"level":     "ERROR",
		"service":   l.service,
		"action":    "logger_marshal_failed",
		"message":   "failed to encode log entry",
		"hostname":  l.hostname,
		"error": ErrorObject{
			Msg:   strings.TrimSpace(err.Error()),
			Stack: string(debug.Stack()),
		},
	}

	if fb, err := json.Marshal(fallback); err == nil {
		fmt.Println(string(fb))
	} else {
		// absolute last resort (very unlikely)
		fmt.Fprintf(os.Stderr, "log marshal failed: %v\n", err)
	}
}

// Debug writes a DEBUG line with optional details.
func (l *Logger) Debug(ctx context.Context, action, msg string, details any) {
	l.emit(LogEntry{
		Timestamp: nowISO(),
		Level:     "DEBUG",
		Service:   l.service,
		Action:    safeAction(action),
		Message:   strings.TrimSpace(msg),
		Hostname:  l.hostname,
		RequestID: requestID(ctx),
		RideID:    rideID(ctx),
		Details:   details,
	})
}

// Info writes an INFO line with optional details.
func (l *Logger) Info(ctx context.Context, action, msg string, details any) {
	l.emit(LogEntry{
		Timestamp: nowISO(),
		Level:     "INFO",
		Service:   l.service,
		Action:    safeAction(action),
		Message:   strings.TrimSpace(msg),
		Hostname:  l.hostname,
		RequestID: requestID(ctx),
		RideID:    rideID(ctx),
		Details:   details,
	})
}

// Error writes an ERROR line and attaches an error stack trace.
func (l *Logger) Error(ctx context.Context, action, msg string, err error, details any) {
	if err == nil {
		err = fmt.Errorf("unknown error")
	}

	l.emit(LogEntry{
		Timestamp: nowISO(),
		Level:     "ERROR",
		Service:   l.service,
		Action:    safeAction(action),
		Message:   strings.TrimSpace(msg),
		Hostname:  l.hostname,
		RequestID: requestID(ctx),
		RideID:    rideID(ctx),
		Details:   details,
		Error: &ErrorObject{
			Msg:   strings.TrimSpace(err.Error()),
			Stack: string(debug.Stack()),
		},
	})
}

// ------------ Context helpers -------------

type ctxKey string

const (
	ctxKeyRequestID ctxKey = "ridehail_request_id"
	ctxKeyRideID    ctxKey = "ridehail_ride_id"
)

// WithRequestID returns a new context carrying request_id.
func (l *Logger) WithRequestID(ctx context.Context, reqID string) context.Context {
	if strings.TrimSpace(reqID) == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyRequestID, reqID)
}

// WithRideID returns a new context carrying ride_id.
func (l *Logger) WithRideID(ctx context.Context, rideID string) context.Context {
	if strings.TrimSpace(rideID) == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyRideID, rideID)
}

// requestID extracts request_id from ctx (if any).
func requestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v := ctx.Value(ctxKeyRequestID); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// rideID extracts ride_id from ctx (if any).
func rideID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v := ctx.Value(ctxKeyRideID); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ----- Small utilities -----

func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func safeAction(a string) string {
	a = strings.TrimSpace(a)
	if a == "" {
		return "unspecified"
	}
	return a
}
