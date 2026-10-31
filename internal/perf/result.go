// Design: (none -- new tool, predates documentation)
// Overview: benchmark.go -- benchmark orchestration producing Result

package perf

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// SessionSetup holds session establishment timing in milliseconds.
type SessionSetup struct {
	Sender   int `json:"sender"`
	Receiver int `json:"receiver"`
}

// Result holds the outcome of a single performance test run.
type Result struct {
	DUTName             string       `json:"dut-name"`
	DUTVersion          string       `json:"dut-version"`
	DUTAddr             string       `json:"dut-addr"`
	DUTPort             int          `json:"dut-port"`
	DUTASN              int          `json:"dut-asn"`
	Routes              int          `json:"routes"`
	Family              string       `json:"family"`
	ForceMP             bool         `json:"force-mp"`
	Seed                uint64       `json:"seed"`
	Timestamp           string       `json:"timestamp"`
	Repeat              int          `json:"repeat"`
	RepeatKept          int          `json:"repeat-kept"`
	WarmupRuns          int          `json:"warmup-runs"`
	IterDelayMs         int          `json:"iter-delay-ms"`
	SessionSetupMs      SessionSetup `json:"session-setup-ms"`
	FirstRouteMs        int          `json:"first-route-ms"`
	ConvergenceMs       int          `json:"convergence-ms"`
	ConvergenceStddevMs int          `json:"convergence-stddev-ms"`
	RoutesSent          int          `json:"routes-sent"`
	RoutesReceived      int          `json:"routes-received"`
	RoutesLost          int          `json:"routes-lost"`
	ThroughputAvg       int          `json:"throughput-avg"`
	ThroughputAvgStddev int          `json:"throughput-avg-stddev"`
	ThroughputPeak      int          `json:"throughput-peak"`
	LatencyP50Ms        int          `json:"latency-p50-ms"`
	LatencyP90Ms        int          `json:"latency-p90-ms"`
	LatencyP99Ms        int          `json:"latency-p99-ms"`
	LatencyP99StddevMs  int          `json:"latency-p99-stddev-ms"`
	LatencyMaxMs        int          `json:"latency-max-ms"`
	WithdrawalMs        int          `json:"withdrawal-ms"`
	WithdrawalStddevMs  int          `json:"withdrawal-stddev-ms"`
}

// Shortfall answers nil when every route the sender announced reached the
// receiver, and otherwise an error naming how many routes of how many arrived.
// A result that lost a route measured a DUT that did not do the work, so its
// timings describe the routes that arrived, or nothing: it is not a
// measurement of forwarding. It is the one judgement of a complete result:
// `le perf send` exits 1 on it, and `le perf run` fails the DUT on it.
func (r *Result) Shortfall() error {
	if r.RoutesSent <= 0 {
		return errors.New("the sender announced no route, so nothing was measured")
	}
	if r.RoutesReceived < r.RoutesSent {
		return fmt.Errorf("%d of %d routes reached the receiver", r.RoutesReceived, r.RoutesSent)
	}
	return nil
}

// ReadNDJSON reads newline-delimited JSON results from the reader.
// Blank lines are skipped. Returns an error on malformed JSON.
func ReadNDJSON(r io.Reader) ([]Result, error) {
	var results []Result

	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		var res Result
		if err := json.Unmarshal([]byte(line), &res); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}

		results = append(results, res)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading NDJSON: %w", err)
	}

	return results, nil
}

// WriteNDJSON writes a single Result as one JSON line to the writer.
//
//nolint:gocritic // hugeParam: Result is passed by value for API simplicity; not a hot path.
func WriteNDJSON(w io.Writer, r Result) error {
	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshaling result: %w", err)
	}

	data = append(data, '\n')

	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("writing result: %w", err)
	}

	return nil
}
