// Design: docs/guide/benchmarking.md -- per-file report publication.
package perfrunner

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/perf"
	"github.com/ze-software/ze/internal/perf/report"
)

// TestReportPublicationRetainsValidOldBytes runs the real result decoder and
// report renderer under the publisher, rather than echoing simulated HTML.
func TestReportPublicationRetainsValidOldBytes(t *testing.T) {
	for _, renderer := range []struct {
		name  string
		write func([]perf.Result, io.Writer) error
	}{{"markdown", report.Markdown}, {"html", report.HTML}, {"doc", report.PerformanceDoc}} {
		t.Run(renderer.name, func(t *testing.T) {
			dir := t.TempDir()
			dest := filepath.Join(dir, "report")
			input := filepath.Join(dir, "result.json")
			clean := `{"dut-name":"ze","convergence-ms":0,"routes-lost":0}`
			if err := os.WriteFile(input, []byte(clean), 0o600); err != nil {
				t.Fatal(err)
			}
			render := func(_ context.Context, out, _ io.Writer, _ string, _ []string, _ []string) error {
				file, err := os.Open(input)
				if err != nil {
					return err
				}
				defer file.Close()
				var row perf.Result
				if err := json.NewDecoder(file).Decode(&row); err != nil {
					return err
				}
				return renderer.write([]perf.Result{row}, out)
			}
			if !generateToFile(context.Background(), render, testReporter, dest, io.Discard) {
				t.Fatal("valid report not published")
			}
			old, err := os.ReadFile(dest)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(old), "0ms") {
				t.Fatal("old report is not valid measured output")
			}
			if err := os.WriteFile(input, []byte(`{"dut-name":"ze","convergence-ms":0,"routes-lost":1}`), 0o600); err != nil {
				t.Fatal(err)
			}
			if generateToFile(context.Background(), render, testReporter, dest, io.Discard) {
				t.Fatal("lossy result replaced valid report")
			}
			retained, err := os.ReadFile(dest)
			if err != nil {
				t.Fatal(err)
			}
			if string(retained) != string(old) {
				t.Fatal("valid old bytes changed after invalid input")
			}
		})
	}
}

// TestReportReplacementFailureRetainsDestination forces rename to fail against
// a nonempty destination directory, keeping its old artifact bytes untouched.
func TestReportReplacementFailureRetainsDestination(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "report")
	if err := os.Mkdir(dest, 0o700); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(dest, "retained.html")
	if err := os.WriteFile(oldPath, []byte("retained report\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	render := func(_ context.Context, out, _ io.Writer, _ string, _ []string, _ []string) error {
		rows, err := perf.ReadNDJSON(strings.NewReader(`{"dut-name":"ze","convergence-ms":0}`))
		if err != nil {
			return err
		}
		return report.HTML(rows, out)
	}
	if generateToFile(context.Background(), render, testReporter, dest, io.Discard) {
		t.Fatal("failed rename reported success")
	}
	old, err := os.ReadFile(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(old) != "retained report\n" {
		t.Fatal("replacement failure changed old destination")
	}
}
