//go:build integration

// Design: docs/architecture/testing/qemu-integration.md -- Docker labs in the Ze-kernel guest
// Related: dockerlab.go -- the action this test boots

package testqemu

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	testdeployment "github.com/ze-software/ze/internal/le/test/deployment"
)

// VALIDATES: AC-8 on the Mac (HVF, arm64) and AC-10 on the nightly (KVM,
// amd64), through the one docker-lab path: the Alpine guest boots Ze's cached
// runtime kernel (Run.assertRuntimeKernel refuses any other), Docker starts in
// it, and the Docker kernel check passes there (AC-5), with no lab after it.
// Method: run the action with no lab and read its exit code and report.
// PREVENTS: a guest route that boots but whose Docker never starts on Ze's
// kernel, or whose kernel lacks a feature Ze enrolls.
//
// Skips when the host has no QEMU for the guest's architecture, which is also
// how it behaves inside a guest, or when the kernel cache holds no runtime
// kernel for it: on 2026-10-10 the Mac's arm64 entries predate the D-5 Docker
// options, so `./ze appliance kernel --target runtime --arch arm64` is owed
// before this test can boot.
func TestDockerLabGuestBootsZeKernel(t *testing.T) {
	system := qemuSystemAMD64
	if GuestArch() == ArchARM64 {
		system = qemuSystemARM64
	}
	if _, err := exec.LookPath(system); err != nil {
		t.Skip("the Docker lab guest needs ", system, ": ", err)
	}
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	cache, err := testdeployment.RuntimeKernelCacheDir(root, GuestArch(), os.Stderr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cache, dockerLabKernelFile)); err != nil {
		t.Skip("no runtime kernel cached for ", GuestArch(), " at ", cache, ": ", err)
	}

	answer, code := runDockerLabHere(leaction.Arguments{keywordTimeout: {"1800s"}})
	if code != 0 {
		t.Fatalf("docker-lab with no lab exited %d: %+v", code, answer)
	}
}
