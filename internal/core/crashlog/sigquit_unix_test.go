//go:build unix

package crashlog

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// sigquitParked is how many goroutines the child parks before it sends itself
// SIGQUIT. Each one adds a stack to the dump, so the dump is several times the
// 64 KiB a Linux pipe holds.
const sigquitParked = 4000

// sigquitDumpFloor is the smallest dump that proves the burst passed the pipe
// buffer many times over.
const sigquitDumpFloor = 256 * 1024

// TestSIGQUITDumpDrainsAfterInit proves that a SIGQUIT goroutine dump far larger
// than a pipe buffer completes after Init, and reaches the real stderr whole.
//
// The Go runtime freezes every goroutine before it prints a fatal dump, and then
// writes the dump to descriptor 2 with a raw write. Init once put a pipe on
// descriptor 2 whose only reader is a goroutine, so the reader was frozen with
// the rest, the pipe filled at 64 KiB, and the dump blocked for ever: `kill -QUIT`
// hung the daemon it was sent to diagnose (2026-09-24).
//
// VALIDATES: a child that runs Init, parks sigquitParked goroutines and sends
// itself SIGQUIT exits with status 2, its stderr holds the whole dump, and the
// next harvest turns the runtime's copy into a crash file holding it too.
// PREVENTS: a diagnostic signal that hangs the daemon with a partial dump.
func TestSIGQUITDumpDrainsAfterInit(t *testing.T) {
	if os.Getenv("ZE_TEST_CRASHLOG_SIGQUIT_CHILD") == "1" {
		Init()
		block := make(chan struct{})
		for range sigquitParked {
			go func() { <-block }()
		}
		if err := syscall.Kill(os.Getpid(), syscall.SIGQUIT); err != nil {
			os.Exit(9)
		}
		time.Sleep(time.Minute)
		os.Exit(8) // unreachable: SIGQUIT ends the process with status 2
	}

	crashDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSIGQUITDumpDrainsAfterInit$") //nolint:gosec // the test binary itself
	cmd.Env = append(os.Environ(),
		"ZE_TEST_CRASHLOG_SIGQUIT_CHILD=1",
		"ZE_CRASH_DIR="+crashDir,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()

	if ctx.Err() != nil {
		t.Fatalf("the child hung on its own SIGQUIT dump; %d bytes reached stderr", stderr.Len())
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("child exit = %v, want ExitError", err)
	}
	if exitErr.ExitCode() != 2 {
		t.Errorf("child exit code = %d, want 2", exitErr.ExitCode())
	}
	dump := stderr.String()
	if !strings.Contains(dump, "SIGQUIT: quit") {
		t.Fatalf("stderr holds no SIGQUIT dump:\n%s", dump[:min(len(dump), 2000)])
	}
	if len(dump) < sigquitDumpFloor {
		t.Fatalf("stderr holds %d bytes of dump, want at least %d", len(dump), sigquitDumpFloor)
	}
	if got := strings.Count(dump, "\ngoroutine "); got < sigquitParked {
		t.Errorf("stderr holds %d goroutine stacks, want at least %d", got, sigquitParked)
	}

	harvestPendingCrashes(crashDir, 5)
	names := listCrashFileNames(crashDir)
	if len(names) != 1 {
		t.Fatalf("crash files after the harvest = %v, want one", names)
	}
	data, err := os.ReadFile(filepath.Join(crashDir, names[0])) //nolint:gosec // a name from the test's own directory
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "SIGQUIT: quit") || len(data) < sigquitDumpFloor {
		t.Fatalf("the crash file holds %d bytes and no whole dump", len(data))
	}
}
