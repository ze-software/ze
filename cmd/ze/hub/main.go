// Design: docs/architecture/hub-architecture.md — hub CLI entry point
//
// Package hub provides the ze hub subcommand.
package hub

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/chaos"
	bgpconfig "codeberg.org/thomas-mangin/ze/internal/component/bgp/config"
	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/hub"
	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/core/network"
	"codeberg.org/thomas-mangin/ze/internal/core/pidfile"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// Run executes the hub with the given config file path and optional CLI plugins.
// store provides the I/O backend (filesystem or blob); used for config reads and reload.
// chaosSeed > 0 enables chaos self-test mode; chaosRate < 0 means "use default".
// Returns exit code.
func Run(store storage.Storage, configPath string, plugins []string, chaosSeed int64, chaosRate float64) int {
	// Read config content first (to probe type without parsing).
	// When reading from stdin, we look for a NUL sentinel that signals
	// "config complete but pipe stays open for liveness monitoring."
	var data []byte
	var stdinOpen bool
	var err error
	if configPath == "-" {
		data, stdinOpen, err = readStdinConfig()
	} else {
		data, err = store.ReadFile(configPath)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read config: %v\n", err)
		return 1
	}

	// Probe config type using shared function
	switch zeconfig.ProbeConfigType(string(data)) {
	case zeconfig.ConfigTypeBGP:
		// Run BGP in-process using YANG parser
		return runBGPInProcess(store, configPath, data, plugins, chaosSeed, chaosRate, stdinOpen)
	case zeconfig.ConfigTypeHub:
		// Run hub orchestrator using hub parser
		// TODO: pass plugins to orchestrator when hub mode supports them
		_ = plugins // Currently unused in hub mode
		return runOrchestratorWithData(store, configPath, data)
	case zeconfig.ConfigTypeUnknown:
		fmt.Fprintf(os.Stderr, "error: config has no recognized block (bgp, plugin)\n")
	}

	return 1
}

// readStdinConfig reads config from stdin, stopping at a NUL byte sentinel
// or EOF. Returns the config data and whether stdin remains open (NUL found).
//
// When stdin remains open, the caller can monitor it for EOF to detect
// upstream process exit — e.g., in a pipeline like "ze-chaos | ze -",
// when the chaos tool exits, stdin closes, and Ze initiates clean shutdown.
//
// When no NUL is found (plain "cat config.conf | ze -"), reading stops at
// EOF with stdinOpen=false — the normal case.
func readStdinConfig() (data []byte, stdinOpen bool, err error) {
	var buf []byte
	tmp := make([]byte, 4096)
	for {
		n, readErr := os.Stdin.Read(tmp)
		if n > 0 {
			for i := range n {
				if tmp[i] == 0 {
					buf = append(buf, tmp[:i]...)
					return buf, true, nil
				}
			}
			buf = append(buf, tmp[:n]...)
		}
		if readErr != nil {
			if readErr == io.EOF {
				return buf, false, nil
			}
			return nil, false, readErr
		}
	}
}

