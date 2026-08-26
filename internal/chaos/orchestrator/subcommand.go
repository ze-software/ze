// Design: docs/architecture/chaos-web-dashboard.md -- replay, shrink, diff actions and network utilities

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/chaos/replay"
	"github.com/ze-software/ze/internal/chaos/scenario"
	"github.com/ze-software/ze/internal/chaos/shrink"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// runReplay opens an event log file and replays it through the validation model.
func runReplay(path string) int {
	f, err := cliio.OpenReader(path) // "-" reads stdin
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
	f, err := cliio.OpenReader(path) // "-" reads stdin
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

	fmt.Fprintf(os.Stderr, "shrink: %d → %d events (%d iterations), property: %s\n",
		result.Original, len(result.Events), result.Iterations, result.Property)
	fmt.Fprintf(os.Stderr, "\nMinimal reproduction (%d steps):\n", len(result.Events))
	for i := range result.Events {
		line := fmt.Sprintf("  %d. [peer %d] %s", i+1, result.Events[i].PeerIndex, result.Events[i].Type)
		if result.Events[i].Prefix.IsValid() {
			var tb textbuf.Buffer
			line = tb.Str(line).Byte(' ').Prefix(result.Events[i].Prefix).String()
		}
		fmt.Fprintln(os.Stderr, line)
	}

	return 0
}

// runDiff opens two event log files and reports the first divergence.
func runDiff(path1, path2 string) int {
	f1, err := cliio.OpenReader(path1) // "-" reads stdin
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: opening diff file 1: %v\n", err)
		return 2
	}
	defer func() {
		if err := f1.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error: closing diff file 1: %v\n", err)
		}
	}()

	f2, err := cliio.OpenReader(path2) // "-" reads stdin (once; a second "-" fails closed)
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

// dashboardURL converts a listen address to a clickable URL.
func dashboardURL(addr string) string {
	var tb textbuf.Buffer
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return tb.Str("http://").Str(addr).String()
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	return tb.Str("http://").HostPort(host, port).String()
}

// checkPortFree verifies that nothing is listening on addr.
func checkPortFree(addr string) error {
	dialer := net.Dialer{Timeout: 500 * time.Millisecond}
	conn, err := dialer.DialContext(context.Background(), "tcp", addr)
	if err == nil {
		if closeErr := conn.Close(); closeErr != nil {
			return fmt.Errorf("port %s in use (close: %w)", addr, closeErr)
		}
		return fmt.Errorf("port %s is already in use — stop the existing process first", addr)
	}

	if opErr, ok := errors.AsType[*net.OpError](err); ok {
		if errors.Is(opErr.Err, syscall.ECONNREFUSED) {
			return nil
		}
		if opErr.Timeout() {
			return nil
		}
	}

	return fmt.Errorf("checking port %s: %w", addr, err)
}

// allocatePort binds a TCP listener on addr:0 and returns the kernel-assigned port.
func allocatePort(ctx context.Context, addr string) (int, error) {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", net.JoinHostPort(addr, "0"))
	if err != nil {
		return 0, fmt.Errorf("allocating port: %w", err)
	}
	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		if closeErr := ln.Close(); closeErr != nil {
			return 0, fmt.Errorf("allocated listener has non-TCP address (close: %w)", closeErr)
		}
		return 0, fmt.Errorf("allocated listener has non-TCP address")
	}
	if err := ln.Close(); err != nil {
		return 0, fmt.Errorf("closing allocated listener: %w", err)
	}
	return tcpAddr.Port, nil
}

// waitForZe waits for Ze to start listening on addr.
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

// PeerFamilyTargets computes per-peer per-family expected route counts from profiles.
func PeerFamilyTargets(profiles []scenario.PeerProfile) map[int]map[string]int {
	targets := make(map[int]map[string]int, len(profiles))
	for i := range profiles {
		fm := make(map[string]int, len(profiles[i].Families))
		for _, fam := range profiles[i].Families {
			if strings.Contains(fam, "unicast") {
				fm[fam] = profiles[i].RouteCount
			} else {
				fm[fam] = profiles[i].RouteCount / 4
			}
		}
		targets[profiles[i].Index] = fm
	}
	return targets
}
