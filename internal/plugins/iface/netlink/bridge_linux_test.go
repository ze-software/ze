//go:build linux

package ifacenetlink

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBridgeSetSTPSysfsWrite(t *testing.T) {
	// VALIDATES: BridgeSetSTP writes correct values to sysfs.
	// PREVENTS: STP enable/disable not reaching the kernel.
	dir := t.TempDir()
	stpDir := filepath.Join(dir, "br0", "bridge")
	if err := os.MkdirAll(stpDir, 0o755); err != nil {
		t.Fatalf("create stp dir: %v", err)
	}

	old := bridgeSysfsRoot
	bridgeSysfsRoot = dir
	t.Cleanup(func() { bridgeSysfsRoot = old })

	b := &netlinkBackend{}

	if err := b.BridgeSetSTP("br0", true); err != nil {
		t.Fatalf("BridgeSetSTP(true): %v", err)
	}
	data, err := os.ReadFile(filepath.Join(stpDir, "stp_state"))
	if err != nil {
		t.Fatalf("read stp_state: %v", err)
	}
	if string(data) != "1" {
		t.Errorf("stp=true: got %q, want %q", string(data), "1")
	}

	if err := b.BridgeSetSTP("br0", false); err != nil {
		t.Fatalf("BridgeSetSTP(false): %v", err)
	}
	data, err = os.ReadFile(filepath.Join(stpDir, "stp_state"))
	if err != nil {
		t.Fatalf("read stp_state: %v", err)
	}
	if string(data) != "0" {
		t.Errorf("stp=false: got %q, want %q", string(data), "0")
	}
}
