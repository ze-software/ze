package kernelbuilder

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

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

// TestSupportedArchesAreTheBuildableOnes proves the arch list a refusal offers
// is derived from the backend's platforms: the Go names, once each.
//
// VALIDATES: round-4 NOTE 1.
func TestSupportedArchesAreTheBuildableOnes(t *testing.T) {
	if got := strings.Join(SupportedArches(), ","); got != "amd64,arm64" {
		t.Errorf("SupportedArches() = %s, want amd64,arm64", got)
	}
	cases := map[string]bool{"amd64": true, "x86_64": true, "arm64": true, "aarch64": true, "riscv64": false, "": false}
	for arch, want := range cases {
		if got := ArchSupported(arch); got != want {
			t.Errorf("ArchSupported(%q) = %v, want %v", arch, got, want)
		}
	}
}
