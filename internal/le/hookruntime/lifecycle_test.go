// Design: docs/architecture/testing/verify-freshness-scope.md -- the session hook reads the ledger's one producer
package hookruntime

import (
	"bytes"
	stdcontext "context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/derived"
	"github.com/ze-software/ze/internal/le/rfc"
)

// TestSessionHookCountsNoDischargedRowAsOwed drives the third consumer of the
// ledger over a discharged row.
//
// The hook holds no rule of its own: it reads commit.ListDebt and counts the
// rows whose status is open, so a discharge narrows what it prints with no edit
// here. That is what the test proves, and the first half is why it can: the
// same shard with no discharge record still warns.
func TestSessionHookCountsNoDischargedRowAsOwed(t *testing.T) {
	root := t.TempDir()
	// The hook builds every registered derived artifact that is absent, which
	// has nothing to do with the ledger and would read a tree this fixture does
	// not hold. Seeding them keeps this test about the ledger alone.
	for _, artifact := range derived.All() {
		writeHookFixture(t, root, artifact.Path, "# derived\n")
	}

	const row = "| 2026-09-08 | aaaaaaaa | a commit | independent critical review | no reviewer | open |"
	writeHookFixture(t, root, "plan/verification-debt/aaaaaaaa.md",
		"| Date | Session | Subject | Gate owed | Reason | Status |\n"+
			"|------|---------|---------|-----------|--------|--------|\n"+row+"\n")

	if printed := runSessionStart(t, root); !strings.Contains(printed, "verification debt: 1 gate(s) owed") {
		t.Fatalf("the hook printed %q, want one owed gate before the discharge", printed)
	}

	digest := sha256.Sum256([]byte(row))
	writeHookFixture(t, root, "plan/verification-debt/discharged/cccccccc.md",
		"| Date | Shard | Line | Row digest | Kind | Commit | Artifact | Authorisation |\n"+
			"|------|-------|------|------------|------|--------|----------|---------------|\n"+
			"| 2026-09-08 | aaaaaaaa.md | 3 | "+hex.EncodeToString(digest[:])+
			" | owner | | | Thomas ordered this commit |\n")

	if printed := runSessionStart(t, root); strings.Contains(printed, "verification debt") {
		t.Fatalf("the hook printed %q, want no owed gate once the row is discharged", printed)
	}
}

// TestSessionStartNamesDueJournalClasses drives the due-class line through the
// hook entry point over committed journals of 9 and 10 rows, and over a tree
// git cannot read. It proves the line appears at the threshold, stays absent
// below it, and that an unreadable journal prints nothing on stdout and says
// why on stderr.
func TestSessionStartNamesDueJournalClasses(t *testing.T) {
	const line = "journal: 1 problem classes are due for a fix pass (./le spec journal report)"
	tests := []struct {
		name string
		rows int
		git  bool
		want bool
	}{
		{name: "nine rows", rows: 9, git: true, want: false},
		{name: "ten rows", rows: 10, git: true, want: true},
		{name: "unreadable journal", rows: 10, git: false, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			// Seed every derived artifact so the hook renders nothing and this
			// test stays about the journal line.
			for _, artifact := range derived.All() {
				writeHookFixture(t, root, artifact.Path, "# derived\n")
			}
			journal := "| Date | Spec | Surface | Symptom | Fix |\n|------|------|---------|---------|-----|\n"
			for day := 1; day <= test.rows; day++ {
				journal += fmt.Sprintf("| 2026-09-%02d | - | cli | symptom %d | fix |\n", day, day)
			}
			writeHookFixture(t, root, "plan/journal/recurring.md", journal)
			if test.git {
				commitHookFixture(t, root)
			}
			printed, noted := runSessionStartStreams(t, root)
			if strings.Contains(noted, "journal: due classes not counted: ") == test.git {
				t.Fatalf("the hook wrote %q to stderr, want the unreadable-journal note only without git", noted)
			}
			if strings.Contains(printed, line) != test.want {
				t.Fatalf("the hook printed %q, want the due line %v", printed, test.want)
			}
			if !test.want && strings.Contains(printed, "journal:") {
				t.Fatalf("the hook printed %q, want no journal line on stdout", printed)
			}
		})
	}
}

