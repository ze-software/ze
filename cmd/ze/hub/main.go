// Package hub provides the ze hub subcommand.
package hub

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	zeconfig "codeberg.org/thomas-mangin/ze/internal/config"
	"codeberg.org/thomas-mangin/ze/internal/hub"
)

// Run executes the hub with the given config file path and optional CLI plugins.
// Returns exit code.
func Run(configPath string, plugins []string) int {
	// Read config content first (to probe type without parsing)
	var data []byte
	var err error
	if configPath == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(configPath) //nolint:gosec // Config path from user
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read config: %v\n", err)
		return 1
	}

	// Probe config type using shared function
	switch zeconfig.ProbeConfigType(string(data)) {
	case zeconfig.ConfigTypeBGP:
		// Run BGP in-process using YANG parser
		return runBGPInProcess(configPath, data, plugins)
	case zeconfig.ConfigTypeHub:
		// Run hub orchestrator using hub parser
		// TODO: pass plugins to orchestrator when hub mode supports them
		_ = plugins // Currently unused in hub mode
		return runOrchestratorWithData(configPath, data)
	case zeconfig.ConfigTypeUnknown:
		fmt.Fprintf(os.Stderr, "error: config has no recognized block (bgp, plugin)\n")
	}

	return 1
}

// runBGPInProcess loads BGP config using YANG parser and runs reactor in-process.
func runBGPInProcess(configPath string, data []byte, plugins []string) int {
	// Use YANG-based config parser with CLI plugins
	reactor, err := zeconfig.LoadReactorWithPlugins(string(data), plugins)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load config: %v\n", err)
		return 1
	}

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Printf("Starting ze BGP with config: %s\n", configPath)

	if err := reactor.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error starting reactor: %v\n", err)
		return 1
	}

	// Wait for either signal or reactor to stop itself
	doneCh := make(chan struct{})
	go func() {
		_ = reactor.Wait(context.Background())
		close(doneCh)
	}()

	fmt.Println("Ze BGP running. Press Ctrl+C to stop.")

	select {
	case <-sigCh:
		fmt.Println("\nShutting down...")
		reactor.Stop()
	case <-doneCh:
		fmt.Println("\nShutting down...")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*1e9)
	defer cancel()

	if err := reactor.Wait(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "warning: shutdown timeout: %v\n", err)
	}

	fmt.Println("Ze BGP stopped.")
	return 0
}

// runOrchestratorWithData parses hub config and runs the orchestrator.
func runOrchestratorWithData(configPath string, data []byte) int {
	cfg, err := hub.ParseHubConfig(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: parse config: %v\n", err)
		return 1
	}
	cfg.ConfigPath = configPath

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
