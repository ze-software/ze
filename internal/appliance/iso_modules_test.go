// Design: docs/architecture/appliance/iso-installer.md -- GRUB target prerequisites.

package appliance

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

func TestISOGRUBModulesRequireSelectedTarget(t *testing.T) {
	root := t.TempDir()
	builder := filepath.Join(root, "bin", "grub-mkstandalone")
	modules := filepath.Join(root, "lib", "grub", isoGRUBTargetARM64)
	if err := os.MkdirAll(modules, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modules, "modinfo.sh"), []byte("modules"), 0o600); err != nil {
		t.Fatal(err)
	}
	if admitted, err := checkISOGRUBModules(builder, isoGRUBTargetARM64); err != nil {
		t.Fatalf("installed target modules rejected: %v", err)
	} else if admitted != modules {
		t.Errorf("admitted directory=%q, want %q", admitted, modules)
	}
	if _, err := checkISOGRUBModules(builder, "missing-efi"); err == nil {
		t.Error("builder executable alone was accepted without selected target modules")
	} else if !strings.Contains(err.Error(), "missing-efi") {
		t.Errorf("missing-module error does not identify its target: %v", err)
	}
}

func TestISOCheckRejectsMissingSelectedGRUBModules(t *testing.T) {
	oldModules, oldLookPath := isoModuleCheckFn, isoLookPathFn
	isoLookPathFn = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	checked := ""
	isoModuleCheckFn = func(_, target string) (string, error) {
		checked = target
		return "", errors.New("selected modules missing")
	}
	t.Cleanup(func() { isoModuleCheckFn, isoLookPathFn = oldModules, oldLookPath })
	if code := checkISOPrerequisites(isoOptions{arch: archARM64}); code != exitError {
		t.Errorf("missing target module set accepted: %d", code)
	}
	if checked != isoGRUBTargetARM64 {
		t.Errorf("prerequisite check used target %q, want %s", checked, isoGRUBTargetARM64)
	}
}

func TestISODoctorReportsMissingHostGRUBModules(t *testing.T) {
	oldModules, oldLookPath := isoModuleCheckFn, doctorLookPathFn
	doctorLookPathFn = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	isoModuleCheckFn = func(string, string) (string, error) { return "", errors.New("selected modules missing") }
	t.Cleanup(func() { isoModuleCheckFn, doctorLookPathFn = oldModules, oldLookPath })
	got := checkGrubBinary(diagnostic.DoctorCheckContext{})
	if len(got) != 1 || got[0].Code != "doctor-appliance-grub" {
		t.Fatalf("missing module diagnosis=%+v", got)
	}
	if !strings.Contains(got[0].Message, "selected modules missing") {
		t.Errorf("module diagnosis lost its cause: %+v", got[0])
	}
}
