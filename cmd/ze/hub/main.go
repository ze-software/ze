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
	"codeberg.org/thomas-mangin/ze/internal/pidfile"
	"codeberg.org/thomas-mangin/ze/internal/sim"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// Run executes the hub with the given config file path and optional CLI plugins.
// chaosSeed > 0 enables chaos self-test mode; chaosRate < 0 means "use default".
// Returns exit code.
func Run(configPath string, plugins []string, chaosSeed int64, chaosRate float64) int {
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
		return runBGPInProcess(configPath, data, plugins, chaosSeed, chaosRate)
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

// acquirePIDFile attempts to acquire a PID file for the given config path.
// Returns the PIDFile (caller must Release) or an error if another instance
// holds the lock. Returns a no-op PIDFile for stdin configs or when the
// PID file location cannot be determined.
func acquirePIDFile(configPath string) (*pidfile.PIDFile, error) {
	if configPath == "-" {
		return pidfile.Noop(), nil
	}
	pidPath, err := pidfile.Location(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: PID file location: %v\n", err)
		return pidfile.Noop(), nil
	}
	pf, err := pidfile.Acquire(pidPath, configPath)
	if err != nil {
		return nil, fmt.Errorf("PID file: %w", err)
	}
	return pf, nil
}

// runBGPInProcess loads BGP config using YANG parser and runs reactor in-process.
func runBGPInProcess(configPath string, data []byte, plugins []string, chaosSeed int64, chaosRate float64) int {
	// Use YANG-based config parser with CLI plugins
	reactor, err := zeconfig.LoadReactorWithPlugins(string(data), configPath, plugins)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load config: %v\n", err)
		return 1
	}

	// Inject chaos wrappers if chaos mode is enabled.
	// CLI flags override env vars/config; seed=0 means disabled, -1 means time-based.
	if chaosSeed != 0 {
		chaosSeed = sim.ResolveSeed(chaosSeed)
		if chaosRate < 0 {
			chaosRate = 0.1 // Default rate when not specified by CLI
		}
		logger := slogutil.Logger("chaos")
		cfg := sim.ChaosConfig{
			Seed:   chaosSeed,
			Rate:   chaosRate,
			Logger: logger,
		}
		clock, dialer, listenerFactory := sim.NewChaosWrappers(
			sim.RealClock{}, &sim.RealDialer{}, sim.RealListenerFactory{}, cfg,
		)
		reactor.SetClock(clock)
		reactor.SetDialer(dialer)
		reactor.SetListenerFactory(listenerFactory)
		logger.Info("chaos self-test mode enabled",
			"seed", chaosSeed,
			"rate", chaosRate,
		)
	}

	// Acquire PID file (prevents duplicate instances)
	pf, err := acquirePIDFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	defer pf.Release()

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

	// Acquire PID file (prevents duplicate instances)
	pf, err := acquirePIDFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	defer pf.Release()

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
				if err := o.Reload(configPath); err != nil {
					fmt.Fprintf(os.Stderr, "reload error: %v\n", err)
					cancel()
					return
				}
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