// acquirePIDFile attempts to acquire a PID file for the given config path.
// Returns the PIDFile (caller must Release) or an error if another instance
// holds the lock. Returns a no-op PIDFile for stdin configs or when the
// PID file location cannot be determined.
// When store is non-nil, the PID is stored in the blob with kill(0) mutual exclusion.
// When store is nil, uses filesystem flock (current behavior).
func acquirePIDFile(store storage.Storage, configPath string) (*pidfile.PIDFile, error) {
	if configPath == "-" {
		return pidfile.Noop(), nil
	}
	if storage.IsBlobStorage(store) {
		pidKey := strings.TrimPrefix(configPath, "/") + ".pid"
		return pidfile.AcquireWithStorage(store, pidKey, configPath)
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
// When stdinOpen is true, a background goroutine monitors stdin for EOF and
// triggers shutdown when the upstream process exits (pipe mode).
func runBGPInProcess(store storage.Storage, configPath string, data []byte, plugins []string, chaosSeed int64, chaosRate float64, stdinOpen bool) int {
	// Use YANG-based config parser with CLI plugins
	reactor, err := bgpconfig.LoadReactorWithPlugins(store, string(data), configPath, plugins)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load config: %v\n", err)
		return 1
	}

	// Inject chaos wrappers if chaos mode is enabled.
	// CLI flags override env vars/config; seed=0 means disabled, -1 means time-based.
	if chaosSeed != 0 {
		chaosSeed = chaos.ResolveSeed(chaosSeed)
		if chaosRate < 0 {
			chaosRate = 0.1 // Default rate when not specified by CLI
		}
		logger := slogutil.Logger("chaos")
		cfg := chaos.ChaosConfig{
			Seed:   chaosSeed,
			Rate:   chaosRate,
			Logger: logger,
		}
		clock, dialer, listenerFactory := chaos.NewChaosWrappers(
			clock.RealClock{}, &network.RealDialer{}, network.RealListenerFactory{}, cfg,
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
	pf, err := acquirePIDFile(store, configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	defer pf.Release()

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Monitor stdin for EOF when running in pipe mode (ze-chaos | ze -).
	// After reading config (delimited by NUL), stdin stays open. When the
	// upstream process exits, the pipe closes and this goroutine triggers
	// clean shutdown — no Ctrl-C needed.
	if stdinOpen {
		go monitorStdinEOF(sigCh)
	}

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

	// Signal readiness to test infrastructure. Written after signal.Notify
	// and reactor.Start so the test runner knows the daemon is fully operational.
	if readyFile := os.Getenv("ZE_READY_FILE"); readyFile != "" {
		if f, err := os.Create(readyFile); err == nil { //nolint:gosec // test infrastructure path from env
			f.Close() //nolint:errcheck,gosec // best-effort readiness signal
		}
	}

	select {
	case <-sigCh:
		fmt.Println("\nShutting down...")
		reactor.Stop()
	case <-doneCh:
		fmt.Println("\nShutting down...")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := reactor.Wait(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "warning: shutdown timeout: %v\n", err)
	}

	fmt.Println("Ze BGP stopped.")
	return 0
}

// monitorStdinEOF blocks until stdin is closed (EOF or error), then sends
// SIGTERM to sigCh to trigger reactor shutdown.
func monitorStdinEOF(sigCh chan<- os.Signal) {
	b := make([]byte, 1)
	if _, err := os.Stdin.Read(b); err != nil {
		fmt.Fprintf(os.Stderr, "\nUpstream pipe closed (%v), shutting down...\n", err)
	} else {
		fmt.Fprintln(os.Stderr, "\nUpstream pipe closed, shutting down...")
	}
	select {
	case sigCh <- syscall.SIGTERM:
	default:
	}
}

// runOrchestratorWithData parses hub config and runs the orchestrator.
func runOrchestratorWithData(store storage.Storage, configPath string, data []byte) int {
	cfg, err := hub.ParseHubConfig(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: parse config: %v\n", err)
		return 1
	}
	cfg.ConfigPath = configPath

	// Acquire PID file (prevents duplicate instances)
	pf, err := acquirePIDFile(store, configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	defer pf.Release()

	o := hub.NewOrchestrator(cfg)
	o.SetStorage(store)

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
	fmt.Fprintf(os.Stderr, "hub: started with config %s\n", configPath)

	// Signal readiness to test infrastructure. Written after signal.Notify
	// and o.Start so the test runner knows signal handlers are registered.
	if readyFile := os.Getenv("ZE_READY_FILE"); readyFile != "" {
		if f, err := os.Create(readyFile); err == nil { //nolint:gosec // test infrastructure path from env
			f.Close() //nolint:errcheck,gosec // best-effort readiness signal
		}
	}

	// Wait for shutdown
	<-ctx.Done()

	// Clean shutdown
	o.Stop()
	return 0
}
