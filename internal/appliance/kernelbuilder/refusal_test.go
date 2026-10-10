package kernelbuilder

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

// lastLine answers the final line of err, where a refusal puts its command.
func lastLine(err error) string {
	lines := strings.Split(err.Error(), "\n")
	return lines[len(lines)-1]
}

// TestBootFailureNamesAnEarlyExit proves a silent QEMU that exits at once is
// reported as an exit with its status, not as a guest that never booted
// within bootTimeout, and the refusal still ends with the QEMU command.
//
// VALIDATES: round-3 NOTE 1.
// PREVENTS: an operator waiting for a boot that QEMU never attempted.
func TestBootFailureNamesAnEarlyExit(t *testing.T) {
	fakeQEMU(t, "exec 1>&-\nexit 3\n")
	req := Request{Root: t.TempDir(), Arch: "amd64", OutputDir: "out", Stderr: os.Stderr}
	err := runQEMUBuild(context.Background(), req, "alpine.iso", "worker", "kvm")
	if err == nil {
		t.Fatal("a QEMU that exited at once was reported as a build")
	}
	if !strings.Contains(err.Error(), "qemu-system-x86_64 exited (exit status 3) before the build VM reached its login prompt") {
		t.Errorf("an early exit is not named as one:\n%v", err)
	}
	if strings.Contains(err.Error(), "within") {
		t.Errorf("an early exit is worded as a boot timeout:\n%v", err)
	}
	if !strings.HasPrefix(lastLine(err), "qemu-system-x86_64 ") {
		t.Errorf("refusal does not end with the QEMU command:\n%v", err)
	}
}

// TestSSHNeverAnsweredNamesTheCommand proves the SSH refusal ends with the
// QEMU command that boots the same VM.
//
// VALIDATES: round-3 ISSUE 4.
func TestSSHNeverAnsweredNamesTheCommand(t *testing.T) {
	err := errSSHNeverAnswered("qemu-system-aarch64", []string{"-accel", "hvf", "-cdrom", "alpine.iso"}, "")
	if got := lastLine(err); got != "qemu-system-aarch64 '-accel' 'hvf' '-cdrom' 'alpine.iso'" {
		t.Errorf("last line = %q, want the QEMU command:\n%v", got, err)
	}
	if !strings.Contains(err.Error(), "SSH into it never answered after 5 attempts") || !strings.Contains(err.Error(), "/usr/sbin/sshd -d") {
		t.Errorf("refusal does not name the cause and how to see it:\n%v", err)
	}
}

// TestCancelledProbeIsNotATimeout proves a host probe stopped by a canceled
// build says so, rather than blaming a hung QEMU or Docker daemon.
//
// VALIDATES: round-3 NOTE 9.
func TestCancelledProbeIsNotATimeout(t *testing.T) {
	fakeQEMU(t, "exec sleep 30\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := availableAccelerators(ctx, "qemu-system-x86_64")
	if err == nil || !strings.Contains(err.Error(), "the kernel build was canceled") || strings.Contains(err.Error(), "did not answer") {
		t.Errorf("a canceled -accel help reads as = %v", err)
	}

	old := runCommandOutput
	t.Cleanup(func() { runCommandOutput = old })
	runCommandOutput = func(ctx context.Context, _ Request, _ string, _ ...string) ([]byte, error) {
		return nil, ctx.Err()
	}
	_, err = dockerRootDir(ctx, Request{})
	if err == nil || !strings.Contains(err.Error(), "the kernel build was canceled") || !errors.Is(err, context.Canceled) {
		t.Errorf("a canceled docker info reads as = %v", err)
	}
}

// TestCheckArchRefusalsEndWithACommand proves both arch refusals end with the
// command that builds this host's kernel.
//
// VALIDATES: round-3 NOTE 6.
func TestCheckArchRefusalsEndWithACommand(t *testing.T) {
	hostIs(t, "amd64")
	for _, arch := range []string{"sparc", "arm64"} {
		err := CheckArch(arch)
		if err == nil {
			t.Fatalf("CheckArch(%s) on an amd64 host = nil", arch)
		}
		if got := lastLine(err); got != "ze appliance kernel --arch amd64" {
			t.Errorf("CheckArch(%s) last line = %q, want the command:\n%v", arch, got, err)
		}
	}
}
