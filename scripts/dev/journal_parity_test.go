// The migration proof for scripts/dev/journal.py. The script and the native
// journal report read the same git HEAD and answer the same bytes and code.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 for ze-journal-report.
// PREVENTS: a port that reads another session's worktree rows, changes the
// recurrence threshold, or turns an unreadable journal into an empty success.
//
// This file stays beside the producer. The later script-removal change removes
// both the old implementation and this temporary old/new comparison.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/le/journal"
)

const journalParityHead = "| Date | Spec | Surface | Symptom | Fix |\n|------|------|---------|---------|-----|\n"

func TestJournalBothHalvesAgreeOnFixtures(t *testing.T) {
	tests := []struct {
		name      string
		committed map[string]string
		worktree  map[string]string
		noGit     bool
	}{
		{
			name: "one row is omitted",
			committed: map[string]string{
				"plan/journal/one.md": journalParityHead +
					"| 2026-08-01 | one | cli | once | fix |\n",
			},
		},
		{
			name: "recurrence span and class sorting",
			committed: map[string]string{
				"plan/journal/zebra.md": journalParityHead +
					"| 2026-08-20 | z | cli | later | fix |\n" +
					"| 2026-08-01 | z | cli | earlier | fix |\n",
				"plan/journal/alpha.md": journalParityHead +
					"| 2026-07-15 | a | cli | first | fix |\n" +
					"| 2026-07-16 | a | cli | second | fix |\n",
			},
		},
		{
			name: "head differs from worktree",
			committed: map[string]string{
				"plan/journal/committed.md": journalParityHead +
					"| 2026-08-01 | a | cli | first | fix |\n" +
					"| 2026-08-03 | b | cli | second | fix |\n",
			},
			worktree: map[string]string{
				"plan/journal/worktree-only.md": journalParityHead +
					"| 2026-08-04 | c | cli | first | fix |\n" +
					"| 2026-08-05 | d | cli | second | fix |\n",
			},
		},
		{
			name:      "worktree journal with no class at head is refused",
			committed: map[string]string{"README.md": "fixture\n"},
			worktree: map[string]string{
				"plan/journal/worktree-only.md": journalParityHead +
					"| 2026-08-04 | c | cli | first | fix |\n",
			},
		},
		{
			name: "malformed pipes are refused",
			committed: map[string]string{
				"plan/journal/bad-pipes.md": journalParityHead +
					"2026-08-01 | a | cli | missing leading pipe | fix |\n" +
					"| 2026-08-02 | b | cli | raw | pipe | fix |\n",
			},
		},
		{
			name: "unparseable date is refused",
			committed: map[string]string{
				"plan/journal/undated.md": journalParityHead +
					"| - | a | cli | no date | fix |\n",
			},
		},
		{
			name:  "git head is unreadable",
			noGit: true,
		},
		{
			name:      "empty journal stays empty",
			committed: map[string]string{"README.md": "fixture\n"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tree := t.TempDir()
			if !test.noGit {
				journalParityWrite(t, tree, test.committed)
				journalParityGit(t, tree, "init", "--quiet", "--initial-branch=main")
				journalParityGit(t, tree, "config", "user.email", "test@example.com")
				journalParityGit(t, tree, "config", "user.name", "Ze Test")
				journalParityGit(t, tree, "config", "commit.gpgsign", "false")
				journalParityGit(t, tree, "add", "--all")
				journalParityGit(t, tree, "commit", "--quiet", "--message=seed journal")
				journalParityWrite(t, tree, test.worktree)
			}

			script := devPyRunScript(t, "journal.py", []string{"--repo", tree}, devPyRoot(t))
			command := journalParityRunGo(tree)
			devPyAgree(t, test.name, script, command, script.Stdout, command.Stdout)
			if script.Stderr != command.Stderr {
				t.Errorf("stderr differs\nscript:\n%s\ncommand:\n%s", script.Stderr, command.Stderr)
			}
		})
	}
}

// TestJournalBothHalvesAgreeOverTheCheckout validates the exact live output and
// exit code. A fixture cannot reveal a class shape that exists only in HEAD.
func TestJournalBothHalvesAgreeOverTheCheckout(t *testing.T) {
	root := devPyRoot(t)
	script := devPyRunScript(t, "journal.py", nil, root)
	command := journalParityRunGo(root)

	devPyAgree(t, "journal over the checkout", script, command, script.Stdout, command.Stdout)
	if script.Stderr != command.Stderr {
		t.Errorf("live stderr differs\nscript:\n%s\ncommand:\n%s", script.Stderr, command.Stderr)
	}
}

func journalParityRunGo(tree string) devPyResult {
	var errOut bytes.Buffer
	report, code := journal.Run(tree, &errOut)
	return devPyResult{Stdout: report.Text(), Stderr: errOut.String(), Code: code}
}

func journalParityWrite(t *testing.T, tree string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		path := filepath.Join(tree, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("fixture file: %v", err)
		}
	}
}

func journalParityGit(t *testing.T, tree string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", tree}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", args[0], err, output)
	}
}
