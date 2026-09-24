// VALIDATES: `le doc index check` judges ai/DOCS-TO-CODE.md, ai/CODE-TO-DOCS.md
// and the anchors in one run, and `le doc index write` regenerates both files
// (AC-29, D-5 of plan/spec-le-subject-first-command-tree.md).
// PREVENTS: the merged check dropping one of the three judgments the four old
// verbs made, or the write leaving one of the two files stale.

package docindex

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// TestDocIndexCheckCoversBothFilesAndAnchors drives the command through Answer.
// It writes both files, then breaks one thing at a time and asks check for its
// verdict: each stale file answers StaleExit, and an anchor to a missing path
// answers 1. A write after each break must bring check back to 0.
func TestDocIndexCheckCoversBothFilesAndAnchors(t *testing.T) {
	root := tree(t, map[string]string{
		"ai/.keep":               "",
		"internal/a/a.go":        "// Design: docs/architecture/x.md -- topic\npackage a\n\nfunc Run() {}\n",
		"docs/architecture/x.md": "# X\n\nRun starts it.\n<!-- source: internal/a/a.go -- Run -->\n",
	})
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	answer := func(verb string) int {
		t.Helper()
		_, code := Answer([]string{verb})
		return code
	}
	edit := func(relative, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(relative)), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", relative, err)
		}
	}

	if code := answer("write"); code != 0 {
		t.Fatalf("write answered %d over a fresh tree", code)
	}
	for _, relative := range []string{OutputRel, CodeOutputRel} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); err != nil {
			t.Fatalf("write did not produce %s: %v", relative, err)
		}
	}
	if code := answer("check"); code != 0 {
		t.Fatalf("check answered %d right after write", code)
	}

	for _, relative := range []string{OutputRel, CodeOutputRel} {
		edit(relative, "stale\n")
		if code := answer("check"); code != StaleExit {
			t.Errorf("a stale %s answered %d, want %d", relative, code, StaleExit)
		}
		if code := answer("write"); code != 0 {
			t.Fatalf("write answered %d over a stale %s", code, relative)
		}
		if code := answer("check"); code != 0 {
			t.Errorf("check answered %d after write regenerated %s", code, relative)
		}
	}

	edit("docs/architecture/y.md", "# Y\n\n<!-- source: internal/gone/gone.go -- Gone -->\n")
	if code := answer("write"); code != 0 {
		t.Fatalf("write answered %d with an unresolved anchor", code)
	}
	if code := answer("check"); code != 1 {
		t.Errorf("an anchor to a missing path answered %d, want 1", code)
	}
}
