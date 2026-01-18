package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"codeberg.org/thomas-mangin/zebgp/pkg/config"
)

// configureSlog sets up slog based on SLOG_LEVEL environment variable.
// Supported levels: DEBUG, INFO, WARN, ERROR (default: INFO).
func configureSlog() {
	levelStr := strings.ToUpper(os.Getenv("SLOG_LEVEL"))
	var level slog.Level
	switch levelStr {
	case "DEBUG":
		level = slog.LevelDebug
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, opts)))
}

func cmdServer(args []string) int {
	configureSlog()
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

	// Wait for signals or reactor self-stop (e.g., tcp.attempts reached)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	fmt.Println("ZeBGP running. Press Ctrl+C to stop.")

	// Wait for either signal or reactor to stop itself
	doneChan := make(chan struct{})
	go func() {
		_ = reactor.Wait(context.Background())
		close(doneChan)
	}()

	select {
	case <-sigChan:
		fmt.Println("\nShutting down...")
		reactor.Stop()
	case <-doneChan:
		fmt.Println("\nShutting down...")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*1e9)
	defer cancel()

	if err := reactor.Wait(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "warning: shutdown timeout: %v\n", err)
	}

	fmt.Println("ZeBGP stopped.")
	return 0
}
