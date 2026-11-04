// Design: docs/contributing/committing.md -- content gates consume the captured prospective tree.
package commit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestJournalCaptureReadsImmutableSelectedSources changes a captured check's
// checkout bytes immediately after the real snapshot lists its index entries.
// The gate and actual generated publication must consume the selected source,
// while preserving those later checkout bytes.
func TestJournalCaptureReadsImmutableSelectedSources(t *testing.T) {
	root := newCommitRepository(t)
	configureCommitAuthor(t, root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "journal-snapshot-fixture")
	const code = "internal/guard/guard.go"
	const proof = "internal/guard/guard_test.go"
	const class = "plan/journal/captured-snapshot.md"
	const selected = "package guard\n\nfunc Guard() {}\n"
	writeCommitFixture(t, root, code, selected)
	writeCommitFixture(t, root, proof, "package guard\n\nimport \"testing\"\n\nfunc TestGuard(t *testing.T) {}\n")
	writeCommitFixture(t, root, class, "# Captured snapshot\n\nCaptured: 2026-08-12 check internal/guard.Guard proof internal/guard.TestGuard\n\n| Date | Spec | Surface | Symptom | Fix |\n|------|------|---------|---------|-----|\n| 2026-08-01 | - | cli | fixture defect | fixed: captured check |\n")
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	marker := filepath.Join(bin, "snapshot-listed")
	wrapper := "#!/bin/bash\nif [ \"${1:-}\" = ls-files ] && [ \"${2:-}\" = -s ] && [ \"${3:-}\" = -z ] && [[ \"${GIT_INDEX_FILE:-}\" = *commit-snapshot-*/index ]] && [ ! -e \"$ZE_TEST_SNAPSHOT_LISTED\" ]; then\n  \"$ZE_TEST_REAL_GIT\" \"$@\" || exit $?\n  : > \"$ZE_TEST_SNAPSHOT_LISTED\"\n  printf 'package guard\\n' > " + shellQuote(code) + "\n  exit 0\nfi\nexec \"$ZE_TEST_REAL_GIT\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o700); err != nil { //nolint:gosec // the real Git forwarding fixture must execute
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ZE_TEST_REAL_GIT", realGit)
	t.Setenv("ZE_TEST_SNAPSHOT_LISTED", marker)
	prepared, err := Create(root, &Options{Subject: "capture immutable selected sources", Files: []string{code, proof, class}})
	if _, reached := os.Stat(marker); reached != nil {
		t.Fatalf("snapshot mutation boundary never ran: %v (create: %v)", reached, err)
	}
	if err != nil {
		t.Fatalf("content gate read concurrent checkout sources instead of the prospective tree: %v", err)
	}
	runCommitScript(t, root, prepared.Script)
	if got := runCommitGitOutput(t, root, "show", "HEAD:"+code); got != selected {
		t.Fatalf("published source = %q, want immutable selected bytes", got)
	}
	body, err := os.ReadFile(filepath.Join(root, code))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(body)) != "package guard" {
		t.Fatalf("publication replaced concurrent checkout bytes: %q", body)
	}
}
