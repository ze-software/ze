// Design: docs/architecture/testing/qemu-integration.md -- split UEFI firmware.

package testqemu

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestISOSplitFirmwareUsesPrivateFlashSnapshot(t *testing.T) {
	installer := installTestInstaller(t, InstallKindISO)
	firmware := filepath.Join(t.TempDir(), "OVMF_CODE_4M.fd")
	vars := strings.Replace(firmware, "_CODE", "_VARS", 1)
	for _, path := range []string{firmware, vars} {
		if err := os.WriteFile(path, []byte("firmware"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	argv, err := installer.ISOArgv("install.iso", "target", "extra", firmware)
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{
		"if=pflash,unit=0,format=raw,readonly=on,file=" + firmware,
		"if=pflash,unit=1,format=raw,snapshot=on,file=" + vars,
	} {
		if !slices.Contains(argv, token) {
			t.Errorf("split UEFI invocation misses %q: %q", token, argv)
		}
	}
	if slices.Contains(argv, "-bios") {
		t.Error("split CODE firmware was passed as a complete BIOS image")
	}
}

func TestISOSplitFirmwareRequiresMatchingVariableImage(t *testing.T) {
	installer := installTestInstaller(t, InstallKindISO)
	firmware := filepath.Join(t.TempDir(), "OVMF_CODE_4M.fd")
	if err := os.WriteFile(firmware, []byte("firmware"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := installer.ISOArgv("install.iso", "target", "extra", firmware); err == nil {
		t.Error("split firmware without its matching VARS image was accepted")
	}
}
