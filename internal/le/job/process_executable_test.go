// Design: docs/contributing/running-commands.md -- descriptor-bound child execution.
package job

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestRunProcessHeldExecutable proves which bytes run after its pathname is
// replaced, and that argv boundaries survive the descriptor-based launch.
func TestRunProcessHeldExecutable(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("descriptor-bound execution is supported on Linux")
	}
	path := filepath.Join(t.TempDir(), "program")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf 'original:%s:%s' \"$1\" \"$2\"\nexit 23\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close() //nolint:errcheck // Read-only test input.
	if err := os.Rename(path, path+".accepted"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf replacement\nexit 24\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	code, err := RunProcess([]string{path, "one two", "three"}, ProcessIO{Executable: file, Stdout: &output, Stderr: &output})
	if err != nil {
		t.Fatal(err)
	}
	if code != 23 {
		t.Errorf("exit=%d, want original program exit 23", code)
	}
	if output.String() != "original:one two:three" {
		t.Errorf("executed bytes/argv=%q", output.String())
	}
	if _, err := file.Stat(); err != nil {
		t.Fatalf("RunProcess closed caller's executable: %v", err)
	}
}
