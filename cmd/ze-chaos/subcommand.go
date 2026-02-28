// Design: docs/architecture/chaos-web-dashboard.md — replay, shrink, diff subcommands and network utilities
// Related: main.go — CLI entry dispatches to these subcommands

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/replay"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/scenario"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/shrink"
)

// runReplay opens an event log file and replays it through the validation model.
func runReplay(path string) int {
	f, err := os.Open(path) // #nosec G304 - path is from CLI flag, not user-controlled web input
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: opening replay file: %v\n", err)
		return 2
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error: closing replay file: %v\n", err)
		}
	}()
	return replay.Run(f, os.Stderr)
}

// runShrink reads a failing event log and minimizes it to the smallest
// subsequence that still triggers the same property violation.
func runShrink(path string, deadline time.Duration, verbose bool) int {
	f, err := os.Open(path) // #nosec G304 - path is from CLI flag
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: opening shrink file: %v\n", err)
		return 2
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error: closing shrink file: %v\n", err)
		}
	}()

	meta, events, parseErr := shrink.ParseLog(f)
	if parseErr != nil {
		fmt.Fprintf(os.Stderr, "error: parsing event log: %v\n", parseErr)
		return 2
	}

	cfg := shrink.Config{
		PeerCount: meta.Peers,
		Deadline:  deadline,
	}
	if verbose {
		cfg.Verbose = os.Stderr
	}

	result, shrinkErr := shrink.Run(events, cfg)
	if shrinkErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", shrinkErr)
		return 1
	}

	// Print human-readable summary.
	fmt.Fprintf(os.Stderr, "shrink: %d → %d events (%d iterations), property: %s\n",
		result.Original, len(result.Events), result.Iterations, result.Property)
	fmt.Fprintf(os.Stderr, "\nMinimal reproduction (%d steps):\n", len(result.Events))
	for i := range result.Events {
		line := fmt.Sprintf("  %d. [peer %d] %s", i+1, result.Events[i].PeerIndex, result.Events[i].Type)
		if result.Events[i].Prefix.IsValid() {
			line += " " + result.Events[i].Prefix.String()
		}
		fmt.Fprintln(os.Stderr, line)
	}

	return 0
}

// runDiff opens two event log files and reports the first divergence.
func runDiff(path1, path2 string) int {
	f1, err := os.Open(path1) // #nosec G304 - path is from CLI flag, not user-controlled web input
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: opening diff file 1: %v\n", err)
		return 2
	}
	defer func() {
		if err := f1.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error: closing diff file 1: %v\n", err)
		}
	}()

	f2, err := os.Open(path2) // #nosec G304 - path is from CLI flag, not user-controlled web input
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: opening diff file 2: %v\n", err)
		return 2
	}
	defer func() {
		if err := f2.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error: closing diff file 2: %v\n", err)
		}
	}()

	return replay.Diff(f1, f2, os.Stderr)
}

// dashboardURL converts a listen address (e.g. ":8080", "0.0.0.0:8080") to a
// clickable URL like "http://localhost:8080". Used for the startup message.
func dashboardURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port)
}

// checkPortFree verifies that nothing is listening on addr.
// Called before starting Ze to fail fast on port conflicts.
func checkPortFree(addr string) error {
	dialer := net.Dialer{Timeout: 500 * time.Millisecond}
	conn, err := dialer.DialContext(context.Background(), "tcp", addr)
	if err == nil {
		// Port is in use — connection succeeded.
		if closeErr := conn.Close(); closeErr != nil {
			return fmt.Errorf("port %s in use (close: %w)", addr, closeErr)
		}
		return fmt.Errorf("port %s is already in use — stop the existing process first", addr)
	}

	// Connection refused or timeout means nothing is listening — port is free.
	// Other errors (permission denied, network unreachable) should propagate.
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if errors.Is(opErr.Err, syscall.ECONNREFUSED) {
			return nil
		}
		if opErr.Timeout() {
			return nil
		}
	}

	return fmt.Errorf("checking port %s: %w", addr, err)
}

// waitForZe waits for Ze to start listening on addr.
// In pipeline mode, Ze is reading piped config and needs time to initialize.
// Uses TCP connect only — no BGP OPEN, to avoid corrupting the peer session
// on the probed port.
func waitForZe(ctx context.Context, addr string, pipeline bool) error {
	maxAttempts := 1
	if pipeline {
		maxAttempts = 15
	}

	var lastErr error
	for attempt := range maxAttempts {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		dialer := net.Dialer{Timeout: time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err == nil {
			if closeErr := conn.Close(); closeErr != nil {
				return fmt.Errorf("probe close: %w", closeErr)
			}
			// Brief delay to let ze process the probe's EOF.
			// The probe hits a per-peer BGP port, creating a session that
			// immediately gets EOF. This sleep gives ze's FSM time to
			// reset to Idle before the real peer connects on this port.
			time.Sleep(200 * time.Millisecond)
			return nil
		}
		lastErr = err

		if attempt < maxAttempts-1 {
			select {
			case <-time.After(time.Duration(attempt+1) * 200 * time.Millisecond):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return fmt.Errorf("ze did not start within timeout on %s: %w", addr, lastErr)
}

// peerFamilyTargets computes per-peer per-family expected route counts from profiles.
// Mirrors the simulator logic: unicast families get full RouteCount,
// non-unicast families (VPN, EVPN, FlowSpec) get RouteCount/4.
func peerFamilyTargets(profiles []scenario.PeerProfile) map[int]map[string]int {
	targets := make(map[int]map[string]int, len(profiles))
	for i := range profiles {
		fm := make(map[string]int, len(profiles[i].Families))
		for _, fam := range profiles[i].Families {
			if strings.Contains(fam, "unicast") {
				fm[fam] = profiles[i].RouteCount
			} else {
				// non-unicast (VPN, EVPN, FlowSpec, Multicast) get RouteCount/4
				fm[fam] = profiles[i].RouteCount / 4
			}
		}
		targets[profiles[i].Index] = fm
	}
	return targets
}
