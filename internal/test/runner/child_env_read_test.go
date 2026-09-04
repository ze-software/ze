package runner

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// childEnvAssignmentPattern finds a literal `NAME=` assignment the runner hands
// a test child. It deliberately does not match a name built at run time: those
// carry their own producer and a reader can follow it.
var childEnvAssignmentPattern = regexp.MustCompile(`"([A-Za-z_][A-Za-z0-9_.]*)=`)

// knownChildEnvExceptions are names this test accepts without a reader in the
// tree, with the reason. Empty on purpose: a name belongs here only when
// something outside this repository reads it.
var knownChildEnvExceptions = map[string]string{}

// VALIDATES: every environment variable the runner sets on a test child by a
// literal name is read by something in this repository.
//
// PREVENTS: a knob that looks live and is not. `SLOG_LEVEL=DEBUG` was set on
// every test child until 2026-09-04 and nothing had ever read it: slogutil
// resolves a level from `ze.log*` alone. It read as though those children logged
// at debug, so a session investigating a failure took an absence of log lines as
// evidence about the daemon rather than about the knob, and drew a wrong
// conclusion from it. A dead env var is worse than no env var, because it
// answers a question nobody asked it.
func TestEveryLiteralChildEnvNameIsReadSomewhere(t *testing.T) {
	data, err := os.ReadFile("runner_exec.go")
	if err != nil {
		t.Fatalf("read runner_exec.go: %v", err)
	}

	names := map[string]bool{}
	for _, match := range childEnvAssignmentPattern.FindAllStringSubmatch(string(data), -1) {
		name := match[1]
		// A name with a dot is a ze config key; env.Get accepts either spelling
		// and the config layer reads it, so those have a reader by construction.
		if strings.Contains(name, ".") {
			continue
		}
		names[name] = true
	}
	if len(names) == 0 {
		t.Skip("no literal underscore-style env assignments found; the shape this test reads has changed")
	}

	for name := range names {
		if reason, ok := knownChildEnvExceptions[name]; ok {
			t.Logf("%s accepted without a reader: %s", name, reason)
			continue
		}
		if !readSomewhereInTree(t, name) {
			t.Errorf("the runner sets %s on every test child and nothing in this repository reads it.\n"+
				"Either wire it to a producer, delete it, or record it in knownChildEnvExceptions with the reason.\n"+
				"A knob that looks live and is not is how a session comes to trust an absence of output.", name)
		}
	}
}

// readSomewhereInTree reports whether the name appears anywhere outside the one
// file that sets it. A grep is the right instrument here: a reader may reach the
// value through env.Get, os.Getenv, a config key lookup or a shell script, and
// this test is about whether ANY consumer exists, not about which.
func readSomewhereInTree(t *testing.T, name string) bool {
	t.Helper()
	// Search CODE only. Prose is not a reader: a journal row or a handover
	// document naming a dead variable would otherwise vouch for it, and the
	// first version of this test passed a mutant for exactly that reason.
	// --full-name makes the paths repository-relative. Without it git grep
	// answers paths relative to the CURRENT directory, so the file that sets
	// the variable comes back as a bare "runner_exec.go", the exclusion below
	// never matches it, and the setter vouches for itself. The second version
	// of this test passed a mutant for exactly that reason.
	cmd := exec.Command("git", "grep", "-l", "--full-name", "--", name,
		":/*.go", ":/*.sh", ":/*.py", ":/*.yaml", ":/*.yml", ":/*.ci", ":/*.conf")
	cmd.Dir = "."
	out, err := cmd.Output()
	if err != nil {
		// git grep exits 1 when it matches nothing, which is the answer.
		return false
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		file := strings.TrimSpace(line)
		if file == "" {
			continue
		}
		// The file that SETS it does not count as reading it, and neither does
		// this test, which names it in its own prose.
		if strings.HasSuffix(file, "internal/test/runner/runner_exec.go") ||
			strings.HasSuffix(file, "internal/test/runner/child_env_read_test.go") {
			continue
		}
		return true
	}
	return false
}
