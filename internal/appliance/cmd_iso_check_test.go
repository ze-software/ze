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
