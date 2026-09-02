// VALIDATES: the published proof share counts a single-polarity requirement as
// proven, keeps a not-applicable requirement in its denominator, and drops a
// whole RFC only when that RFC's own public row says Ze does not implement it.
// PREVENTS: a headline percentage raised by annotating requirements away, which
// is what excluding {not-applicable} from the denominator did until 2026-09-02,
// over a set the owner ruling of 2026-08-31 presumes is mostly misclassified.

package rfc

import "testing"

// TestProvenShareOfCountsWhatTheOwnerDecided drives ProvenShareOf over a corpus
// holding one of each case the 2026-09-02 decision turns on, and checks the two
// populations it publishes.
//
// The method is one enrolled RFC Ze implements carrying all four requirement
// shapes, beside three RFCs that must each leave the denominator for a
// different reason. A test over the happy shape alone would pass against a
// version that counted every RFC.
func TestProvenShareOfCountsWhatTheOwnerDecided(t *testing.T) {
	metas := map[string]Meta{
		"rfc1": {Enrolment: enrolmentEnrolled, Support: "core", Status: "Supported"},
		"rfc2": {Enrolment: enrolmentEnrolled, Support: "core", Status: "Unsupported"},
		"rfc3": {Enrolment: enrolmentEnrolled},
		"rfc4": {Enrolment: dispositionOutOfScope, Support: "core", Status: "Supported"},
	}
	requirements := []Requirement{
		{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust},
		{RFC: "rfc1", RID: "RFC1-1-2", Level: levelMust,
			Annotation: &Annotation{Kind: AnnotationSinglePolarity, Polarity: PolarityPositive}},
		{RFC: "rfc1", RID: "RFC1-1-3", Level: levelMust,
			Annotation: &Annotation{Kind: "not-applicable"}},
		{RFC: "rfc1", RID: "RFC1-1-4", Level: levelMust},
		{RFC: "rfc2", RID: "RFC2-1-1", Level: levelMust},
		{RFC: "rfc2", RID: "RFC2-1-2", Level: levelMust},
		{RFC: "rfc3", RID: "RFC3-1-1", Level: levelMust},
		{RFC: "rfc3", RID: "RFC3-1-2", Level: levelMust},
		{RFC: "rfc4", RID: "RFC4-1-1", Level: levelMust},
		{RFC: "rfc4", RID: "RFC4-1-2", Level: levelMust},
	}
	tags := []Tag{
		{RID: "RFC1-1-1", Polarity: PolarityPositive, File: "a_test.go"},
		{RID: "RFC1-1-1", Polarity: PolarityNegative, File: "a_test.go"},
		{RID: "RFC2-1-1", Polarity: PolarityPositive, File: "a_test.go"},
		{RID: "RFC2-1-1", Polarity: PolarityNegative, File: "a_test.go"},
		{RID: "RFC4-1-1", Polarity: PolarityPositive, File: "a_test.go"},
		{RID: "RFC4-1-1", Polarity: PolarityNegative, File: "a_test.go"},
	}

	share, err := ProvenShareOf(metas, requirements, tags, nil)
	if err != nil {
		t.Fatalf("ProvenShareOf: %v", err)
	}

	// rfc1 alone is implemented and enrolled. Its not-applicable and its
	// untested requirement stay in the denominator; its single-polarity one
	// joins its both-polarity one in the numerator.
	if share.Proven != 2 {
		t.Errorf("Proven = %d, want 2 (one both-polarity, one single-polarity)", share.Proven)
	}
	if share.Gated != 4 {
		t.Errorf("Gated = %d, want 4: a not-applicable requirement stays in the denominator",
			share.Gated)
	}
	if share.RFCs != 1 {
		t.Errorf("RFCs = %d, want 1: Unsupported and no-row RFCs leave the denominator",
			share.RFCs)
	}
	// rfc4 is not enrolled, so its obligations are gated by nothing and it is
	// absent from the wider population too, tagged tests and all.
	if share.Inspected != 3 {
		t.Errorf("Inspected = %d, want 3 enrolled RFCs", share.Inspected)
	}
	if share.GatedInspected != 8 {
		t.Errorf("GatedInspected = %d, want 8 (4 + 2 + 2 over the enrolled RFCs)",
			share.GatedInspected)
	}
	if got := share.Percent(); got != "50.0" {
		t.Errorf("Percent() = %q, want \"50.0\"", got)
	}
}

// TestProvenShareOfRefusesAnEmptyPopulation checks that a corpus in which no
// enrolled RFC claims support is an error rather than a zero.
//
// A ProvenShare of {0, 0} would render "0 of 0 (NaN%)" on the home page, and a
// caller cannot tell that from a ledger stating Ze proves nothing.
func TestProvenShareOfRefusesAnEmptyPopulation(t *testing.T) {
	metas := map[string]Meta{
		"rfc1": {Enrolment: enrolmentEnrolled, Support: "core", Status: "Unsupported"},
	}
	requirements := []Requirement{{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust}}

	if _, err := ProvenShareOf(metas, requirements, nil, nil); err == nil {
		t.Fatal("ProvenShareOf answered a share over an empty population, want an error")
	}
}
