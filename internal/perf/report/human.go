// Design: docs/architecture/system-architecture.md -- human-readable perf result output

package report

import (
	"encoding/json"
	"fmt"
	"io"

	"codeberg.org/thomas-mangin/ze/internal/perf"
)

// PrintHuman writes a human-readable benchmark summary to w.
func PrintHuman(w io.Writer, r *perf.Result) {
	fmt.Fprintf(w, "DUT: %s", r.DUTName)

	if r.DUTVersion != "" {
		fmt.Fprintf(w, " %s", r.DUTVersion)
	}

	fmt.Fprintf(w, " @ %s:%d (AS %d)\n", r.DUTAddr, r.DUTPort, r.DUTASN)
	fmt.Fprintf(w, "Routes: %d %s (seed %d)\n", r.Routes, r.Family, r.Seed)
	fmt.Fprintf(w, "Iterations: %d measured, %d warmup, %d kept\n", r.Repeat, r.WarmupRuns, r.RepeatKept)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Sessions:")
	fmt.Fprintf(w, "  sender   -> established in %dms\n", r.SessionSetupMs.Sender)
	fmt.Fprintf(w, "  receiver -> established in %dms\n", r.SessionSetupMs.Receiver)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Propagation:")
	fmt.Fprintf(w, "  first route:   %dms\n", r.FirstRouteMs)

	if r.ConvergenceStddevMs > 0 {
		fmt.Fprintf(w, "  convergence:   %dms +/- %dms\n", r.ConvergenceMs, r.ConvergenceStddevMs)
	} else {
		fmt.Fprintf(w, "  convergence:   %dms\n", r.ConvergenceMs)
	}

	if r.ThroughputAvgStddev > 0 {
		fmt.Fprintf(w, "  throughput:    %d routes/sec +/- %d\n", r.ThroughputAvg, r.ThroughputAvgStddev)
	} else {
		fmt.Fprintf(w, "  throughput:    %d routes/sec\n", r.ThroughputAvg)
	}

	fmt.Fprintf(w, "  routes:        %d/%d (%d lost)\n", r.RoutesReceived, r.RoutesSent, r.RoutesLost)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Latency distribution:")
	fmt.Fprintf(w, "  p50:   %dms\n", r.LatencyP50Ms)
	fmt.Fprintf(w, "  p90:   %dms\n", r.LatencyP90Ms)

	if r.LatencyP99StddevMs > 0 {
		fmt.Fprintf(w, "  p99:   %dms +/- %dms\n", r.LatencyP99Ms, r.LatencyP99StddevMs)
	} else {
		fmt.Fprintf(w, "  p99:   %dms\n", r.LatencyP99Ms)
	}

	fmt.Fprintf(w, "  max:   %dms\n", r.LatencyMaxMs)

	if r.WithdrawalMs > 0 {
		fmt.Fprintf(w, "\nWithdrawal:\n")
		if r.WithdrawalStddevMs > 0 {
			fmt.Fprintf(w, "  time:  %dms +/- %dms\n", r.WithdrawalMs, r.WithdrawalStddevMs)
		} else {
			fmt.Fprintf(w, "  time:  %dms\n", r.WithdrawalMs)
		}
	}
}

// WriteJSON marshals the result to indented JSON and writes to w.
func WriteJSON(w io.Writer, r *perf.Result) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}

	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}
