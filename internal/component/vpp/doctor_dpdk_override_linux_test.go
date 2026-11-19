//go:build linux

// Design: docs/research/vpp-deployment-reference.md -- exact original override readiness
// Related: dpdk_override_reader_test.go -- complete native file reader controls

package vpp

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// Doctor must reject an ambiguous original override before binding. The real
// legacy file and absent raw file preserve the kernel capability failure.
func TestCheckVPPDPDKOriginalOverrideAmbiguous(t *testing.T) {
	f := newDPDKOverrideDoctorFixture(t)
	writeOverrideFixture(t, f.directory, "driver_override", "(null)\n")
	diags := checkVPPDPDK(diagnostic.DoctorCheckContext{Tree: dpdkTree(dpdkCleanupPCI)})
	if len(diags) != 1 {
		t.Fatalf("ambiguous original override diagnostics = %d, want 1: %+v", len(diags), diags)
	}
	if !errors.Is(f.failure, os.ErrNotExist) {
		t.Fatalf("original missing raw getter cause lost: %v", f.failure)
	}
	f.requireFailure(t, diags[0])
	fix := strings.ToLower(diags[0].Fix)
	if !strings.Contains(fix, "upgrade") || !strings.Contains(fix, "kernel") || !strings.Contains(fix, "driver_override_raw") {
		t.Fatalf("missing truthful kernel capability recovery: %q", diags[0].Fix)
	}
	f.requireReadOnly(t, "ixgbe")
}

// A real raw-read failure must reach doctor rather than be replaced by the
// readable legacy policy. A directory gives the same PathError as the reader.
func TestCheckVPPDPDKOriginalOverrideRawFailure(t *testing.T) {
	f := newDPDKOverrideDoctorFixture(t)
	if err := os.Mkdir(filepath.Join(f.directory, "driver_override_raw"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeOverrideFixture(t, f.directory, "driver_override", "legacy-policy\n")
	diags := checkVPPDPDK(diagnostic.DoctorCheckContext{Tree: dpdkTree(dpdkCleanupPCI)})
	if len(diags) != 1 {
		t.Fatalf("original raw read failure diagnostics = %d, want 1: %+v", len(diags), diags)
	}
	var cause *os.PathError
	if !errors.As(f.failure, &cause) || cause.Path != filepath.Join(f.directory, "driver_override_raw") {
		t.Fatalf("original raw read cause lost: %v", f.failure)
	}
	f.requireFailure(t, diags[0])
	f.requireReadOnly(t, "ixgbe")
}

// A raw literal policy remains restorable even when the legacy view disagrees;
// doctor uses the same capture producer as binding and preserves its bytes.
func TestCheckVPPDPDKOriginalOverrideRawReady(t *testing.T) {
	f := newDPDKOverrideDoctorFixture(t)
	writeOverrideFixture(t, f.directory, "driver_override_raw", "(null)\n")
	writeOverrideFixture(t, f.directory, "driver_override", "legacy-policy\n")
	if diags := checkVPPDPDK(diagnostic.DoctorCheckContext{Tree: dpdkTree(dpdkCleanupPCI)}); len(diags) != 0 {
		t.Fatalf("restorable raw override diagnostics = %d, want 0: %+v", len(diags), diags)
	}
	if f.failure != nil || f.original != (dpdkDriverRestore{driver: "ixgbe", override: "(null)"}) {
		t.Fatalf("doctor changed original capture: %+v, %v", f.original, f.failure)
	}
	f.requireReadOnly(t, "ixgbe")
}

// Borrowed VFIO state needs no original override and must not acquire custody.
// An absent raw/legacy pair therefore cannot create a new dependency for it.
func TestCheckVPPDPDKOriginalOverrideBorrowedVFIO(t *testing.T) {
	f := newDPDKOverrideDoctorFixture(t)
	f.native.driver = "vfio-pci"
	if diags := checkVPPDPDK(diagnostic.DoctorCheckContext{Tree: dpdkTree(dpdkCleanupPCI)}); len(diags) != 0 {
		t.Fatalf("borrowed VFIO diagnostics = %d, want 0: %+v", len(diags), diags)
	}
	if f.failure != nil || f.original.driver != "vfio-pci" {
		t.Fatalf("borrowed original state lost: %+v, %v", f.original, f.failure)
	}
	f.requireReadOnly(t, "vfio-pci")
}

type dpdkOverrideDoctorFixture struct {
	native    *dpdkCleanupFixture
	directory string
	reads     int
	original  dpdkDriverRestore
	failure   error
}

func newDPDKOverrideDoctorFixture(t *testing.T) *dpdkOverrideDoctorFixture {
	t.Helper()
	loaded := make(map[string]bool, len(vfioModules))
	for _, module := range vfioModules {
		loaded[module] = true
	}
	withDPDKHost(t, loaded, nil)
	f := &dpdkOverrideDoctorFixture{native: newDPDKCleanupFixture(t), directory: t.TempDir()}
	f.native.ops.readDriverOverride = func(pci string) (string, error) {
		f.native.checkPCI(pci)
		return readDriverOverrideAt(f.directory)
	}
	previous := dpdkOriginalState
	dpdkOriginalState = func(pci string) (dpdkDriverRestore, error) {
		f.reads++
		f.original, f.failure = readDPDKOriginalState(pci, f.native.ops)
		return f.original, f.failure
	}
	t.Cleanup(func() { dpdkOriginalState = previous })
	return f
}

func (f *dpdkOverrideDoctorFixture) requireFailure(t *testing.T, diag diagnostic.Diagnostic) {
	t.Helper()
	if diag.Code != doctorVPPDPDKCode || diag.Severity != diagnostic.SeverityError {
		t.Fatalf("original capture diagnostic = %+v", diag)
	}
	if diag.Actual != f.failure.Error() || !strings.Contains(diag.Message, dpdkCleanupPCI) {
		t.Fatalf("diagnostic lost original PCI/cause: %+v, %v", diag, f.failure)
	}
	if diag.Path != filepath.Join(f.directory, "driver_override_raw") || diag.Fix == "" {
		t.Fatalf("diagnostic lost original path/recovery: %+v", diag)
	}
}

func (f *dpdkOverrideDoctorFixture) requireReadOnly(t *testing.T, driver string) {
	t.Helper()
	if f.reads != 1 || f.native.driver != driver || f.native.override != dpdkCleanupOverride {
		t.Fatalf("doctor changed original state: reads=%d driver=%q override=%q", f.reads, f.native.driver, f.native.override)
	}
	if f.native.detachCalls != 0 || f.native.bindCalls != 0 || f.native.unbindCalls != 0 || f.native.rebindCalls != 0 || len(f.native.overrideWrites) != 0 || len(f.native.binder.savedDrivers) != 0 {
		t.Fatal("doctor issued a device mutation or acquired cleanup custody")
	}
}
