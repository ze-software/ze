package main

import (
	"os"
	"path/filepath"
	"testing"
)

// These tests drive scripts/dev/spec-closure-check.py, the mechanical detector
// written for the Stop-hook closure gate (block-premature-stop.sh) and used by
// the commit-time reminder in commit_helper.py. See ai/rules/planning.md
// "Spec Closure" and ai/rules/repo-maintenance.md.
//
// The Stop hook was registered on no event from 41e5fa44f (2026-06-29) until
// Thomas re-registered it on 2026-07-31. The closure gate runs again, so both
// consumers are live: the Stop hook and the commit-time reminder. These tests
// guard --spec, because the exit-3 contract is what the Stop gate binds to.

const specMetaInProgress = `# Spec: widget

| Field | Value |
|-------|-------|
| Status | in-progress |
| Phase | 3/3 |

## Task
Build the widget.
`

// VALIDATES: an in-progress spec whose stem-matching learned summary is already
// committed is flagged (commit A ran, commit B "git rm the spec" never did).
// PREVENTS: implemented specs being silently orphaned in plan/ forever, which
// this whole gate exists to stop.
func TestSpecClosureFlagsCommittedButOpenSpec(t *testing.T) {
	root := makeCommitHelperFixture(t)
	configFixtureGit(t, root)
	writeFixture(t, root, ".gitignore", "tmp/*\n")
	writeFixture(t, root, "plan/spec-widget.md", specMetaInProgress)
	writeFixture(t, root, "plan/learned/900-widget.md", "# widget\n\nlesson body\n")
	commitFixtureAll(t, root, "feat: widget")

	// --spec exits 3 and explains closure on stderr.
	_, stderr, code := runSpecClosure(t, root, "--spec", "plan/spec-widget.md")
	if code != 3 {
		t.Fatalf("expected exit 3, got %d\nstderr:\n%s", code, stderr)
	}
	mustContain(t, stderr, "COMPLETED BUT NOT CLOSED")
	mustContain(t, stderr, "git rm plan/spec-widget.md")

	// --list surfaces it on stdout.
	stdout, _, lcode := runSpecClosure(t, root, "--list")
	if lcode != 0 {
		t.Fatalf("--list exit %d\n%s", lcode, stdout)
	}
	mustContain(t, stdout, "plan/spec-widget.md")
}

// VALIDATES: a learned summary that exists on disk but is NOT committed does not
// flag the spec. commit_helper.py `learned-next` creates the file early,
// mid-implementation, so on-disk presence alone is not evidence of completion.
// PREVENTS: the Stop hook wedging an active implementation session.
func TestSpecClosureIgnoresUncommittedLearned(t *testing.T) {
	root := makeCommitHelperFixture(t)
	configFixtureGit(t, root)
	writeFixture(t, root, ".gitignore", "tmp/*\n")
	writeFixture(t, root, "plan/spec-widget.md", specMetaInProgress)
	commitFixtureAll(t, root, "wip: widget spec")
	// Learned file exists on disk but is never committed.
	writeFixture(t, root, "plan/learned/900-widget.md", "# widget\n\nlesson body\n")

	_, stderr, code := runSpecClosure(t, root, "--spec", "plan/spec-widget.md")
	if code != 0 {
		t.Fatalf("expected exit 0 (learned not committed), got %d\nstderr:\n%s", code, stderr)
	}
}

// VALIDATES: the ack escape hatch (tmp/session/.closure-ack-<stem>) suppresses
// the block so a genuinely-still-open spec is not a hard wedge.
func TestSpecClosureAckEscapeHatch(t *testing.T) {
	root := makeCommitHelperFixture(t)
	configFixtureGit(t, root)
	writeFixture(t, root, ".gitignore", "tmp/*\n")
	writeFixture(t, root, "plan/spec-widget.md", specMetaInProgress)
	writeFixture(t, root, "plan/learned/900-widget.md", "# widget\n\nlesson body\n")
	commitFixtureAll(t, root, "feat: widget")
	writeFixture(t, root, "tmp/session/.closure-ack-widget", "still finishing phase 3\n")

	_, stderr, code := runSpecClosure(t, root, "--spec", "plan/spec-widget.md")
	if code != 0 {
		t.Fatalf("expected exit 0 with ack present, got %d\nstderr:\n%s", code, stderr)
	}
}

