// Related: gotoolchain.go -- New, which refuses when no go is on PATH
//
// VALIDATES: le refuses up front, in plain words, when the Go toolchain is not
// on PATH: the Go side in New, and the launcher script before it builds bin/le.
// Each message names the missing program, the version go.mod pins, and the fix.
// PREVENTS: the owner's Linux run of 2026-10-10, where a non-interactive shell
// had no go on PATH and `./le setup docker-kernel check` answered only
// `exec: "go": executable file not found in $PATH` and `exited 127`.

package gotoolchain

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// requireAll fails the test for each phrase message does not carry.
func requireAll(t *testing.T, message string, phrases ...string) {
	t.Helper()
	for _, phrase := range phrases {
		if !strings.Contains(message, phrase) {
			t.Errorf("the refusal does not say %q:\n%s", phrase, message)
		}
	}
}

// TestNoGoOnPathIsRefusedByName empties PATH of go and asks New for the
// toolchain. The refusal has to name the cause, the pinned version and the fix,
// and carry no exit code or exec string in their place.
func TestNoGoOnPathIsRefusedByName(t *testing.T) {
	empty := t.TempDir()
	t.Setenv("PATH", empty)

	_, err := New(fixture(t, goodManifest, goodGoMod))
	if err == nil {
		t.Fatal("New answered a toolchain with no go on PATH; every Go command it prefixes would then fail with exit 127")
	}
	message := err.Error()
	requireAll(t, message,
		"no `go` program on PATH",
		empty,
		"go1.26.6",
		"./le setup tools",
		"https://go.dev/dl/",
		"export PATH=",
		"~/.profile",
	)
	if strings.Contains(message, "executable file not found") {
		t.Errorf("the refusal carries the exec string instead of saying it in the reader's terms:\n%s", message)
	}
}

// TestNoPinStillNamesTheFix covers a go.mod that pins no toolchain: the
// refusal says so rather than naming an empty version.
func TestNoPinStillNamesTheFix(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := New(fixture(t, goodManifest, "module example.com/x\n\ngo 1.26\n"))
	if err == nil {
		t.Fatal("New answered a toolchain with no go on PATH")
	}
	requireAll(t, err.Error(), "go.mod pins no toolchain", "./le setup tools")
}

// TestAGoOnPathIsAccepted puts a stand-in go on PATH, so the refusal above is
// shown to turn on the lookup and not on the fixture.
func TestAGoOnPathIsAccepted(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil { //nolint:gosec // an executable stand-in needs the execute bit
		t.Fatalf("write the stand-in go: %v", err)
	}
	t.Setenv("PATH", bin)

	if _, err := New(fixture(t, goodManifest, goodGoMod)); err != nil {
		t.Fatalf("New refused with a go on PATH: %v", err)
	}
}

// TestTheLauncherRefusesWithoutGo runs a copy of the `le` launcher script in a
// checkout with no bin/le and a PATH that holds only the programs the script
// needs before it builds. It has to stop before `go build` with a message that
// names the cause, the pinned version and the fix.
func TestTheLauncherRefusesWithoutGo(t *testing.T) {
	repository, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Fatalf("this test runs the launcher under sh, and none is on PATH: %v", err)
	}

	checkout := t.TempDir()
	for _, name := range []string{"le", "feature-tags", "feature-gates.txt", "go.mod"} {
		body, readErr := os.ReadFile(filepath.Join(repository, name)) //nolint:gosec // the files this repository's launcher reads
		if readErr != nil {
			t.Fatalf("read %s: %v", name, readErr)
		}
		if writeErr := os.WriteFile(filepath.Join(checkout, name), body, 0o700); writeErr != nil { //nolint:gosec // the launcher copy is executed
			t.Fatalf("copy %s: %v", name, writeErr)
		}
	}

	// The tools the script runs before it reaches `go build`, and no go.
	tools := t.TempDir()
	for _, name := range []string{"dirname", "awk", "sort", "tr", "sed", "mkdir", "rm"} {
		path, lookErr := exec.LookPath(name)
		if lookErr != nil {
			t.Fatalf("the launcher needs %s and none is on PATH: %v", name, lookErr)
		}
		if linkErr := os.Symlink(path, filepath.Join(tools, name)); linkErr != nil {
			t.Fatalf("link %s: %v", name, linkErr)
		}
	}

	command := exec.Command(shell, filepath.Join(checkout, "le"), "help") //nolint:gosec,noctx // the script copied above; it stops before any long work
	command.Env = []string{"PATH=" + tools, "HOME=" + t.TempDir()}
	output, runErr := command.CombinedOutput()
	if runErr == nil {
		t.Fatalf("the launcher succeeded with no go on PATH:\n%s", output)
	}
	pin, err := goToolchainPin(repository)
	if err != nil {
		t.Fatalf("read the pin: %v", err)
	}
	requireAll(t, string(output), "no `go` program on PATH", pin, "https://go.dev/dl/", "export PATH=", "~/.profile")
	// dash says `go: not found`, bash says `go: command not found`.
	for _, symptom := range []string{"go: not found", "go: command not found"} {
		if strings.Contains(string(output), symptom) {
			t.Errorf("the launcher reached `go build` instead of refusing first:\n%s", output)
		}
	}
}
