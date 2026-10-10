package appliance

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/crashlog"
)

// gokExitChildEnv selects the child half of TestGokExitReachesStderr.
const gokExitChildEnv = "ZE_TEST_GOK_EXIT_CHILD"

// gokExitMessage is what the fake gok prints before it exits.
const gokExitMessage = "ERROR:\n  copyGlobsToBoot: fake gok failure"

// TestGokExitReachesStderr proves that an error gok prints before it ends the
// process reaches the operator.
//
// VALIDATES: `ze appliance build` shows gok's own "ERROR:" line when gok fails.
// PREVENTS: exit status 1 with no error at all, which is what an arm64 build
// answered on 2026-10-10.
//
// Method: gok's packer.Main writes its error to os.Stderr and calls os.Exit(1),
// so control never returns to runGokBuild. crashlog.Init has pointed os.Stderr
// at a relay pipe, and os.Exit kills the relay goroutines before they copy the
// line out. The behavior needs a process that dies, so the test re-runs its own
// binary: the child arms crashlog exactly as `ze` does, runs runGokInProcess
// over a fake gok that prints and exits, and the parent reads the child's real
// stderr.
func TestGokExitReachesStderr(t *testing.T) {
	if os.Getenv(gokExitChildEnv) == "1" {
		runGokExitChild()
		return
	}

	work := t.TempDir()
	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^TestGokExitReachesStderr$")
	cmd.Dir = work
	cmd.Env = append(os.Environ(), gokExitChildEnv+"=1", "ze_crash_dir="+filepath.Join(work, "crash"))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("child answered %v, want exit status 1 from the fake gok; stderr:\n%s", err, stderr.String())
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("child exit code %d, want 1; stderr:\n%s", exitErr.ExitCode(), stderr.String())
	}
	if !strings.Contains(stderr.String(), gokExitMessage) {
		t.Fatalf("gok's error never reached stderr; got:\n%s", stderr.String())
	}
}

// runGokExitChild is the child half: it never returns, because the fake gok
// ends the process the way packer.Main does.
func runGokExitChild() {
	if err := os.MkdirAll(filepath.Join("gokrazy", "modcache"), 0o750); err != nil {
		os.Exit(3)
	}
	crashlog.Init()
	gokExecuteFn = func(context.Context, []string) error {
		os.Stderr.WriteString(gokExitMessage + "\n") //nolint:errcheck // the parent asserts what arrived
		os.Exit(1)
		return nil
	}
	if err := runGokInProcess(nil); err != nil {
		os.Exit(4)
	}
	os.Exit(5)
}
