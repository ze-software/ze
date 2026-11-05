// Design: docs/architecture/testing/qemu-integration.md -- native suite deadlines.
// Related: alltests.go -- the suite consumer and non-suite child execution.

package testqemu

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/test/runner"
)

// suiteKillGrace preserves the former timeout wrapper's TERM-to-KILL budget.
const suiteKillGrace = 15 * time.Second

// suiteTimeout refuses an absent or unbounded cap before any child starts.
func (a *allTestsRun) suiteTimeout() (time.Duration, error) {
	duration, err := time.ParseDuration(a.Timeout)
	if err != nil {
		return 0, fmt.Errorf("qemu: invalid suite timeout %q. Fix: set ZE_QEMU_SUITE_TIMEOUT to a positive duration such as 900s: %w", a.Timeout, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("qemu: suite timeout %q is not positive. Fix: set ZE_QEMU_SUITE_TIMEOUT to a positive duration such as 900s", a.Timeout)
	}
	return duration, nil
}

// suiteChild owns the suite deadline, streamed tally and process group through
// the runner's existing sole-reaper lifecycle. A successful start MUST defer
// Close before using the owner; Wait and StopGroup retain its cleanup verdict.
func (a *allTestsRun) suiteChild(argv, environ []string, tally io.Writer) int {
	duration, err := a.suiteTimeout()
	if err != nil {
		a.note(err.Error())
		return gaterun.CannotStart
	}
	if a.Run != nil {
		return a.Run(argv, environ, tally)
	}
	if len(argv) == 0 {
		a.note("qemu: suite declared no command. Fix: register its executable and test selectors")
		return gaterun.CannotStart
	}
	//nolint:gosec // The suite registry and the guest shim supply this closed argv.
	command := exec.CommandContext(context.Background(), argv[0], argv[1:]...)
	command.Dir, command.Env = a.Workspace, environ
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	if tally != nil {
		command.Stdout = io.MultiWriter(os.Stdout, tally)
	}
	owner, err := runner.StartServerProcess(command)
	if err != nil {
		a.note("qemu: cannot start suite " + argv[0] + ": " + err.Error() + ". Fix: inspect the guest executable and its permissions")
		return gaterun.CannotStart
	}
	defer owner.Close() //nolint:errcheck // Wait or StopGroup below retains cleanup errors; this also covers panic unwinding.
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-owner.Done():
		return gaterun.ExitCode(owner.Wait())
	case <-timer.C:
		// A completed command retains its own verdict when both become ready.
		select {
		case <-owner.Done():
			return gaterun.ExitCode(owner.Wait())
		default:
		}
		if err := owner.StopGroup(syscall.SIGTERM, suiteKillGrace, 0); err != nil {
			a.note("qemu: timed-out suite cleanup failed: " + err.Error() + ". Fix: inspect the suite's remaining processes")
		}
		if code := gaterun.ExitCode(owner.Wait()); code == 128+int(syscall.SIGKILL) {
			return code
		}
		return 124
	}
}
