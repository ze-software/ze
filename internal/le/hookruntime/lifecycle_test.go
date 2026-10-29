// Design: docs/architecture/testing/verify-freshness-scope.md -- the session hook reads the ledger's one producer
package hookruntime

import (
	"bytes"
	stdcontext "context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/derived"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
	"github.com/ze-software/ze/internal/le/rfc"
	specpath "github.com/ze-software/ze/internal/le/spec/path"
	"github.com/ze-software/ze/internal/le/verify"
)

// TestSubagentContextResolvesClaim drives the JSON hook entry over every current
// category and unresolved claims, so a filename cannot become an invented path.
func TestSubagentContextResolvesClaim(t *testing.T) {
	const claim = "spec-context-probe.md"
	tests := []struct {
		name  string
		paths []string
		want  string
	}{
		{name: "missing", want: "no such spec"},
		{name: "duplicate", paths: []string{
			"plan/release/fixes/" + claim,
			"plan/post-release/engineering/" + claim,
		}, want: "two buckets"},
	}
	for _, category := range specpath.Categories() {
		tests = append(tests, struct {
			name  string
			paths []string
			want  string
		}{name: category.Name, paths: []string{category.Dir + "/" + claim}})
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeHookFixture(t, root, "tmp/session/.session-context-parent", claim+"\n")
			for _, path := range test.paths {
				writeHookFixture(t, root, path, "# Spec: context probe\n")
			}
			code, output, diagnostics := runHook(t, root, "subagent-context",
				map[string]any{"session_id": "context-parent"})
			if code != 0 {
				t.Fatalf("code = %d, want advisory context; stderr: %s", code, diagnostics)
			}
			var response struct {
				Output struct {
					Event   string `json:"hookEventName"`
					Context string `json:"additionalContext"`
				} `json:"hookSpecificOutput"`
			}
			if err := json.Unmarshal([]byte(output), &response); err != nil {
				t.Fatal(err)
			}
			if response.Output.Event != "SubagentStart" {
				t.Fatalf("event = %q", response.Output.Event)
			}
			context := response.Output.Context
			if test.want != "" {
				for _, want := range []string{"Cannot resolve the session's claimed spec " + claim, test.want, "./le spec claim"} {
					if !strings.Contains(context, want) {
						t.Errorf("context missing %q: %s", want, context)
					}
				}
				if strings.Contains(context, "Spec claimed by the session that spawned you:") {
					t.Errorf("unresolved claim emitted a spec path: %s", context)
				}
				return
			}
			if !strings.Contains(context, "Spec claimed by the session that spawned you: "+test.paths[0]+"\n") {
				t.Errorf("context missing category-qualified path: %s", context)
			}
			if strings.Contains(context, "Cannot resolve") {
				t.Errorf("resolved claim emitted a failure: %s", context)
			}
		})
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
				journal += fmt.Sprintf("| 2026-09-%02d | - | cli | symptom %d | fixed: fix |\n", day, day)
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