// commitHookFixture commits the whole fixture tree, because the journal
// report reads git HEAD and never the working tree.
func commitHookFixture(t *testing.T, root string) {
	t.Helper()
	for _, arguments := range [][]string{
		{"init", "--quiet"},
		{"add", "--all"},
		{"-c", "user.email=test@example.com", "-c", "user.name=Ze Test", "-c", "commit.gpgsign=false",
			"commit", "--quiet", "--message=seed"},
	} {
		command := exec.CommandContext(t.Context(), "git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %s in the fixture: %v\n%s", arguments[0], err, output)
		}
	}
}

// runSessionStart runs the session-start hook over one throwaway root and
// answers what it printed.
func runSessionStart(t *testing.T, root string) string {
	t.Helper()
	printed, _ := runSessionStartStreams(t, root)
	return printed
}

// runSessionStartStreams runs the session-start hook over one throwaway root
// and answers its stdout and its stderr.
func runSessionStartStreams(t *testing.T, root string) (string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	hookSessionStart(context{root: root, input: map[string]any{}}, &out, &errOut)
	return out.String(), errOut.String()
}

func writeHookFixture(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// artifactsWithPolicy answers the registered artifacts under one session-start
// policy, and fails when there are none: every assertion below is over a
// population, and an empty one agrees with anything.
func artifactsWithPolicy(t *testing.T, policy derived.SessionStartPolicy) []derived.Artifact {
	t.Helper()
	var held []derived.Artifact
	for _, artifact := range derived.All() {
		if artifact.SessionStart == policy {
			held = append(held, artifact)
		}
	}
	if len(held) == 0 {
		t.Fatalf("no registered artifact declares policy %d, so this test asserts nothing", policy)
	}
	return held
}

// TestSessionStartBuildsEveryArtifactDeclaredForIt proves the session hook reads
// the registry rather than a list of names.
//
// Two hardcoded os.Stat blocks named ai/DOCS-TO-CODE.md and ai/CODE-TO-DOCS.md
// until 2026-09-11, so a third artifact was absent at every session start until
// somebody edited this hook. The assertion is over derived.All(), which is why
// a fourth artifact moves this test with it and needs no line here.
//
// It reads SessionStartBuild alone, because an artifact's policy is now part of
// its registration and the deferred ones are the sibling test's subject.
func TestSessionStartBuildsEveryArtifactDeclaredForIt(t *testing.T) {
	root := derivedFixture(t)
	artifacts := artifactsWithPolicy(t, derived.SessionStartBuild)
	for _, artifact := range artifacts {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(artifact.Path))); err != nil {
			t.Fatalf("clear %s: %v", artifact.Path, err)
		}
	}

	printed := runSessionStart(t, root)

	for _, artifact := range artifacts {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(artifact.Path))); err != nil {
			t.Errorf("%s is still absent after a session start: %v", artifact.Path, err)
		}
		if !strings.Contains(printed, "Built "+artifact.Path) {
			t.Errorf("the hook printed no line for %s:\n%s", artifact.Path, printed)
		}
	}
}

