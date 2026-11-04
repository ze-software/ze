// Design: docs/contributing/rfc-implementation-guide.md -- retries preserve the ruling and verdict.
package rfc

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/le/human/humantest"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestOwnerRulingRefusalPreservesRetryArguments shell-decodes the printed retry,
// replays the registered writer under a PTY with a different operator identity,
// and reads the original namespace's ledger to verify the ruling's exact meaning.
func TestOwnerRulingRefusalPreservesRetryArguments(t *testing.T) {
	humantest.Streams(t, "piped-both")
	root, _ := fixtureCommit(t, map[string]string{
		"pkg/widget_test.go": "package pkg\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n",
	})
	t.Setenv("ZE_REPO_ROOT", root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "ruling-origin-fixture")
	t.Setenv("ZE_AGENT_ID", "origin-agent")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	session, err := lepath.CommitSessionFor(root, "ruling-origin-fixture", "")
	if err != nil {
		t.Fatal(err)
	}
	reason := "owner ruling 2026-11-03 accepts Thomas's \"$widget\" and `printf marker`"
	_, err = Approve(root, session, "pkg.TestX", reason)
	t.Setenv("ZE_REPO_ROOT", t.TempDir())
	t.Setenv("CLAUDE_CODE_SESSION_ID", "operator-fixture")
	t.Setenv("ZE_AGENT_ID", "operator-agent")
	env.ResetCache()
	got := humantest.RetryArguments(t, err)
	want := []string{"./le", "rfc", "approve", "unit", "pkg.TestX", "reason", reason}
	if !slices.Equal(got, want) {
		t.Errorf("retry argv = %q, want %q", got, want)
	}
	t.Run("retry", func(t *testing.T) {
		humantest.Streams(t, "terminal")
		if _, code := Answer(got[2:]); code != 0 {
			t.Fatalf("printed retry exit = %d, want 0", code)
		}
		rows, err := approvalRows(root, session)
		if err != nil {
			t.Fatal(err)
		}
		if rows["pkg.TestX"] != reason || ownerRuling(rows["pkg.TestX"]) != "2026-11-03" {
			t.Fatalf("replayed ruling = %q, want %q in the original namespace", rows["pkg.TestX"], reason)
		}
	})
}