// TestSessionStartCountsOnlyUncapturedOrRecurredClasses drives the due line
// through the hook over a class captured by a check. Twelve rows dated on or
// before the capture print no due line, where the same rows uncaptured would;
// one row dated after it prints the line, because a check capture is due on
// its first recurrence.
func TestSessionStartCountsOnlyUncapturedOrRecurredClasses(t *testing.T) {
	const line = "journal: 1 problem classes are due for a fix pass (./le spec journal report)"
	for _, test := range []struct {
		name  string
		after int
		want  bool
	}{{name: "rows before the capture", after: 0, want: false}, {name: "one row after", after: 1, want: true}} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			for _, artifact := range derived.All() {
				writeHookFixture(t, root, artifact.Path, "# derived\n")
			}
			writeHookFixture(t, root, "internal/guard/guard.go", "package guard\n\nfunc Guard() {}\n")
			writeHookFixture(t, root, "internal/guard/guard_test.go",
				"package guard\n\nimport \"testing\"\n\nfunc TestGuard(t *testing.T) {}\n")
			journal := "Captured: 2026-09-12 check internal/guard.Guard proof internal/guard.TestGuard\n\n" +
				"| Date | Spec | Surface | Symptom | Fix |\n|------|------|---------|---------|-----|\n"
			for day := 1; day <= 12+test.after; day++ {
				journal += fmt.Sprintf("| 2026-09-%02d | - | cli | symptom %d | fixed: fix |\n", day, day)
			}
			writeHookFixture(t, root, "plan/journal/captured.md", journal)
			commitHookFixture(t, root)
			printed, noted := runSessionStartStreams(t, root)
			if strings.Contains(noted, "journal:") {
				t.Fatalf("the hook wrote %q to stderr, want the journal counted", noted)
			}
			if strings.Contains(printed, line) != test.want {
				t.Fatalf("the hook printed %q, want the due line %v", printed, test.want)
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
	// The roadmap renders from a committed plan tree (git HEAD), never the
	// working tree, so the fixture holds one spec and is a checkout, or that
	// member cannot render at all.
	writeHookFixture(t, root, "plan/release/fixes/spec-fixture.md",
		"# Spec: fixture\n\n| Field | Value |\n|---|---|\n| Status | ready |\n| Depends | - |\n"+
			"| Phase | 1/1 |\n| Updated | 2026-10-29 |\n| Priority | P1 |\n| Release reason | Session start fixture |\n")
	commitHookFixture(t, root)
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

	// The tree can render them, every one. Without this, a fixture that simply
	// cannot build a member of the family would pass every assertion above for
	// that member.
	for _, rendered := range artifacts {
		if err := rendered.Rebuild(root); err != nil {
			t.Fatalf("the fixture cannot render %s at all, so nothing above was a decision: %v",
				rendered.Path, err)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rendered.Path))); err != nil {
			t.Fatalf("%s does not land in this fixture even when rendered on purpose: %v", rendered.Path, err)
		}
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
// whole session-start message with it, the push-gate warning
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
			// The identity is pinned: with none in the environment and no
			// harness ancestor, the hook mints one and caches it under
			// tmp/session/ in the fixture, which carries no .gitignore, so
			// the cache file reads as an untracked file and the tree as dirty.
			t.Setenv("CLAUDE_CODE_SESSION_ID", "session-start-fixture")
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

// TestSessionStartReportsDerivedDebt drives the session-start hook over real
// git checkouts with a real upstream and asserts the debt line the derivation
// produces: one warning and the owed commits when HEAD's tree is not the green
// tree, nothing when it is, a stderr note naming the fix when the main
// checkout has no upstream, and silence in a linked worktree with none,
// because a worktree agent never pushes. Each run stays inside the hook's
// budget, which is what A-5 claims for one rev-list and one log.
// VALIDATES: AC-22, AC-11, AC-12, A-5.
func TestSessionStartReportsDerivedDebt(t *testing.T) {
	const warning = "verification debt: 1 commit(s) owe a scoped verify since "
	t.Run("a commit after the green record", func(t *testing.T) {
		root := derivedDebtCheckout(t, true)
		green := derivedDebtCommit(t, root, "the verified change")
		derivedDebtGreen(t, root, green)
		derivedDebtCommit(t, root, "a change after the verify")

		started := time.Now()
		printed, noted := runSessionStartStreams(t, root)
		if elapsed := time.Since(started); elapsed >= sessionStartBudget {
			t.Fatalf("the hook took %s, past its %s budget", elapsed, sessionStartBudget)
		}
		if !strings.Contains(printed, warning+green[:7]) {
			t.Fatalf("the hook printed %q, want %q naming the green commit", printed, warning)
		}
		if !strings.Contains(printed, "push refused until cleared") {
			t.Fatalf("the hook printed %q, want the push refusal named", printed)
		}
		if !strings.Contains(printed, "a change after the verify") {
			t.Fatalf("the hook printed %q, want the owed commit's subject", printed)
		}
		if strings.Contains(noted, "verification debt") {
			t.Fatalf("the hook said on stderr %q over a derivation that answered", noted)
		}
	})
	t.Run("HEAD's tree is the green tree", func(t *testing.T) {
		root := derivedDebtCheckout(t, true)
		derivedDebtGreen(t, root, derivedDebtCommit(t, root, "the verified change"))
		printed, noted := runSessionStartStreams(t, root)
		if strings.Contains(printed, "verification debt") || strings.Contains(noted, "verification debt") {
			t.Fatalf("the hook warned over a verified HEAD: stdout %q stderr %q", printed, noted)
		}
	})
	t.Run("the main checkout has no upstream", func(t *testing.T) {
		root := derivedDebtCheckout(t, false)
		derivedDebtGreen(t, root, derivedDebtCommit(t, root, "the verified change"))
		printed, noted := runSessionStartStreams(t, root)
		if strings.Contains(printed, "verification debt") {
			t.Fatalf("the hook printed %q, want nothing on stdout when it cannot derive", printed)
		}
		if !strings.Contains(noted, "git branch --set-upstream-to") {
			t.Fatalf("the hook said %q on stderr, want the missing upstream and its fix", noted)
		}
	})
	t.Run("a linked worktree has no upstream", func(t *testing.T) {
		root := derivedDebtCheckout(t, true)
		linked := filepath.Join(t.TempDir(), "linked")
		derivedDebtGit(t, root, "worktree", "add", "-q", "-b", "agent", linked)
		for _, artifact := range derived.All() {
			writeHookFixture(t, linked, artifact.Path, "# derived\n")
		}
		printed, noted := runSessionStartStreams(t, linked)
		if strings.Contains(printed, "verification debt") || strings.Contains(noted, "verification debt") {
			t.Fatalf("a linked worktree with no upstream spoke of debt: stdout %q stderr %q", printed, noted)
		}
	})
}

// derivedDebtCheckout is a committed Ze checkout on main with every derived
// artifact seeded, so the hook renders nothing else, and, when upstream is
// set, a bare upstream that holds the baseline.
func derivedDebtCheckout(t *testing.T, upstream bool) string {
	t.Helper()
	root := t.TempDir()
	writeHookFixture(t, root, "go.mod", "module fixture\n")
	writeHookFixture(t, root, "feature-gates.txt", "")
	derivedDebtGit(t, root, "init", "-q", "--initial-branch=main")
	derivedDebtGit(t, root, "add", "go.mod", "feature-gates.txt")
	derivedDebtGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
		"commit", "-q", "-m", "baseline")
	for _, artifact := range derived.All() {
		writeHookFixture(t, root, artifact.Path, "# derived\n")
	}
	if !upstream {
		return root
	}
	remote := filepath.Join(t.TempDir(), "origin.git")
	derivedDebtGit(t, root, "init", "-q", "--bare", "--initial-branch=main", remote)
	derivedDebtGit(t, root, "remote", "add", "origin", remote)
	derivedDebtGit(t, root, "push", "-q", "-u", "origin", "main")
	return root
}

