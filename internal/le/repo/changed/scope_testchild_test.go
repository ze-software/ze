// Related: ../../go/toolchain/gotoolchain.go -- Toolchain.Environment, the test child's environment
//
// VALIDATES: a Go test the verify gate starts inherits neither change-scope
// variable, under the spelling ScopeFileKey and ScopeTagsKey declare.
// PREVENTS: a test narrowed by the gate's change set, and a test environment
// that names a file under one run's worktree, so two runs of one commit hand
// the test different values the Go test cache cannot see (the
// spec-verify-gate-warm-timed-first-red assumption A-7). gotoolchain cannot
// import this package, so it spells the two keys itself; this test is the
// check that compares the copy with its declaration.

package repochanged

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
)

func TestATestChildInheritsNoChangeScope(t *testing.T) {
	t.Setenv("GOFLAGS", "-trimpath")
	for _, key := range []string{ScopeFileKey, ScopeTagsKey} {
		t.Setenv(key, "/worktree/tmp/verify/scope")
	}
	environ := gotoolchain.Toolchain{Root: t.TempDir(), Procs: 1}.Environment(gotoolchain.EnvOptions{Test: true})
	for _, key := range []string{ScopeFileKey, ScopeTagsKey} {
		if kept := env.Without(environ, key); len(kept) != len(environ) {
			t.Errorf("a test child inherits %s; Toolchain.Environment strips every spelling of it for EnvOptions{Test: true}", key)
		}
	}
	for _, entry := range environ {
		if strings.Contains(entry, "/worktree/tmp/verify/scope") {
			t.Errorf("a test child's environment still names the gate's scope file: %s", entry)
		}
	}
}
