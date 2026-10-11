package kernelbuilder

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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

// TestStoppedDuringGuestBuildNamesTheStage proves a build stopped while the
// kernel compiles in the build VM is named as the build stopping at that
// stage, never as "kernel build failed: signal: killed", which reads as a
// compile failure. A build out of Ze's limit (WithBuildLimit) names the limit,
// and a deadline ends with the ssh command that watches a running build, never
// the QEMU command, which boots a VM with no build in it. The fake QEMU prints the login prompt and
// SSHD_READY at once; the fake ssh answers the readiness probe and sleeps on
// the build command, so every stop lands in sshRun under buildCtx, about 4 s
// in. The kernel tarball is cached so nothing is downloaded.
//
// VALIDATES: round-7 ISSUE 2, round-8 ISSUE 2 and ISSUE 3.
// PREVENTS: an operator who pressed Ctrl-C, or whose build ran past Ze's
// limit, told the kernel failed to compile, or sent to a VM with no build.
func TestStoppedDuringGuestBuildNamesTheStage(t *testing.T) {
	cases := []struct {
		name    string
		context func() (context.Context, context.CancelFunc)
		want    error
		say     string
	}{
		{"canceled", func() (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(context.Background())
			stop := time.AfterFunc(6*time.Second, cancel)
			return ctx, func() { stop.Stop(); cancel() }
		}, context.Canceled, "stopped while it built the kernel: the kernel build was canceled"},
		{"caller deadline", func() (context.Context, context.CancelFunc) {
			return context.WithTimeout(context.Background(), 6*time.Second)
		}, context.DeadlineExceeded, "stopped while it built the kernel: the kernel build ran out of the time its caller gave it"},
		{"build limit", func() (context.Context, context.CancelFunc) {
			return WithBuildLimit(context.Background(), 6*time.Second)
		}, context.DeadlineExceeded, "stopped while it built the kernel: the kernel build did not finish within the 6s limit Ze gives one kernel build"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeQEMU(t, "echo login:\necho SSHD_READY\nexec sleep 60\n")
			fakeGuestBuildSSH(t)
			req := Request{Root: t.TempDir(), Arch: "amd64", Version: "6.1.1", Profile: "p", OutputDir: "out", Stderr: os.Stderr, Stdout: os.Stderr}
			cacheKernelTarball(t, req)
			ctx, cancel := tc.context()
			defer cancel()
			err := runQEMUBuild(ctx, req, "alpine.iso", "worker", "kvm")
			if err == nil {
				t.Fatal("a build stopped during the guest build was reported as a build")
			}
			if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), tc.say) {
				t.Errorf("a build stopped during the guest build does not name the stop, want %q (%v):\n%v", tc.say, tc.want, err)
			}
			if strings.Contains(err.Error(), "kernel build failed") || strings.Contains(err.Error(), "signal: killed") {
				t.Errorf("a stopped build is reported as a compile failure:\n%v", err)
			}
			if strings.Contains(err.Error(), "stall") || strings.Contains(err.Error(), "too slow") {
				t.Errorf("a stopped build guesses at a cause the code cannot tell:\n%v", err)
			}
			if errors.Is(tc.want, context.DeadlineExceeded) {
				lines := strings.Split(err.Error(), "\n")
				if last := lines[len(lines)-1]; !strings.HasPrefix(last, "ssh -t ") || !strings.HasSuffix(last, " root@localhost top") {
					t.Errorf("a build out of time does not end with the ssh command that watches a build:\n%v", err)
				}
				if strings.Contains(err.Error(), "qemu-system-x86_64 ") {
					t.Errorf("a build out of time sends the operator to a QEMU command that boots no build:\n%v", err)
				}
			}
		})
	}
}

// fakeGuestBuildSSH puts an ssh beside the fake QEMU that answers the
// readiness probe (`true`) at once and sleeps on any other command, so the
// guest build never ends on its own.
func fakeGuestBuildSSH(t *testing.T) {
	t.Helper()
	qemu, err := exec.LookPath("qemu-system-x86_64")
	if err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncase \" $* \" in *\" true \") exit 0 ;; esac\nexec sleep 60\n"
	if err := os.WriteFile(filepath.Join(filepath.Dir(qemu), "ssh"), []byte(script), 0o755); err != nil { //nolint:gosec // test executable
		t.Fatal(err)
	}
}

// cacheKernelTarball writes the kernel tarball where runQEMUBuild looks for
// it, so the build reaches the guest build without a download.
func cacheKernelTarball(t *testing.T, req Request) {
	t.Helper()
	build := filepath.Join(req.Root, "tmp", backendQEMU, "build", alpineArch(req.Arch))
	if err := os.MkdirAll(build, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(build, kernelTarballName(req.Version)), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
}