// derivedDebtCommit commits one new file named after the subject and answers
// the commit's full SHA.
func derivedDebtCommit(t *testing.T, root, subject string) string {
	t.Helper()
	path := "change-" + strings.ReplaceAll(subject, " ", "-") + ".txt"
	writeHookFixture(t, root, path, subject+"\n")
	derivedDebtGit(t, root, "add", "--", path)
	derivedDebtGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
		"commit", "-q", "-m", subject)
	return strings.TrimSpace(derivedDebtGit(t, root, "rev-parse", "HEAD"))
}

// derivedDebtGreen records a green, uncut verification over commit at root.
// derivedDebtGreen records a green scoped verify (`./le verify changed`) over
// commit at root, anchored where the run would anchor it: the merge-base of
// commit and @{u}, or commit itself in a checkout with no upstream, where the
// derivation refuses on the missing upstream before it reads the record.
func derivedDebtGreen(t *testing.T, root, commit string) {
	t.Helper()
	tree := strings.TrimSpace(derivedDebtGit(t, root, "rev-parse", commit+"^{tree}"))
	upstream, anchor := commit, commit
	resolve := exec.CommandContext(t.Context(), "git", "rev-parse", "@{u}")
	resolve.Dir = root
	if out, err := resolve.Output(); err == nil {
		upstream = strings.TrimSpace(string(out))
		anchor = strings.TrimSpace(derivedDebtGit(t, root, "merge-base", commit, upstream))
	}
	record, err := verify.NewScopedRecord(commit, tree,
		repochanged.Anchor{Commit: anchor, Upstream: "@{u}", UpstreamCommit: upstream}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := verify.WriteScopedRecord(root, record); err != nil {
		t.Fatal(err)
	}
}

// TestSessionStartDebtLineNamesVerifyChanged drives the session-start hook
// over a checkout with an unpushed non-prose commit after the scoped record:
// the debt line says the commits owe a scoped verify and names
// `./le verify changed`, and it never names a full verify.
// VALIDATES: AC-19.
// PREVENTS: an agent told at session start to run the full gate it may not run.
func TestSessionStartDebtLineNamesVerifyChanged(t *testing.T) {
	root := derivedDebtCheckout(t, true)
	scoped := derivedDebtCommit(t, root, "the verified change")
	derivedDebtGreen(t, root, scoped)
	derivedDebtCommit(t, root, "a change after the verify")
	printed, _ := runSessionStartStreams(t, root)
	at := strings.Index(printed, "verification debt: ")
	if at < 0 {
		t.Fatalf("the hook printed %q, want a debt line", printed)
	}
	line, _, _ := strings.Cut(printed[at:], "\n")
	want := "1 commit(s) owe a scoped verify since " + scoped[:7] + ": run ./le verify changed"
	if !strings.Contains(line, want) {
		t.Fatalf("the debt line is %q, want %q", line, want)
	}
	for _, full := range []string{"full verify", "verify worktree", "verify current"} {
		if strings.Contains(line, full) {
			t.Fatalf("the debt line %q names a full verify (%q)", line, full)
		}
	}
}

func derivedDebtGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", args...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return string(output)
}
