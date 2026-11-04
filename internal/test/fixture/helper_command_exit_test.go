// Design: docs/architecture/testing/ci-format.md -- foreground helper exit assertions.

package fixture

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// TestHelperCommandExit runs the registered driver in child test processes to
// prove requested statuses preserve stdout without reporting observer failures.
func TestHelperCommandExit(t *testing.T) {
	if marker := slices.Index(os.Args, "--fixture-helper-command-exit"); marker >= 0 {
		os.Exit(Run(append([]string{"runner/helper-command-exit"}, os.Args[marker+1:]...)))
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, output, failure string
		args                  []string
		code                  int
	}{
		{name: "first", args: []string{"first-helper", "3"}, output: "first-helper\n", code: 3},
		{name: "zero", args: []string{"zero-helper", "0"}, output: "zero-helper\n", code: 0},
		{name: "final", args: []string{"final-helper", "7"}, output: "final-helper\n", code: 7},
		{name: "upper bound", args: []string{"upper-helper", "255"}, output: "upper-helper\n", code: 255},
		{name: "missing arguments", code: 1, failure: "requires <stdout-line> <exit-code>"},
		{name: "missing status", args: []string{"no-status"}, code: 1, failure: "requires <stdout-line> <exit-code>"},
		{name: "extra argument", args: []string{"extra", "3", "stray"}, code: 1, failure: "requires <stdout-line> <exit-code>"},
		{name: "negative status", args: []string{"negative", "-1"}, code: 1, failure: "parse helper exit code"},
		{name: "overflow status", args: []string{"overflow", "256"}, code: 1, failure: "parse helper exit code"},
		{name: "invalid status", args: []string{"invalid", "three"}, code: 1, failure: "parse helper exit code"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"-test.run=^TestHelperCommandExit$", "--", "--fixture-helper-command-exit"}, tc.args...)
			cmd := exec.CommandContext(t.Context(), executable, args...)
			cmd.Dir = t.TempDir()
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				if _, ok := errors.AsType[*exec.ExitError](err); !ok {
					t.Fatalf("start helper: %v", err)
				}
			}
			if code := cmd.ProcessState.ExitCode(); code != tc.code {
				t.Fatalf("exit=%d, want %d; stderr=%q", code, tc.code, stderr.String())
			}
			if output := stdout.String(); output != tc.output {
				t.Fatalf("stdout=%q, want %q", output, tc.output)
			}
			if tc.failure != "" {
				if !strings.Contains(stderr.String(), observerFailure) {
					t.Fatalf("invalid input did not report observer failure: %q", stderr.String())
				}
				if !strings.Contains(stderr.String(), tc.failure) {
					t.Fatalf("stderr=%q, want %q", stderr.String(), tc.failure)
				}
				return
			}
			if stderr.Len() != 0 {
				t.Fatalf("requested exit reported a failure: %q", stderr.String())
			}
		})
	}
}
