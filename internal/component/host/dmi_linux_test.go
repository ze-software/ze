//go:build linux

package host

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: AC-5 — `show host dmi` returns system/board/bios/chassis
// fields read from /sys/class/dmi/id/. Fields not present in firmware
// remain empty (JSON omits them).
func TestDetectDMI_FullFields(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	dmi, err := d.DetectDMI()
	if err != nil {
		t.Fatalf("DetectDMI: %v", err)
	}
	if dmi == nil {
		t.Fatal("DMIInfo nil")
	}
	// Whitebox N100 board reports all "Default string" plus real BIOS
	// fields.
	if got, want := dmi.BIOSVendor, "American Megatrends International, LLC."; got != want {
		t.Errorf("BIOSVendor = %q, want %q", got, want)
	}
	if got, want := dmi.BIOSVersion, "BKHD1264NP4LV11R007A"; got != want {
		t.Errorf("BIOSVersion = %q, want %q", got, want)
	}
	if got, want := dmi.BIOSDate, "11/20/2023"; got != want {
		t.Errorf("BIOSDate = %q, want %q", got, want)
	}
	if got, want := dmi.BIOSRevision, "5.26"; got != want {
		t.Errorf("BIOSRevision = %q, want %q", got, want)
	}
	if dmi.SystemVendor == "" {
		t.Error("SystemVendor empty")
	}
	if dmi.BoardVendor == "" {
		t.Error("BoardVendor empty")
	}
	if dmi.ChassisType == "" {
		t.Error("ChassisType empty")
	}
}

// VALIDATES: AC-21 — missing sysfs file is non-fatal; absent fields
// stay empty, no error recorded. Fixture has only BIOS + system
// fields; board_version, board_serial, chassis_serial absent.
func TestDetectDMI_MissingFields(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	dmi, err := d.DetectDMI()
	if err != nil {
		t.Fatalf("DetectDMI: %v", err)
	}
	if dmi.BoardVersion != "" {
		t.Errorf("BoardVersion = %q, want empty (absent in fixture)", dmi.BoardVersion)
	}
	if dmi.BoardSerial != "" {
		t.Errorf("BoardSerial = %q, want empty", dmi.BoardSerial)
	}
	if dmi.ChassisSerial != "" {
		t.Errorf("ChassisSerial = %q, want empty", dmi.ChassisSerial)
	}
}

// VALIDATES: AC-21 / platform gating — detector returns ErrUnsupported
// when /sys/class/dmi/id/ doesn't exist (container without /sys DMI or
// a non-x86 board).
func TestDetectDMI_MissingDir(t *testing.T) {
	d := &Detector{Root: "testdata/nonexistent-no-dmi"}
	_, err := d.DetectDMI()
	if err == nil {
		t.Fatal("expected error for missing dmi dir, got nil")
	}
}

// VALIDATES: AC-20 — permission error reading a sysfs DMI field is
// NON-fatal. DetectDMI returns StatusDone equivalent (no top-level
// error), populates the readable fields, AND records the unreadable
// path in DMIInfo.Errors with the wrapped error.
// PREVENTS: silent empty section on boxes with partially-locked-down
// sysfs (operator cannot distinguish "no hardware" from "no read
// permission").
func TestDetectDMI_PermissionDenied(t *testing.T) {
	// Skip when the test process can bypass DAC (running as root or
	// with CAP_DAC_READ_SEARCH). The fixture chmod would be a no-op.
	if os.Geteuid() == 0 {
		t.Skip("root bypasses DAC; permission-denied path untestable as root")
	}

	// Build a throwaway DMI tree under t.TempDir so the 0000-mode file
	// never contaminates the repo.
	root := t.TempDir()
	dmiDir := filepath.Join(root, "sys", "class", "dmi", "id")
	if err := os.MkdirAll(dmiDir, 0o755); err != nil {
		t.Fatalf("mkdir dmiDir: %v", err)
	}
	// Readable field — must still populate.
	readable := filepath.Join(dmiDir, "sys_vendor")
	if err := os.WriteFile(readable, []byte("KnownVendor\n"), 0o644); err != nil {
		t.Fatalf("write sys_vendor: %v", err)
	}
	// Unreadable field — must record in Errors.
	locked := filepath.Join(dmiDir, "product_serial")
	if err := os.WriteFile(locked, []byte("SECRET\n"), 0o000); err != nil {
		t.Fatalf("write product_serial: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o644) }) // so t.TempDir cleanup can remove it

	d := &Detector{Root: root}
	dmi, err := d.DetectDMI()
	if err != nil {
		t.Fatalf("DetectDMI top-level error: %v (want non-fatal)", err)
	}
	if dmi == nil {
		t.Fatal("DMIInfo nil")
	}
	if dmi.SystemVendor != "KnownVendor" {
		t.Errorf("SystemVendor = %q, want KnownVendor (readable field must still populate)", dmi.SystemVendor)
	}
	if dmi.SystemSerial != "" {
		t.Errorf("SystemSerial = %q, want empty (file was unreadable)", dmi.SystemSerial)
	}
	if len(dmi.Errors) == 0 {
		t.Fatal("Errors empty; want one entry for the unreadable path")
	}
	found := false
	for _, e := range dmi.Errors {
		if strings.HasSuffix(e.Path, "product_serial") {
			found = true
			if !strings.Contains(strings.ToLower(e.Err), "permission") &&
				!strings.Contains(strings.ToLower(e.Err), "denied") {
				t.Errorf("Errors entry for product_serial = %q, want a permission-denied message", e.Err)
			}
		}
	}
	if !found {
		t.Errorf("Errors did not contain product_serial entry: %+v", dmi.Errors)
	}
}
