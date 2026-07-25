// Design: docs/architecture/system-architecture.md -- human-readable perf result output

package report

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/ze-software/ze/internal/perf"
)

// PrintHuman writes a human-readable benchmark summary to w.
func PrintHuman(w io.Writer, r *perf.Result) {
	fmt.Fprintf(w, "DUT: %s", r.DUTName) //nolint:errcheck // report output

	if r.DUTVersion != "" {
		fmt.Fprintf(w, " %s", r.DUTVersion) //nolint:errcheck // report output
	}

	fmt.Fprintf(w, " @ %s:%d (AS %d)\n", r.DUTAddr, r.DUTPort, r.DUTASN)                                  //nolint:errcheck // report output
	fmt.Fprintf(w, "Routes: %d %s (seed %d)\n", r.Routes, r.Family, r.Seed)                               //nolint:errcheck // report output
	fmt.Fprintf(w, "Iterations: %d measured, %d warmup, %d kept\n", r.Repeat, r.WarmupRuns, r.RepeatKept) //nolint:errcheck // report output

	fmt.Fprintln(w)                                                                  //nolint:errcheck // report output
	fmt.Fprintln(w, "Sessions:")                                                     //nolint:errcheck // report output
	fmt.Fprintf(w, "  sender   -> established in %dms\n", r.SessionSetupMs.Sender)   //nolint:errcheck // report output
	fmt.Fprintf(w, "  receiver -> established in %dms\n", r.SessionSetupMs.Receiver) //nolint:errcheck // report output

	fmt.Fprintln(w)                                           //nolint:errcheck // report output
	fmt.Fprintln(w, "Propagation:")                           //nolint:errcheck // report output
	fmt.Fprintf(w, "  first route:   %dms\n", r.FirstRouteMs) //nolint:errcheck // report output

	if r.ConvergenceStddevMs > 0 {
		fmt.Fprintf(w, "  convergence:   %dms +/- %dms\n", r.ConvergenceMs, r.ConvergenceStddevMs) //nolint:errcheck // report output
	} else {
		fmt.Fprintf(w, "  convergence:   %dms\n", r.ConvergenceMs) //nolint:errcheck // report output
	}

	if r.ThroughputAvgStddev > 0 {
		fmt.Fprintf(w, "  throughput:    %d routes/sec +/- %d\n", r.ThroughputAvg, r.ThroughputAvgStddev) //nolint:errcheck // report output
	} else {
		fmt.Fprintf(w, "  throughput:    %d routes/sec\n", r.ThroughputAvg) //nolint:errcheck // report output
	}

	fmt.Fprintf(w, "  routes:        %d/%d (%d lost)\n", r.RoutesReceived, r.RoutesSent, r.RoutesLost) //nolint:errcheck // report output

	fmt.Fprintln(w)                                   //nolint:errcheck // report output
	fmt.Fprintln(w, "Latency distribution:")          //nolint:errcheck // report output
	fmt.Fprintf(w, "  p50:   %dms\n", r.LatencyP50Ms) //nolint:errcheck // report output
	fmt.Fprintf(w, "  p90:   %dms\n", r.LatencyP90Ms) //nolint:errcheck // report output

	if r.LatencyP99StddevMs > 0 {
		fmt.Fprintf(w, "  p99:   %dms +/- %dms\n", r.LatencyP99Ms, r.LatencyP99StddevMs) //nolint:errcheck // report output
	} else {
		fmt.Fprintf(w, "  p99:   %dms\n", r.LatencyP99Ms) //nolint:errcheck // report output
	}

	fmt.Fprintf(w, "  max:   %dms\n", r.LatencyMaxMs) //nolint:errcheck // report output

	if r.WithdrawalMs > 0 {
		fmt.Fprintf(w, "\nWithdrawal:\n") //nolint:errcheck // report output
		if r.WithdrawalStddevMs > 0 {
			fmt.Fprintf(w, "  time:  %dms +/- %dms\n", r.WithdrawalMs, r.WithdrawalStddevMs) //nolint:errcheck // report output
		} else {
			fmt.Fprintf(w, "  time:  %dms\n", r.WithdrawalMs) //nolint:errcheck // report output
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
