// Design: docs/guide/benchmarking.md -- stdout error propagation.
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReportCommandsPropagateStdoutFailure invokes real command readers with a
// closed stdout file. A valid measurement must not turn an I/O failure into success.
func TestReportCommandsPropagateStdoutFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clean.json")
	if err := os.WriteFile(path, []byte(`{"dut-name":"ze","convergence-ms":0,"routes-lost":0}`), 0o600); err != nil {
		t.Fatal(err)
	}
	closed, err := os.CreateTemp(dir, "closed-stdout")
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"", "--md", "--html", "--doc"} {
		args := []string{path}
		if mode != "" {
			args = append([]string{mode}, args...)
		}
		code, _, diagnostic := captureReportCommand(t, func() int {
			os.Stdout = closed
			return cmdReport(args)
		})
		if code != 1 {
			t.Errorf("compare %s exit = %d, want 1", mode, code)
		}
		if !strings.Contains(diagnostic, "writing") {
			t.Errorf("compare %s lacks write failure: %q", mode, diagnostic)
		}
	}
	for _, mode := range []string{"--md", "--html"} {
		code, _, diagnostic := captureReportCommand(t, func() int {
			os.Stdout = closed
			return cmdTrack([]string{mode, path})
		})
		if code != 1 {
			t.Errorf("track %s exit = %d, want 1", mode, code)
		}
		if !strings.Contains(diagnostic, "writing") {
			t.Errorf("track %s lacks write failure: %q", mode, diagnostic)
		}
	}
}
