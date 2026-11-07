// Design: docs/architecture/testing/runner-architecture.md -- one owner per started child.
// Related: runner_exec_util.go -- startWithETXTBSYRetry and process deadlines.

package runner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// ownedProcess is published only after a successful Start. Start prepares it
// privately first so cancellation uses the same identity lock from launch.
// Its zero and nil values are not usable. The runner MUST defer finish before
// doing anything else; direct users MUST defer cleanup and check lifecycleError.
// One worker calls Cmd.Wait and closes done after output copying ends. Wait is
// safe for concurrent callers and returns that same exit result every time.
// Lifecycle setup and cleanup belong to the starting goroutine.
type ownedProcess struct {
	cmd          *exec.Cmd
	done         chan struct{}
	err          error
	observe      func(*ownedProcess) error
	lifecycleErr error
	lifetimeDone chan struct{}
	cleanupOnce  sync.Once
	groupSignal  func(int, syscall.Signal) error
	// signalMu serializes numeric signals with retirement. Retired is a guard,
	// not an exit result: Cmd.Wait still owns output drain and result publication.
	signalMu sync.Mutex
	retired  bool
	// Written under signalMu before retirement; read only after done closes.
	// A successful pre-exit owned KILL earns intentionalKill. Post-exit
	// descendant cleanup in retire does not. Unknown observations stay errors.
	intentionalKill    bool
	stopObservationErr error
}

func (p *ownedProcess) reap() {
	observe := p.observe
	if observe == nil {
		observe = (*ownedProcess).beforeReap
	}
	p.lifecycleErr = observe(p)
	p.err = p.cmd.Wait()
	_ = p.retire(false) //nolint:errcheck // Retirement without group cleanup cannot fail.
	close(p.done)
}

// observeExit retries interrupted kernel observations. An error other than
// ECHILD still leaves this sole waiter owning the child, so stop it before
// retiring cancellation. ECHILD grants no permission to signal a number.
func (p *ownedProcess) observeExit(wait func() error) error {
	for {
		err := wait()
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			if !errors.Is(err, syscall.ECHILD) {
				p.kill()
			}
			_ = p.retire(false) //nolint:errcheck // Retirement without group cleanup cannot fail.
			return fmt.Errorf("observe child exit before reap: %w", err)
		}
		if err := p.retire(true); err != nil {
			return fmt.Errorf("clean child process group before reap: %w", err)
		}
		return nil
	}
}

// retire revokes every alias's permission to signal, even if final cleanup fails.
// clean is permitted only while an unreaped leader pins the group identity
// (beforeReap). An absent group is benign, and so is a group whose only members
// are zombies, which Darwin answers with EPERM (exitedGroupSignalErr). Other
// signal errors reach the owner.
func (p *ownedProcess) retire(clean bool) error {
	p.signalMu.Lock()
	defer p.signalMu.Unlock()
	if p.retired {
		return nil
	}
	if !clean {
		p.retired = true
		return nil
	}
	pgid := p.cmd.Process.Pid
	err := p.groupSignal(-pgid, syscall.SIGKILL)
	p.retired = true
	if err == nil {
		return nil
	}
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return exitedGroupSignalErr(pgid, err)
}

// Wait joins the sole reaper and returns only Cmd.Wait's result for exit-status
// assertions. The runner MUST defer finish to check lifecycleError independently
// of those assertions. Other owners MUST defer cleanup and check lifecycleError.
func (p *ownedProcess) Wait() error {
	<-p.done
	return p.err
}

// lifecycleError joins the owner before reading observation or retirement failure.
// It is independent of the child's exit status and cannot satisfy an exit assertion.
func (p *ownedProcess) lifecycleError() error {
	<-p.done
	if p.stopObservationErr == nil {
		return p.lifecycleErr
	}
	if p.lifecycleErr == nil {
		return p.stopObservationErr
	}
	return errors.Join(p.lifecycleErr, p.stopObservationErr)
}

const failTypeProcessLifecycle = "process_lifecycle_failed"

// finish MUST be deferred immediately after each runner start, before resource
// release. Every return, including self-validation and named-stop removal, must
// retain this check. Joining errors here preserves diagnostics only: lifecycle
// failure unconditionally fails the verdict, regardless of any expected exit.
func (p *ownedProcess) finish(rec *Record, passed *bool) {
	p.cleanup()
	if err := p.lifecycleError(); err != nil {
		rec.Error = errors.Join(rec.Error, fmt.Errorf("child %d lifecycle failed: %w", p.cmd.Process.Pid, err))
		rec.FailureType = failTypeProcessLifecycle
		rec.SetState(StateFail)
		rec.recordStep("process-lifecycle", false, err.Error())
		*passed = false
	}
}

// cleanup MUST be deferred immediately after start succeeds, before any later
// operation can fail. It stops the owned group and joins output and lifetime
// workers before the caller can remove files or release a port lease.
func (p *ownedProcess) cleanup() {
	p.cleanupOnce.Do(func() {
		p.kill()
		_ = p.Wait() //nolint:errcheck // cleanup preserves the asserted exit result
		if p.lifetimeDone != nil {
			<-p.lifetimeDone
		}
	})
}

func (p *ownedProcess) kill() {
	_ = p.cancel() //nolint:errcheck // an exited group needs no signal
}

// cancel is installed before Start, so even immediate context cancellation uses
// this owner. Returning ErrProcessDone preserves a successful exit when exec's
// context watcher arrives after retirement. The check and syscall share a lock.
func (p *ownedProcess) cancel() error {
	p.signalMu.Lock()
	defer p.signalMu.Unlock()
	if p.retired {
		return os.ErrProcessDone
	}
	exited, probeErr := p.exitBeforeSignal()
	signalErr := p.groupSignal(-p.cmd.Process.Pid, syscall.SIGKILL)
	if probeErr == nil && !exited && signalErr == nil {
		p.intentionalKill = true
	}
	return signalErr
}

// exitBeforeSignal MUST run under signalMu. Retain the first failed causal
// observation independently of signal errors, so a benign ESRCH cannot erase
// it. Unknown observation never prevents the caller's owned cleanup attempt.
func (p *ownedProcess) exitBeforeSignal() (bool, error) {
	exited, err := p.exitedBeforeStop()
	if err != nil && p.stopObservationErr == nil {
		p.stopObservationErr = fmt.Errorf("observe child %d before owned signal: %w", p.cmd.Process.Pid, err)
	}
	return exited, err
}

// stop preserves a daemon's opportunity to stop its children after SIGTERM.
// After the grace, the group is killed and the same reaper is joined.
func (p *ownedProcess) stop() error {
	select {
	case <-p.done:
		return p.err
	default:
	}
	_ = p.cmd.Process.Signal(syscall.SIGTERM) //nolint:errcheck // the child can exit before the signal
	timer := time.NewTimer(TeardownGraceTimeout)
	defer timer.Stop()
	select {
	case <-p.done:
		return p.err
	case <-timer.C:
		p.kill()
		return p.Wait()
	}
}
