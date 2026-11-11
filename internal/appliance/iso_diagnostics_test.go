// Design: docs/architecture/appliance/iso-installer.md -- actionable ISO refusals.

package appliance

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
)

func TestISOCheckNameRefusalsGiveHostDriverRepair(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    []string
		problem string
		fix     string
	}{
		{"extra-name", []string{"--check", "lab", "extra"}, "one appliance name", "./ze-host appliance iso --help"},
		{"invalid-name", []string{"--check", "../lab"}, "../lab", "./ze-host appliance --dir " + shellArg(getBaseDir()) + " list"},
	} {
		t.Run(test.name, func(t *testing.T) {
			code := exitOK
			output := captureStderr(t, func() { code = runIso(test.args) })
			if code != exitError {
				t.Fatalf("invalid ISO prerequisite request returned %d", code)
			}
			assertISOOperatorRepair(t, output, test.problem, test.fix)
		})
	}
}

func TestISOCheckMissingConfigPreservesCauseAndRepair(t *testing.T) {
	oldBase := baseDir
	baseDir = t.TempDir()
	t.Cleanup(func() { baseDir = oldBase })
	code := exitOK
	output := captureStderr(t, func() { code = runIso([]string{"--check", "missing"}) })
	if code != exitError {
		t.Fatalf("missing named configuration returned %d", code)
	}
	assertISOOperatorRepair(t, output, "missing", "./ze-host appliance --dir "+shellArg(baseDir)+" list")
	if !strings.Contains(output, ConfigPath(baseDir, "missing")) {
		t.Errorf("named configuration failure lost its underlying path: %s", output)
	}
}

func TestISOInvalidNameRepairRetainsSelectedDirectory(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
	}{
		{"check", []string{"--check", "../bad"}},
		{"build", []string{"../bad"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			oldBase := baseDir
			baseDir = filepath.Join(t.TempDir(), "appliance directory")
			t.Cleanup(func() { baseDir = oldBase })
			if err := os.MkdirAll(baseDir, 0o700); err != nil {
				t.Fatal(err)
			}
			code := exitOK
			output := captureStderr(t, func() { code = runIso(test.args) })
			if code != exitError {
				t.Fatalf("invalid appliance name returned %d", code)
			}
			assertISOOperatorRepair(t, output, "../bad", "./ze-host appliance --dir "+shellArg(baseDir)+" list")
		})
	}
}

func TestISOBuildFailurePreservesSpecificOperatorRepair(t *testing.T) {
	oldBase := baseDir
	t.Cleanup(func() { baseDir = oldBase })
	appDir := setupIsoTestAppliance(t)
	image := filepath.Join(appDir, "ze-diagnostic.img")
	writeIsoTestFile(t, image, "image")
	writeIsoTestChecksum(t, image)
	kernel, initrd, _ := setupIsoBuilderTest(t)
	cause := errors.New("fixture GRUB assembly refused")
	repair := "sudo apt-get install grub-efi-amd64-bin"
	isoBuilderFn = func(isoBuilderCall) error {
		return opererr.Wrap(cause, "GRUB EFI modules are unavailable", "the selected modules are required to make this ISO bootable", repair)
	}
	code := exitOK
	output := captureStderr(t, func() {
		code = runIso([]string{"--kernel", kernel, "--initrd", initrd, isoTestApplianceName})
	})
	if code != exitError {
		t.Fatalf("failed ISO assembly returned %d", code)
	}
	assertISOOperatorRepair(t, output, "GRUB EFI modules", repair)
	if !strings.Contains(output, cause.Error()) {
		t.Errorf("ISO build failure lost its underlying cause: %s", output)
	}
	if matches, err := filepath.Glob(filepath.Join(appDir, "*.iso")); err != nil {
		t.Fatal(err)
	} else if len(matches) != 0 {
		t.Errorf("failed ISO assembly left completed output: %v", matches)
	}
}

