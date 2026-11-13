// Design: docs/architecture/testing/runner-architecture.md -- owned suite servers.
package runner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
)

// ServerProcess owns one successfully started child. Its zero and nil values
// are invalid. The caller MUST defer Close, Stop or StopGroup before releasing
// files, ports or scheduler leases, and MUST NOT call Cmd.Wait itself. All stop
// methods and Close share one cached result; Wait and Done observe the same reaper.
type ServerProcess struct {
	process   *ownedProcess
	closeOnce sync.Once
	closeErr  error
}

// StartServerProcess starts a CommandContext on the persistent spawner thread,
// using the runner's existing identity-safe cancellation and sole Wait worker.
// A positive Cmd.WaitDelay supplies the caller's pipe-drain bound; otherwise the
// existing runner teardown grace applies.
// On success the caller MUST defer Close, Stop or StopGroup before another operation.
func StartServerProcess(cmd *exec.Cmd) (*ServerProcess, error) {
	cmd.SysProcAttr = forkAttributes(cmd.SysProcAttr)
	waitDelay := cmd.WaitDelay
	owned := boundProcessDeadline(cmd)
	if waitDelay > 0 {
		cmd.WaitDelay = waitDelay
	}
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
	s.closeOnce.Do(func() { s.closeErr = s.closeResult(serverSignalError(s.process.cancel()), syscall.SIGKILL, 0) })
	return s.closeErr
}

// Wait joins the sole reaper and returns its exit and lifecycle errors. It is
// safe for concurrent observers. The caller MUST still defer Close: waiting
// alone cannot stop a child that has not exited.
func (s *ServerProcess) Wait() error {
	return errors.Join(s.process.Wait(), s.process.lifecycleError())
}

// Done closes only after the sole reaper finishes. Observers MUST still defer
// Close and check Wait or Close; readiness alone cannot establish a clean exit.
func (s *ServerProcess) Done() <-chan struct{} {
	return s.process.done
}

// Stop signals the leader and grants its explicit grace before joining
// the owned group. A nonzero expected exit is accepted only when this stop
// signaled a parent not already observed exited by the kernel. The caller MUST
// defer Stop or Close immediately after StartServerProcess. Both return the
// same cached result, including every signal and lifecycle failure.
func (s *ServerProcess) Stop(signal syscall.Signal, grace time.Duration, expectedExitCode int) error {
	s.closeOnce.Do(func() { s.stop(signal, grace, expectedExitCode, false) })
	return s.closeErr
}

// StopGroup signals the whole owned process group, grants the explicit grace,
// then kills the group if needed and joins its sole reaper. It shares Stop's
// identity check and cached result. The caller MUST defer Close, Stop or
// StopGroup immediately after StartServerProcess, including error paths.
func (s *ServerProcess) StopGroup(signal syscall.Signal, grace time.Duration, expectedExitCode int) error {
	s.closeOnce.Do(func() { s.stop(signal, grace, expectedExitCode, true) })
	return s.closeErr
}

func (s *ServerProcess) stop(signal syscall.Signal, grace time.Duration, expectedExitCode int, wholeGroup bool) {
	signalErr, acceptedSignal, acceptedExitCode := s.signalStop(signal, expectedExitCode, wholeGroup)
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case <-s.process.done:
	case <-timer.C:
		signalErr = errors.Join(signalErr, serverSignalError(s.process.cancel()))
	}
	s.closeErr = s.closeResult(signalErr, acceptedSignal, acceptedExitCode)
}

// signalStop MUST hold the identity lock through the nonreaping exit probe and
// signal. An unretired identity alone is not proof that its parent is alive.
func (s *ServerProcess) signalStop(signal syscall.Signal, expectedExitCode int, wholeGroup bool) (error, syscall.Signal, int) {
	s.process.signalMu.Lock()
	defer s.process.signalMu.Unlock()
	if s.process.retired {
		return nil, 0, 0
	}
	exited, probeErr := s.process.exitBeforeSignal()
	if probeErr == nil && exited {
		return nil, 0, 0
	}
	target := s.process.cmd.Process.Pid
	if wholeGroup {
		target = -target
	}
	signalErr := s.process.groupSignal(target, signal)
	if probeErr != nil || signalErr != nil {
		return serverSignalError(signalErr), 0, 0
	}
	if signal == syscall.SIGKILL {
		s.process.intentionalKill = true
	}
	return nil, signal, expectedExitCode
}

func (s *ServerProcess) closeResult(signalErr error, expected syscall.Signal, expectedExitCode int) error {
	waitErr := s.process.Wait()
	if exited, ok := errors.AsType[*exec.ExitError](waitErr); ok {
		if expectedExitCode != 0 {
			if exited.ExitCode() == expectedExitCode {
				waitErr = nil
			}
		}
		if status, ok := exited.Sys().(syscall.WaitStatus); ok {
			if status.Signaled() {
				if status.Signal() == syscall.SIGKILL {
					if s.process.intentionalKill {
						waitErr = nil
					}
				} else if status.Signal() == expected {
					waitErr = nil
				}
			}
		}
	}
	cleanupErr := errors.Join(signalErr, s.process.lifecycleError())
	if cleanupErr == nil {
		return waitErr
	}
	return fmt.Errorf("server child %d cleanup failed: %w", s.process.cmd.Process.Pid, errors.Join(waitErr, cleanupErr))
}

func serverSignalError(err error) error {
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
