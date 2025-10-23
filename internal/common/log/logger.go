package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"ride-hail/internal/common/contextx"
)

func New(service string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Key = "timestamp"
			}
			return a
		},
	}).WithAttrs([]slog.Attr{
		slog.String("service", service),
	})
	slog.SetDefault(slog.New(handler))

	return slog.New(handler)
}

func Info(ctx context.Context, log *slog.Logger, action, message string) {
	log.Info(message,
		"action", action,
		"hostname", hostname(),
		"request_id", contextx.GetRequestID(ctx),
		"ride_id", contextx.GetRideID(ctx),
	)
}

func Error(ctx context.Context, log *slog.Logger, action, message string, err error) {
	if err == nil {
		log.Error(message,
			"action", action,
			"hostname", hostname(),
			"request_id", contextx.GetRequestID(ctx),
			"ride_id", contextx.GetRideID(ctx),
		)
		return
	}

	log.Error(message,
		"action", action,
		"hostname", hostname(),
		"request_id", contextx.GetRequestID(ctx),
		"ride_id", contextx.GetRideID(ctx),
		slog.Group("error",
			"msg", err.Error(),
			"stack", shortStack(3, 8),
		),
	)
}

func shortStack(skip, max int) string {
	pcs := make([]uintptr, 64)
	n := runtime.Callers(skip, pcs)
	frames := runtime.CallersFrames(pcs[:n])

	var b strings.Builder
	count := 0
	for {
		f, more := frames.Next()
		fn := f.Function
		if strings.HasPrefix(fn, "runtime.") || strings.Contains(fn, "/logger.") {
			if !more {
				break
			}
			continue
		}
		file := filepath.Base(f.File)
		if i := strings.LastIndex(fn, "."); i >= 0 && i+1 < len(fn) {
			fn = fn[i+1:]
		}
		fmt.Fprintf(&b, "%s %s:%d\n", fn, file, f.Line)
		count++
		if count >= max || !more {
			break
		}
	}
	return strings.TrimSpace(b.String())
}

func hostname() string {
	name, _ := os.Hostname()
	return name
}