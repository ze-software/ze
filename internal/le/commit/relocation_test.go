// Related: review.go — closureStem, relocatedSpecs
//
// VALIDATES: a spec removed from plan/ and added under plan/future/ in the same
// commit is a relocation, and does not count as a closure.
// PREVENTS: the one-closure-per-commit rule refusing a batch of relocations. A
// closure retires a spec because its work is done; a relocation retires nothing
// and only re-files an open spec as work that does not hold the release
// (plan/future/README.md). Counting the second as the first made a triage sweep
// unlandable, and the only way past it was to split one bookkeeping change into
// as many commits as it moved specs.
package commit

import "testing"

// TestRelocatingManySpecsIsNotAClosure is the case that was refused. Thirty
// specs moved in one commit named thirty closures, and the gate allows one.
func TestRelocatingManySpecsIsNotAClosure(t *testing.T) {
	root := t.TempDir()

	added := []string{
		"plan/future/spec-ntp-server.md",
		"plan/future/spec-vrf-later.md",
		"plan/future/spec-fleet-1-device-registry.md",
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

// TestARealClosureStillCounts is the discrimination case, and the one that fails
// if relocatedSpecs is widened until nothing is a closure. A spec removed and
// NOT re-filed is closed, and the review gate must still see it.
func TestARealClosureStillCounts(t *testing.T) {
	root := t.TempDir()

	stem, err := closureStem(root, []string{"plan/future/spec-elsewhere.md"},
		[]string{"plan/spec-finished.md"})
	if err != nil {
		t.Fatalf("closureStem: %v", err)
	}
	if stem != "finished" {
		t.Fatalf("closureStem = %q, want \"finished\": a spec removed and not re-filed "+
			"under plan/future is closed, and its review gate still applies", stem)
	}
}

// TestAClosureBesideRelocationsIsStillRefused keeps the rule the relocation
// carve-out must not weaken: one commit may close one spec, and moving thirty
// others alongside does not buy a second closure.
func TestAClosureBesideRelocationsIsStillRefused(t *testing.T) {
	root := t.TempDir()

	added := []string{"plan/future/spec-moved-a.md", "plan/future/spec-moved-b.md"}
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

// TestOneClosureBesideRelocationsIsAllowed is the shape a real closure takes
// when it happens to travel with bookkeeping: exactly one spec closes.
func TestOneClosureBesideRelocationsIsAllowed(t *testing.T) {
	root := t.TempDir()

	stem, err := closureStem(root,
		[]string{"plan/future/spec-moved-a.md"},
		[]string{"plan/spec-moved-a.md", "plan/spec-closed-one.md"})
	if err != nil {
		t.Fatalf("closureStem: %v", err)
	}
	if stem != "closed-one" {
		t.Fatalf("closureStem = %q, want \"closed-one\"", stem)
	}
}

// TestANameThatOnlyMatchesInPlanFutureDoesNotExcuseAClosure keeps the match
// keyed on the basename rather than on the stem appearing anywhere. A spec added
// under plan/future with a DIFFERENT name excuses nothing.
func TestANameThatOnlyMatchesInPlanFutureDoesNotExcuseAClosure(t *testing.T) {
	root := t.TempDir()

	stem, err := closureStem(root,
		[]string{"plan/future/spec-ntp-server-phase-2.md"},
		[]string{"plan/spec-ntp-server.md"})
	if err != nil {
		t.Fatalf("closureStem: %v", err)
	}
	if stem != "ntp-server" {
		t.Fatalf("closureStem = %q, want \"ntp-server\": plan/future gained a different "+
			"spec, so the removed one was closed rather than moved", stem)
	}
}
