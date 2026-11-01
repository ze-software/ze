package perf_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/perf"
)

// VALIDATES: AC-6 "le perf send with -sender-first, and without it: the result
// JSON carries "pass": "rib-propagation", and "pass": "live-forwarding",
// respectively".
// METHOD: run RunBenchmark against the in-process test forwarder once per
// pass, write each result as the NDJSON line `le perf send --output` writes,
// and read the "pass" key back out of the raw JSON object.
// PREVENTS: a sender-first result that is indistinguishable from a
// live-forwarding one, which let the comparison show the propagation pass
// under the bare DUT name.
func TestRunResultRecordsPass(t *testing.T) {
	cases := []struct {
		name        string
		senderFirst bool
		want        perf.Pass
		wantJSON    string
	}{
		{"live forwarding", false, perf.PassLiveForwarding, "live-forwarding"},
		{"sender first", true, perf.PassRIBPropagation, "rib-propagation"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()

			fwd := perf.NewTestForwarder(t)
			go fwd.Run(ctx)

			host, portText, err := net.SplitHostPort(fwd.Addr())
			if err != nil {
				t.Fatalf("splitting forwarder address %q: %v", fwd.Addr(), err)
			}
			port, err := strconv.Atoi(portText)
			if err != nil {
				t.Fatalf("parsing port %q: %v", portText, err)
			}

			cfg := perf.BenchmarkConfig{
				DUTAddr:        host,
				DUTPort:        port,
				DUTASN:         65000,
				DUTName:        "test-forwarder",
				SenderAddr:     "127.0.0.1",
				SenderASN:      65001,
				ReceiverAddr:   "127.0.0.1",
				ReceiverASN:    65002,
				Routes:         10,
				Family:         "ipv4/unicast",
				Seed:           42,
				Warmup:         100 * time.Millisecond,
				ConnectTimeout: 5 * time.Second,
				Duration:       10 * time.Second,
				Repeat:         1,
				SenderFirst:    tc.senderFirst,
			}

			result, err := perf.RunBenchmark(ctx, cfg, io.Discard)
			if err != nil {
				t.Fatalf("RunBenchmark: %v", err)
			}
			if result.Pass != tc.want {
				t.Errorf("Result.Pass = %v, want %v", result.Pass, tc.want)
			}

			var line bytes.Buffer
			if err := perf.WriteNDJSON(&line, result); err != nil {
				t.Fatalf("WriteNDJSON: %v", err)
			}
			var object map[string]any
			if err := json.Unmarshal(line.Bytes(), &object); err != nil {
				t.Fatalf("the result line does not parse: %v\n%s", err, line.String())
			}
			if got := object["pass"]; got != tc.wantJSON {
				t.Errorf(`result JSON "pass" = %v, want %q: %s`, got, tc.wantJSON, strings.TrimSpace(line.String()))
			}

			cancel()
		})
	}
}

// VALIDATES: the security review row "Result JSON with an unknown pass value
// is refused with the value and the accepted values"; a missing field reads
// as PassUnrecorded and a recorded value reads back as itself.
// METHOD: unmarshal result objects carrying each shape of the "pass" key.
// PREVENTS: a mistyped or future pass value read silently as a known pass or
// as no pass.
func TestResultPassJSON(t *testing.T) {
	var missing perf.Result
	if err := json.Unmarshal([]byte(`{"dut-name":"ze"}`), &missing); err != nil {
		t.Fatalf("a result with no pass field: %v", err)
	}
	if missing.Pass != perf.PassUnrecorded {
		t.Errorf("missing pass field read as %v, want %v", missing.Pass, perf.PassUnrecorded)
	}
	if got, want := missing.Pass.String(), "not recorded"; got != want {
		t.Errorf("PassUnrecorded.String() = %q, want %q", got, want)
	}

	var recorded perf.Result
	if err := json.Unmarshal([]byte(`{"dut-name":"ze","pass":"rib-propagation"}`), &recorded); err != nil {
		t.Fatalf("a result with a known pass: %v", err)
	}
	if recorded.Pass != perf.PassRIBPropagation {
		t.Errorf("pass read as %v, want %v", recorded.Pass, perf.PassRIBPropagation)
	}

	for _, value := range []string{`"sender-first"`, `""`} {
		var unknown perf.Result
		err := json.Unmarshal([]byte(`{"dut-name":"ze","pass":`+value+`}`), &unknown)
		if err == nil {
			t.Fatalf("pass %s was accepted, want it refused", value)
		}
		for _, want := range []string{value, "live-forwarding", "rib-propagation"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refusal of pass %s = %q, lacks %q", value, err.Error(), want)
			}
		}
	}

	data, err := json.Marshal(perf.Result{DUTName: "ze"})
	if err != nil {
		t.Fatalf("marshal a result with no pass: %v", err)
	}
	if strings.Contains(string(data), `"pass"`) {
		t.Errorf("a result with no pass wrote a pass key: %s", data)
	}
}

// VALIDATES: only a named pass has a result file name.
// METHOD: ask each pass value, named and unnamed, for its file name.
// PREVENTS: PassUnrecorded or a stray value silently given "<dut>.json", the
// live-forwarding name.
func TestPassResultFileRefusesAnUnnamedPass(t *testing.T) {
	for _, pass := range []perf.Pass{perf.PassUnrecorded, perf.Pass(200)} {
		name, err := pass.ResultFile("ze")
		if err == nil {
			t.Errorf("%v.ResultFile(ze) = %q, want a refusal", pass, name)
			continue
		}
		for _, want := range []string{"live-forwarding", "rib-propagation"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refusal %q lacks %q", err.Error(), want)
			}
		}
	}
	for pass, want := range map[perf.Pass]string{perf.PassLiveForwarding: "ze.json", perf.PassRIBPropagation: "ze-propagation.json"} {
		if got, err := pass.ResultFile("ze"); err != nil || got != want {
			t.Errorf("%v.ResultFile(ze) = %q, %v, want %q", pass, got, err, want)
		}
	}
}
