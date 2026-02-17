// Package report provides the exit summary for ze-bgp-chaos validation runs.
package report

import (
	"fmt"
	"io"
	"time"
)

// PropertyLine holds per-property pass/fail for the summary.
type PropertyLine struct {
	Name string
	Pass bool
}

// Summary holds all metrics for the final exit report.
type Summary struct {
	Seed      uint64
	Duration  time.Duration
	PeerCount int
	IBGPCount int
	EBGPCount int

	// Route counts.
	Announced int
	Received  int
	Missing   int
	Extra     int

	// Convergence latency stats.
	MinLatency time.Duration
	AvgLatency time.Duration
	MaxLatency time.Duration
	P99Latency time.Duration
	SlowRoutes int

	// Chaos injection stats.
	ChaosEvents   int
	Reconnections int
	Withdrawn     int

	// Property check results (empty when --properties not used).
	Properties []PropertyLine
}

// Pass returns true when there are no validation failures.
// A run with zero announced routes is always a failure — it means
// no peers connected and the test exercised nothing.
func (s *Summary) Pass() bool {
	if s.Announced == 0 {
		return false
	}
	if s.Missing != 0 || s.Extra != 0 || s.SlowRoutes != 0 {
		return false
	}
	for _, p := range s.Properties {
		if !p.Pass {
			return false
		}
	}
	return true
}

// reportWriter wraps an io.Writer and tracks the first error.
type reportWriter struct {
	w   io.Writer
	err error
}

func (rw *reportWriter) printf(format string, args ...any) {
	if rw.err != nil {
		return
	}
	_, rw.err = fmt.Fprintf(rw.w, format, args...)
}

// Write prints the summary to w and returns the exit code (0=pass, 1=fail).
func (s *Summary) Write(w io.Writer) int {
	verdict := "PASS"
	exitCode := 0
	if !s.Pass() {
		verdict = "FAIL"
		exitCode = 1
	}

	rw := &reportWriter{w: w}

	rw.printf("── ze-bgp-chaos ──────────────────────────\n")
	rw.printf("  seed:  %d\n", s.Seed) //nolint:gosec // seed is display-only
	if s.IBGPCount > 0 && s.EBGPCount > 0 {
		rw.printf("  run:   %s, %d peers (%d iBGP, %d eBGP)\n", s.Duration, s.PeerCount, s.IBGPCount, s.EBGPCount)
	} else {
		rw.printf("  run:   %s, %d peers\n", s.Duration, s.PeerCount)
	}
	rw.printf("  routes: %d announced, %d received, %d missing, %d extra",
		s.Announced, s.Received, s.Missing, s.Extra)
	if s.SlowRoutes > 0 {
		rw.printf(", %d slow", s.SlowRoutes)
	}
	rw.printf("\n")

	if s.Announced > 0 {
		rw.printf("  latency: min=%s avg=%s max=%s p99=%s\n",
			formatDuration(s.MinLatency),
			formatDuration(s.AvgLatency),
			formatDuration(s.MaxLatency),
			formatDuration(s.P99Latency))
	}

	if s.ChaosEvents > 0 {
		rw.printf("  chaos: %d events, %d reconnections, %d withdrawn\n",
			s.ChaosEvents, s.Reconnections, s.Withdrawn)
	}

	if len(s.Properties) > 0 {
		rw.printf("  properties:\n")
		for _, p := range s.Properties {
			status := "PASS"
			if !p.Pass {
				status = "FAIL"
			}
			rw.printf("    %-25s %s\n", p.Name, status)
		}
	}

	rw.printf("  result: %s\n", verdict)
	rw.printf("──────────────────────────────────────────\n")

	if rw.err != nil {
		return 1
	}
	return exitCode
}

// formatDuration formats a duration in a compact human-readable form.
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0"
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return d.Truncate(time.Millisecond).String()
}