func TestISOBuildFailureGivesSelectedPrerequisiteCheck(t *testing.T) {
	oldBase := baseDir
	t.Cleanup(func() { baseDir = oldBase })
	appDir := setupIsoTestAppliance(t)
	image := filepath.Join(appDir, "ze-unexpected-failure.img")
	writeIsoTestFile(t, image, "image")
	writeIsoTestChecksum(t, image)
	kernel, initrd, _ := setupIsoBuilderTest(t)
	cause := errors.New("fixture builder cannot write its output")
	isoBuilderFn = func(isoBuilderCall) error { return cause }
	code := exitOK
	output := captureStderr(t, func() {
		code = runIso([]string{"--kernel", kernel, "--initrd", initrd, isoTestApplianceName})
	})
	if code != exitError {
		t.Fatalf("unexpected builder failure returned %d", code)
	}
	assertISOOperatorRepair(t, output, "cannot build installer ISO",
		"./ze-host appliance --dir "+shellArg(baseDir)+" iso --check "+shellArg(isoTestApplianceName))
	if !strings.Contains(output, cause.Error()) {
		t.Errorf("ISO refusal lost its underlying builder cause: %s", output)
	}
}

func TestISODoctorMissingModulesGivesRepairAndReason(t *testing.T) {
	oldModules, oldLook := isoModuleCheckFn, doctorLookPathFn
	doctorLookPathFn = func(string) (string, error) { return "/usr/bin/grub-mkstandalone", nil }
	repair := "sudo apt-get install grub-efi-amd64-bin"
	isoModuleCheckFn = func(string, string) (string, error) {
		return "", opererr.Wrap(os.ErrPermission, "GRUB EFI modules are unavailable", "the selected modules are required to make this ISO bootable", repair)
	}
	t.Cleanup(func() { isoModuleCheckFn, doctorLookPathFn = oldModules, oldLook })
	diags := checkGrubBinary(diagnostic.DoctorCheckContext{})
	if len(diags) != 1 {
		t.Fatalf("missing module diagnosis=%+v", diags)
	}
	if diags[0].Fix != repair {
		t.Errorf("doctor module repair=%q", diags[0].Fix)
	}
	if diags[0].Help == "" {
		t.Error("doctor does not explain why missing GRUB modules prevent ISO creation")
	}
	if !strings.Contains(diags[0].Message, os.ErrPermission.Error()) {
		t.Errorf("doctor lost module inspection cause: %+v", diags[0])
	}
}

func TestISOGRUBRepairCommandsMatchDistroAndTarget(t *testing.T) {
	for _, test := range []struct {
		distro string
		target string
		want   string
	}{
		{"debian", isoGRUBTargetAMD64, "sudo apt-get install grub-efi-amd64-bin"},
		{"ubuntu", isoGRUBTargetARM64, "sudo apt-get install grub-efi-arm64-bin"},
		{"fedora", isoGRUBTargetAMD64, "sudo dnf install grub2-tools-extra grub2-efi-x64-modules"},
		{"rocky", isoGRUBTargetARM64, "sudo dnf install grub2-tools-extra grub2-efi-aa64-modules"},
	} {
		t.Run(test.distro+"/"+test.target, func(t *testing.T) {
			command, err := isoGRUBInstallCommand(test.distro, test.target)
			if err != nil {
				t.Fatal(err)
			}
			if command != test.want {
				t.Errorf("GRUB prerequisite repair=%q, want %q", command, test.want)
			}
		})
	}
	if command, err := isoGRUBInstallCommand("unrecognized", isoGRUBTargetAMD64); err == nil {
		t.Errorf("unknown distribution was assigned a guessed repair: %q", command)
	}
}

func assertISOOperatorRepair(t *testing.T, output, problem, fix string) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 3 {
		t.Errorf("ISO refusal needs problem, explanation and repair: %q", output)
	}
	if !strings.Contains(output, problem) {
		t.Errorf("ISO refusal omits its subject %q: %s", problem, output)
	}
	if last := lines[len(lines)-1]; last != "fix: "+fix {
		t.Errorf("ISO refusal final line=%q, want exact repair %q", last, fix)
	}
}
