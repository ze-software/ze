// VALIDATES: a harness command registered through harnesstool forwards every
// word after `test <name>` to its handler, a trailing help word included, and
// answers the handler's exit code (AC-32 of
// plan/spec-le-subject-first-command-tree.md).
package harnesstool

import (
	"slices"
	"testing"

	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// TestAnswerForwardsWordsAndExitCode drives the real dispatcher over a probe
// command, so the forwarding flag and the adapter are both on the path.
func TestAnswerForwardsWordsAndExitCode(t *testing.T) {
	var received []string
	calls := 0
	const name = "test zzprobe"
	leroot.Register(name, leroot.GroupSuite, Answer(func(args []string) int {
		calls++
		received = args
		return 7
	}), Meta("a probe harness command"))
	leroot.RegisterForwarding(name)

	if !leroot.Forwards(name) {
		t.Fatal("test zzprobe does not forward its words")
	}
	for _, words := range [][]string{{"a", "b"}, {"x", "--help"}, {}} {
		argv := append([]string{"test", "zzprobe"}, words...)
		code := leroot.Dispatch("le", argv)
		if code != 7 {
			t.Errorf("le %v answered %d, want the handler's 7", argv, code)
		}
		if !slices.Equal(received, words) && (len(received) != 0 || len(words) != 0) {
			t.Errorf("le %v: handler received %q, want %q", argv, received, words)
		}
	}
	if calls != 3 {
		t.Errorf("handler ran %d times, want 3", calls)
	}
}
