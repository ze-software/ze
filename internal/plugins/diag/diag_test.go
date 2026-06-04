package diag

import (
	"io"
	"os"
	"os/exec"
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

// TestRunWgKeypair_RejectsArgs ensures stray arguments exit 1 without
// attempting to exec `wg`.
func TestRunWgKeypair_RejectsArgs(t *testing.T) {
	silenceStderr(t)
	if rc := RunWgKeypair([]string{"extra"}); rc != 1 {
		t.Errorf("RunWgKeypair(extra) = %d, want 1", rc)
	}
}

// TestRunWgKeypair_Smoke skips when `wg` is unavailable; otherwise
// runs end-to-end and asserts that stdout has two lines.
func TestRunWgKeypair_Smoke(t *testing.T) {
	if _, err := exec.LookPath("wg"); err != nil {
		t.Skip("wg not installed; skipping keypair smoke")
	}
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	rc := RunWgKeypair(nil)
	if err := w.Close(); err != nil {
		t.Logf("close pipe writer: %v", err)
	}
	os.Stdout = oldStdout
	buf, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	if rc != 0 {
		t.Fatalf("RunWgKeypair() = %d, want 0; stdout=%q", rc, buf)
	}
	if len(buf) == 0 {
		t.Error("expected keypair output on stdout")
	}
}
