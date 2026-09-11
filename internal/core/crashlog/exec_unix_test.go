// VALIDATES: a program started by Exec inherits the real stderr, not the
// capture pipe, so its output reaches the caller.
// PREVENTS: an execve past the flush. The replacement image keeps fd 2 pointing
// at a pipe whose reading goroutine died with the image that held it, so its
// log vanishes and the write blocks once 64 KiB have accumulated. Measured in
// the QEMU guest on 2026-09-11.

//go:build unix

package crashlog

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestExecKeepsStderrForTheNewImage drives Init and the execve together,
// because that pair is where the descriptor is lost. A test that skips Init
// installs no pipe and passes whether Exec flushes or not.
func TestExecKeepsStderrForTheNewImage(t *testing.T) {
	const marker = "LAUNCHED-ON-STDERR"

	if os.Getenv("ZE_TEST_CRASHLOG_EXEC_CHILD") == "1" {
		Init()
		if err := Exec("/bin/sh", []string{"sh", "-c", "echo " + marker + " 1>&2"}, os.Environ()); err != nil {
			t.Fatalf("Exec /bin/sh: %v", err)
		}
		return // unreachable: a successful execve never returns
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestExecKeepsStderrForTheNewImage") //nolint:gosec // the test binary itself
	cmd.Env = append(os.Environ(),
		"ZE_TEST_CRASHLOG_EXEC_CHILD=1",
		"ZE_CRASH_DIR="+t.TempDir(),
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("child exit = %v; stderr: %s", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), marker) {
		t.Errorf("child stderr = %q, want it to contain %q; the launched program wrote into the crashlog pipe", stderr.String(), marker)
	}
}
