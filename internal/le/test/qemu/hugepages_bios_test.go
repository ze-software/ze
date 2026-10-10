package testqemu

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// VALIDATES: review round 1 NOTE 6 of spec-appliance-ships-ze-kernel. The
// hugepage, mpls-boot and crash-capture proofs find the aarch64 UEFI firmware
// the way `le test qemu run` does (Homebrew's prefixes first), so an arm64 run
// on a Mac needs no ze_vpp_hp_aarch64_bios.
// PREVENTS: the arm64 proofs skipping with "firmware not found" at the Linux
// path on a Mac that holds Homebrew's firmware.
// Method: a fixture run whose HOMEBREW_PREFIX holds the firmware file.
func TestHugepagesFindsTheHomebrewArm64Firmware(t *testing.T) {
	run := fixtureRun(t, ArchARM64)
	prefix := filepath.Join(run.Tree, "brew")
	bios := filepath.Join(prefix, "share", "qemu", "edk2-aarch64-code.fd")
	if err := os.MkdirAll(filepath.Dir(bios), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bios, []byte("firmware"), 0o600); err != nil {
		t.Fatal(err)
	}
	ops := run.ops
	ops.Getenv = func(name string) string {
		if name == "HOMEBREW_PREFIX" {
			return prefix
		}
		return ""
	}
	if got := hostArm64BIOS(ops); got != bios {
		t.Fatalf("hostArm64BIOS = %q, want the Homebrew firmware %q", got, bios)
	}
}

// VALIDATES: with no Homebrew firmware, the proofs keep the documented Linux
// path, so their skip names the path they looked at last.
// Method: a Linux host with no brew and no HOMEBREW_PREFIX.
func TestHugepagesFallsBackToTheLinuxFirmwarePath(t *testing.T) {
	run := fixtureRun(t, ArchARM64)
	ops := run.ops
	ops.GOOS = "linux"
	ops.Getenv = func(string) string { return "" }
	ops.Look = func(string) (string, error) { return "", errors.New("no brew") }
	if got := hostArm64BIOS(ops); got != DefaultBios {
		t.Fatalf("hostArm64BIOS = %q, want the Linux default %q", got, DefaultBios)
	}
}
