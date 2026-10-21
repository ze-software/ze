//go:build integration

// Design: docs/architecture/testing/qemu-integration.md -- Docker labs on the docker-host image
// Related: dockerlab.go -- the action this test boots

package testqemu

import (
	"os/exec"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// VALIDATES: spec AC-9 on the Mac (HVF, arm64) and on the nightly (KVM,
// amd64), through the one docker-lab path: the docker-host image for the
// host's architecture boots, its dockerd answers over the forwarded socket,
// and the host le's Docker kernel check passes against it, with no lab after
// it.
// Method: run the action with no lab and read its exit code and report.
// PREVENTS: a guest route that boots but whose Docker never answers, or whose
// kernel lacks a feature Ze enrolls.
//
// Skips when the host has no QEMU for its architecture. The image is built or
// taken from the docker-host cache by the action itself (EnsureImage).
func TestDockerLabGuestBootsZeKernel(t *testing.T) {
	system := qemuSystemAMD64
	if GuestArch() == ArchARM64 {
		system = qemuSystemARM64
	}
	if _, err := exec.LookPath(system); err != nil {
		t.Skip("the Docker lab guest needs ", system, ": ", err)
	}

	answer, code := runDockerLabHere(leaction.Arguments{keywordTimeout: {"1800s"}})
	if code != 0 {
		t.Fatalf("docker-lab with no lab exited %d: %+v", code, answer)
	}
}
