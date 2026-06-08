package cmd

import (
	"bytes"
	"strings"
	"testing"
)

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
