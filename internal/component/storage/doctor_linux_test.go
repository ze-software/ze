//go:build linux

// Design: docs/architecture/storage/smart-health.md -- the readiness check this component owns
// Detail: doctor_linux.go -- checkSmartEnabled and its registration

package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/smart"
)

// smartTree builds the smallest config that enables SMART management.
func smartTree(enabled bool) *config.Tree {
	tree := config.NewTree()
	cfg := tree.GetOrCreateContainer("storage").GetOrCreateContainer("smart")
	if enabled {
		cfg.Set("enabled", "true")
	}
	return tree
}

// withBlockDevices stands in a sysfs holding the named whole devices, plus a
// partition under the first one so the walk has something to skip, and a
// SMART query that answers unavailable for every device when locked is set.
// The names are read from a temporary directory the walk is pointed at, so the
// partition marker is a real file rather than a stub.
func withBlockDevices(t *testing.T, locked bool, names ...string) {
	t.Helper()
	root := t.TempDir()
	for i, name := range names {
		if err := os.MkdirAll(filepath.Join(root, name), 0o750); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			part := filepath.Join(root, name+"p1")
			if err := os.MkdirAll(part, 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(part, "partition"), []byte("1\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	previousRead, previousDetect := smartReadBlockDir, smartDetect
	smartReadBlockDir = func(string) ([]os.DirEntry, error) { return os.ReadDir(root) }
	smartDetect = func(string, string) *smart.Info { return &smart.Info{Healthy: true, Unavailable: locked} }
	t.Cleanup(func() {
		smartReadBlockDir = previousRead
		smartDetect = previousDetect
	})
}

// TestCheckSmartEnabledWarnsWhenNoDeviceAnswers drives the check over an
// enabled config on a host whose devices all refuse SMART.
//
// VALIDATES: doctor-smart-access at warning severity, and the partition under
// the first device is not counted as a device.
// PREVENTS: SMART management enabled in a process that can never read a
// device, found only in the manager's log.
func TestCheckSmartEnabledWarnsWhenNoDeviceAnswers(t *testing.T) {
	withBlockDevices(t, true, "nvme0n1", "sda")

	diags := checkSmartEnabled(diagnostic.DoctorCheckContext{Tree: smartTree(true)})
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %d, want 1: %+v", len(diags), diags)
	}
	if diags[0].Code != codeSmartAccess || diags[0].Severity != diagnostic.SeverityWarning {
		t.Fatalf("diagnostic = %s/%s, want %s at warning", diags[0].Code, diags[0].Severity, codeSmartAccess)
	}
}

// TestCheckSmartEnabledWarnsWhenSysfsCannotBeRead covers the first verdict.
//
// VALIDATES: doctor-smart-sysfs at warning severity, carrying the read error.
// PREVENTS: an unreadable sysfs reading as a host with no devices, which the
// check would pass in silence.
func TestCheckSmartEnabledWarnsWhenSysfsCannotBeRead(t *testing.T) {
	previous := smartReadBlockDir
	smartReadBlockDir = func(string) ([]os.DirEntry, error) { return nil, errors.New("permission denied") }
	t.Cleanup(func() { smartReadBlockDir = previous })

	diags := checkSmartEnabled(diagnostic.DoctorCheckContext{Tree: smartTree(true)})
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %d, want 1: %+v", len(diags), diags)
	}
	if diags[0].Code != codeSmartSysfs || !strings.Contains(diags[0].Message, "permission denied") {
		t.Fatalf("diagnostic = %+v, want %s carrying the error", diags[0], codeSmartSysfs)
	}
}

// TestCheckSmartEnabledIsSilentWhenNotNeeded is the negative half: a device
// that answers, a host with no whole device, a disabled config, no config, and
// no tree.
//
// VALIDATES: no diagnostic in each case.
// PREVENTS: a SMART warning on every box that never enabled it, and on one
// whose devices answer.
func TestCheckSmartEnabledIsSilentWhenNotNeeded(t *testing.T) {
	withBlockDevices(t, false, "nvme0n1")
	if diags := checkSmartEnabled(diagnostic.DoctorCheckContext{Tree: smartTree(true)}); len(diags) != 0 {
		t.Fatalf("device answers: diagnostics = %d, want 0: %+v", len(diags), diags)
	}

	withBlockDevices(t, true)
	if diags := checkSmartEnabled(diagnostic.DoctorCheckContext{Tree: smartTree(true)}); len(diags) != 0 {
		t.Fatalf("no device: diagnostics = %d, want 0: %+v", len(diags), diags)
	}

	withBlockDevices(t, true, "sda")
	if diags := checkSmartEnabled(diagnostic.DoctorCheckContext{Tree: smartTree(false)}); len(diags) != 0 {
		t.Fatalf("disabled: diagnostics = %d, want 0: %+v", len(diags), diags)
	}
	if diags := checkSmartEnabled(diagnostic.DoctorCheckContext{Tree: config.NewTree()}); len(diags) != 0 {
		t.Fatalf("no block: diagnostics = %d, want 0: %+v", len(diags), diags)
	}
	if diags := checkSmartEnabled(diagnostic.DoctorCheckContext{}); len(diags) != 0 {
		t.Fatalf("nil tree: diagnostics = %d, want 0: %+v", len(diags), diags)
	}
}

// TestSmartDoctorCheckRegistered asks the registry the doctor runner reads
// whether it holds this component's check, at the phase and order the
// declaration states, and whether every code it emits resolves for
// `ze explain`.
//
// VALIDATES: the init() in register_linux.go installed the check and the
// registry accepted it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green, and a code an operator cannot look up.
func TestSmartDoctorCheckRegistered(t *testing.T) {
	want := smartDoctorCheck
	var found *diagnostic.DoctorCheck
	checks := diagnostic.DoctorChecksForPhase(want.Phase)
	for i := range checks {
		if checks[i].Name == want.Name {
			found = &checks[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("doctor check %q is not registered for phase %q", want.Name, want.Phase)
	}
	if found.Order != want.Order {
		t.Fatalf("order = %d, want %d", found.Order, want.Order)
	}
	if found.Component != want.Component {
		t.Fatalf("component = %q, want %q", found.Component, want.Component)
	}

	diagnostic.RegisterBuiltinCodes()
	for _, code := range want.Codes {
		if diagnostic.Lookup(code) == nil {
			t.Errorf("diagnostic code %q is not registered", code)
		}
	}
}