// TestSessionStartRendersNoDeferredArtifact is the budget half, over a tree that
// CAN render the deferred family.
//
// VALIDATES: an artifact registered SessionStartDefer is left absent by a
// session start, and no line claims it was built.
// PREVENTS: the 2026-09-11 regression. The five RFC artifacts registered with
// the three documentation indexes, so the hook rendered 194 shards inside its 5
// second timeout: measured at 2.68s and 2.03s with all eight present and 5.32s
// with the five absent. Absent is the common state, because their predicate
// covers every `*.go` write in every session. A hook killed at its timeout
// loses the whole session-start message.
//
// The corpus is what makes this a decision rather than an inability: the last
// step renders one of the deferred artifacts by hand and requires it to land.
func TestSessionStartRendersNoDeferredArtifact(t *testing.T) {
	root := derivedFixture(t)
	rfcCorpusFixture(t, root)
	artifacts := artifactsWithPolicy(t, derived.SessionStartDefer)
	for _, artifact := range artifacts {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(artifact.Path))); !os.IsNotExist(err) {
			t.Fatalf("%s is in the fixture before the hook ran, so its absence after proves nothing", artifact.Path)
		}
	}

	printed := runSessionStart(t, root)

	for _, artifact := range artifacts {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(artifact.Path))); !os.IsNotExist(err) {
			t.Errorf("the session start rendered %s, which is deferred to the command that names it",
				artifact.Path)
		}
		if strings.Contains(printed, "Built "+artifact.Path) {
			t.Errorf("the hook reported building a deferred artifact:\n%s", printed)
		}
	}

	// The tree can render them. Without this, a fixture that simply cannot
	// build the family would pass every assertion above.
	rendered := artifacts[0]
	if err := rendered.Rebuild(root); err != nil {
		t.Fatalf("the fixture cannot render %s at all, so nothing above was a decision: %v",
			rendered.Path, err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rendered.Path))); err != nil {
		t.Fatalf("%s does not land in this fixture even when rendered on purpose: %v", rendered.Path, err)
	}
}

// rfcCorpusFixture writes the smallest tree the RFC generator renders from: one
// enrolled summary gating one MUST, that RFC's own text, and the workflow
// directory the carrier walk reads.
//
// The same corpus drives the .ci scenario end to end
// (internal/test/fixture/misc_fixture_runner_rfcledger.go). It is spelled again
// here because this package cannot import that one, and both exist to make an
// absent artifact mean a decision rather than an empty tree.
func rfcCorpusFixture(t *testing.T, root string) {
	t.Helper()
	writeHookFixture(t, root, "docs/features/.keep", "")
	for rel, body := range rfc.FixtureFiles() {
		writeHookFixture(t, root, rel, body)
	}
}

// TestSessionStartLeavesAPresentArtifactAlone bounds what this hook does.
//
// The rebuild is ABSENT-ONLY because the hook has a budget: `.claude/settings.json`
// gives `le ai hooks session-start` 5 seconds, and rendering all three
// artifacts does not fit inside it. A hook killed at its timeout loses the
// whole session-start message with it, the verification-debt warning
// included, and leaves every artifact after the kill
// point exactly as it found them. Measure it before changing this:
//
//	dir=$(./le session scratch ensure)
//	echo '{}' | time ./le ai hooks session-start > "$dir/session-start.log" 2>&1
//
// The cost of the bound is a STATED limitation: a write no Write or Edit hook
// sees leaves the artifact present and stale until the next hooked write to one
// of its inputs. `docs/contributing/navigating-the-code.md` names it, and the
// spec's Known Limitations carries it.
func TestSessionStartLeavesAPresentArtifactAlone(t *testing.T) {
	root := derivedFixture(t)
	present := filepath.Join(root, "ai", "DOCS-TO-CODE.md")
	const body = "written by something no hook saw\n"
	if err := os.WriteFile(present, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	printed := runSessionStart(t, root)

	after, err := os.ReadFile(present) //nolint:gosec // a fixture path this test built under t.TempDir()
	if err != nil {
		t.Fatalf("read the artifact after the session start: %v", err)
	}
	if string(after) != body {
		t.Errorf("a present artifact was rebuilt, which the hook has no budget for:\n%s", after)
	}
	if strings.Contains(printed, "Built ai/DOCS-TO-CODE.md") {
		t.Errorf("the hook reported building an artifact that was already there:\n%s", printed)
	}
}

// TestSessionStartPrintsItsNoticeWhenAStepOverruns drives the hook with a step
// that outlasts the budget, the shape a loaded machine gives the ledger read.
//
// VALIDATES: the fixed notice is the first line whatever a step costs,
// the hook returns at its budget instead of waiting for the step, one stderr
// line names the step that ran out and every step after it, and no report
// after the cut reaches stdout.
// PREVENTS: the harness killing the hook at its 5 s timeout and dropping the
// whole message, the fixed notice included.
func TestSessionStartPrintsItsNoticeWhenAStepOverruns(t *testing.T) {
	root := t.TempDir()
	for _, artifact := range derived.All() {
		writeHookFixture(t, root, artifact.Path, "# derived\n")
	}
	release := make(chan struct{})
	budget, steps := sessionStartBudget, sessionStartSteps
	t.Cleanup(func() {
		close(release)
		sessionStartBudget, sessionStartSteps = budget, steps
	})
	sessionStartBudget = 100 * time.Millisecond
	slow := sessionStartStep{name: "slow", run: func(stdcontext.Context, *sessionStart, *sessionStartReport) { <-release }}
	sessionStartSteps = append([]sessionStartStep{slow}, steps...)

	started := time.Now()
	printed, noted := runSessionStartStreams(t, root)
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("the hook returned after %s, want it cut at its %s budget", elapsed, sessionStartBudget)
	}
	if !strings.HasPrefix(printed, "Warning: RULE: Read spec + source files BEFORE writing any code") {
		t.Fatalf("the hook printed %q, want the fixed notice first", printed)
	}
	names := make([]string, 0, len(sessionStartSteps))
	for _, step := range sessionStartSteps {
		names = append(names, step.name)
	}
	want := "session-start: the 100ms budget ran out, so these reports were skipped: " + strings.Join(names, ", ") + "\n"
	if noted != want {
		t.Fatalf("the hook wrote %q to stderr, want %q", noted, want)
	}
	for _, report := range []string{"Clean tree", "uncommitted", "specs", "Tip:"} {
		if strings.Contains(printed, report) {
			t.Fatalf("the hook printed %q after the cut, want no report containing %q", printed, report)
		}
	}
}

