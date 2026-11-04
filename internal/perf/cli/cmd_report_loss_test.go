// Design: docs/guide/benchmarking.md -- report input boundaries.
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureReportCommand uses files rather than pipes because a valid HTML report
// can exceed pipe capacity. Callers MUST NOT run concurrently with other capture.
func captureReportCommand(t *testing.T, run func() int) (int, string, string) {
	t.Helper()
	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdout, stderr
	defer func() { os.Stdout, os.Stderr = oldOut, oldErr }()
	code := run()
	out, err := os.ReadFile(stdout.Name())
	if err != nil {
		t.Fatal(err)
	}
	diagnostic, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatal(err)
	}
	return code, string(out), string(diagnostic)
}

// TestCompareCommandRefusesLoss exercises every compare format using the real
// JSON file reader, including a lossy row hidden by a newer same-pass result.
func TestCompareCommandRefusesLoss(t *testing.T) {
	dir := t.TempDir()
	old, clean := filepath.Join(dir, "old.json"), filepath.Join(dir, "new.json")
	for path, data := range map[string]string{
		old:   `{"dut-name":"ze","family":"ipv4/unicast","pass":"live-forwarding","timestamp":"2026-11-01T10:00:00Z","routes-sent":100000,"routes-received":99999,"routes-lost":1,"convergence-ms":0}`,
		clean: `{"dut-name":"ze","family":"ipv4/unicast","pass":"live-forwarding","timestamp":"2026-11-01T11:00:00Z","routes-sent":100000,"routes-received":100000,"routes-lost":0,"convergence-ms":50}`,
	} {
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, flag := range []string{"", "--md", "--html", "--doc"} {
		for _, files := range [][]string{{old, clean}, {clean, old}} {
			args := files
			if flag != "" {
				args = append([]string{flag}, files...)
			}
			code, out, diagnostic := captureReportCommand(t, func() int { return cmdReport(args) })
			if code != 1 {
				t.Errorf("%v exit = %d, want 1", args, code)
			}
			if out != "" {
				t.Errorf("%v wrote invalid report", args)
			}
			for _, want := range []string{old, "ze", "1", "lost"} {
				if !strings.Contains(diagnostic, want) {
					t.Errorf("%v diagnostic %q lacks %q", args, diagnostic, want)
				}
			}
		}
	}
}

// TestTrackCommandValidatesBeforeLast puts loss outside the displayed window;
// both output formats and regression-check mode must refuse the full raw input.
func TestTrackCommandValidatesBeforeLast(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.ndjson")
	data := `{"dut-name":"ze","routes-sent":100000,"routes-received":0,"routes-lost":100000,"convergence-ms":0}` + "\n" + `{"dut-name":"ze","routes-sent":100000,"routes-received":100000,"routes-lost":0,"convergence-ms":50}` + "\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"--md", "--html", "--check"} {
		code, out, diagnostic := captureReportCommand(t, func() int { return cmdTrack([]string{mode, "--last", "1", path}) })
		if code != 1 {
			t.Errorf("%s exit = %d, want 1", mode, code)
		}
		if out != "" {
			t.Errorf("%s wrote invalid report", mode)
		}
		for _, want := range []string{path, "ze", "100000", "lost"} {
			if !strings.Contains(diagnostic, want) {
				t.Errorf("%s diagnostic %q lacks %q", mode, diagnostic, want)
			}
		}
	}
}

// TestCompareCommandConvergencePresence checks the actual JSON reader instead of
// fabricating a direct Result that could not distinguish missing from zero.
func TestCompareCommandConvergencePresence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ze.json")
	for _, field := range []string{"", `,"convergence-ms":0`} {
		if err := os.WriteFile(path, []byte(`{"dut-name":"ze","routes-lost":0`+field+`}`), 0o600); err != nil {
			t.Fatal(err)
		}
		for _, mode := range []string{"--md", "--html", "--doc"} {
			code, out, diagnostic := captureReportCommand(t, func() int { return cmdReport([]string{mode, path}) })
			if field != "" {
				if code != 0 || !strings.Contains(out, "0ms") {
					t.Errorf("explicit zero: exit %d, output %q, diagnostic %q", code, out, diagnostic)
				}
				continue
			}
			if code != 1 || out != "" {
				t.Errorf("missing convergence: exit %d, output %q", code, out)
			}
			if !strings.Contains(diagnostic, path) || !strings.Contains(diagnostic, "convergence") {
				t.Errorf("missing diagnostic: %q", diagnostic)
			}
		}
	}
}
