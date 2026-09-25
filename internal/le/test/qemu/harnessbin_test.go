package testqemu

import (
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// TestQemuHarnessVariableHasOneDefault proves le.qemu.test.bin has exactly one
// default, derived from the guest architecture (AC-10).
//
// Method: the registered entry carries no default, so no second reader can
// take a fixed one. With both spellings unset, qemuTestBin answers the
// cross-build for QEMU_GOARCH, for each architecture. The keep-alive hint and
// netnsGuestBinaries both call qemuTestBin.
//
// VALIDATES: AC-10, one default for le.qemu.test.bin.
// PREVENTS: the fixed arm64 default returning beside the derived one.
func TestQemuHarnessVariableHasOneDefault(t *testing.T) {
	if runTestBinEntry.Default != "" {
		t.Fatalf("le.qemu.test.bin registers the default %q, want none: qemuTestBin derives it", runTestBinEntry.Default)
	}
	t.Setenv("LE_QEMU_TEST_BIN", "")
	t.Setenv("ZE_QEMU_TEST_BIN", "")
	for _, arch := range []string{ArchAMD64, ArchARM64} {
		t.Setenv("QEMU_GOARCH", arch)
		env.ResetCache()
		want := filepath.Join("bin", "le-test-linux-"+arch)
		if got := qemuTestBin(); got != want {
			t.Errorf("QEMU_GOARCH=%s: qemuTestBin() = %q, want %q", arch, got, want)
		}
	}
	t.Cleanup(env.ResetCache)
}

// TestQemuHarnessVariableReadsBothSpellings proves the retired spelling still
// answers, and that the LE_ spelling wins when both are set (AC-9).
func TestQemuHarnessVariableReadsBothSpellings(t *testing.T) {
	t.Cleanup(env.ResetCache)
	t.Setenv("LE_QEMU_TEST_BIN", "")
	t.Setenv("ZE_QEMU_TEST_BIN", "bin/old")
	env.ResetCache()
	if got := qemuTestBin(); got != "bin/old" {
		t.Errorf("ZE_QEMU_TEST_BIN alone: qemuTestBin() = %q, want bin/old", got)
	}
	t.Setenv("LE_QEMU_TEST_BIN", "bin/new")
	env.ResetCache()
	if got := qemuTestBin(); got != "bin/new" {
		t.Errorf("both set: qemuTestBin() = %q, want bin/new", got)
	}
}
