// Design: docs/contributing/running-commands.md -- refusals preserve the owner's requested operation.
package humantest

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// RetryArguments asks a shell to decode the printed retry into argv without
// executing le, then installs any printed env assignments for a producer replay.
// Test operands MUST be harmless even if broken quoting evaluates a substitution.
func RetryArguments(t testing.TB, refusal error) []string {
	t.Helper()
	if refusal == nil {
		t.Fatal("expected a terminal refusal")
	}
	lines := strings.Split(refusal.Error(), "\n")
	line := strings.TrimSpace(lines[len(lines)-1])
	decode := "set -- " + line + "; printf '%s\\000' \"$@\""
	if strings.HasPrefix(line, "env ") {
		// Intercept env so the shell supplies its real parsed operands but
		// never invokes the printed le executable.
		decode = "env() { printf '%s\\000' \"$@\"; }; " + line
	} else if !strings.HasPrefix(line, "./le ") {
		t.Fatalf("refusal has no retry command: %v", refusal)
	}
	command := exec.CommandContext(t.Context(), "bash", "-c", decode)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("decode retry %q: %v", line, err)
	}
	arguments := strings.Split(strings.TrimSuffix(string(output), "\x00"), "\x00")
	for len(arguments) != 0 {
		if arguments[0] == "./le" {
			env.ResetCache()
			t.Cleanup(env.ResetCache)
			return arguments
		}
		key, value, assignment := strings.Cut(arguments[0], "=")
		if !assignment {
			t.Fatalf("unexpected retry prefix %q", arguments[0])
		}
		t.Setenv(key, value)
		arguments = arguments[1:]
	}
	t.Fatal("retry command has no ./le executable")
	return nil
}
