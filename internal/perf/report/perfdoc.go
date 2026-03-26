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

	// DUT setup section.
	if err := writeDUTSetup(results, w); err != nil {
		return fmt.Errorf("writing DUT setup: %w", err)
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

// dutSetupNotes maps DUT names to their configuration summaries for the performance doc.
// Each entry describes the BGP-relevant settings used in the benchmark config files
// under test/perf/configs/.
var dutSetupNotes = map[string]string{
	"ze": `**Ze** -- Go BGP daemon, goroutine-based, kernel TCP stack.
  Config: passive peers, route-reflector plugin (bgp-rs), 1M prefix limit per family.
  Transport: kernel TCP (standard Docker networking).`,

	"frr": `**FRR** (Free Range Routing) -- C BGP daemon, kernel TCP stack.
  Config: passive peers, PERMIT route-maps in/out (no filtering).
  Transport: kernel TCP (standard Docker networking).`,

	"bird": `**BIRD** -- C BGP daemon, kernel TCP stack.
  Config: passive peers, import/export all (no filtering).
  Transport: kernel TCP (standard Docker networking).`,

	"gobgp": `**GoBGP** -- Go BGP daemon, kernel TCP stack.
  Config: passive peers, default accept policy.
  Transport: kernel TCP (standard Docker networking).`,

	"rustbgpd": `**rustbgpd** -- Rust BGP daemon, kernel TCP stack.
  Config: passive peers, default policy.
  Transport: kernel TCP (standard Docker networking).`,

	"rustybgp": `**RustyBGP** -- Rust BGP daemon, kernel TCP stack.
  Config: passive peers, default policy.
  Transport: kernel TCP (standard Docker networking).`,

	"freertr": `**freeRtr** -- Java BGP daemon with its own TCP/IP stack.
  Config: passive peers, 256KB buffer-size, extended-update enabled,
  advertisement-interval-tx 0, incremental bestpath (1M limit), no safe-ebgp.
  JVM: 2GB heap with ZGC (low-pause garbage collector).
  Transport: rawInt bridge (UDP encapsulation between Docker eth0 and freeRtr's
  virtual interface layer) -- adds latency vs kernel TCP used by other DUTs.`,
}

// writeDUTSetup writes a "DUT Setup" section describing the configuration of each
// DUT that appears in the results.
func writeDUTSetup(results []perf.Result, w io.Writer) error {
	// Collect unique DUT names in result order.
	seen := make(map[string]bool)
	var names []string

	for i := range results {
		name := results[i].DUTName
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}

	if len(names) == 0 {
		return nil
	}

	if _, err := fmt.Fprintf(w, "## DUT Setup\n\n"); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "All DUTs run in Docker containers on the same host. Each DUT is\n"+
		"configured with two passive BGP peers (sender AS 65001, receiver AS 65002)\n"+
		"and AS 65000 as the local AS. The benchmark tool (ze-perf) establishes both\n"+
		"sessions, injects routes via the sender, and measures when they arrive at\n"+
		"the receiver.\n\n"); err != nil {
		return err
	}

	for _, name := range names {
		note, ok := dutSetupNotes[name]
		if !ok {
			note = fmt.Sprintf("**%s** -- no setup notes available.", name)
		}

		if _, err := fmt.Fprintf(w, "- %s\n", note); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(w, "\nConfig files: `test/perf/configs/`\n\n"); err != nil {
		return err
	}

	return nil
}
