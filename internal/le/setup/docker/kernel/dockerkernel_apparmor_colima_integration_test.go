//go:build integration

// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: dockerkernel_apparmor_colima.go -- the macOS half this test drives
//
// Run by hand on a Mac whose Docker client uses Colima:
//
//	./le job run label colima-apparmor command go test -tags integration -count=1 \
//	  -run '^TestAppArmorColimaHostOnThisMac$' ./internal/le/setup/docker/kernel/

package dockerkernel

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/le/dockerhost"
	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// VALIDATES: against the real Colima VM this Mac's Docker client uses, the
// action finds the VM, reads its AppArmor state and its apparmor_parser
// without root, and the VM reads a profile staged under this checkout's tmp/,
// so the two root steps it plans can succeed.
// METHOD: every non-root question the action asks runs for real; the root
// steps are printed, never run.
// PREVENTS: a Colima route proven only against a scripted VM.
func TestAppArmorColimaHostOnThisMac(t *testing.T) {
	if runtime.GOOS != osDarwin {
		t.Skip("the Colima route runs on macOS only")
	}
	host, err := appArmorDarwinHost(t.Context(), runVMCommand, dockerhost.FileExists)
	if err != nil {
		t.Fatalf("the Colima VM refused: %v", err)
	}
	t.Logf("profile %s, parser %s, via %q", host.profile, host.parser, host.via)
	profile, registered := interoplab.LookupAppArmorProfile(kernelcap.ProbeAppArmorProfileName)
	if !registered {
		t.Fatal("ze-kernel-probe is not registered")
	}
	// The action stages under the checkout's tmp/, which Colima's home mount
	// shows the VM; t.TempDir is under /var/folders, which it does not.
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find the checkout: %v", err)
	}
	dir, err := os.MkdirTemp(filepath.Join(root, "tmp"), "apparmor-test-")
	if err != nil {
		t.Fatalf("make the staging directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	staged, err := stageAppArmorProfile(dir, profile)
	if err != nil {
		t.Fatalf("stage the profile: %v", err)
	}
	if err := appArmorStagedVisible(t.Context(), host, staged, runVMCommand); err != nil {
		t.Fatalf("the VM cannot read the staged profile: %v", err)
	}
	for _, step := range appArmorLoadSteps(profile, staged, host) {
		t.Logf("root step: %s", strings.Join(step.Argv, " "))
	}
}
