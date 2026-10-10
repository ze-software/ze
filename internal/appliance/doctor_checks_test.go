package appliance

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
		"appliance-runtime-kernel",
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
	useE2FSDir(t, "")
	diags := checkE2fsprogs(diagnostic.DoctorCheckContext{})
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Code != "doctor-appliance-e2fsprogs" {
		t.Errorf("code = %q, want doctor-appliance-e2fsprogs", diags[0].Code)
	}
}

// setTestRuntimeBuilder makes doctorRuntimeBuilderFn answer err for every arch,
// so the runtime kernel check runs with no Docker and no QEMU.
func setTestRuntimeBuilder(t *testing.T, err error) {
	t.Helper()
	old := doctorRuntimeBuilderFn
	doctorRuntimeBuilderFn = func(string) (string, error) {
		if err != nil {
			return "", err
		}
		return "docker", nil
	}
	t.Cleanup(func() { doctorRuntimeBuilderFn = old })
}

// TestDoctorRuntimeKernelCached verifies a host holding the runtime kernel for
// its arch in the cache passes, even with no builder installed: the image build
// serves the cache entry and starts nothing.
//
// VALIDATES: the doctor check for the runtime kernel `ze appliance build` needs.
// PREVENTS: a warning on a host that can build images.
func TestDoctorRuntimeKernelCached(t *testing.T) {
	t.Chdir(t.TempDir())
	writeRuntimeKernelRegistry(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	setTestRuntimeBuilder(t, errors.New("no builder available"))
	cached, err := kernelCachePathFor(defaultKernelVersion, runtime.GOARCH, runtimeKernelProfile, kernelTargetRuntime)
	if err != nil {
		t.Fatal(err)
	}
	writeRuntimeKernelTree(t, cached, runtime.GOARCH)

	if diags := checkRuntimeKernel(diagnostic.DoctorCheckContext{}); len(diags) != 0 {
		t.Errorf("a cached runtime kernel was reported: %+v", diags)
	}
}

// TestDoctorRuntimeKernelBuildable verifies a host with no cache entry but a
// usable builder passes: the first image build builds the kernel.
//
// VALIDATES: the doctor check accepts a usable builder in place of the cache.
func TestDoctorRuntimeKernelBuildable(t *testing.T) {
	t.Chdir(t.TempDir())
	writeRuntimeKernelRegistry(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	setTestRuntimeBuilder(t, nil)

	if diags := checkRuntimeKernel(diagnostic.DoctorCheckContext{}); len(diags) != 0 {
		t.Errorf("a host with a usable builder was reported: %+v", diags)
	}
}

// TestDoctorRuntimeKernelMissing verifies a host with neither the cache entry
// nor a builder gets a warning that names the missing cache entry, why the
// image build needs it, and the commands that fix it.
//
// VALIDATES: the doctor check reports the condition before `ze appliance build` fails on it.
func TestDoctorRuntimeKernelMissing(t *testing.T) {
	t.Chdir(t.TempDir())
	writeRuntimeKernelRegistry(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	setTestRuntimeBuilder(t, errors.New("no builder available"))
	cached, err := kernelCachePathFor(defaultKernelVersion, runtime.GOARCH, runtimeKernelProfile, kernelTargetRuntime)
	if err != nil {
		t.Fatal(err)
	}

	diags := checkRuntimeKernel(diagnostic.DoctorCheckContext{})
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %+v", len(diags), diags)
	}
	if diags[0].Code != "doctor-appliance-runtime-kernel" {
		t.Errorf("code = %q, want doctor-appliance-runtime-kernel", diags[0].Code)
	}
	if diags[0].Severity != diagnostic.SeverityWarning {
		t.Errorf("severity = %v, want warning", diags[0].Severity)
	}
	for _, want := range []string{
		cached,
		"ze appliance build",
		"install Docker, or QEMU and Go",
		"ze appliance kernel --target runtime --arch " + runtime.GOARCH,
	} {
		if !strings.Contains(diags[0].Message, want) {
			t.Errorf("message does not name %q:\n%s", want, diags[0].Message)
		}
	}
}

// TestDoctorRuntimeKernelOutsideSourceTree verifies a host that cannot read the
// runtime kernel config gets a warning naming the config directory, rather
// than a pass it cannot support.
//
// VALIDATES: a check that cannot answer says so.
func TestDoctorRuntimeKernelOutsideSourceTree(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	setTestRuntimeBuilder(t, nil)

	diags := checkRuntimeKernel(diagnostic.DoctorCheckContext{})
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %+v", len(diags), diags)
	}
	if !strings.Contains(diags[0].Message, runtimeKernelConfigDir) {
		t.Errorf("message does not name %s:\n%s", runtimeKernelConfigDir, diags[0].Message)
	}
}
