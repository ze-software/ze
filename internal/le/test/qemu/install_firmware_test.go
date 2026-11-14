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

// VALIDATES: a firmware descriptor that does not parse decides the run only
// when the code image has no other pairing, and a stateless descriptor maps
// its code image alone into a read-only pflash chip.
// METHOD: lay out a share/qemu directory per case (code, optional vars,
// firmware/*.json) and read the drives or the refusal ISOArgv answers.
// PREVENTS: one malformed descriptor for an unrelated firmware stopping a
// proof whose own image is described, a malformed descriptor that may pair
// the image being skipped in silence, and a stateless image refused for a
// variable image its descriptor says it does not have.
func TestISOFirmwareDescriptorDecisions(t *testing.T) {
	const malformed = `{"mapping":`
	pair := func(code, vars string) string {
		return `{"mapping":{"device":"flash","executable":{"filename":"/x/` + code + `"},` +
			`"nvram-template":{"filename":"/x/` + vars + `"}}}`
	}
	for _, test := range []struct {
		name, code    string
		files         map[string]string
		varsDrive     string
		codeOnly      bool
		refusalNaming string
	}{
		{
			name: "unrelated malformed beside a parsed pairing", code: "edk2-aarch64-code.fd",
			files: map[string]string{
				"edk2-arm-vars.fd": "vars", "firmware/50-other.json": malformed,
				"firmware/60-edk2-aarch64.json": pair("edk2-aarch64-code.fd", "edk2-arm-vars.fd"),
			},
			varsDrive: "edk2-arm-vars.fd",
		},
		{
			name: "unrelated malformed beside a renamed image", code: "OVMF_CODE_4M.fd",
			files:     map[string]string{"OVMF_VARS_4M.fd": "vars", "firmware/50-other.json": malformed},
			varsDrive: "OVMF_VARS_4M.fd",
		},
		{
			name: "malformed is the only possible pairing", code: "edk2-aarch64-code.fd",
			files:         map[string]string{"firmware/60-edk2-aarch64.json": malformed},
			refusalNaming: "firmware/60-edk2-aarch64.json",
		},
		{
			name: "malformed may describe a complete image", code: "QEMU_EFI.fd",
			files:         map[string]string{"firmware/60-edk2-aarch64.json": malformed},
			refusalNaming: "firmware/60-edk2-aarch64.json",
		},
		{
			name: "stateless descriptor", code: "edk2-aarch64-code.fd",
			files: map[string]string{"firmware/60-edk2-aarch64.json": `{"mapping":{"device":"flash","mode":"stateless",` +
				`"executable":{"filename":"/x/edk2-aarch64-code.fd"}}}`},
			codeOnly: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			installer := installTestInstaller(t, InstallKindISO)
			installer.Options.Arch = ArchARM64
			share := filepath.Join(t.TempDir(), "qemu")
			if err := os.MkdirAll(filepath.Join(share, "firmware"), 0o700); err != nil {
				t.Fatal(err)
			}
			firmware := filepath.Join(share, test.code)
			if err := os.WriteFile(firmware, []byte("firmware"), 0o600); err != nil {
				t.Fatal(err)
			}
			for name, data := range test.files {
				if err := os.WriteFile(filepath.Join(share, name), []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			argv, err := installer.ISOArgv("install.iso", "target", "extra", firmware)
			if test.refusalNaming != "" {
				if err == nil {
					t.Fatalf("a malformed descriptor that may pair %s was ignored: %q", test.code, argv)
				}
				if !strings.Contains(err.Error(), filepath.Join(share, test.refusalNaming)) {
					t.Fatalf("refusal does not name the malformed descriptor: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(argv, "if=pflash,unit=0,format=raw,readonly=on,file="+firmware) {
				t.Errorf("code image is not a read-only pflash drive: %q", argv)
			}
			if slices.Contains(argv, "-bios") {
				t.Errorf("flash firmware was passed as -bios: %q", argv)
			}
			unit1 := slices.ContainsFunc(argv, func(arg string) bool { return strings.HasPrefix(arg, "if=pflash,unit=1,") })
			if test.codeOnly {
				if unit1 {
					t.Errorf("stateless firmware was given a variable drive: %q", argv)
				}
				return
			}
			if !slices.Contains(argv, "if=pflash,unit=1,format=raw,snapshot=on,file="+filepath.Join(share, test.varsDrive)) {
				t.Errorf("variable image %s is not a snapshot pflash drive: %q", test.varsDrive, argv)
			}
		})
	}
}
