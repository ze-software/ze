// Design: docs/guide/benchmarking.md -- comparison validity.
package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/perf"
)

var validityRenderers = []struct {
	name   string
	render func([]perf.Result, io.Writer) error
}{
	{"markdown", Markdown},
	{"html", HTML},
	{"doc", PerformanceDoc},
	{"trend-md", func(rows []perf.Result, w io.Writer) error { return Trend(rows, w, "md") }},
	{"trend-html", func(rows []perf.Result, w io.Writer) error { return Trend(rows, w, "html") }},
}

// TestComparisonRefusesLossBeforeSelection feeds decoded raw rows in both orders
// to every report entry, so latest selection cannot hide an older lossy row.
func TestComparisonRefusesLossBeforeSelection(t *testing.T) {
	for _, received := range []string{"99999", "0"} {
		loss := "1"
		if received == "0" {
			loss = "100000"
		}
		old := `{"dut-name":"ze","family":"ipv4/unicast","pass":"live-forwarding","timestamp":"2026-11-01T10:00:00Z","routes-sent":100000,"routes-received":` + received + `,"routes-lost":` + loss + `,"convergence-ms":0}`
		clean := `{"dut-name":"ze","family":"ipv4/unicast","pass":"live-forwarding","timestamp":"2026-11-01T11:00:00Z","routes-sent":100000,"routes-received":100000,"routes-lost":0,"convergence-ms":50}`
		for _, input := range []string{old + "\n" + clean, clean + "\n" + old} {
			rows, err := perf.ReadNDJSON(strings.NewReader(input))
			if err != nil {
				t.Fatal(err)
			}
			for _, renderer := range validityRenderers {
				t.Run(renderer.name+"/"+received+"/"+rows[0].Timestamp, func(t *testing.T) {
					var out bytes.Buffer
					err := renderer.render(rows, &out)
					if err == nil {
						t.Fatal("lossy raw row was ranked")
					}
					for _, want := range []string{"ze", "ipv4/unicast", "live-forwarding", "2026-11-01T10:00:00Z", loss} {
						if !strings.Contains(err.Error(), want) {
							t.Errorf("error %q lacks %q", err, want)
						}
					}
					if out.Len() != 0 {
						t.Fatalf("invalid report wrote %d bytes", out.Len())
					}
				})
			}
		}
	}
}

// TestComparisonConvergencePresence decodes missing, null and explicit zero
// measurements and requires absence to refuse without mistaking zero for absence.
func TestComparisonConvergencePresence(t *testing.T) {
	for _, field := range []string{"", `,"convergence-ms":null`, `,"convergence-ms":0`} {
		var row perf.Result
		if err := json.Unmarshal([]byte(`{"dut-name":"ze","family":"ipv4/unicast","routes-lost":0`+field+`}`), &row); err != nil {
			t.Fatal(err)
		}
		for _, renderer := range validityRenderers {
			t.Run(renderer.name+field, func(t *testing.T) {
				var out bytes.Buffer
				err := renderer.render([]perf.Result{row}, &out)
				if field == `,"convergence-ms":0` {
					if err != nil {
						t.Fatalf("explicit zero refused: %v", err)
					}
					if !strings.Contains(out.String(), "0ms") {
						t.Fatal("explicit zero not rendered")
					}
					return
				}
				if err == nil {
					t.Fatal("missing convergence ranked as zero")
				}
				if !strings.Contains(err.Error(), "convergence") {
					t.Fatal(err)
				}
				if out.Len() != 0 {
					t.Fatal("missing convergence wrote report bytes")
				}
			})
		}
	}
}

type reportFailWriter struct {
	bytes.Buffer
	failure error
}

func (w *reportFailWriter) Write(p []byte) (int, error) {
	if w.Len() != 0 {
		return 0, w.failure
	}
	return w.Buffer.Write(p)
}

// TestReportWriteFailurePropagates lets one write reach stdout before failure;
// the error must propagate without pretending that the first bytes rolled back.
func TestReportWriteFailurePropagates(t *testing.T) {
	rows, err := perf.ReadNDJSON(strings.NewReader(`{"dut-name":"ze","convergence-ms":0}`))
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("report destination failed")
	for _, renderer := range validityRenderers {
		t.Run(renderer.name, func(t *testing.T) {
			w := &reportFailWriter{failure: failure}
			if err := renderer.render(rows, w); !errors.Is(err, failure) {
				t.Fatalf("error = %v", err)
			}
			if w.Len() == 0 {
				t.Fatal("first successful write unexpectedly rolled back")
			}
		})
	}
}

// TestRawLossDiagnosticRoundTrip retains lossy evidence through JSON and NDJSON
// serialization, without accidentally treating the raw writers as report gates.
func TestRawLossDiagnosticRoundTrip(t *testing.T) {
	for _, convergence := range []string{"", `,"convergence-ms":0`} {
		var row perf.Result
		if err := json.Unmarshal([]byte(`{"dut-name":"ze","routes-sent":100000,"routes-received":99999,"routes-lost":1`+convergence+`}`), &row); err != nil {
			t.Fatal(err)
		}
		for _, write := range []func(io.Writer, *perf.Result) error{WriteJSON, func(w io.Writer, r *perf.Result) error { return perf.WriteNDJSON(w, *r) }} {
			var out bytes.Buffer
			if err := write(&out, &row); err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(out.Bytes(), &fields); err != nil {
				t.Fatal(err)
			}
			if string(fields["routes-lost"]) != "1" {
				t.Fatal("loss diagnostic erased")
			}
			_, present := fields["convergence-ms"]
			if present != (convergence != "") {
				t.Fatalf("convergence presence changed: %s", out.Bytes())
			}
		}
	}
}
