package appliance

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFakeGo puts a go on a PATH of its own that prints answer for
// `go env GOVERSION`.
func writeFakeGo(t *testing.T, answer string) {
	t.Helper()
	bin := t.TempDir()
	goBin := filepath.Join(bin, "go")
	script := "#!/bin/sh\necho " + answer + "\n"
	if err := os.WriteFile(goBin, []byte(script), 0o755); err != nil { //nolint:gosec // G306: the fake go must be executable
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
}

// writeGoMod writes a go.mod requiring go goVersion in a new root, with the
// gokrazy module cache the image build reads, and returns the root.
func writeGoMod(t *testing.T, goVersion string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "gokrazy", "modcache"), 0o750); err != nil {
		t.Fatal(err)
	}
	mod := "module github.com/ze-software/ze\n\ngo " + goVersion + "\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(mod), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestCheckGoToolchain verifies the go on PATH is held against the go
// directive: an older go, a missing go and an unparseable version are refused
// naming the fix, and a go at or above the directive passes.
//
// VALIDATES: review round 1 ISSUE 3, the host go checked before the build.
// PREVENTS: an old go found by gok only after a 30-minute kernel build.
func TestCheckGoToolchain(t *testing.T) {
	cases := []struct {
		name   string
		answer string // "" means no go on PATH
		want   []string
	}{
		{name: "older", answer: "go1.26.3", want: []string{"is go1.26.3, older than go 1.27.0", "install Go 1.27.0 or newer from https://go.dev/dl/"}},
		{name: "missing", answer: "", want: []string{"no go on PATH", "install Go 1.27.0 or newer"}},
		{name: "devel", answer: "devel", want: []string{`version "devel"`, "install a Go release"}},
		{name: "equal", answer: "go1.27.0"},
		{name: "newer", answer: "go1.27.1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := writeGoMod(t, "1.27.0")
			if tc.answer == "" {
				t.Setenv("PATH", t.TempDir())
			} else {
				writeFakeGo(t, tc.answer)
			}

			err := checkGokHost(root)
			if len(tc.want) == 0 {
				if err != nil {
					t.Fatalf("a go at or above the directive was refused: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("go %q passed against go 1.27.0", tc.answer)
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal does not name %q:\n%v", want, err)
				}
			}
		})
	}
}

// TestCheckGokHostNamesAMissingModcache verifies a build run outside the ze
// source tree root is refused naming the module cache and where to run.
//
// VALIDATES: review round 1 ISSUE 3, the module cache checked before the build.
func TestCheckGokHostNamesAMissingModcache(t *testing.T) {
	root := t.TempDir()
	err := checkGokHost(root)
	if err == nil {
		t.Fatal("a root with no gokrazy/modcache passed")
	}
	for _, want := range []string{filepath.Join(root, "gokrazy", "modcache"), "root of the ze checkout"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %q:\n%v", want, err)
		}
	}
}

// TestBuildRefusesAnOldGoBeforeTheKernel verifies buildOne refuses a go older
// than the go directive before it resolves the runtime kernel or runs gok.
//
// VALIDATES: review round 1 ISSUE 3.
// PREVENTS: a cold kernel cache building for 30 minutes before gok finds the
// old go.
//
// Method: a valid appliance config, a ze root fixture with go 1.27.0, a fake
// go1.26.3 on PATH, and a kernel resolver and gok that fail the test if called.
// The refusal is read from stderr, so a build that fails later for another
// reason (no e2fsprogs on the host) does not pass it.
func TestBuildRefusesAnOldGoBeforeTheKernel(t *testing.T) {
	oldBase := baseDir
	baseDir = t.TempDir()
	t.Cleanup(func() { baseDir = oldBase })
	if err := os.MkdirAll(filepath.Join(baseDir, "go-app"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig("go-app")
	if err := saveConfig(ConfigPath(baseDir, "go-app"), &cfg); err != nil {
		t.Fatal(err)
	}
	t.Chdir(writeGoMod(t, "1.27.0"))
	writeFakeGo(t, "go1.26.3")

	oldTree := runtimeKernelTreeFn
	runtimeKernelTreeFn = func(string) (string, error) {
		t.Fatal("the runtime kernel resolved before the host go was checked")
		return "", nil
	}
	t.Cleanup(func() { runtimeKernelTreeFn = oldTree })
	oldGok := gokBuildFn
	gokBuildFn = func([]string) error {
		t.Fatal("gok ran with a go older than the go directive")
		return nil
	}
	t.Cleanup(func() { gokBuildFn = oldGok })

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStderr := os.Stderr
	os.Stderr = write
	code := buildOne("go-app")
	os.Stderr = oldStderr
	write.Close() //nolint:errcheck,gosec // test pipe
	stderr, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}

	if code != exitError {
		t.Errorf("buildOne with go1.26.3 = %d, want exitError (%d)", code, exitError)
	}
	if !strings.Contains(string(stderr), "older than go 1.27.0") {
		t.Errorf("buildOne did not refuse the old go first; stderr:\n%s", stderr)
	}
}
