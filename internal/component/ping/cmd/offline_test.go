package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// silenceStderr redirects os.Stderr to /dev/null for the duration of
// the test. Restores on cleanup.
func silenceStderr(t *testing.T) {
	t.Helper()
	old := os.Stderr
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open devnull: %v", err)
	}
	os.Stderr = devnull
	t.Cleanup(func() {
		os.Stderr = old
		if err := devnull.Close(); err != nil {
			t.Logf("close devnull: %v", err)
		}
	})
}

// TestRunPing_ValidationErrors covers every validation path that
// returns 1 before sending ICMP.
func TestRunPing_ValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"no target", []string{}},
		{"two targets", []string{"1.1.1.1", "2.2.2.2"}},
		{"unknown flag", []string{"--evil", "1.1.1.1"}},
		{"count negative", []string{"--count", "-1", "1.1.1.1"}},
		{"count zero", []string{"--count", "0", "1.1.1.1"}},
		{"count too large", []string{"--count", "100001", "1.1.1.1"}},
		{"count non-int", []string{"--count", "abc", "1.1.1.1"}},
		{"source not ip", []string{"--source", "not-ip", "1.1.1.1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			silenceStderr(t)
			if rc := RunPing(tc.args); rc != 1 {
				t.Errorf("RunPing(%v) = %d, want 1", tc.args, rc)
			}
		})
	}
}

// TestRunPing_Help asserts that -h exits 0 (flag.ErrHelp handled).
func TestRunPing_Help(t *testing.T) {
	silenceStderr(t)
	if rc := RunPing([]string{"-h"}); rc != 0 {
		t.Errorf("RunPing(-h) = %d, want 0", rc)
	}
}

// TestRunPing_ShellMetaTarget rejects targets with shell metacharacters
// before they reach DNS resolution.
//
// PREVENTS: garbage targets triggering slow DNS lookups.
func TestRunPing_ShellMetaTarget(t *testing.T) {
	bad := []string{
		"a;rm -rf /",
		"$(echo x)",
		"`id`",
		"host|cat",
		"host with space",
	}
	for _, target := range bad {
		t.Run(target, func(t *testing.T) {
			silenceStderr(t)
			if rc := RunPing([]string{target}); rc != 1 {
				t.Errorf("RunPing(%q) = %d, want 1", target, rc)
			}
		})
	}
}

// TestPrintPingResults verifies the human-readable output format for
// both success and all-timeout cases.
//
// PREVENTS: type-assertion panics and format regressions in printPingResults.
func TestPrintPingResults(t *testing.T) {
	t.Run("with replies", func(t *testing.T) {
		results := map[string]any{
			"destination":  "10.0.0.1",
			"sent":         3,
			"received":     2,
			"loss-percent": 33.3,
			"min-rtt-ms":   1.5,
			"avg-rtt-ms":   2.0,
			"max-rtt-ms":   2.5,
			"replies": []map[string]any{
				{"seq": 0, "status": "ok", "rtt-ms": 1.5},
				{"seq": 1, "status": "timeout"},
				{"seq": 2, "status": "ok", "rtt-ms": 2.5},
			},
		}
		var buf bytes.Buffer
		printPingResults(&buf, results)
		out := buf.String()

		if !strings.Contains(out, "PING 10.0.0.1") {
			t.Errorf("missing destination header in output: %s", out)
		}
		if !strings.Contains(out, "33.3%") {
			t.Errorf("missing loss percentage in output: %s", out)
		}
		if !strings.Contains(out, "seq=1  timeout") {
			t.Errorf("missing timeout reply in output: %s", out)
		}
		if !strings.Contains(out, "rtt min/avg/max") {
			t.Errorf("missing RTT summary in output: %s", out)
		}
	})

	t.Run("all timeout", func(t *testing.T) {
		results := map[string]any{
			"destination":  "10.0.0.99",
			"sent":         2,
			"received":     0,
			"loss-percent": 100.0,
			"replies": []map[string]any{
				{"seq": 0, "status": "timeout"},
				{"seq": 1, "status": "timeout"},
			},
		}
		var buf bytes.Buffer
		printPingResults(&buf, results)
		out := buf.String()

		if !strings.Contains(out, "100.0%") {
			t.Errorf("missing 100%% loss in output: %s", out)
		}
		if strings.Contains(out, "rtt min/avg/max") {
			t.Errorf("RTT summary should be absent when no replies received: %s", out)
		}
	})
}
