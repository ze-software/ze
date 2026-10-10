package buildgokrazy

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunRefusesAnOldGoBeforeTheKernel verifies `./le build gokrazy` refuses a
// go older than the go directive before prepareArgs resolves the runtime
// kernel, naming both versions.
//
// VALIDATES: review round 1 ISSUE 3 of spec-appliance-ships-ze-kernel.
// PREVENTS: a cold kernel cache building for 30 minutes before gok finds the
// old go.
//
// Method: a root holding go.mod (go 1.27.0) and gokrazy/modcache, a fake go
// answering go1.26.3 as the only go on PATH, and a kernel resolver that fails
// the test if called. The refusal is read from stderr.
func TestRunRefusesAnOldGoBeforeTheKernel(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "gokrazy", "modcache"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/ze-software/ze\n\ngo 1.27.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte("#!/bin/sh\necho go1.26.3\n"), 0o755); err != nil { //nolint:gosec // G306: the fake go must be executable
		t.Fatal(err)
	}
	t.Chdir(root)
	t.Setenv("PATH", bin)
	for _, key := range []string{"GOMODCACHE", "GOFLAGS", "GOPROXY", "GOTOOLCHAIN", "CGO_ENABLED"} {
		t.Setenv(key, "")
	}

	old := runtimeKernelTreeFn
	runtimeKernelTreeFn = func(string) (string, error) {
		t.Fatal("the runtime kernel resolved before the host go was checked")
		return "", nil
	}
	t.Cleanup(func() { runtimeKernelTreeFn = old })

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStderr := os.Stderr
	os.Stderr = write
	code := Run([]string{"--parent_dir", filepath.Join(root, "gokrazy"), "-i", "ze", "overwrite", "--full", "x.img"})
	os.Stderr = oldStderr
	write.Close() //nolint:errcheck,gosec // test pipe
	stderr, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}

	if code == 0 {
		t.Fatal("Run with go1.26.3 succeeded against go 1.27.0")
	}
	if !strings.Contains(string(stderr), "is go1.26.3, older than go 1.27.0") {
		t.Errorf("Run did not refuse the old go; stderr:\n%s", stderr)
	}
}
