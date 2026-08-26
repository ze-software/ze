package main

// AC-11 for the release-candidate gate: effective-verify.sh and
// `le evidence release-candidate-check` do the same thing.
//
// A shell script and a Go command share no process, so no test can call both.
// They do share the one effect this gate has on the world: the container it
// starts. So both halves are pointed at the same recording stand-ins for docker
// and git, and what is compared is the argv that would have reached the docker
// daemon -- which is the whole of what this gate does, since the container
// script decides everything that happens after it.
//
// The git QUERIES are deliberately not compared. The script asks git three
// questions and the command asks one, because the command already knows which
// checkout it is in (lepath.Root) and reads the dirty paths out of the porcelain
// listing it already has. A query changes nothing outside the process, so a
// difference in how each half learns a fact is not a difference in what it does.
//
// This file lives beside the script rather than beside the port, so that step 14
// deletes the script and its parity proof in one commit.

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/letools/evidence"
)

// stubs writes a recording docker and git onto a PATH directory, and answers
// that directory.
//
// Each stub appends its whole argv to a file, one NUL-terminated word per
// argument, so an argument carrying a newline (the container script does)
// survives the round trip.
func stubs(t *testing.T, fixture string) string {
	t.Helper()

	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil { //nolint:gosec // a stub on a test's own PATH must be executable
			t.Fatalf("write the %s stub: %v", name, err)
		}
	}

	write("docker", "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\0' \"$a\"; done >> \"$ZE_RECORD_DOCKER\"\nexit ${ZE_DOCKER_EXIT:-0}\n")
	write("git", "#!/bin/sh\ncase \"$*\" in\n  *'rev-parse --show-toplevel'*) printf '%s\\n' \""+fixture+"\" ;;\n  *status*) printf '%s' \"${ZE_GIT_STATUS:-}\" ;;\nesac\nexit 0\n")

	return dir
}

// recorded answers the argv the docker stub was called with, one call per outer
// entry.
func recorded(t *testing.T, path string) []string {
	t.Helper()

	body, err := os.ReadFile(path) //nolint:gosec // the test wrote this path
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read the docker recording: %v", err)
	}
	words := strings.Split(string(body), "\x00")
	if len(words) > 0 && words[len(words)-1] == "" {
		words = words[:len(words)-1]
	}
	return words
}

// runScript runs effective-verify.sh against the fixture with the stubs in
// front of PATH, and answers its exit status and the docker argv it produced.
func runScript(t *testing.T, stubDir, status, dockerExit string) (int, []string) {
	t.Helper()

	record := filepath.Join(t.TempDir(), "docker-argv")
	// The stubs answer at once, so a minute means the script is stuck rather
	// than slow, and the test reports that instead of hanging the suite.
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "effective-verify.sh")
	cmd.Dir = "."
	cmd.Env = append(os.Environ(),
		"PATH="+stubDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ZE_RECORD_DOCKER="+record,
		"ZE_GIT_STATUS="+status,
		"ZE_DOCKER_EXIT="+dockerExit,
	)
	out, err := cmd.CombinedOutput()

	code := 0
	if err != nil {
		exit, ok := errors.AsType[*exec.ExitError](err)
		if !ok {
			t.Fatalf("run the script: %v\n%s", err, out)
		}
		code = exit.ExitCode()
	}
	return code, recorded(t, record)
}

// runCommand runs the ported command against the fixture with the same stubs,
// through the production seams, and answers the same two things.
func runCommand(t *testing.T, fixture, stubDir, status, dockerExit string) (int, []string) {
	t.Helper()

	record := filepath.Join(t.TempDir(), "docker-argv")
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ZE_RECORD_DOCKER", record)
	t.Setenv("ZE_GIT_STATUS", status)
	t.Setenv("ZE_DOCKER_EXIT", dockerExit)

	report, err := evidence.NewRunner(fixture).Run()
	code := report.Code
	if err != nil {
		code = 1
	}
	return code, recorded(t, record)
}

