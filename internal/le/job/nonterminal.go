// Design: docs/architecture/testing/qemu-integration.md -- noninteractive guest compiler ownership.
// Related: job.go -- OwnProcessGroup selects this path without changing terminal jobs.
package job

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/test/runner"
)

// streamNonterminal owns the command's group through the shared sole reaper.
// The caller MUST pass its cancellation root and MUST release no ticket until
// this function returns. This function MUST close and join the owner on every
// successfully started path, including natural exit and cancellation.
func (a *Admission) streamNonterminal(ctx context.Context, command *exec.Cmd) (code int) {
	if ctx.Err() != nil {
		return gaterun.CannotStart
	}
	// Preserve stream's existing SIGINT/TERM handling for callers whose root
	// is Background, without forwarding to a numeric PID outside the owner.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	owner, err := runner.StartServerProcess(command)
	if err != nil {
		a.note(errorLine(err))
		return gaterun.CannotStart
	}
	defer func() {
		if err := owner.Close(); err != nil {
			a.note(errorLine(err))
			if code == 0 {
				code = 1
			}
		}
	}()

	stopSignal := syscall.SIGTERM
	select {
	case <-ctx.Done():
	case caught := <-signals:
		if caught == os.Interrupt {
			stopSignal = syscall.SIGINT
		}
	case <-owner.Done():
		return gaterun.ExitCode(owner.Wait())
	}
	// Stop and Close share one cleanup result. The deferred Close MUST
	// check it, including any signal or lifecycle failure from this stop.
	_ = owner.Stop(stopSignal, runner.TeardownGraceTimeout, 0) //nolint:errcheck // deferred Close checks the same cached result
	// Wait is the actual command outcome, not Close's expected-stop policy.
	// Keeping it prevents an unexpected signal exit from becoming success.
	return gaterun.ExitCode(owner.Wait())
}
