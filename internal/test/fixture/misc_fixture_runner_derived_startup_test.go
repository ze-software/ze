// Design: docs/architecture/core-design.md -- startup proves context before artifact absence.
package fixture

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDerivedLifecycleRejectsUnprovedStartup drives the registered fixture entry,
// not a startup parser. Only the session-start command response is controlled;
// every named read and write reaches the supplied native le unchanged.
// MUTATION: discard startup's status/output/deadline and every invalid response
// earns the startup marker and completes the otherwise real lifecycle.
func TestDerivedLifecycleRejectsUnprovedStartup(t *testing.T) {
	binary, err := nativeLEBinary()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name         string
		startup      string
		wantDeadline bool
	}{
		{
			name:    "unknown",
			startup: "exec \"$native\" ai hooks derived-fixture-unknown\n",
		},
		{
			name:    "refused-after-real-context",
			startup: "\"$native\" \"$@\" || exit $?\nexit 2\n",
		},
		{
			name:    "noop",
			startup: "exit 0\n",
		},
		{
			name:    "unrelated-context",
			startup: "printf '%s\\n' 'the session started'\nexit 0\n",
		},
		{
			name:    "prefixes-only",
			startup: "printf '%s\\n' 'Warning: RULE:' 'Rules:'\nexit 0\n",
		},
		{
			name:    "missing-warning",
			startup: derivedStartupOmitNotice("Warning: RULE: Read spec + source files BEFORE writing any code"),
		},
		{
			name:    "missing-rules",
			startup: derivedStartupOmitNotice("Rules: ai/rules/TRIGGERS.md names every rule with the situation it covers -- when one matches, read that rule's file in full before acting"),
		},
		{
			name: "hung-after-real-context",
			// sleep(timeout-under-test): the semantic command must outlive the actual
			// five-second harness deadline; exec leaves only its owned child.
			startup:      "\"$native\" \"$@\" || exit $?\nexec sleep 6\n",
			wantDeadline: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			audit := derivedStartupCommand(t, binary, test.startup)
			code, output := derivedStartupRunFixture(t)
			if code != 1 {
				t.Errorf("fixture exit = %d, want assertion refusal 1:\n%s", code, output)
			}
			if !strings.Contains(output, observerFailure+": ") {
				t.Errorf("fixture did not emit its authoritative assertion-failure result:\n%s", output)
			}
			if strings.Contains(output, observerFailure+": fixture panic:") {
				t.Errorf("fixture panicked instead of rejecting the startup response:\n%s", output)
			}
			if !strings.Contains(output, "derived-fixture-ready\n") {
				t.Errorf("fixture did not reach the prepared startup stage:\n%s", output)
			}
			// Run's failure report serializes the wrapped context sentinel. Keep
			// deadline cancellation distinct from a late successful child exit.
			if test.wantDeadline && !strings.Contains(output, context.DeadlineExceeded.Error()) {
				t.Errorf("hung startup did not fail by its context deadline:\n%s", output)
			}
			if strings.Contains(output, "session-start-left-the-artifact-absent") {
				t.Errorf("unproved startup earned the lifecycle success marker:\n%s", output)
			}
			commands, err := os.ReadFile(audit)
			if err != nil {
				t.Fatal(err)
			}
			if string(commands) != "ai hooks session-start\n" {
				t.Fatalf("invalid startup did not stop at its native command boundary: %s", commands)
			}
		})
	}
}

// TestDerivedLifecycleAcceptsRealStartup retains all useful lifecycle outcomes:
// actual required startup context, deferred initial rendering, invalidation and
// a named read that rebuilds from the edited tree.
func TestDerivedLifecycleAcceptsRealStartup(t *testing.T) {
	binary, err := nativeLEBinary()
	if err != nil {
		t.Fatal(err)
	}
	audit := derivedStartupCommand(t, binary, "exec \"$native\" \"$@\"\n")
	code, output := derivedStartupRunFixture(t)
	if code != 0 {
		t.Fatalf("real startup/lifecycle fixture exited %d:\n%s", code, output)
	}
	for _, marker := range []string{
		"derived-fixture-ready",
		"session-start-left-the-artifact-absent",
		"initial-read-rendered-the-artifact",
		"input-write-removed-the-artifact",
		"read-materialized-the-artifact",
		"grep-read-the-edited-tree",
	} {
		if !strings.Contains(output, marker+"\n") {
			t.Errorf("real lifecycle omitted %q:\n%s", marker, output)
		}
	}
	commands, err := os.ReadFile(audit)
	if err != nil {
		t.Fatal(err)
	}
	for command, want := range map[string]int{
		"ai hooks session-start\n":      1,
		"ai hooks pretool-bash\n":       2,
		"ai hooks posttool-writeedit\n": 1,
	} {
		if got := strings.Count(string(commands), command); got != want {
			t.Errorf("native boundary %q reached %d times, want %d: %s", command, got, want, commands)
		}
	}
}

// derivedStartupCommand installs a semantic command fixture: startup alone can
// vary, while every other argv is exec-forwarded to the native producer.
func derivedStartupCommand(t *testing.T, binary, startup string) string {
	t.Helper()
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	audit := filepath.Join(bin, "commands")
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
	script := "#!/bin/sh\nnative=" + quote(binary) + "\n" +
		"printf '%s\\n' \"$*\" >> " + quote(audit) + "\n" +
		"if [ \"$#\" -eq 3 ] && [ \"$1\" = ai ] && [ \"$2\" = hooks ] && [ \"$3\" = session-start ]; then\n" +
		startup + "fi\nexec \"$native\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "le"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return audit
}

// derivedStartupOmitNotice filters one exact line from the real hook's output;
// its other context and exit status survive unchanged.
func derivedStartupOmitNotice(notice string) string {
	quoted := "'" + strings.ReplaceAll(notice, "'", "'\"'\"'") + "'"
	return "\"$native\" \"$@\" > startup-context\ncode=$?\n" +
		"while IFS= read -r line; do\n" +
		"if [ \"$line\" != " + quoted + " ]; then printf '%s\\n' \"$line\"; fi\n" +
		"done < startup-context\nexit \"$code\"\n"
}

// derivedStartupRunFixture enters the registry route used by le test fixture.
// Output goes to an owned file, not a pipe that could block a verbose refusal.
func derivedStartupRunFixture(t *testing.T) (int, string) {
	t.Helper()
	// Each Run models a fresh fixture process. ReportFailure intentionally emits
	// once per process; preserve the surrounding test process's latch afterward.
	previousReport := reportedOnce.Swap(false)
	defer reportedOnce.Store(previousReport)
	t.Chdir(t.TempDir())
	capture := filepath.Join(t.TempDir(), "output")
	file, err := os.Create(capture)
	if err != nil {
		t.Fatal(err)
	}
	code := func() int {
		previousOut, previousErr := os.Stdout, os.Stderr
		os.Stdout, os.Stderr = file, file
		defer func() { os.Stdout, os.Stderr = previousOut, previousErr }()
		return Run([]string{"runner/le-derived-artifact-lifecycle"})
	}()
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	return code, string(output)
}
