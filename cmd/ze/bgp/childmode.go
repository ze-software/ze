// Design: docs/architecture/core-design.md — BGP CLI commands

package bgp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/chaos"
	bgpconfig "codeberg.org/thomas-mangin/ze/internal/component/bgp/config"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/reactor"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/core/network"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// isChildMode returns true if ze bgp should run as a hub child process.
// Detection order:
// 1. --child flag
// 2. ZE_CHILD_MODE=1 environment variable.
func isChildMode(args []string) bool {
	// Check for --child flag
	if slices.Contains(args, "--child") {
		return true
	}

	// Check environment variable
	if os.Getenv("ZE_CHILD_MODE") == "1" {
		return true
	}

	return false
}

// parseChildArgs extracts config path from child mode arguments.
func parseChildArgs(args []string) string {
	for i, arg := range args {
		if arg == "--config" && i+1 < len(args) {
			return args[i+1]
		}
		if after, ok := strings.CutPrefix(arg, "--config="); ok {
			return after
		}
	}
	return ""
}

// writeDeclare writes Stage 1 declaration to the output.
// This declares the BGP schema and handlers to the hub.
func writeDeclare(w io.Writer) error {
	declarations := []string{
		"declare module ze-bgp-conf",
		"declare namespace urn:ze:bgp:conf",
		"declare handler bgp",
		"declare handler bgp.peer",
		"declare priority 100",
		"declare done",
	}

	for _, decl := range declarations {
		if _, err := fmt.Fprintln(w, decl); err != nil {
			return err
		}
	}

	return nil
}

// writeCapability writes Stage 3 capability declaration.
func writeCapability(w io.Writer) error {
	_, err := fmt.Fprintln(w, "capability done")
	return err
}

// writeReady writes Stage 5 ready signal.
func writeReady(w io.Writer) error {
	_, err := fmt.Fprintln(w, "ready")
	return err
}

// parseChildConfig parses JSON config from the hub.
func parseChildConfig(r io.Reader) (map[string]any, error) {
	var cfg map[string]any

	// Read until we get a complete JSON object
	// The hub sends config as a single JSON line after "config "
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Config done marker (no config sent) - check before parsing JSON
		if line == "config done" {
			return make(map[string]any), nil
		}

		// Config line format: "config {json...}"
		if after, ok := strings.CutPrefix(line, "config "); ok {
			jsonStr := after
			if err := json.Unmarshal([]byte(jsonStr), &cfg); err != nil {
				return nil, fmt.Errorf("parse config JSON: %w", err)
			}
			return cfg, nil
		}

		// Try direct JSON parse (for testing)
		if strings.HasPrefix(line, "{") {
			if err := json.Unmarshal([]byte(line), &cfg); err != nil {
				return nil, fmt.Errorf("parse config JSON: %w", err)
			}
			return cfg, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	return nil, fmt.Errorf("no config received")
}

// runChildModeWithArgs runs BGP as a hub child process using the 5-stage protocol.
func runChildModeWithArgs(args []string) int {
	// Parse config path from arguments
	configPath := parseChildArgs(args)

	// Stage 1: Declare schema and handlers
	if err := writeDeclare(os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error: stage 1 declare: %v\n", err)
		return 1
	}

	// Stage 2: Receive config acknowledgment from hub
	// Hub sends "config done" - we don't need JSON config since we have the file path
	_, err := parseChildConfig(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: stage 2 config: %v\n", err)
		return 1
	}

	// Stage 3: Declare capabilities
	if err := writeCapability(os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error: stage 3 capability: %v\n", err)
		return 1
	}

	// Stage 4: Registry (handled by hub, we just need to wait for registry done)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "registry done" {
			break
		}
	}

	// Stage 5: Signal ready
	if err := writeReady(os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error: stage 5 ready: %v\n", err)
		return 1
	}

	// Load and start reactor if config path provided
	if configPath == "" {
		fmt.Fprintf(os.Stderr, "error: no config path provided (use --config)\n")
		return 1
	}

	reactor, err := bgpconfig.LoadReactorFile(storage.NewFilesystem(), configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load config: %v\n", err)
		return 1
	}

	// Inject chaos wrappers from environment variables (child processes
	// inherit env vars from the parent, but not CLI flags).
	if seed := os.Getenv("ze.bgp.chaos.seed"); seed != "" {
		injectChaosFromEnv(reactor, seed, os.Getenv("ze.bgp.chaos.rate"))
	} else if seed := os.Getenv("ze_chaos_seed"); seed != "" {
		injectChaosFromEnv(reactor, seed, os.Getenv("ze_chaos_rate"))
	}

	if err := reactor.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error: start reactor: %v\n", err)
		return 1
	}

	// Wait for shutdown signal (stdin closes when hub terminates)
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break // EOF or error - hub closed pipe
		}

		// Handle commands from hub
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Handle shutdown command
		if line == "shutdown" {
			break
		}

		// Route commands to reactor (future: implement command handling)
		fmt.Fprintf(os.Stderr, "child: unhandled command: %s\n", line)
	}

	// Graceful shutdown
	reactor.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 10e9)
	defer cancel()
	_ = reactor.Wait(ctx)

	return 0
}

// injectChaosFromEnv configures chaos wrappers on the reactor from env var values.
// seedStr is the string value of the chaos seed env var.
// rateStr is the string value of the chaos rate env var (empty = default 0.1).
func injectChaosFromEnv(r *reactor.Reactor, seedStr, rateStr string) {
	seed, err := strconv.ParseInt(seedStr, 10, 64)
	if err != nil || seed == 0 {
		return
	}
	seed = chaos.ResolveSeed(seed)

	rate := 0.1
	if rateStr != "" {
		if f, parseErr := strconv.ParseFloat(rateStr, 64); parseErr == nil && f >= 0 && f <= 1.0 {
			rate = f
		}
	}

	logger := slogutil.Logger("chaos")
	cfg := chaos.ChaosConfig{
		Seed:   seed,
		Rate:   rate,
		Logger: logger,
	}
	clock, dialer, listenerFactory := chaos.NewChaosWrappers(
		clock.RealClock{}, &network.RealDialer{}, network.RealListenerFactory{}, cfg,
	)
	r.SetClock(clock)
	r.SetDialer(dialer)
	r.SetListenerFactory(listenerFactory)
	logger.Info("chaos self-test mode enabled (child)",
		"seed", seed,
		"rate", rate,
	)
}
