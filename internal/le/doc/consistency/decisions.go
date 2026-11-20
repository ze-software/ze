// Design: docs/contributing/documentation-testing.md -- the doc consistency checks
// Related: register.go -- registers judgeDecisionRows as a change-set check
//
// decisions.go owns the `## Design decisions` section of a design page: its
// parser, the `design-decisions` tree check, and the change-set check that
// refuses a commit which changes or removes a decision row
// (plan/release/validation/spec-design-needs-owner-approval.md, AC-8 to AC-10).
package docconsistency

import "github.com/ze-software/ze/internal/le/commit/changecheck"

// judgeDecisionRows answers the refusal of a commit that adds a malformed
// decision row, or changes or removes one present at the parent. It is
// registered and accepts every change yet: phase 2 of that spec builds it.
func judgeDecisionRows(_ changecheck.Change) (string, error) {
	return "", nil
}
