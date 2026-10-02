package rfc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: the coverage ratchet accepts a polarity lost against HEAD only
// when the owner ruled the move, and refuses every other loss exactly as before.
// PREVENTS: an owner-ruled tag move (OWNER RULING 6(b), 2026-10-02) being
// unreachable, and the exception turning into a hatch that one of the two
// halves alone opens.
//
// The method is one accepted case, then each half removed or mismatched in
// turn. Every refusal case must still produce the ratchet's own message, so a
// case that passes for an unrelated reason cannot hide a hole.
func TestCoverageRatchetAcceptsAnOwnerRuledTagMove(t *testing.T) {
	const (
		movedUnit   = "internal/component/bfd/session_test.go::TestWidgetRefused"
		movedRow    = "bfd.TestWidgetRefused"
		otherUnit   = "internal/component/bfd/peer_test.go::TestWidgetIgnored"
		otherRow    = "bfd.TestWidgetIgnored"
		ruled       = "D-15: owner ruling 2026-09-30 moves the negative to the row it proves"
		annotated   = "the peer role is invisible to Ze, owner ruling 2026-09-30"
		ratchetText = "is no longer proven"
	)
	single := func(polarity, reason string) *Annotation {
		return &Annotation{Kind: AnnotationSinglePolarity, Polarity: polarity, Reason: reason}
	}
	requirement := func(annotation *Annotation) Requirement {
		return Requirement{RFC: selftestStem, RID: selftestRIDSend, Level: levelMust, Section: "2",
			Source: selftestSummaryRel, Line: 5, Annotation: annotation}
	}
	positive := Cover{RID: selftestRIDSend, Polarity: PolarityPositive, Unit: "internal/component/bfd/send_test.go::TestSend"}
	negative := Cover{RID: selftestRIDSend, Polarity: PolarityNegative, Unit: movedUnit}
	held := map[Cover][]Tag{positive: nil, negative: nil}
	current := []Tag{{RID: selftestRIDSend, Polarity: PolarityPositive, File: "internal/component/bfd/send_test.go", Line: 3}}
	enrolled := map[string]bool{selftestStem: true}

	cases := []struct {
		name       string
		annotation *Annotation
		held       map[Cover][]Tag
		approvals  map[string]string
		accepted   bool
	}{
		{"ruling and annotation cite one ruling", single(PolarityPositive, annotated), held,
			map[string]string{movedRow: ruled}, true},
		{"no approval", single(PolarityPositive, annotated), held, nil, false},
		{"approval cites no ruling", single(PolarityPositive, annotated), held,
			map[string]string{movedRow: "D-15: the author moved it"}, false},
		{"approval names another unit", single(PolarityPositive, annotated), held,
			map[string]string{otherRow: ruled}, false},
		{"no annotation", nil, held, map[string]string{movedRow: ruled}, false},
		{"annotation cites no ruling", single(PolarityPositive, "no receiver input exists"), held,
			map[string]string{movedRow: ruled}, false},
		{"annotation cites another ruling", single(PolarityPositive, "owner ruling 6"), held,
			map[string]string{movedRow: ruled}, false},
		{"annotation keeps the lost polarity", single(PolarityNegative, annotated), held,
			map[string]string{movedRow: ruled}, false},
		{"another unit also lost the polarity unapproved", single(PolarityPositive, annotated),
			map[Cover][]Tag{positive: nil, negative: nil,
				{RID: selftestRIDSend, Polarity: PolarityNegative, Unit: otherUnit}: nil},
			map[string]string{movedRow: ruled}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := checkCoverageRatchet([]Requirement{requirement(tc.annotation)}, current, enrolled,
				tc.held, enrolled, tc.approvals)
			if tc.accepted {
				if len(errs) != 0 {
					t.Fatalf("an owner-ruled move was refused: %v", errs)
				}
				return
			}
			if len(errs) != 1 || !strings.Contains(errs[0], ratchetText) {
				t.Fatalf("want the coverage ratchet's refusal, got %v", errs)
			}
		})
	}
}

// VALIDATES: the approval rows the ratchet reads are the ones `./le rfc
// approve` writes, keyed by unit, and an absent file is no approval.
// PREVENTS: the exception reading a grammar the writer does not produce, which
// would leave every owner-ruled move refused with nothing to say why.
func TestApprovalRowsReadWhatApproveWrites(t *testing.T) {
	root := t.TempDir()
	rows, err := approvalRows(root, "0badcafe")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("an absent approval file approved %v", rows)
	}
	if _, err := Approve(root, "0badcafe", "bfd.TestWidget", "owner ruling 2026-09-30 moves it"); err != nil {
		t.Fatal(err)
	}
	if _, err := Approve(root, "0badcafe", "bfd.TestOther", "owner ruling 6"); err != nil {
		t.Fatal(err)
	}
	rows, err = approvalRows(root, "0badcafe")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"bfd.TestWidget": "owner ruling 2026-09-30 moves it", "bfd.TestOther": "owner ruling 6"}
	if len(rows) != len(want) {
		t.Fatalf("rows %v, want %v", rows, want)
	}
	for unit, reason := range want {
		if rows[unit] != reason {
			t.Fatalf("rows %v, want %v", rows, want)
		}
	}
	if _, statErr := os.Stat(filepath.Join(root, ApprovalPath("0badcafe"))); statErr != nil {
		t.Fatal(statErr)
	}
}
