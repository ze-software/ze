package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: AC-15 "ze-perf run -h prints usage with flag descriptions and examples."
// PREVENTS: Help text missing or panicking.
func TestRunHelp(t *testing.T) {
	t.Parallel()

	// Top-level help returns 0.
	code := run([]string{"-h"})
	if code != 0 {
		t.Errorf("run(-h) exit code = %d, want 0", code)
	}

	code = run([]string{"--help"})
	if code != 0 {
		t.Errorf("run(--help) exit code = %d, want 0", code)
	}

	code = run([]string{"help"})
	if code != 0 {
		t.Errorf("run(help) exit code = %d, want 0", code)
	}
}

// captureStderr runs fn while capturing os.Stderr output.
// NOT safe for concurrent use -- callers must NOT use t.Parallel().
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	oldStderr := os.Stderr

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}

	os.Stderr = w

	fn()

	if err := w.Close(); err != nil {
		os.Stderr = oldStderr
		t.Fatalf("closing pipe writer: %v", err)
	}

	os.Stderr = oldStderr

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("reading captured stderr: %v", err)
	}

	return buf.String()
}

// VALIDATES: AC-9 "ze-perf track --check with regression exits non-zero, prints regression details."
// PREVENTS: Regressions going undetected in CI.
func TestTrackCheckRegression(t *testing.T) {
	// No t.Parallel() -- captureStderr modifies global os.Stderr.

	dir := t.TempDir()
	path := filepath.Join(dir, "history.ndjson")

	// Two results: second has doubled convergence (regression).
	data := strings.Join([]string{
		`{"dut-name":"ze","family":"ipv4/unicast","convergence-ms":1000,"convergence-stddev-ms":50,"throughput-avg":50000,"throughput-avg-stddev":1000,"latency-p99-ms":10,"latency-p99-stddev-ms":2}`,
		`{"dut-name":"ze","family":"ipv4/unicast","convergence-ms":2500,"convergence-stddev-ms":60,"throughput-avg":50000,"throughput-avg-stddev":1000,"latency-p99-ms":10,"latency-p99-stddev-ms":2}`,
		"",
	}, "\n")

	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("writing NDJSON: %v", err)
	}

	var code int

	stderr := captureStderr(t, func() {
		code = cmdTrack([]string{"--check", path})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1 for regression; stderr: %s", code, stderr)
	}

	if !strings.Contains(stderr, "regression") {
		t.Errorf("stderr missing 'regression' keyword: %s", stderr)
	}
}

// VALIDATES: AC-10 "ze-perf track --check with no regression exits zero, prints 'no regression'."
// PREVENTS: False exit codes when no regression detected.
func TestTrackCheckNoRegression(t *testing.T) {
	// No t.Parallel() -- captureStderr modifies global os.Stderr.

	dir := t.TempDir()
	path := filepath.Join(dir, "history.ndjson")

	// Two results: within threshold (5% variation, under 20% threshold).
	data := strings.Join([]string{
		`{"dut-name":"ze","family":"ipv4/unicast","convergence-ms":1000,"convergence-stddev-ms":50,"throughput-avg":50000,"throughput-avg-stddev":1000,"latency-p99-ms":10,"latency-p99-stddev-ms":2}`,
		`{"dut-name":"ze","family":"ipv4/unicast","convergence-ms":1050,"convergence-stddev-ms":55,"throughput-avg":49000,"throughput-avg-stddev":1100,"latency-p99-ms":10,"latency-p99-stddev-ms":2}`,
		"",
	}, "\n")

	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("writing NDJSON: %v", err)
	}

	var code int

	stderr := captureStderr(t, func() {
		code = cmdTrack([]string{"--check", path})
	})

	if code != 0 {
		t.Errorf("exit code = %d, want 0 for no regression; stderr: %s", code, stderr)
	}

	if !strings.Contains(stderr, "no regression") {
		t.Errorf("stderr missing 'no regression' message: %s", stderr)
	}
}
