// Design: docs/contributing/testing.md -- the accounting refuses what a silent walk would pass
package population

import (
	"slices"
	"strings"
	"testing"
)

func members(names ...string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

// Goal: an empty population must not read as full coverage. Method: assess a
// claim whose walk found nothing, and require the error to name the subject, so
// a reader of the failing gate knows which set came back empty.
func TestAssessRefusesAnEmptyPopulation(t *testing.T) {
	claim := Claim{Subject: "lint tracked Go", Population: nil, Walked: members("a.go")}

	coverage, err := claim.Assess()
	if err == nil {
		t.Fatalf("an empty population was accounted as coverage: %#v", coverage)
	}
	if !strings.Contains(err.Error(), "lint tracked Go population is empty") {
		t.Fatalf("refusal does not name the subject: %v", err)
	}
}

// Goal: a member no walk reached and no excuse explains is the finding this
// package exists for. Method: claim three members, walk two, and require the
// third to be named in Unexcused, carried in Blind, and to set a non-zero code.
func TestAssessNamesAMemberNoWalkCoveredAndNoExcuseExplains(t *testing.T) {
	claim := Claim{
		Subject:    "test",
		Population: members("a.go", "b.go", "c.go"),
		Walked:     members("a.go", "b.go"),
	}

	coverage, err := claim.Assess()
	if err != nil {
		t.Fatalf("assess: %v", err)
	}
	if coverage.Code == 0 {
		t.Fatalf("an unexcused member passed: %#v", coverage)
	}
	if !slices.Equal(coverage.Unexcused, []string{"c.go"}) {
		t.Fatalf("unexcused is %v, want [c.go]", coverage.Unexcused)
	}
	if len(coverage.Blind) != 1 || coverage.Blind[0].Member != "c.go" {
		t.Fatalf("blind is %#v, want one row for c.go", coverage.Blind)
	}
	if coverage.Blind[0].Reason != defaultUnexcused {
		t.Fatalf("reason is %q, want the default", coverage.Blind[0].Reason)
	}
	if coverage.Population != 3 || coverage.Walked != 2 {
		t.Fatalf("counts are %d/%d, want 3/2", coverage.Walked, coverage.Population)
	}
}

// Goal: an excuse that has stopped being needed hides the next member to land on
// that path, so it fails too. Method: excuse one member the walk now reaches and
// one that has left the population, and require both to be reported as healed.
func TestAssessFailsAnExcuseThatIsNoLongerNeeded(t *testing.T) {
	claim := Claim{
		Subject:    "test",
		Population: members("a.go", "b.go"),
		Walked:     members("a.go", "b.go"),
		Excused:    map[string]string{"a.go": "was blind", "deleted.go": "was tracked"},
	}

	coverage, err := claim.Assess()
	if err != nil {
		t.Fatalf("assess: %v", err)
	}
	if coverage.Code == 0 {
		t.Fatalf("a healed excuse passed: %#v", coverage)
	}
	if !slices.Equal(coverage.Healed, []string{"a.go", "deleted.go"}) {
		t.Fatalf("healed is %v, want [a.go deleted.go]", coverage.Healed)
	}
}

// Goal: the accounting balances when every blind member is excused, and only
// then. Method: excuse the one member the walk misses, and require a zero code
// with the excuse carried into the report.
func TestAssessBalancesWhenEveryBlindMemberIsExcused(t *testing.T) {
	claim := Claim{
		Subject:    "test",
		Population: members("a.go", "b.go"),
		Walked:     members("a.go"),
		Excused:    map[string]string{"b.go": "a separate module"},
	}

	coverage, err := claim.Assess()
	if err != nil {
		t.Fatalf("assess: %v", err)
	}
	if coverage.Code != 0 {
		t.Fatalf("a fully excused walk failed: %#v", coverage)
	}
	if len(coverage.Blind) != 1 || coverage.Blind[0].Reason != "a separate module" {
		t.Fatalf("blind is %#v, want b.go carrying its stated reason", coverage.Blind)
	}
	if len(coverage.Unexcused) != 0 || len(coverage.Healed) != 0 {
		t.Fatalf("a balanced claim reported findings: %#v", coverage)
	}
}

// Goal: each gate names the walk a reader must extend, so the wording is the
// gate's own. Method: set UnexcusedReason and require it on the blind row.
func TestAssessKeepsTheGatesOwnWordingForAnUnexcusedMember(t *testing.T) {
	claim := Claim{
		Subject:         "test",
		Population:      members("a.go"),
		Walked:          nil,
		UnexcusedReason: "NOT COVERED BY ANY PASS",
	}

	coverage, err := claim.Assess()
	if err != nil {
		t.Fatalf("assess: %v", err)
	}
	if len(coverage.Blind) != 1 || coverage.Blind[0].Reason != "NOT COVERED BY ANY PASS" {
		t.Fatalf("blind is %#v, want the gate's own wording", coverage.Blind)
	}
}

// Goal: a walked member outside the claimed population must not inflate the
// count, because the claim is what is under test. Method: walk a member the
// population never named, and require the counts to answer for the claim alone.
func TestAssessIgnoresAWalkedMemberOutsideThePopulation(t *testing.T) {
	claim := Claim{
		Subject:    "test",
		Population: members("a.go"),
		Walked:     members("a.go", "generated.go"),
	}

	coverage, err := claim.Assess()
	if err != nil {
		t.Fatalf("assess: %v", err)
	}
	if coverage.Population != 1 || coverage.Walked != 1 || coverage.Code != 0 {
		t.Fatalf("counts are %#v, want the claimed population alone", coverage)
	}
}
