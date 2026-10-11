package kernelbuilder

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// The tests in this file prove a build VM wait that ended because the build's
// own context did is reported as the build stopping, at the stage it reached,
// never as a guest that failed. Each runs runQEMUBuild against a fake QEMU that
// boots no guest, and stops the context while a wait is in progress.

// TestCanceledBootWaitIsNotATimeout proves a build canceled while the VM boots
// is reported as canceled: the wait for the login prompt ended because the
// caller stopped the build, not because the guest failed to boot.
//
// VALIDATES: round-4 NOTE 4.
// PREVENTS: an operator who pressed Ctrl-C told to debug a guest that never
// booted.
func TestCanceledBootWaitIsNotATimeout(t *testing.T) {
	fakeQEMU(t, "exec sleep 30\n")
	req := Request{Root: t.TempDir(), Arch: "amd64", OutputDir: "out", Stderr: os.Stderr}
	ctx, cancel := context.WithCancel(context.Background())
	stop := time.AfterFunc(200*time.Millisecond, cancel)
	defer stop.Stop()
	err := runQEMUBuild(ctx, req, "alpine.iso", "worker", "kvm")
	if err == nil {
		t.Fatal("a canceled boot wait was reported as a build")
	}
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "the kernel build was canceled") {
		t.Errorf("a canceled build is not named as one:\n%v", err)
	}
	if strings.Contains(err.Error(), "within") || strings.Contains(err.Error(), "never booted") {
		t.Errorf("a canceled build is worded as a boot timeout:\n%v", err)
	}
}

// TestBootWaitOutOfTimeEndsWithTheQEMUCommand proves a build whose caller's
// deadline passes while the VM boots says so, and ends with the QEMU command
// that boots the same VM on a line of its own, so the operator can watch the
// console where it stalls.
//
// VALIDATES: round-5 NOTE 3.
// PREVENTS: a deadline reported with no next step, or as a guest that
// failed to boot within bootTimeout.
func TestBootWaitOutOfTimeEndsWithTheQEMUCommand(t *testing.T) {
	fakeQEMU(t, "exec sleep 30\n")
	req := Request{Root: t.TempDir(), Arch: "amd64", OutputDir: "out", Stderr: os.Stderr}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := runQEMUBuild(ctx, req, "alpine.iso", "worker", "kvm")
	if err == nil {
		t.Fatal("a boot wait past its deadline was reported as a build")
	}
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "ran out of the time its caller gave it") {
		t.Errorf("a build out of time is not named as one:\n%v", err)
	}
	if !strings.Contains(err.Error(), "stopped before its login prompt") {
		t.Errorf("the refusal does not name the stage the VM reached:\n%v", err)
	}
	lines := strings.Split(err.Error(), "\n")
	if last := lines[len(lines)-1]; !strings.HasPrefix(last, "qemu-system-x86_64 ") || !strings.Contains(last, "alpine.iso") {
		t.Errorf("the refusal does not end with the QEMU command on its own line:\n%v", err)
	}
}

// TestCanceledSSHSetupIsNotAnSSHFailure proves a build canceled while the VM
// sets up SSH, after its login prompt, is reported as canceled: the wait for
// SSHD_READY ended because the caller stopped the build, so the bootstrap is
// not retried and sshd is not blamed.
//
// VALIDATES: round-5 NOTE 2.
// PREVENTS: an operator who pressed Ctrl-C told that SSH never answered after
// five attempts, and sent to run sshd -d.
func TestCanceledSSHSetupIsNotAnSSHFailure(t *testing.T) {
	fakeQEMU(t, "echo login:\nexec sleep 30\n")
	req := Request{Root: t.TempDir(), Arch: "amd64", OutputDir: "out", Stderr: os.Stderr}
	ctx, cancel := context.WithCancel(context.Background())
	// The login prompt is answered at once; runQEMUBuild then waits 1 s and
	// 3 s before the first bootstrap, so at 5 s it is in the SSHD_READY wait.
	stop := time.AfterFunc(5*time.Second, cancel)
	defer stop.Stop()
	err := runQEMUBuild(ctx, req, "alpine.iso", "worker", "kvm")
	if err == nil {
		t.Fatal("a canceled SSH setup was reported as a build")
	}
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "stopped while it set up SSH: the kernel build was canceled") {
		t.Errorf("a build canceled during the SSH setup is not named as one:\n%v", err)
	}
	if strings.Contains(err.Error(), "never answered") || strings.Contains(err.Error(), "sshd -d") {
		t.Errorf("a canceled build is reported as an SSH failure:\n%v", err)
	}
}

// TestStoppedAfterLoginPromptNamesTheStage proves a build stopped in either
// pause between the login prompt and the first bootstrap is reported as the
// build stopping after its login prompt, never as a bare context error. The
// fake QEMU answers the prompt at once; runQEMUBuild then pauses 1 s, sends
// root, and pauses 3 s, so a stop at 300 ms lands in the first pause and one
// at 2 s in the second.
//
// VALIDATES: round-6 NOTE 1.
// PREVENTS: "context canceled" with no stage and no next step.
func TestStoppedAfterLoginPromptNamesTheStage(t *testing.T) {
	cases := []struct {
		name  string
		after time.Duration
	}{
		{"first pause", 300 * time.Millisecond},
		{"second pause", 2 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeQEMU(t, "echo login:\nexec sleep 30\n")
			req := Request{Root: t.TempDir(), Arch: "amd64", OutputDir: "out", Stderr: os.Stderr}
			ctx, cancel := context.WithCancel(context.Background())
			stop := time.AfterFunc(tc.after, cancel)
			defer stop.Stop()
			err := runQEMUBuild(ctx, req, "alpine.iso", "worker", "kvm")
			if err == nil {
				t.Fatal("a build stopped after its login prompt was reported as a build")
			}
			if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "stopped after its login prompt: the kernel build was canceled") {
				t.Errorf("a build stopped after its login prompt does not name the stage:\n%v", err)
			}
		})
	}
}
