// Design: docs/research/vpp-deployment-reference.md -- exact driver override restoration
// Related: dpdk.go -- actual complete per-device native file reader
package vpp

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Real temporary device directories exercise the same complete reader used by
// readDriverOverride, without host sysfs or fabricated native error values.
func TestDPDKCleanupOverrideReaderRawUnset(t *testing.T) {
	dir := t.TempDir()
	writeOverrideFixture(t, dir, "driver_override_raw", "\n")
	writeOverrideFixture(t, dir, "driver_override", "legacy-policy\n")
	checkOverrideFixture(t, dir, "")
}

func TestDPDKCleanupOverrideReaderRawLiteralNull(t *testing.T) {
	dir := t.TempDir()
	writeOverrideFixture(t, dir, "driver_override_raw", "(null)\n")
	writeOverrideFixture(t, dir, "driver_override", "legacy-policy\n")
	checkOverrideFixture(t, dir, "(null)")
}

func TestDPDKCleanupOverrideReaderRawPreservesOriginalBytes(t *testing.T) {
	dir := t.TempDir()
	original := " \tpolicy \t\r"
	writeOverrideFixture(t, dir, "driver_override_raw", original+"\n")
	writeOverrideFixture(t, dir, "driver_override", "legacy-policy\n")
	checkOverrideFixture(t, dir, original)
}

func TestDPDKCleanupOverrideReaderLegacyPreservesOriginalBytes(t *testing.T) {
	dir := t.TempDir()
	original := " \tpolicy \t\r"
	writeOverrideFixture(t, dir, "driver_override", original+"\n")
	checkOverrideFixture(t, dir, original)
}

func TestDPDKCleanupOverrideReaderLegacyUnambiguous(t *testing.T) {
	dir := t.TempDir()
	writeOverrideFixture(t, dir, "driver_override", "original-policy\n")
	checkOverrideFixture(t, dir, "original-policy")
}

func TestDPDKCleanupOverrideReaderLegacyAmbiguousRefuses(t *testing.T) {
	dir := t.TempDir()
	writeOverrideFixture(t, dir, "driver_override", "(null)\n")
	_, err := readDriverOverrideAt(dir)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ambiguous legacy state lost the missing native getter cause: %v", err)
	}
	message := strings.ToLower(err.Error())
	if !strings.Contains(message, "upgrade") || !strings.Contains(message, "kernel") || !strings.Contains(message, "driver_override_raw") {
		t.Fatalf("ambiguous native state lacks an actionable kernel upgrade instruction: %v", err)
	}
}

func TestDPDKCleanupOverrideReaderRawFailureDoesNotFallBack(t *testing.T) {
	dir := t.TempDir()
	raw := filepath.Join(dir, "driver_override_raw")
	if err := os.Mkdir(raw, 0o700); err != nil {
		t.Fatal(err)
	}
	writeOverrideFixture(t, dir, "driver_override", "original-policy\n")
	_, err := readDriverOverrideAt(dir)
	var failure *os.PathError
	if !errors.As(err, &failure) {
		t.Fatalf("actual raw read failure was replaced by legacy success: %v", err)
	}
	if failure.Path != raw {
		t.Fatalf("reader lost its original raw failure path: %q", failure.Path)
	}
}

func TestDPDKCleanupOverrideReaderMissingBothKeepsCause(t *testing.T) {
	_, err := readDriverOverrideAt(t.TempDir())
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing native override files lost their actual cause: %v", err)
	}
}

func TestDPDKCleanupOverrideReaderAmbiguousRefusesBeforeMutation(t *testing.T) {
	dir := t.TempDir()
	writeOverrideFixture(t, dir, "driver_override", "(null)\n")
	fixture := newDPDKCleanupFixture(t)
	fixture.ops.readDriverOverride = func(pci string) (string, error) {
		fixture.checkPCI(pci)
		return readDriverOverrideAt(dir)
	}
	err := fixture.binder.bindPCIWithOps(dpdkCleanupPCI, fixture.ops)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ambiguous original override was admitted: %v", err)
	}
	if fixture.driver != "ixgbe" || fixture.override != dpdkCleanupOverride {
		t.Fatal("ambiguous-state refusal changed original native state")
	}
	if fixture.detachCalls != 0 || fixture.bindCalls != 0 || fixture.unbindCalls != 0 || fixture.rebindCalls != 0 || len(fixture.overrideWrites) != 0 {
		t.Fatal("ambiguous-state refusal issued a native device write")
	}
	if len(fixture.binder.savedDrivers) != 0 {
		t.Fatal("ambiguous-state refusal acquired original cleanup custody")
	}
}

func writeOverrideFixture(t *testing.T, dir, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func checkOverrideFixture(t *testing.T, dir, original string) {
	t.Helper()
	actual, err := readDriverOverrideAt(dir)
	if err != nil {
		t.Fatal(err)
	}
	if actual != original {
		t.Fatalf("original override bytes changed: got %q, want %q", actual, original)
	}
}

// Nonempty saved payloads may already reach the kernel store's length limit.
// The actual writer must not add a byte; only clearing needs a framing newline.
func TestDPDKCleanupOverrideWriterPreservesNonemptyPayload(t *testing.T) {
	for _, original := range []string{"policy", strings.Repeat("a", os.Getpagesize()-2)} {
		dir := t.TempDir()
		path := filepath.Join(dir, "driver_override")
		writeOverrideFixture(t, dir, "driver_override", "previous")
		if err := setDriverOverrideAt(dir, original); err != nil {
			t.Fatal(err)
		}
		actual, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(actual) != original {
			t.Fatalf("saved nonempty payload gained or lost bytes: got length %d, want %d", len(actual), len(original))
		}
	}
}

func TestDPDKCleanupOverrideWriterClearsWithOneNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "driver_override")
	writeOverrideFixture(t, dir, "driver_override", "previous")
	if err := setDriverOverrideAt(dir, ""); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != "\n" {
		t.Fatalf("empty override did not emit exactly its clearing newline: %q", actual)
	}
}