// VALIDATES: once the spec file is gone (commit B ran), there is nothing to
// close and the gate is silent.
func TestSpecClosureClosedSpecIsClean(t *testing.T) {
	root := makeCommitHelperFixture(t)
	configFixtureGit(t, root)
	writeFixture(t, root, ".gitignore", "tmp/*\n")
	writeFixture(t, root, "plan/learned/900-widget.md", "# widget\n\nlesson body\n")
	commitFixtureAll(t, root, "feat: widget (spec already removed)")

	_, _, code := runSpecClosure(t, root, "--spec", "plan/spec-widget.md")
	if code != 0 {
		t.Fatalf("expected exit 0 for absent spec, got %d", code)
	}
}

// VALIDATES: an umbrella spec (has child spec-<stem>-<N>-*.md) is NOT hard-blocked
// even when its own-stem learned summary is committed, because umbrellas close via
// their children. PREVENTS the umbrella false-positive class the triage audit found
// (fib-depth, anomaly-0, perf-next-0, tiers-0 were all umbrellas).
func TestSpecClosureUmbrellaNotBlocked(t *testing.T) {
	root := makeCommitHelperFixture(t)
	configFixtureGit(t, root)
	writeFixture(t, root, ".gitignore", "tmp/*\n")
	writeFixture(t, root, "plan/spec-widget.md", specMetaInProgress)
	writeFixture(t, root, "plan/spec-widget-2-foo.md", specMetaInProgress) // child => widget is an umbrella
	writeFixture(t, root, "plan/learned/900-widget.md", "# widget\n\nlesson body\n")
	commitFixtureAll(t, root, "feat: widget umbrella + child")

	_, stderr, code := runSpecClosure(t, root, "--spec", "plan/spec-widget.md")
	if code != 0 {
		t.Fatalf("umbrella must not hard-block, got exit %d\nstderr:\n%s", code, stderr)
	}
	// It should still surface under NEEDS VERIFICATION, not high-confidence.
	stdout, _, _ := runSpecClosure(t, root, "--list")
	mustContain(t, stdout, "NEEDS VERIFICATION")
	mustContain(t, stdout, "spec-widget.md  [umbrella]")
}

// VALIDATES: a spec whose only committed learned signal is a sibling/predecessor
// summary (slug != stem, body reference) is NOT hard-blocked — it is "needs
// verification" only. PREVENTS the weak-match false-positive class (config-apply,
// sr-policy, perf-next-2 were flagged by predecessor/sibling summaries).
func TestSpecClosureWeakMatchNotBlocked(t *testing.T) {
	root := makeCommitHelperFixture(t)
	configFixtureGit(t, root)
	writeFixture(t, root, ".gitignore", "tmp/*\n")
	// Body references a predecessor summary whose slug does not match the stem.
	spec := specMetaInProgress + "\nPrior work: see plan/learned/900-gadget-migration.md\n"
	writeFixture(t, root, "plan/spec-widget.md", spec)
	writeFixture(t, root, "plan/learned/900-gadget-migration.md", "# gadget\n\nlesson\n")
	commitFixtureAll(t, root, "feat: widget referencing predecessor")

	_, stderr, code := runSpecClosure(t, root, "--spec", "plan/spec-widget.md")
	if code != 0 {
		t.Fatalf("weak ref match must not hard-block, got exit %d\nstderr:\n%s", code, stderr)
	}
	stdout, _, _ := runSpecClosure(t, root, "--list")
	mustContain(t, stdout, "spec-widget.md  [weak-match]")
}

func runSpecClosure(t *testing.T, fixtureRoot string, args ...string) (string, string, int) {
	t.Helper()
	script := filepath.Join(repoRoot(t), "scripts", "dev", "spec-closure-check.py")
	return runFixtureCommandAllowError(t, fixtureRoot, "python3", append([]string{script}, args...)...)
}

func configFixtureGit(t *testing.T, root string) {
	t.Helper()
	runFixtureCommand(t, root, "config", "user.email", "test@example.com")
	runFixtureCommand(t, root, "config", "user.name", "Ze Test")
	runFixtureCommand(t, root, "config", "commit.gpgsign", "false")
}

func commitFixtureAll(t *testing.T, root, message string) {
	t.Helper()
	runFixtureCommand(t, root, "add", "-A")
	runFixtureCommand(t, root, "commit", "-m", message)
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		t.Fatalf("fixture is not a git repo: %v", err)
	}
}
