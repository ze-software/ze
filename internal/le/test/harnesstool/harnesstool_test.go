// VALIDATES: a harness command registered through harnesstool forwards every
// word after `test <name>` to its handler, a trailing help word included, and
// answers the handler's exit code (AC-32 of
// spec-le-subject-first-command-tree).
package harnesstool

import (
	"slices"
	"testing"

	leroot "github.com/ze-software/ze/internal/le/le/root"
	// The probe registers `test zzprobe`, and the dependencies link areas
	// under repo and under the namespaces below. Dispatch refuses a first word
	// with no summary, so the repo and site areas and every namespace package
	// are linked, as the composition root links them.
	_ "github.com/ze-software/ze/internal/le/ai"
	_ "github.com/ze-software/ze/internal/le/arch"
	_ "github.com/ze-software/ze/internal/le/build"
	_ "github.com/ze-software/ze/internal/le/chaos"
	_ "github.com/ze-software/ze/internal/le/cli"
	_ "github.com/ze-software/ze/internal/le/config"
	_ "github.com/ze-software/ze/internal/le/data"
	_ "github.com/ze-software/ze/internal/le/doc"
	_ "github.com/ze-software/ze/internal/le/go"
	_ "github.com/ze-software/ze/internal/le/lab"
	_ "github.com/ze-software/ze/internal/le/plugin"
	_ "github.com/ze-software/ze/internal/le/repo"
	_ "github.com/ze-software/ze/internal/le/site"
	_ "github.com/ze-software/ze/internal/le/spec"
	_ "github.com/ze-software/ze/internal/le/test"
	_ "github.com/ze-software/ze/internal/le/web"
	_ "github.com/ze-software/ze/internal/le/yang"
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
	leroot.RegisterForwarding(name, Helper)

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
