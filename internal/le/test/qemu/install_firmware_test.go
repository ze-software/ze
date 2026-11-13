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

// VALIDATES: QEMU's bundled edk2 code images pair with the variable image
// QEMU's own firmware descriptor names, for both target architectures.
// METHOD: lay out Homebrew's share/qemu shape (code, vars, firmware/*.json
// with Cellar paths) in a temporary directory, with no file the rename
// derivation would name, and read the pflash drives ISOArgv answers.
// PREVENTS: edk2-aarch64-code.fd refused for a missing edk2-aarch64-vars.fd
// that QEMU never ships, beside the edk2-arm-vars.fd it does.
func TestISOSplitFirmwarePairsQEMUDescriptorVars(t *testing.T) {
	for _, test := range []struct{ arch, code, vars, descriptor string }{
		{ArchARM64, "edk2-aarch64-code.fd", "edk2-arm-vars.fd", "60-edk2-aarch64.json"},
		{ArchAMD64, "edk2-x86_64-code.fd", "edk2-i386-vars.fd", "60-edk2-x86_64.json"},
	} {
		t.Run(test.arch, func(t *testing.T) {
			installer := installTestInstaller(t, InstallKindISO)
			installer.Options.Arch = test.arch
			share := filepath.Join(t.TempDir(), "share", "qemu")
			if err := os.MkdirAll(filepath.Join(share, "firmware"), 0o700); err != nil {
				t.Fatal(err)
			}
			firmware := filepath.Join(share, test.code)
			vars := filepath.Join(share, test.vars)
			cellar := "/opt/homebrew/Cellar/qemu/11.1.3/share/qemu/"
			descriptor := `{"interface-types":["uefi"],"mapping":{"device":"flash",` +
				`"executable":{"filename":"` + cellar + test.code + `","format":"raw"},` +
				`"nvram-template":{"filename":"` + cellar + test.vars + `","format":"raw"}}}`
			for path, data := range map[string]string{
				firmware: "firmware", vars: "vars",
				filepath.Join(share, "firmware", test.descriptor): descriptor,
			} {
				if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
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
					t.Errorf("descriptor-paired invocation misses %q: %q", token, argv)
				}
			}
		})
	}
}

// VALIDATES: a QEMU-bundled code image whose descriptor-named variable
// image is absent is still refused, naming that image.
// PREVENTS: the descriptor lookup turning a missing vars file into -bios.
func TestISOSplitFirmwareDescriptorVarsMissingIsRefused(t *testing.T) {
	installer := installTestInstaller(t, InstallKindISO)
	installer.Options.Arch = ArchARM64
	share := filepath.Join(t.TempDir(), "qemu")
	if err := os.MkdirAll(filepath.Join(share, "firmware"), 0o700); err != nil {
		t.Fatal(err)
	}
	firmware := filepath.Join(share, "edk2-aarch64-code.fd")
	descriptor := `{"mapping":{"device":"flash","executable":{"filename":"/x/edk2-aarch64-code.fd"},` +
		`"nvram-template":{"filename":"/x/edk2-arm-vars.fd"}}}`
	for path, data := range map[string]string{
		firmware: "firmware", filepath.Join(share, "firmware", "60-edk2-aarch64.json"): descriptor,
	} {
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	_, err := installer.ISOArgv("install.iso", "target", "extra", firmware)
	if err == nil {
		t.Fatal("split firmware without its descriptor-named variable image was accepted")
	}
	if !strings.Contains(err.Error(), filepath.Join(share, "edk2-arm-vars.fd")) {
		t.Errorf("refusal does not name the descriptor's variable image: %v", err)
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
