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

// specMetaPastAFixedWindow is the same spec with the preamble a real spec
// carries: a warning line and the six-line authoring comment plan/TEMPLATE.md
// ships. The Status row then sits on line 14, past the 12-line window the
// detector used to read. plan/spec-support-export.md has exactly this shape and
// was reported "unknown" for it.
const specMetaPastAFixedWindow = `# Spec: widget

> WARNING: Critical review is required before implementation and commit.

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Phase | 3/3 |

## Task
Build the widget.

## Risks & Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | something | a basis | a cost | a check | unvalidated |
`

// A finished Review Gate: the section /ze-close appends, with nothing left to
// tick before closing. With a journal row this is the closure signal.
const specReviewGateDone = `
## Review Gate

| Round | Blockers | Issues |
|-------|----------|--------|
| 2 | 0 | 0 |

- [x] re-run shows 0 BLOCKER, 0 ISSUE
`

// A Review Gate mid-loop: the box that must be ticked before closing is still
// open, so the spec is not finished.
const specReviewGateOpen = `
## Review Gate

| Round | Blockers | Issues |
|-------|----------|--------|
| 1 | 2 | 3 |

- [ ] re-run shows 0 BLOCKER, 0 ISSUE
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

// VALIDATES: the detector reads the Status row wherever the metadata table puts
// it, so a spec written from plan/TEMPLATE.md with the authoring preamble it
// ships is judged on its real status.
// PREVENTS: the whole detector going silent on a spec it cannot read. Status
// drives every branch here, and a fixed-window scan that misses the row reports
// "unknown", which is neither in-progress nor closed, so the spec is skipped and
// nothing says why. The Assumptions table below the first heading ends its
// header "| Status |", so the scan must stop before reaching it rather than
// widen (spec-fixit-spec-status-metadata-window).
func TestSpecClosureReadsStatusPastAFixedWindow(t *testing.T) {
	root := makeCommitHelperFixture(t)
	configFixtureGit(t, root)
	writeFixture(t, root, ".gitignore", "tmp/*\n")
	writeFixture(t, root, "plan/spec-widget.md", specMetaPastAFixedWindow)
	writeFixture(t, root, "plan/learned/900-widget.md", "# widget\n\nlesson body\n")
	commitFixtureAll(t, root, "feat: widget with a template preamble")

	_, stderr, code := runSpecClosure(t, root, "--spec", "plan/spec-widget.md")
	if code != 3 {
		t.Fatalf("expected exit 3, got %d: the Status row sits on line 14 and the detector must still read it\nstderr:\n%s", code, stderr)
	}
	mustContain(t, stderr, "COMPLETED BUT NOT CLOSED")

	stdout, _, lcode := runSpecClosure(t, root, "--list")
	if lcode != 0 {
		t.Fatalf("--list exit %d\n%s", lcode, stdout)
	}
	mustContain(t, stdout, "plan/spec-widget.md")
}

// VALIDATES: a learned summary that exists on disk but is NOT committed does not
// flag the spec. On-disk presence alone is not evidence of completion.
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

// VALIDATES: AC-6 -- a committed journal row naming a spec stem, on a spec
// whose Review Gate is finished, is accepted as the closure artifact by
// spec-closure-check.py, so a spec can close with a journal row instead of a
// plan/learned/NNN-<stem>.md file.
// PREVENTS: the closure gate ignoring journal evidence and leaving specs that
// closed via a journal row in "completed but not closed" limbo.
func TestSpecClosureAcceptsJournalRow(t *testing.T) {
	root := makeCommitHelperFixture(t)
	configFixtureGit(t, root)
	writeFixture(t, root, ".gitignore", "tmp/*\n")
	writeFixture(t, root, "plan/spec-widget.md", specMetaInProgress+specReviewGateDone)
	writeFixture(t, root, "plan/journal/some-class.md",
		"| Date | Spec | Surface | Symptom | Fix |\n"+
			"|------|------|---------|---------|-----|\n"+
			"| 2026-08-09 | widget | gate | it refused | fixed |\n")
	commitFixtureAll(t, root, "feat: widget with journal")

	// --spec exits 3: the journal row names this spec's stem.
	_, stderr, code := runSpecClosure(t, root, "--spec", "plan/spec-widget.md")
	if code != 3 {
		t.Fatalf("expected exit 3, got %d\nstderr:\n%s", code, stderr)
	}
	mustContain(t, stderr, "COMPLETED BUT NOT CLOSED")

	// --json includes the journal-match field.
	stdout, _, jcode := runSpecClosure(t, root, "--json")
	if jcode != 0 {
		t.Fatalf("--json exit %d\n%s", jcode, stdout)
	}
	mustContain(t, stdout, "journal-match")
	mustContain(t, stdout, "plan/journal/some-class.md")
}

// VALIDATES: a journal row whose Spec cell is "-" does NOT flag the spec as
// completed-but-not-closed. Rows written outside a spec carry "-".
func TestSpecClosureIgnoresJournalDashSpec(t *testing.T) {
	root := makeCommitHelperFixture(t)
	configFixtureGit(t, root)
	writeFixture(t, root, ".gitignore", "tmp/*\n")
	writeFixture(t, root, "plan/spec-widget.md", specMetaInProgress+specReviewGateDone)
	writeFixture(t, root, "plan/journal/some-class.md",
		"| Date | Spec | Surface | Symptom | Fix |\n"+
			"|------|------|---------|---------|-----|\n"+
			"| 2026-08-09 | - | gate | it refused | fixed |\n")
	commitFixtureAll(t, root, "feat: widget with dash journal")

	_, _, code := runSpecClosure(t, root, "--spec", "plan/spec-widget.md")
	if code != 0 {
		t.Fatalf("expected exit 0 (journal Spec is dash), got %d", code)
	}
}

// VALIDATES: a journal row naming a spec that has NO finished Review Gate does
// not flag the spec. A row is written when a problem is FOUND, mid-work, so the
// row alone is not evidence that the spec is finished.
// PREVENTS: the Stop hook (block-premature-stop.sh, which exits 3 on this
// signal) blocking every stop for the rest of a session from the moment the
// session writes its first journal row.
func TestSpecClosureIgnoresMidWorkJournalRow(t *testing.T) {
	root := makeCommitHelperFixture(t)
	configFixtureGit(t, root)
	writeFixture(t, root, ".gitignore", "tmp/*\n")
	writeFixture(t, root, "plan/spec-widget.md", specMetaInProgress)
	writeFixture(t, root, "plan/journal/some-class.md",
		"| Date | Spec | Surface | Symptom | Fix |\n"+
			"|------|------|---------|---------|-----|\n"+
			"| 2026-08-09 | widget | gate | it refused | fixed |\n")
	commitFixtureAll(t, root, "wip: widget journal row")

	_, stderr, code := runSpecClosure(t, root, "--spec", "plan/spec-widget.md")
	if code != 0 {
		t.Fatalf("expected exit 0 (no Review Gate yet), got %d\nstderr:\n%s", code, stderr)
	}
}

// VALIDATES: an unfinished Review Gate (a box still to tick before closing) is
// not a finished one, so the journal row still does not flag the spec.
// PREVENTS: the gate section being read as a closure signal the moment
// /ze-review appends it, before the review loop has reached zero.
func TestSpecClosureIgnoresUnfinishedReviewGate(t *testing.T) {
	root := makeCommitHelperFixture(t)
	configFixtureGit(t, root)
	writeFixture(t, root, ".gitignore", "tmp/*\n")
	writeFixture(t, root, "plan/spec-widget.md", specMetaInProgress+specReviewGateOpen)
	writeFixture(t, root, "plan/journal/some-class.md",
		"| Date | Spec | Surface | Symptom | Fix |\n"+
			"|------|------|---------|---------|-----|\n"+
			"| 2026-08-09 | widget | gate | it refused | fixed |\n")
	commitFixtureAll(t, root, "wip: widget review round 1")

	_, stderr, code := runSpecClosure(t, root, "--spec", "plan/spec-widget.md")
	if code != 0 {
		t.Fatalf("expected exit 0 (gate unfinished), got %d\nstderr:\n%s", code, stderr)
	}
}

// VALIDATES: AC-4 -- a malformed journal row is NAMED on stderr by the closure
// reader, not skipped in silence, and the readable rows still count.
// PREVENTS: the third copy of the row parser (the one that lived here and
// returned None for a malformed row) silently dropping evidence.
func TestSpecClosureNamesMalformedJournalRow(t *testing.T) {
	root := makeCommitHelperFixture(t)
	configFixtureGit(t, root)
	writeFixture(t, root, ".gitignore", "tmp/*\n")
	writeFixture(t, root, "plan/spec-widget.md", specMetaInProgress+specReviewGateDone)
	writeFixture(t, root, "plan/journal/some-class.md",
		"| Date | Spec | Surface | Symptom | Fix |\n"+
			"|------|------|---------|---------|-----|\n"+
			"| 2026-08-08 | widget | gate |\n"+
			"| 2026-08-09 | widget | gate | it refused | fixed |\n")
	commitFixtureAll(t, root, "feat: widget with a broken row")

	_, stderr, code := runSpecClosure(t, root, "--spec", "plan/spec-widget.md")
	if code != 3 {
		t.Fatalf("expected exit 3, got %d\nstderr:\n%s", code, stderr)
	}
	mustContain(t, stderr, "malformed journal row in plan/journal/some-class.md")
}

// VALIDATES: the README example row is not read as closure evidence.
// PREVENTS: plan/journal/README.md's fenced example naming a real spec stem and
// flagging that spec as completed-but-not-closed forever.
func TestSpecClosureIgnoresJournalReadme(t *testing.T) {
	root := makeCommitHelperFixture(t)
	configFixtureGit(t, root)
	writeFixture(t, root, ".gitignore", "tmp/*\n")
	writeFixture(t, root, "plan/spec-widget.md", specMetaInProgress+specReviewGateDone)
	writeFixture(t, root, "plan/journal/README.md",
		"# Problem Journal\n\n```\n"+
			"| Date | Spec | Surface | Symptom | Fix |\n"+
			"|------|------|---------|---------|-----|\n"+
			"| 2026-08-09 | widget | gate | example row | example fix |\n```\n")
	commitFixtureAll(t, root, "docs: journal readme")

	_, stderr, code := runSpecClosure(t, root, "--spec", "plan/spec-widget.md")
	if code != 0 {
		t.Fatalf("expected exit 0 (README is not evidence), got %d\nstderr:\n%s", code, stderr)
	}
}

func runSpecClosure(t *testing.T, fixtureRoot string, args ...string) (string, string, int) {
	t.Helper()
	script := filepath.Join(repoRoot(t), "scripts", "dev", "spec-closure-check.py")
	// Read the detector before running it. go test caches a result against the
	// files the TEST BINARY opened, and it cannot see a file an exec'd
	// interpreter reads, so without this read every test here reports its last
	// verdict from the cache after the detector itself changes. Measured
	// 2026-08-22 while proving TestSpecClosureReadsStatusPastAFixedWindow
	// discriminates: reverting _status to its fixed 12-line window left the
	// package "ok (cached)", and the test went red only under -count=1.
	// scripts/status/verify_run_test.go reads its own gate for the same reason.
	if _, err := os.ReadFile(script); err != nil {
		t.Fatalf("read %s: %v", script, err)
	}
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