// VALIDATES: over a clean tree, the script and the command start docker with
// byte-identical argv, container script included, and both exit 0.
// PREVENTS: the port changing what runs in the container -- a dropped cache
// volume, a different image, a rewritten script -- which no output comparison
// would see, because the gate's whole output comes from inside the container.
func TestScriptAndCommandStartTheSameContainer(t *testing.T) {
	fixture := t.TempDir()
	stubDir := stubs(t, fixture)

	scriptCode, scriptArgv := runScript(t, stubDir, "", "0")
	commandCode, commandArgv := runCommand(t, fixture, stubDir, "", "0")

	if scriptCode != 0 || commandCode != 0 {
		t.Fatalf("a clean tree answered script=%d command=%d, want 0 and 0", scriptCode, commandCode)
	}
	if len(scriptArgv) == 0 {
		t.Fatal("the script started no container")
	}
	if len(scriptArgv) != len(commandArgv) {
		t.Fatalf("the script passed %d arguments and the command passed %d:\nscript:  %q\ncommand: %q",
			len(scriptArgv), len(commandArgv), scriptArgv, commandArgv)
	}
	for i := range scriptArgv {
		if scriptArgv[i] != commandArgv[i] {
			t.Errorf("argument %d is %q in the script and %q in the command", i, scriptArgv[i], commandArgv[i])
		}
	}
}

// VALIDATES: a dirty tree refuses in both halves, and neither starts a
// container.
// PREVENTS: the port judging a release candidate against a tree that carries
// uncommitted work, which is the one thing this gate exists to make impossible.
func TestNeitherHalfRunsOverADirtyTree(t *testing.T) {
	fixture := t.TempDir()
	stubDir := stubs(t, fixture)
	const dirty = " M internal/component/bgp/reactor/peer.go\n?? tmp/scratch\n"

	scriptCode, scriptArgv := runScript(t, stubDir, dirty, "0")
	commandCode, commandArgv := runCommand(t, fixture, stubDir, dirty, "0")

	if scriptCode != 1 || commandCode != 1 {
		t.Errorf("a dirty tree answered script=%d command=%d, want 1 and 1", scriptCode, commandCode)
	}
	if len(scriptArgv) != 0 {
		t.Errorf("the script started a container over a dirty tree: %q", scriptArgv)
	}
	if len(commandArgv) != 0 {
		t.Errorf("the command started a container over a dirty tree: %q", commandArgv)
	}
}

// VALIDATES: the container's own exit status is what each half answers.
// PREVENTS: a flattened 1, which would stop commit_helper.py telling a gate
// that ran and failed from a gate that could not run (AC-8).
func TestBothHalvesAnswerTheContainerExitStatus(t *testing.T) {
	for _, code := range []string{"1", "2", "3", "125"} {
		t.Run(code, func(t *testing.T) {
			fixture := t.TempDir()
			stubDir := stubs(t, fixture)

			scriptCode, _ := runScript(t, stubDir, "", code)
			commandCode, _ := runCommand(t, fixture, stubDir, "", code)

			if scriptCode != commandCode {
				t.Errorf("the script answered %d and the command answered %d", scriptCode, commandCode)
			}
		})
	}
}

// VALIDATES: the container script the command carries is the one the script
// hands to bash, character for character.
// PREVENTS: a silent edit to either copy while both exist. The argv comparison
// above already covers it, and this states the fact on its own so a failure
// says WHICH thing drifted.
func TestTheContainerScriptIsTheSameProgram(t *testing.T) {
	body, err := os.ReadFile("effective-verify.sh")
	if err != nil {
		t.Fatalf("read the script: %v", err)
	}

	text := string(body)
	open := strings.Index(text, "bash -lc '")
	if open < 0 {
		t.Fatal("the script no longer hands a program to bash -lc")
	}
	start := open + len("bash -lc '")
	end := strings.Index(text[start:], "'\n")
	if end < 0 {
		t.Fatal("the script's bash program is not closed")
	}

	if got := text[start : start+end]; got != evidence.ContainerScript {
		t.Errorf("the container program differs.\nscript:\n%s\ncommand:\n%s", got, evidence.ContainerScript)
	}
}
