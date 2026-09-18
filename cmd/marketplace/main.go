// Command marketplace is the platform's single binary.
package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aleogr/marketplace/internal/platform/config"
	"github.com/aleogr/marketplace/internal/platform/httpx"
	"github.com/aleogr/marketplace/internal/platform/logging"
	"github.com/aleogr/marketplace/internal/platform/version"
)

// shutdownGrace is how long in-flight requests have to finish after SIGTERM.
// Cloud Run allows up to ten seconds before it kills the instance.
const shutdownGrace = 8 * time.Second

func main() {
	if err := run(context.Background(), os.LookupEnv, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "marketplace: %v\n", err)
		os.Exit(1)
	}
}

// run starts the server and returns when the process is asked to stop, or
// immediately with an error when it cannot start.
func run(ctx context.Context, lookup config.Lookup, stdout io.Writer) error {
	cfg, err := config.Load(lookup)
	if err != nil {
		return err
	}

	log := logging.New(stdout, cfg.LogLevel)

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return fmt.Errorf("cannot listen on port %d: %w", cfg.Port, err)
	}

	log.InfoContext(ctx, "server started",
		"version", version.String(),
		"port", cfg.Port,
		"indexable", cfg.Indexable,
		"providers", string(cfg.ProvidersMode),
	)

	if err := httpx.Serve(ctx, listener, httpx.Handler(), shutdownGrace); err != nil {
		return err
	}

	log.InfoContext(context.WithoutCancel(ctx), "server stopped")
	return nil
}
