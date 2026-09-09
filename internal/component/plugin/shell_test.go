// Design: docs/architecture/api/process-protocol.md -- how an external plugin is started
// Overview: shell.go -- the probe these tests exercise
//
// Goal: prove the probe names the shell it looked for, so a caller reports the
// absent dependency rather than the plugin that could not start. Method: give
// the probe a path this test owns, because no test can take /bin/sh away from
// the host it runs on.

package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellAvailableNamesTheAbsentShell(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "sh")

	err := shellAvailable(absent)
	if err == nil {
		t.Fatal("a shell that is not on disk must be reported, and it passed")
	}
	if !strings.Contains(err.Error(), absent) {
		t.Errorf("the error must name the shell the probe looked for, and it says %q", err.Error())
	}
}

func TestShellAvailableAcceptsAShellOnDisk(t *testing.T) {
	present := filepath.Join(t.TempDir(), "sh")
	if err := os.WriteFile(present, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write the stand-in shell: %v", err)
	}

	if err := shellAvailable(present); err != nil {
		t.Errorf("a shell on disk must pass, and it answered %q", err.Error())
	}
}

// TestShellAvailableRefusesWhatTheForkCannotExecute holds what the probe is
// for: a doctor check that passes while every plugin start fails sends the
// operator to the plugin, and the fault is the shell. The fork needs a file it
// can execute, so the probe asks for exactly that.
func TestShellAvailableRefusesWhatTheForkCannotExecute(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "sh")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatalf("make the directory standing where the shell would be: %v", err)
	}

	unreadable := filepath.Join(t.TempDir(), "sh")
	if err := os.WriteFile(unreadable, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatalf("write the non-executable stand-in shell: %v", err)
	}

	cases := []struct {
		name  string
		shell string
	}{
		{name: "a directory at the shell's path", shell: directory},
		{name: "a file with no execute bit", shell: unreadable},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := shellAvailable(testCase.shell)
			if err == nil {
				t.Fatal("a shell the fork cannot execute must be reported, and it passed")
			}
			if !strings.Contains(err.Error(), testCase.shell) {
				t.Errorf("the error must name the shell the probe looked for, and it says %q", err.Error())
			}
		})
	}
}

// TestShellAvailableAnswersForTheUserZeRunsAs holds what a permission probe is
// for: a mode carries three answers, and only the one for this process decides
// whether the fork starts. A shell with mode 0700 owned by root is the
// production shape of that, and an unprivileged test cannot write such a file,
// so this case inverts the ownership and keeps the property: the owner holds no
// execute bit and the group and the world hold one.
//
// A probe reading the bits sees 0o077&0o111, passes, and reports a healthy host
// on which every plugin start fails with EACCES. Root is the one user for whom
// passing is correct, because the kernel lets root execute any file that
// anybody can execute, so the expectation follows the user the test runs as.
func TestShellAvailableAnswersForTheUserZeRunsAs(t *testing.T) {
	othersOnly := filepath.Join(t.TempDir(), "sh")
	if err := os.WriteFile(othersOnly, []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatalf("write the stand-in shell this user cannot execute: %v", err)
	}
	// The mode is set by a chmod because os.WriteFile takes the process umask
	// off the mode it is given, and the world bits are what this case needs.
	if err := os.Chmod(othersOnly, 0o077); err != nil {
		t.Fatalf("set the mode this user cannot execute: %v", err)
	}

	err := shellAvailable(othersOnly)

	if os.Geteuid() == 0 {
		if err != nil {
			t.Errorf("root may execute a file anybody can execute, and the probe answered %q", err.Error())
		}
		return
	}
	if err == nil {
		t.Fatal("a shell whose execute bits belong to other users must be reported, and it passed")
	}
	if !strings.Contains(err.Error(), othersOnly) {
		t.Errorf("the error must name the shell the probe looked for, and it says %q", err.Error())
	}
}
