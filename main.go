package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	admindashboard "ride-hail/cmd/admin_service"
	driverlocationservice "ride-hail/cmd/driver_location_service"
	rideservice "ride-hail/cmd/ride_service"
	"ride-hail/internal/cli"
	"syscall"
	"time"
)

func main() {
	// quick path for global help
	if len(os.Args) == 2 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		cli.PrintUsage(os.Stdout)
		os.Exit(0)
	}

	// parse mode and collect the remaining args for that mode
	mode, svcArgs, err := cli.ParseMode(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		cli.PrintUsage(os.Stderr)
		os.Exit(2)
	}

	// context cancelled on SIGINT/SIGTERM for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// run the service specified by the mode flag
	switch mode {

	case cli.ModeRide:
		fs := flag.NewFlagSet(cli.ModeRide, flag.ContinueOnError)
		maxConc := fs.Int("max-concurrent", 100, "Maximum number of concurrent HTTP requests to process")
		cli.AttachUsage(fs, cli.ModeRide)

		if err := fs.Parse(svcArgs); err != nil {
			if err == flag.ErrHelp {
				os.Exit(0)
			}
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(2)
		}
		if *maxConc < 1 {
			fmt.Fprintln(os.Stderr, "Error: --max-concurrent must be >= 1")
			fs.Usage()
			os.Exit(2)
		}
		if err := rideservice.Run(ctx, *maxConc); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

	case cli.ModeDriverLoc:
		fs := flag.NewFlagSet(cli.ModeDriverLoc, flag.ContinueOnError)
		prefetch := fs.Int("prefetch", 8, "RabbitMQ prefetch count for consumer channels")
		maxConc := fs.Int("max-concurrent", 200, "Maximum number of concurrent tasks (offers, updates) to process")
		cli.AttachUsage(fs, cli.ModeDriverLoc)

		if err := fs.Parse(svcArgs); err != nil {
			if err == flag.ErrHelp {
				os.Exit(0)
			}
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(2)
		}
		if *prefetch <= 0 {
			fmt.Fprintln(os.Stderr, "Error: --prefetch must be > 0")
			fs.Usage()
			os.Exit(2)
		}
		if *maxConc < 1 {
			fmt.Fprintln(os.Stderr, "Error: --max-concurrent must be >= 1")
			fs.Usage()
			os.Exit(2)
		}
		if err := driverlocationservice.Run(ctx, *prefetch, *maxConc); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

	case cli.ModeAdmin:
		fs := flag.NewFlagSet(cli.ModeAdmin, flag.ContinueOnError)
		maxConc := fs.Int("max-concurrent", 50, "Maximum number of concurrent HTTP requests to process")
		cli.AttachUsage(fs, cli.ModeAdmin)

		if err := fs.Parse(svcArgs); err != nil {
			if err == flag.ErrHelp {
				os.Exit(0)
			}
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(2)
		}
		if *maxConc < 1 {
			fmt.Fprintln(os.Stderr, "Error: --max-concurrent must be >= 1")
			fs.Usage()
			os.Exit(2)
		}
		if err := admindashboard.Run(ctx, *maxConc); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

	default:
		// should not happen because ParseMode validates known modes
		fmt.Fprintln(os.Stderr, "Error: unknown mode")
		os.Exit(2)
	}

	// tiny delay to let deferred logs flush on very fast exits
	select {
	case <-ctx.Done():
	case <-time.After(10 * time.Millisecond):
	}
}
