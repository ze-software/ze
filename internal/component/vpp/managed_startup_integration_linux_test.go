//go:build integration && linux

// Design: docs/architecture/vpp-host-tuning.md -- original startup file type
// The original FIFO question runs through MAIN's scoped Ze-kernel carrier.
package vpp

import (
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestManagedStartupCLIRejectsFIFOWithoutBlocking(t *testing.T) {
	path := filepath.Join(t.TempDir(), "commands.fifo")
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	settings := defaultTestSettings()
	settings.StartupCLIFile = path
	if err := checkManagedStartupCLI(settings, openStartupCLIFile); err == nil {
		t.Fatal("nonregular FIFO accepted as managed startup commands")
	}
}
