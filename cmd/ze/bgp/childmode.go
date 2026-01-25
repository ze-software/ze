package bgp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/config"
)

// isChildMode returns true if ze bgp should run as a hub child process.
// Detection order:
// 1. --child flag
// 2. ZE_CHILD_MODE=1 environment variable.
func isChildMode(args []string) bool {
	// Check for --child flag
	for _, arg := range args {
		if arg == "--child" {
			return true
		}
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
		if strings.HasPrefix(arg, "--config=") {
			return strings.TrimPrefix(arg, "--config=")
		}
	}
	return ""
}

// writeDeclare writes Stage 1 declaration to the output.
// This declares the BGP schema and handlers to the hub.
func writeDeclare(w io.Writer) error {
	declarations := []string{
		"declare module ze-bgp",
		"declare namespace urn:ze:bgp",
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
		if strings.HasPrefix(line, "config ") {
			jsonStr := strings.TrimPrefix(line, "config ")
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

	reactor, err := config.LoadReactorFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load config: %v\n", err)
		return 1
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
