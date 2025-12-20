package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/exa-networks/zebgp/pkg/config"
)

func cmdServer(args []string) int {
	fs := flag.NewFlagSet("server", flag.ExitOnError)
	dryRun := fs.Bool("n", false, "dry-run (validate only, don't start)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: zebgp [server] [options] <config-file>

Start the ZeBGP daemon.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Signals:
  SIGTERM, SIGINT   Graceful shutdown
  SIGHUP            Reload configuration
  SIGUSR1           Dump status

Examples:
  zebgp config.conf
  zebgp server config.conf
  zebgp server -n config.conf   # validate only
`)
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: missing config file\n")
		fs.Usage()
		return 1
	}

	configPath := fs.Arg(0)

	// Load config and create reactor
	reactor, err := config.LoadReactorFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if *dryRun {
		fmt.Printf("✓ %s: configuration valid\n", configPath)
		return 0
	}

	// Start reactor
	fmt.Printf("Starting ZeBGP with config: %s\n", configPath)

	if err := reactor.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error starting reactor: %v\n", err)
		return 1
	}

	// Wait for signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	fmt.Println("ZeBGP running. Press Ctrl+C to stop.")

	<-sigChan
	fmt.Println("\nShutting down...")

	reactor.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*1e9)
	defer cancel()

	if err := reactor.Wait(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "warning: shutdown timeout: %v\n", err)
	}

	fmt.Println("ZeBGP stopped.")
	return 0
}