// TestSessionStartReportsTheTree drives the tree report through the hook over
// a clean repository and over a directory git cannot read.
//
// VALIDATES: a clean repository prints "Clean tree" and no stderr note; a
// status git cannot give is said on stderr and never printed as a clean tree.
// PREVENTS: a failed git status reading as a clean tree, which the hook printed
// until 2026-09-26.
func TestSessionStartReportsTheTree(t *testing.T) {
	tests := []struct {
		name  string
		git   bool
		clean bool
		note  string
	}{
		{name: "clean repository", git: true, clean: true, note: ""},
		{name: "no repository", git: false, clean: false, note: "session-start: git status failed, so the tree was not counted: "},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			for _, artifact := range derived.All() {
				writeHookFixture(t, root, artifact.Path, "# derived\n")
			}
			if test.git {
				commitHookFixture(t, root)
			}
			printed, noted := runSessionStartStreams(t, root)
			if strings.Contains(printed, "Clean tree") != test.clean {
				t.Fatalf("the hook printed %q, want the clean-tree line %v", printed, test.clean)
			}
			if test.note == "" && strings.Contains(noted, "session-start:") {
				t.Fatalf("the hook wrote %q to stderr, want no session-start note", noted)
			}
			if test.note != "" && !strings.Contains(noted, test.note) {
				t.Fatalf("the hook wrote %q to stderr, want %q", noted, test.note)
			}
		})
	}
}

// TestSessionStartPrintsAReadyReportPastTheDeadline asks for the next report
// when a report and the deadline are both ready, 100 times.
//
// VALIDATES: the ready report is answered every time.
// PREVENTS: select picking the deadline at random and printing a finished step
// as skipped, which one in two tries would do.
func TestSessionStartPrintsAReadyReportPastTheDeadline(t *testing.T) {
	deadline, cancel := stdcontext.WithCancel(t.Context())
	cancel()
	for try := range 100 {
		reports := make(chan *sessionStartReport, 1)
		reports <- &sessionStartReport{}
		if _, arrived := nextSessionStartReport(deadline, reports); !arrived {
			t.Fatalf("try %d: a ready report was dropped for a passed deadline", try)
		}
		if _, arrived := nextSessionStartReport(deadline, reports); arrived {
			t.Fatalf("try %d: an empty channel answered a report after the deadline", try)
		}
	}
}
