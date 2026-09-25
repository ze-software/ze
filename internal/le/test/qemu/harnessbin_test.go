package testqemu

import (
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// TestQemuGuestRunsLinuxLe proves the guest harness is a linuxle build for the
// guest architecture under tmp/, and that no harness variable names it (AC-40).
// Method: guestLeRel for each guest architecture, then the env registry.
//
// VALIDATES: the guest le lives at tmp/qemu/linux-<arch>/le and neither
// le.qemu.test.bin nor ze.qemu.test.bin is registered.
// PREVENTS: a guest running a harness file a variable points at, rather than
// the le built from this checkout.
func TestQemuGuestRunsLinuxLe(t *testing.T) {
	for _, arch := range []string{ArchAMD64, ArchARM64} {
		want := filepath.Join("tmp", "qemu", "linux-"+arch, "le")
		if got := guestLeRel(arch); got != want {
			t.Errorf("guestLeRel(%s) = %q, want %q", arch, got, want)
		}
	}
	for _, key := range []string{"le.qemu.test.bin", "ze.qemu.test.bin", "le.test.bin", "ze.test.bin"} {
		if env.IsRegistered(key) {
			t.Errorf("%s is still registered: the guest harness is the linuxle build, not a variable", key)
		}
	}
}
