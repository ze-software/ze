// Design: docs/guide/benchmarking.md -- report formats.
// Related: cmd_report_loss_test.go -- captureReportCommand.

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCompareHasNoJSONFormat proves `le perf compare` offers no JSON
// comparison format. Method: write one clean result row, check the default
// format accepts it, then ask for --json on the same file.
//
// VALIDATES: cmdReport refuses --json with a non-zero exit, no report on
// stdout, and a diagnostic naming the flag, while the same row is reportable.
// PREVENTS: a JSON comparison format appearing (or a stray flag being
// accepted in silence) that the usage text says does not exist.
func TestCompareHasNoJSONFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clean.json")
	row := `{"dut-name":"ze","family":"ipv4/unicast","pass":"live-forwarding","timestamp":"2026-11-01T11:00:00Z","routes-sent":100000,"routes-received":100000,"routes-lost":0,"convergence-ms":50}`
	if err := os.WriteFile(path, []byte(row), 0o600); err != nil {
		t.Fatal(err)
	}

	code, out, diagnostic := captureReportCommand(t, func() int { return cmdReport([]string{path}) })
	if code != 0 || out == "" {
		t.Fatalf("control: the clean row must report, exit %d, stdout %q, stderr %q", code, out, diagnostic)
	}

	code, out, diagnostic = captureReportCommand(t, func() int { return cmdReport([]string{"--json", path}) })
	if code == 0 {
		t.Fatalf("--json exit = 0, want non-zero; stdout %q", out)
	}
	if out != "" {
		t.Errorf("--json wrote a report: %q", out)
	}
	if !strings.Contains(diagnostic, "-json") {
		t.Errorf("diagnostic %q does not name the refused flag", diagnostic)
	}
}
