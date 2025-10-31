package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

const (
	ModeRide      = "ride-service"
	ModeDriverLoc = "driver-location-service"
	ModeAdmin     = "admin-service"
)

// isKnownMode checks if the provided mode name is known.
func isKnownMode(s string) (string, bool) {
	switch s {
	case ModeRide, "ride", "r":
		return ModeRide, true
	case ModeDriverLoc, "driver", "driver-service", "dl":
		return ModeDriverLoc, true
	case ModeAdmin, "admin", "a":
		return ModeAdmin, true
	default:
		return "", false
	}
}

// ParseMode supports:
//
//	--mode=<value>
//	<value> (subcommand shorthand), e.g., `ride-service --port=3000`
func ParseMode(args []string) (string, []string, error) {
	var mode string
	var out []string

	for i := range args {
		arg := args[i]
		if after, ok := strings.CutPrefix(arg, "--mode="); ok {
			mode = after
			continue
		}

		if mode == "" {
			if m, ok := isKnownMode(arg); ok {
				mode = m
				continue
			}
		}
		out = append(out, arg)
	}

	if mode == "" {
		return "", out, errors.New("no mode specified: use --mode=<service>")
	}

	if m, ok := isKnownMode(mode); ok {
		mode = m
	}

	return mode, out, nil
}

// PrintUsage prints the usage information with examples.
func PrintUsage(w io.Writer) {
	fmt.Fprint(w, "\033[36m") // cyan

	fmt.Fprintln(w, `Usage:
  ./ride-hail-system --mode=<service> [flags]

Services (modes):
  ride-service                 HTTP API and orchestrator for ride lifecycle
  driver-location-service      Driver ops, matching, and real-time locations
  admin-service                Admin monitoring and metrics API

Examples:
  ./ride-hail-system --mode=ride-service --max-concurrent=150
  ./ride-hail-system --mode=driver-location-service --prefetch=8 --max-concurrent=200
  ./ride-hail-system --mode=admin-service --max-concurrent=50`)

	fmt.Fprint(w, "\033[0m") // reset
}

// AttachUsage wires a concise per-mode usage to a FlagSet.
func AttachUsage(fs *flag.FlagSet, mode string) {
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: ./ride-hail-system --mode=%s [flags]\n", mode)
		fs.PrintDefaults()
	}
}
