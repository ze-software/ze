// Design: docs/architecture/testing/ci-format.md -- per-command helper exits.

package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestForegroundHelperCommandExit parses real directives and runs their children
// to prove exit= governs each helper without replacing the file-level contract.
func TestForegroundHelperCommandExit(t *testing.T) {
	cases := []struct {
		name, command, exit, failure string
	}{
		{name: "zero", command: "echo helper-output", exit: "0"},
		{name: "nonzero", command: "echo helper-output; exit 3", exit: "3"},
		{name: "upper bound", command: "echo helper-output; exit 255", exit: "255"},
		{name: "expected nonzero", command: "echo helper-output", exit: "3", failure: "expected exit code 3, got 0"},
		{name: "expected zero", command: "echo helper-output; exit 3", exit: "0", failure: "expected exit code 0, got 3"},
		{name: "wrong nonzero", command: "echo helper-output; exit 4", exit: "3", failure: "expected exit code 3, got 4"},
		{name: "signal is not an exit", command: "echo helper-output; kill -TERM $$", exit: "0", failure: "expected exit code 0, got -1"},
		{name: "undeclared zero", command: "echo helper-output"},
		{name: "undeclared nonzero", command: "echo helper-output; exit 3", failure: "exit status 3"},
	}
	for _, position := range []string{"intermediate", "final"} {
		for _, tc := range cases {
			t.Run(position+"/"+tc.name, func(t *testing.T) {
				body := "cmd=foreground:seq=1:exec=sh -c '" + tc.command + "'"
				if tc.exit != "" {
					body += ":exit=" + tc.exit
				}
				body += "\nexpect=stdout:contains=helper-output\n"
				if position == "intermediate" {
					body += "cmd=foreground:seq=2:exec=sh -c 'echo next-command'\nexpect=stdout:contains=next-command\nexpect=exit:code=0\n"
				}
				r, rec := helperExitRecord(t, body)
				passed := r.runTest(t.Context(), rec, &RunOptions{})
				if passed != (tc.failure == "") {
					t.Fatalf("passed=%v, error=%v, want failure %q", passed, rec.Error, tc.failure)
				}
				if tc.failure != "" {
					if rec.Error == nil || !strings.Contains(rec.Error.Error(), tc.failure) {
						t.Fatalf("error=%v, want %q", rec.Error, tc.failure)
					}
					if !strings.Contains(rec.ClientStdout, "helper-output") {
						t.Fatalf("failure lost helper output: %q", rec.ClientStdout)
					}
					if strings.Contains(rec.ClientStdout, "next-command") {
						t.Fatal("continued after a failed helper")
					}
				}
			})
		}
	}
}

// TestForegroundHelperExitTimeout keeps the deadline authoritative even when
// the helper declares a status and the file also asserts that status.
func TestForegroundHelperExitTimeout(t *testing.T) {
	for _, position := range []string{"intermediate", "final"} {
		t.Run(position, func(t *testing.T) {
			body := "option=timeout:value=100ms\ncmd=foreground:seq=1:exec=sh -c 'echo helper-output; exec sleep 30':exit=0\nexpect=stdout:contains=helper-output\n"
			if position == "intermediate" {
				body += "cmd=foreground:seq=2:exec=sh -c 'echo next-command'\n"
			}
			body += "expect=exit:code=0\n"
			r, rec := helperExitRecord(t, body)
			if r.runTest(t.Context(), rec, &RunOptions{}) {
				t.Fatal("a helper running past its deadline passed")
			}
			if rec.FailureType != stateTimeout {
				t.Fatalf("failure=%q error=%v, want timeout", rec.FailureType, rec.Error)
			}
			if rec.Error == nil || !strings.Contains(rec.Error.Error(), "cmd seq=1") {
				t.Fatalf("timeout lost the helper identity: %v", rec.Error)
			}
			if strings.Contains(rec.ClientStdout, "next-command") {
				t.Fatal("continued after the deadline")
			}
		})
	}
}

// TestForegroundHelperFileExit keeps both assertions active when exit= and
// expect=exit describe the final helper, including conflicting expectations.
func TestForegroundHelperFileExit(t *testing.T) {
	for _, tc := range []struct{ name, exit, fileExit, failure string }{
		{"both match", "3", "3", ""},
		{"file mismatch", "3", "0", "expected exit code 0, got 3"},
		{"command mismatch", "0", "3", "cmd seq=1"},
		{"file only", "", "3", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := "cmd=foreground:seq=1:exec=sh -c 'echo helper-output; exit 3'"
			if tc.exit != "" {
				body += ":exit=" + tc.exit
			}
			body += "\nexpect=stdout:contains=helper-output\nexpect=exit:code=" + tc.fileExit + "\n"
			r, rec := helperExitRecord(t, body)
			if passed := r.runTest(t.Context(), rec, &RunOptions{}); passed != (tc.failure == "") {
				t.Fatalf("passed=%v error=%v, want failure %q", passed, rec.Error, tc.failure)
			}
			if tc.failure != "" {
				if rec.Error == nil || !strings.Contains(rec.Error.Error(), tc.failure) {
					t.Fatalf("error=%v, want %q", rec.Error, tc.failure)
				}
			}
		})
	}
}

// helperExitRecord uses the same discovery and execution entry points as a suite.
func helperExitRecord(t *testing.T, body string) (*Runner, *Record) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "helper-exit.ci")
	if err := os.WriteFile(path, []byte("option=timeout:value=10s\n"+body), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := NewEncodingTests(dir)
	rec, err := tests.parseAndAdd(path)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRunner(tests, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Cleanup)
	rec.Active = true
	return r, rec
}
