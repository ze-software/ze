// Related: review.go — closureStem, relocatedSpecs
//
// VALIDATES: a spec removed from one release bucket and added under another in
// the same commit is a relocation, and does not count as a closure.
// PREVENTS: the one-closure-per-commit rule refusing a batch of relocations. A
// closure retires a spec because its work is done; a relocation retires nothing
// and only re-files an open spec under the release that owes it. Counting the
// second as the first made a triage sweep unlandable, and the only way past it
// was to split one bookkeeping change into as many commits as it moved specs.
package commit

import "testing"

// TestRelocatingManySpecsIsNotAClosure is the case that was refused. Thirty
// specs moved in one commit named thirty closures, and the gate allows one.
func TestRelocatingManySpecsIsNotAClosure(t *testing.T) {
	root := t.TempDir()

	added := []string{
		"plan/immediate/spec-ntp-server.md",
		"plan/pre-release/spec-vrf-later.md",
		"plan/immediate/spec-fleet-1-device-registry.md",
	}
	removed := []string{
		"plan/spec-ntp-server.md",
		"plan/spec-vrf-later.md",
		"plan/spec-fleet-1-device-registry.md",
	}

	stem, err := closureStem(root, added, removed)
	if err != nil {
		t.Fatalf("three relocations were read as closures: %v", err)
	}
	if stem != "" {
		t.Fatalf("closureStem = %q, want empty: a relocation closes nothing", stem)
	}
}

// TestRelocationRunsInEveryDirection keeps the rule symmetric. A triage sweep
// re-files work upward as often as downward, so pre-release to plan/ and
// immediate to pre-release are relocations exactly as plan/ to immediate is.
func TestRelocationRunsInEveryDirection(t *testing.T) {
	root := t.TempDir()

	added := []string{
		"plan/spec-shipped-later.md",
		"plan/pre-release/spec-promoted.md",
	}
	removed := []string{
		"plan/pre-release/spec-shipped-later.md",
		"plan/immediate/spec-promoted.md",
	}

	stem, err := closureStem(root, added, removed)
	if err != nil {
		t.Fatalf("relocations out of a bucket were read as closures: %v", err)
	}
	if stem != "" {
		t.Fatalf("closureStem = %q, want empty: a relocation closes nothing", stem)
	}
}

// TestARealClosureStillCounts is the discrimination case, and the one that fails
// if relocatedSpecs is widened until nothing is a closure. A spec removed and
// NOT re-filed is closed, and the review gate must still see it.
func TestARealClosureStillCounts(t *testing.T) {
	root := t.TempDir()

	stem, err := closureStem(root, []string{"plan/immediate/spec-elsewhere.md"},
		[]string{"plan/spec-finished.md"})
	if err != nil {
		t.Fatalf("closureStem: %v", err)
	}
	if stem != "finished" {
		t.Fatalf("closureStem = %q, want \"finished\": a spec removed and not re-filed "+
			"in another bucket is closed, and its review gate still applies", stem)
	}
}

// TestAClosureFromAnyBucketCounts is what the buckets added. Closing a spec
// removes it from wherever it sits, so a gate that watched plan/ alone let every
// pre-release and immediate closure past with no review artifact.
func TestAClosureFromAnyBucketCounts(t *testing.T) {
	for _, removed := range []string{"plan/immediate/spec-finished.md", "plan/pre-release/spec-finished.md"} {
		stem, err := closureStem(t.TempDir(), []string{removed}, []string{removed})
		if err != nil {
			t.Fatalf("closureStem over %s: %v", removed, err)
		}
		if stem != "finished" {
			t.Errorf("closureStem over %s = %q, want \"finished\"", removed, stem)
		}
	}
}

// TestAClosureBesideRelocationsIsStillRefused keeps the rule the relocation
// carve-out must not weaken: one commit may close one spec, and moving thirty
// others alongside does not buy a second closure.
func TestAClosureBesideRelocationsIsStillRefused(t *testing.T) {
	root := t.TempDir()

	added := []string{"plan/immediate/spec-moved-a.md", "plan/pre-release/spec-moved-b.md"}
	removed := []string{
		"plan/spec-moved-a.md",
		"plan/spec-moved-b.md",
		"plan/spec-closed-one.md",
		"plan/spec-closed-two.md",
	}

	if _, err := closureStem(root, added, removed); err == nil {
		t.Fatal("two genuine closures rode a relocation batch through the gate")
	}
}

