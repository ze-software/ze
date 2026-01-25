// Package hub provides the ze hub subcommand.
package hub

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"codeberg.org/thomas-mangin/ze/internal/hub"
)

// Run executes the hub with the given config file path.
// Returns exit code.
func Run(configPath string) int {
	// Load config
	cfg, err := hub.LoadHubConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load config: %v\n", err)
		return 1
	}

	// Create orchestrator
	o := hub.NewOrchestrator(cfg)

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		for sig := range sigCh {
			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				fmt.Fprintf(os.Stderr, "received %s, shutting down...\n", sig)
				cancel()
				return
			case syscall.SIGHUP:
				fmt.Fprintf(os.Stderr, "received SIGHUP, reloading config...\n")
				// TODO: Implement config reload
			}
		}
	}()

	// Start orchestrator
	if err := o.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: start: %v\n", err)
		return 1
	}

	// Wait for shutdown
	<-ctx.Done()

	// Clean shutdown
	o.Stop()
	return 0
}
