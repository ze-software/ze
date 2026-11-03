// Design: docs/architecture/testing/runner-architecture.md -- owned suite servers.
package runner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/ze-software/ze/internal/component/plugin"
)

// ServerProcess owns one successfully started server. Its zero and nil values
// are invalid. The caller MUST call Close before releasing files, ports or
// scheduler leases, and MUST NOT call Cmd.Wait itself. Close is single-owner.
type ServerProcess struct {
	process *ownedProcess
}

// StartServerProcess starts a CommandContext on the persistent spawner thread,
// using the runner's existing identity-safe cancellation and sole Wait worker.
// On success the caller MUST defer Close before performing another operation.
func StartServerProcess(cmd *exec.Cmd) (*ServerProcess, error) {
	cmd.SysProcAttr = plugin.NewSysProcAttr()
	owned := boundProcessDeadline(cmd)
	if err := plugin.StartChild(cmd); err != nil {
		return nil, err
	}
	go owned.reap()
	return &ServerProcess{process: owned}, nil
}

// Close stops the owned group and actually joins its sole reaper. It does not
// return on a timer while the child still owns resources. An intentional kill
// is expected; an unexpected exit, pipe-drain or lifecycle failure is not.
func (s *ServerProcess) Close() error {
	killErr := s.process.cancel()
	if errors.Is(killErr, os.ErrProcessDone) {
		killErr = nil
	}
	if errors.Is(killErr, syscall.ESRCH) {
		killErr = nil
	}
	waitErr := s.process.Wait()
	if exited, ok := errors.AsType[*exec.ExitError](waitErr); ok {
		if status, ok := exited.Sys().(syscall.WaitStatus); ok {
			if status.Signaled() && status.Signal() == syscall.SIGKILL {
				waitErr = nil
			}
		}
	}
	if err := errors.Join(killErr, waitErr, s.process.lifecycleError()); err != nil {
		return fmt.Errorf("server child %d cleanup failed: %w", s.process.cmd.Process.Pid, err)
	}
	return nil
}
