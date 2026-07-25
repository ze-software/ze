package appliance

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

func TestApplianceDoctorChecksRegistered(t *testing.T) {
	names := diagnostic.DoctorCheckNames()
	expected := []string{
		"appliance-kernel",
		"appliance-initrd",
		"appliance-grub",
		"appliance-xorriso",
		"appliance-e2fsprogs",
	}
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	for _, want := range expected {
		if !nameSet[want] {
			t.Errorf("doctor check %q not registered", want)
		}
	}
}

func TestDoctorKernelPresent(t *testing.T) {
	cacheDir := t.TempDir()
	t.Chdir(t.TempDir())
	writeInstallerKernelRegistry(t)
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	cached := kernelCachePath(defaultKernelVersion, kernelCacheVariant(archAMD64, defaultKernelProfile))
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cached, []byte("k"), 0o644); err != nil {
		t.Fatal(err)
	}

	diags := checkKernelArtifact(diagnostic.DoctorCheckContext{})
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics when kernel present, got %d", len(diags))
	}
}

func TestDoctorKernelRejectsStaleFallback(t *testing.T) {
	cacheDir := t.TempDir()
	root := t.TempDir()
	t.Chdir(root)
	writeInstallerKernelRegistry(t)
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	kernelPath := filepath.Join(root, "build", "kernel", "Image")
	if err := os.MkdirAll(filepath.Dir(kernelPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(kernelPath, []byte("k"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeIsoTestFile(t, filepath.Join(root, "build", "kernel", ".variant"), archAMD64+"-custom-"+defaultKernelVersion+"-docker")

	diags := checkKernelArtifact(diagnostic.DoctorCheckContext{})
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Code != "doctor-appliance-kernel" {
		t.Errorf("code = %q, want doctor-appliance-kernel", diags[0].Code)
	}
}

func TestDoctorKernelRejectsVariantWithoutImage(t *testing.T) {
	cacheDir := t.TempDir()
	root := t.TempDir()
	t.Chdir(root)
	writeInstallerKernelRegistry(t)
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	buildDir := filepath.Join(root, "build", "kernel")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeIsoTestFile(t, filepath.Join(buildDir, ".variant"), archAMD64+"-"+defaultKernelProfile+"-"+defaultKernelVersion+"-docker")

	diags := checkKernelArtifact(diagnostic.DoctorCheckContext{})
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Code != "doctor-appliance-kernel" {
		t.Errorf("code = %q, want doctor-appliance-kernel", diags[0].Code)
	}
}

func TestDoctorKernelMissing(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Chdir(t.TempDir())

	diags := checkKernelArtifact(diagnostic.DoctorCheckContext{})
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Code != "doctor-appliance-kernel" {
		t.Errorf("code = %q, want doctor-appliance-kernel", diags[0].Code)
	}
	if diags[0].Severity != diagnostic.SeverityWarning {
		t.Errorf("severity = %q, want warning", diags[0].Severity)
	}
}

func TestDoctorGrubPresent(t *testing.T) {
	old := doctorLookPathFn
	doctorLookPathFn = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	t.Cleanup(func() { doctorLookPathFn = old })

	diags := checkGrubBinary(diagnostic.DoctorCheckContext{})
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics when grub present, got %d", len(diags))
	}
}

func TestDoctorGrubMissing(t *testing.T) {
	old := doctorLookPathFn
	doctorLookPathFn = func(name string) (string, error) { return "", errors.New("not found") }
	t.Cleanup(func() { doctorLookPathFn = old })

	diags := checkGrubBinary(diagnostic.DoctorCheckContext{})
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Code != "doctor-appliance-grub" {
		t.Errorf("code = %q, want doctor-appliance-grub", diags[0].Code)
	}
}

func TestDoctorXorrisoMissing(t *testing.T) {
	old := doctorLookPathFn
	doctorLookPathFn = func(name string) (string, error) { return "", errors.New("not found") }
	t.Cleanup(func() { doctorLookPathFn = old })

	diags := checkXorrisoBinary(diagnostic.DoctorCheckContext{})
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Code != "doctor-appliance-xorriso" {
		t.Errorf("code = %q, want doctor-appliance-xorriso", diags[0].Code)
	}
}

func TestDoctorE2fsprogsMissing(t *testing.T) {
	old := e2fsDir
	e2fsDir = ""
	t.Cleanup(func() { e2fsDir = old })

	diags := checkE2fsprogs(diagnostic.DoctorCheckContext{})
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Code != "doctor-appliance-e2fsprogs" {
		t.Errorf("code = %q, want doctor-appliance-e2fsprogs", diags[0].Code)
	}
}
