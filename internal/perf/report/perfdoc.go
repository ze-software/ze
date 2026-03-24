// Design: (none -- new tool, predates documentation)
package report

import (
	"fmt"
	"io"
	"runtime"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/perf"
)

// PerformanceDoc writes a full docs/performance.md document with disclaimers,
// environment details, and comparison tables from benchmark results.
func PerformanceDoc(results []perf.Result, w io.Writer) error {
	if _, err := fmt.Fprint(w, `# Performance Comparison

> **All benchmarks are lies.**
>
> These numbers measure one specific scenario (route propagation latency through
> a single DUT with two peers) on one specific machine under artificial conditions.
> They do not predict real-world performance. Different hardware, different route
> counts, different address families, different policies, different network
> conditions will all produce different results.
>
> Use these numbers to understand *relative* differences between implementations,
> not as absolute performance claims. If performance matters to you, run ze-perf
> on your own hardware with your own workload.

## Methodology

Ze-perf establishes two BGP sessions with a device under test (DUT): a sender
and a receiver. The sender injects routes and records when each was sent. The
receiver parses incoming UPDATEs and records when each prefix arrived.
Propagation latency = time received minus time sent, matched by prefix.

Each benchmark runs multiple iterations. Results show the **median** across
iterations with standard deviation. Outlier iterations (beyond 2 stddev from
median convergence time) are automatically discarded.

`); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	// Environment section from the first result's metadata.
	if _, err := fmt.Fprintf(w, "## Environment\n\n"); err != nil {
		return fmt.Errorf("writing env header: %w", err)
	}

	if _, err := fmt.Fprintf(w, "| Field | Value |\n|-------|-------|\n"); err != nil {
		return fmt.Errorf("writing env table header: %w", err)
	}

	if _, err := fmt.Fprintf(w, "| Platform | %s/%s |\n", runtime.GOOS, runtime.GOARCH); err != nil {
		return fmt.Errorf("writing platform: %w", err)
	}

	if _, err := fmt.Fprintf(w, "| Virtualization | Docker (Colima VM) |\n"); err != nil {
		return fmt.Errorf("writing virtualization: %w", err)
	}

	if _, err := fmt.Fprintf(w, "| Date | %s |\n", time.Now().Format("2006-01-02")); err != nil {
		return fmt.Errorf("writing date: %w", err)
	}

	if len(results) > 0 {
		r := results[0]
		if _, err := fmt.Fprintf(w, "| Routes | %s |\n", fmtNum(r.Routes)); err != nil {
			return fmt.Errorf("writing routes: %w", err)
		}

		if _, err := fmt.Fprintf(w, "| Seed | %d |\n", r.Seed); err != nil {
			return fmt.Errorf("writing seed: %w", err)
		}

		if _, err := fmt.Fprintf(w, "| Iterations | %d measured, %d warmup |\n", r.Repeat, r.WarmupRuns); err != nil {
			return fmt.Errorf("writing iterations: %w", err)
		}
	}

	if _, err := fmt.Fprintf(w, "\n**These results were collected on a development laptop using Docker "+
		"containers via Colima. A dedicated server with bare-metal networking would produce "+
		"different (likely faster and more consistent) numbers.**\n\n"); err != nil {
		return fmt.Errorf("writing caveat: %w", err)
	}

	// Results tables.
	if _, err := fmt.Fprintf(w, "## Results\n\n"); err != nil {
		return fmt.Errorf("writing results header: %w", err)
	}

	groups := groupResults(results)
	keys := sortedGroupKeys(groups)

	for _, key := range keys {
		header := fmt.Sprintf("### %s", key.Family)
		if key.ForceMP {
			header += " (force-MP)"
		}

		if _, err := fmt.Fprintln(w, header); err != nil {
			return fmt.Errorf("writing group header: %w", err)
		}

		if _, err := fmt.Fprintln(w); err != nil {
			return fmt.Errorf("writing newline: %w", err)
		}

		grouped := groups[key]
		sortByConvergence(grouped)

		if _, err := fmt.Fprintln(w, "| DUT | Convergence | +/- | Throughput (r/s) | +/- | p50 | p99 | +/- | Max | Lost |"); err != nil {
			return fmt.Errorf("writing table header: %w", err)
		}

		if _, err := fmt.Fprintln(w, "|-----|-------------|-----|------------------|-----|-----|-----|-----|-----|------|"); err != nil {
			return fmt.Errorf("writing table sep: %w", err)
		}

		for j := range grouped {
			r := &grouped[j]
			line := fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s | %s | %s | %d |",
				r.DUTName,
				fmtMs(r.ConvergenceMs),
				fmtMs(r.ConvergenceStddevMs),
				fmtNum(r.ThroughputAvg),
				fmtNum(r.ThroughputAvgStddev),
				fmtMs(r.LatencyP50Ms),
				fmtMs(r.LatencyP99Ms),
				fmtMs(r.LatencyP99StddevMs),
				fmtMs(r.LatencyMaxMs),
				r.RoutesLost,
			)
			if _, err := fmt.Fprintln(w, line); err != nil {
				return fmt.Errorf("writing row: %w", err)
			}
		}

		if _, err := fmt.Fprintln(w); err != nil {
			return fmt.Errorf("writing trailing newline: %w", err)
		}
	}

	// Interpretation guide.
	if _, err := fmt.Fprint(w, `## Reading the Results

**Convergence** is the time from the first UPDATE sent to the last UPDATE
received. Lower is better. This is the primary metric -- it answers "how long
until all routes are propagated?"

**Throughput** is routes received per second, averaged over the convergence
window. Higher is better. Zero means all routes arrived in a single burst
(sub-second convergence with coalesced TCP delivery).

**p50/p99** are per-route latency percentiles. p50 is the median route's
latency; p99 is the slowest 1%. The gap between p50 and p99 shows how
consistent the DUT's forwarding is.

**+/-** columns show standard deviation across iterations. Small stddev means
consistent performance; large stddev means the measurement is noisy.

**Lost** should always be zero. Any lost routes indicate the DUT failed to
forward some prefixes.

## Reproducing

`+"```bash\n"+`make ze-perf
python3 test/perf/run.py
`+"```\n"+`
Requires Docker. See [Benchmarking Guide](guide/benchmarking.md) for details.
`); err != nil {
		return fmt.Errorf("writing interpretation: %w", err)
	}

	return nil
}
