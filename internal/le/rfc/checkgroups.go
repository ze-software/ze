// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Related: check.go -- the control flow that files every finding under its group
//
// checkgroups.go names the groups of checks `le rfc check` runs, in words a
// reader who never ran the command can follow. The public compliance page
// prints one line for each group, so a reader learns what the gate verifies
// and what a failure means before meeting a single diagnostic.
package rfc

// CheckGroupKey names one group of checks. It is the value Finding.Check
// carries, so each finding states which group raised it and no reader parses
// the message back apart to find out.
type CheckGroupKey string

// The groups, in the order check() runs them. The empty key is no group: a
// finding that carries it was raised by a check that does not name its group,
// and a renderer says so rather than filing it under a guess.
const (
	CheckRequirementLists CheckGroupKey = "requirement-lists"
	CheckTestEvidence     CheckGroupKey = "test-evidence"
	CheckAnnotations      CheckGroupKey = "annotations"
	CheckPublicStatus     CheckGroupKey = "public-status"
	CheckAudits           CheckGroupKey = "audits"
	CheckProofRecords     CheckGroupKey = "proof-records"
	CheckTextReviews      CheckGroupKey = "text-reviews"
)

// CheckGroup is one group as a reader meets it: its name, what its checks verify, and
// what a failure means for someone reading the published figures.
type CheckGroup struct {
	Key CheckGroupKey
	// Name is the short name a reader meets for the group.
	Name     string
	Verifies string
	Failure  string
}

// CheckGroups is every group, declared once. The compliance page reads it to
// print a line for each group, passing or not, so a group with no finding is
// still shown as a check that ran and passed.
var CheckGroups = []CheckGroup{
	{Key: CheckRequirementLists, Name: "Requirement lists",
		Verifies: "Each RFC's requirement list is readable and consistent: requirement " +
			"identifiers are not reused or lost, each requirement quotes the RFC text it " +
			"cites, and the links it declares between RFCs and profiles agree with the RFC texts.",
		Failure: "A requirement list needs repair. The figures for the RFCs named in the " +
			"contributor detail are computed from that list."},
	{Key: CheckTestEvidence, Name: "Test evidence",
		Verifies: "Every gated MUST-level requirement has its tests, or an annotation that says " +
			"why not, test evidence already recorded is not lost, and every test that claims " +
			"a requirement compiles and is named for what it covers.",
		Failure: "The test evidence for a requirement is missing, lost, or not where it is " +
			"claimed to be."},
	{Key: CheckAnnotations, Name: "Requirement annotations",
		Verifies: "Every annotation that excuses a requirement from Ze's own tests still names " +
			"code that exists, or quotes the RFC sentence it relies on.",
		Failure: "An excuse for a requirement no longer holds as written and must be justified again."},
	{Key: CheckPublicStatus, Name: "Published support status",
		Verifies: "The support status each RFC publishes agrees with its requirements and its " +
			"declared gaps, and a status that promises support is backed by a review of the RFC text.",
		Failure: "A published support status says something its requirements do not support."},
	{Key: CheckAudits, Name: "Reviewer verdicts",
		Verifies: "Each reviewer verdict recorded on a requirement is well formed and still " +
			"describes the code and tests it judged.",
		Failure: "A reviewer verdict is malformed or out of date, and that requirement must be " +
			"reviewed again before the verdict counts."},
	{Key: CheckProofRecords, Name: "Recorded breaks",
		Verifies: "Each recorded break still matches the test, the claim and the code it was " +
			"recorded against, and no recorded proof is lost.",
		Failure: "A test's proof is missing or no longer matches what it rests on, so it is not " +
			"counted as proven by a recorded break."},
	{Key: CheckTextReviews, Name: "Reviews of RFC text",
		Verifies: "Each sign-off of a sentence-by-sentence review of an RFC's text is valid and " +
			"current.",
		Failure: "A review of an RFC's text must be repaired or done again. Until then, nothing " +
			"bounds what that RFC's requirement list may have left out."},
}

// inGroup files every finding of one check under its group.
func inGroup(key CheckGroupKey, findings []Finding) []Finding {
	for i := range findings {
		findings[i].Check = key
	}
	return findings
}
