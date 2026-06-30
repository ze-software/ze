package appliance

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIsoCheckAllReady(t *testing.T) {
	dir := t.TempDir()

	kernelPath := filepath.Join(dir, "kernel")
	if err := os.WriteFile(kernelPath, []byte("k"), 0o644); err != nil {
		t.Fatal(err)
	}
	initrdPath := filepath.Join(dir, "initrd.img.gz")
	if err := os.WriteFile(initrdPath, []byte("i"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldLookPath := isoLookPathFn
	isoLookPathFn = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	t.Cleanup(func() { isoLookPathFn = oldLookPath })

	code := checkISOPrerequisites(isoOptions{
		kernelPath: kernelPath,
		initrdPath: initrdPath,
	})
	if code != exitOK {
		t.Errorf("checkISOPrerequisites = %d, want %d", code, exitOK)
	}
}

func TestIsoCheckRejectsStaleDefaultKernelFallback(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	writeInstallerKernelRegistry(t)
	kernelPath := filepath.Join(root, "build", "kernel", "Image")
	if err := os.MkdirAll(filepath.Dir(kernelPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(kernelPath, []byte("k"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeIsoTestFile(t, filepath.Join(root, "build", "kernel", ".variant"), archAMD64+"-custom-"+defaultKernelVersion+"-docker")
	initrdPath := filepath.Join(root, "build", "initrd", "initrd.img.gz")
	if err := os.MkdirAll(filepath.Dir(initrdPath), 0o755); err != nil {
		t.Fatal(err)
	}
	writeIsoTestFile(t, initrdPath, "default-initrd")

	oldLookPath := isoLookPathFn
	isoLookPathFn = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	t.Cleanup(func() { isoLookPathFn = oldLookPath })

	code := checkISOPrerequisites(isoOptions{
		initrdPath: initrdPath,
	})
	if code != exitError {
		t.Errorf("checkISOPrerequisites = %d, want %d", code, exitError)
	}
}

func TestIsoCheckMissing(t *testing.T) {
	oldLookPath := isoLookPathFn
	isoLookPathFn = func(name string) (string, error) { return "", exec.ErrNotFound }
	t.Cleanup(func() { isoLookPathFn = oldLookPath })

	code := checkISOPrerequisites(isoOptions{
		kernelPath: "/nonexistent/kernel",
		initrdPath: "/nonexistent/initrd",
	})
	if code != exitError {
		t.Errorf("checkISOPrerequisites = %d, want %d", code, exitError)
	}
}
