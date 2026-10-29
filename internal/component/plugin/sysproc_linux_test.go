//go:build linux

package plugin

import (
	"errors"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
	"time"
)

// TestStartChildSurvivesForkingThreadExit proves a plugin started through
// StartChild is not killed when the goroutine that asked for the start ends on
// a locked OS thread, which the Go runtime then destroys.
//
// Pdeathsig is delivered when the THREAD that forked the child exits, not when
// ze exits. A goroutine that locks its thread and returns without unlocking it
// (kernelcap.InThrowawayNetworkNamespace does so on every capability probe,
// including the reload gate's) destroys whatever runtime thread it ran on. A
// plugin forked earlier from that thread was SIGKILLed with no log line in
// the middle of a SIGHUP reload.
//
// Method: ask for the start from a goroutine that locks its thread and returns
// still locked, so the runtime destroys the thread the request came from. Then
// give the kernel a second to deliver a death signal, and require the child to
// still be running. A start that forks on the requesting thread fails this.
func TestStartChildSurvivesForkingThreadExit(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "sleep", "30")
	cmd.SysProcAttr = NewSysProcAttr()

	started := make(chan error, 1)
	go startFromDoomedThread(cmd, started)
	if err := <-started; err != nil {
		t.Fatalf("start the child: %v", err)
	}
	exited := make(chan error, 1)
	go waitForChild(cmd, exited)
	select {
	case err := <-exited:
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
				t.Fatalf("the child died of %v when the thread that asked for its start exited", status.Signal())
			}
		}
		t.Fatalf("the child ended early: %v", err)
	case <-time.After(time.Second):
	}

	if err := KillProcessGroup(cmd.Process.Pid); err != nil {
		t.Errorf("stop the child: %v", err)
	}
	<-exited
}

// startFromDoomedThread asks for cmd's start from a goroutine that leaves its
// OS thread locked, so the runtime destroys that thread when it returns.
func startFromDoomedThread(cmd *exec.Cmd, started chan<- error) {
	runtime.LockOSThread()
	started <- StartChild(cmd)
}

// waitForChild reports the child's exit; it is the process-wait bridge of the
// test above.
func waitForChild(cmd *exec.Cmd, exited chan<- error) {
	exited <- cmd.Wait()
}