// TestAJournalCarryingSeveralSessionsRowsClosesNothing is the deadlock this
// gate used to create in a shared checkout. Five sessions add rows to one class
// file, a commit that names that file stages every row, and reading rows as
// closures then refused the commit for specs this session never touched. The
// rows could land only through that commit, so nothing could ever clear it.
//
// A removed spec file is the closure. A row is evidence about one.
func TestAJournalCarryingSeveralSessionsRowsClosesNothing(t *testing.T) {
	root := newCommitRepository(t)
	path := "plan/journal/shared.md"
	header := "| Date | Spec | Surface | Symptom | Fix |\n|------|------|---------|---------|-----|\n"
	writeCommitFixture(t, root, path, header)
	runCommitGit(t, root, "add", "--", path)
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t",
		"-c", "commit.gpgsign=false", "commit", "-q", "-m", "journal baseline")

	writeCommitFixture(t, root, path, header+
		"| 2026-08-30 | spec-one | cli | theirs | fixed |\n"+
		"| 2026-08-30 | spec-two | web | theirs | fixed |\n"+
		"| 2026-08-30 | - | doc | mine | fixed |\n")

	stem, err := closureStem(root, []string{path}, nil)
	if err != nil {
		t.Fatalf("a shared journal refused the commit: %v", err)
	}
	if stem != "" {
		t.Fatalf("closureStem = %q, want empty: this commit removes no spec", stem)
	}
}

// TestARemovedSpecOutranksForeignJournalRows keeps the artifact bound to the
// spec this commit actually closes, not to whatever rows the shared class file
// happened to carry.
func TestARemovedSpecOutranksForeignJournalRows(t *testing.T) {
	root := newCommitRepository(t)
	path := "plan/journal/shared.md"
	header := "| Date | Spec | Surface | Symptom | Fix |\n|------|------|---------|---------|-----|\n"
	writeCommitFixture(t, root, path, header)
	runCommitGit(t, root, "add", "--", path)
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t",
		"-c", "commit.gpgsign=false", "commit", "-q", "-m", "journal baseline")

	writeCommitFixture(t, root, path, header+"| 2026-08-30 | spec-theirs | cli | theirs | fixed |\n")

	stem, err := closureStem(root, []string{path}, []string{"plan/spec-mine.md"})
	if err != nil {
		t.Fatalf("closureStem: %v", err)
	}
	if stem != "mine" {
		t.Fatalf("closureStem = %q, want \"mine\": the removed spec is the closure", stem)
	}
}

// TestOneClosureBesideRelocationsIsAllowed is the shape a real closure takes
// when it happens to travel with bookkeeping: exactly one spec closes.
func TestOneClosureBesideRelocationsIsAllowed(t *testing.T) {
	root := t.TempDir()

	stem, err := closureStem(root,
		[]string{"plan/immediate/spec-moved-a.md"},
		[]string{"plan/spec-moved-a.md", "plan/spec-closed-one.md"})
	if err != nil {
		t.Fatalf("closureStem: %v", err)
	}
	if stem != "closed-one" {
		t.Fatalf("closureStem = %q, want \"closed-one\"", stem)
	}
}

// TestANameThatOnlyMatchesInAnotherBucketDoesNotExcuseAClosure keeps the match
// keyed on the file name rather than on the stem appearing anywhere. A spec
// added under another bucket with a DIFFERENT name excuses nothing.
func TestANameThatOnlyMatchesInAnotherBucketDoesNotExcuseAClosure(t *testing.T) {
	root := t.TempDir()

	stem, err := closureStem(root,
		[]string{"plan/immediate/spec-ntp-server-phase-2.md"},
		[]string{"plan/spec-ntp-server.md"})
	if err != nil {
		t.Fatalf("closureStem: %v", err)
	}
	if stem != "ntp-server" {
		t.Fatalf("closureStem = %q, want \"ntp-server\": another bucket gained a different "+
			"spec, so the removed one was closed rather than moved", stem)
	}
}

// TestARemovalListedInBothPathsAndRemovedIsStillAClosure pins the same-bucket
// case. Every removed path is also a path of the commit, so a rule that asked
// only "is this name in another bucket entry" would read every closure as a
// relocation of itself.
func TestARemovalListedInBothPathsAndRemovedIsStillAClosure(t *testing.T) {
	root := t.TempDir()

	stem, err := closureStem(root, []string{"plan/spec-finished.md"}, []string{"plan/spec-finished.md"})
	if err != nil {
		t.Fatalf("closureStem: %v", err)
	}
	if stem != "finished" {
		t.Fatalf("closureStem = %q, want \"finished\": the same bucket is no relocation", stem)
	}
}
