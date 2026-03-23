package perf

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// VALIDATES: Result struct marshals to JSON with kebab-case keys and round-trips correctly.
// PREVENTS: Wrong JSON field names breaking interop with external tools.
func TestJSONResult(t *testing.T) {
	t.Run("round-trip", func(t *testing.T) {
		orig := Result{
			DUTName:             "ze",
			DUTVersion:          "1.0.0",
			DUTAddr:             "127.0.0.1",
			DUTPort:             179,
			DUTASN:              65000,
			Routes:              1000,
			Family:              "ipv4/unicast",
			ForceMP:             true,
			Seed:                42,
			Timestamp:           "2026-03-22T10:00:00Z",
			Repeat:              5,
			RepeatKept:          4,
			WarmupRuns:          1,
			IterDelayMs:         500,
			SessionSetupMs:      SessionSetup{Sender: 10, Receiver: 20},
			FirstRouteMs:        15,
			ConvergenceMs:       1200,
			ConvergenceStddevMs: 50,
			RoutesSent:          1000,
			RoutesReceived:      998,
			RoutesLost:          2,
			ThroughputAvg:       5000,
			ThroughputAvgStddev: 200,
			ThroughputPeak:      7500,
			LatencyP50Ms:        5,
			LatencyP90Ms:        12,
			LatencyP99Ms:        25,
			LatencyP99StddevMs:  3,
			LatencyMaxMs:        30,
		}

		data, err := json.Marshal(orig)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		var decoded Result
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if decoded != orig {
			t.Errorf("round-trip mismatch:\n  got:  %+v\n  want: %+v", decoded, orig)
		}
	})

	t.Run("kebab-case keys", func(t *testing.T) {
		r := Result{
			DUTName:        "ze",
			DUTVersion:     "1.0.0",
			DUTAddr:        "127.0.0.1",
			DUTPort:        179,
			DUTASN:         65000,
			Routes:         1000,
			Family:         "ipv4/unicast",
			SessionSetupMs: SessionSetup{Sender: 10, Receiver: 20},
		}

		data, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		raw := string(data)

		expectedKeys := []string{
			`"dut-name"`,
			`"dut-version"`,
			`"dut-addr"`,
			`"dut-port"`,
			`"dut-asn"`,
			`"routes"`,
			`"family"`,
			`"force-mp"`,
			`"seed"`,
			`"timestamp"`,
			`"repeat"`,
			`"repeat-kept"`,
			`"warmup-runs"`,
			`"iter-delay-ms"`,
			`"session-setup-ms"`,
			`"first-route-ms"`,
			`"convergence-ms"`,
			`"convergence-stddev-ms"`,
			`"routes-sent"`,
			`"routes-received"`,
			`"routes-lost"`,
			`"throughput-avg"`,
			`"throughput-avg-stddev"`,
			`"throughput-peak"`,
			`"latency-p50-ms"`,
			`"latency-p90-ms"`,
			`"latency-p99-ms"`,
			`"latency-p99-stddev-ms"`,
			`"latency-max-ms"`,
		}

		for _, key := range expectedKeys {
			if !strings.Contains(raw, key) {
				t.Errorf("missing kebab-case key %s in JSON output:\n%s", key, raw)
			}
		}

		// Verify nested session-setup-ms has kebab-case keys too.
		if !strings.Contains(raw, `"sender"`) {
			t.Errorf("missing sender key in session-setup-ms")
		}
		if !strings.Contains(raw, `"receiver"`) {
			t.Errorf("missing receiver key in session-setup-ms")
		}
	})
}

// VALIDATES: NDJSON read/write round-trip works for multiple results.
// PREVENTS: Corrupt or mis-parsed performance result files.
func TestNDJSONParsing(t *testing.T) {
	t.Run("write and read back", func(t *testing.T) {
		results := []Result{
			{DUTName: "ze", DUTVersion: "1.0", Routes: 100, ConvergenceMs: 500},
			{DUTName: "ze", DUTVersion: "1.1", Routes: 200, ConvergenceMs: 600},
			{DUTName: "ze", DUTVersion: "1.2", Routes: 300, ConvergenceMs: 700},
		}

		var buf bytes.Buffer
		for _, r := range results {
			if err := WriteNDJSON(&buf, r); err != nil {
				t.Fatalf("WriteNDJSON: %v", err)
			}
		}

		parsed, err := ReadNDJSON(&buf)
		if err != nil {
			t.Fatalf("ReadNDJSON: %v", err)
		}

		if len(parsed) != 3 {
			t.Fatalf("expected 3 results, got %d", len(parsed))
		}

		for i, r := range parsed {
			if r.DUTName != results[i].DUTName {
				t.Errorf("result[%d] DUTName: got %q, want %q", i, r.DUTName, results[i].DUTName)
			}
			if r.Routes != results[i].Routes {
				t.Errorf("result[%d] Routes: got %d, want %d", i, r.Routes, results[i].Routes)
			}
			if r.ConvergenceMs != results[i].ConvergenceMs {
				t.Errorf("result[%d] ConvergenceMs: got %d, want %d", i, r.ConvergenceMs, results[i].ConvergenceMs)
			}
		}
	})

	t.Run("empty input", func(t *testing.T) {
		parsed, err := ReadNDJSON(strings.NewReader(""))
		if err != nil {
			t.Fatalf("ReadNDJSON on empty: %v", err)
		}
		if len(parsed) != 0 {
			t.Errorf("expected 0 results from empty input, got %d", len(parsed))
		}
	})

	t.Run("blank lines skipped", func(t *testing.T) {
		var buf bytes.Buffer
		if err := WriteNDJSON(&buf, Result{DUTName: "ze", Routes: 100}); err != nil {
			t.Fatal(err)
		}
		buf.WriteString("\n") // blank line
		if err := WriteNDJSON(&buf, Result{DUTName: "ze", Routes: 200}); err != nil {
			t.Fatal(err)
		}

		parsed, err := ReadNDJSON(&buf)
		if err != nil {
			t.Fatalf("ReadNDJSON: %v", err)
		}
		if len(parsed) != 2 {
			t.Fatalf("expected 2 results, got %d", len(parsed))
		}
	})

	t.Run("malformed line returns error", func(t *testing.T) {
		input := `{"dut-name":"ze","routes":100}
not valid json
{"dut-name":"ze","routes":200}
`
		_, err := ReadNDJSON(strings.NewReader(input))
		if err == nil {
			t.Error("expected error on malformed JSON line, got nil")
		}
	})
}
