// Design: docs/contributing/spec-workflow.md -- a spec reaches ready only with Thomas's approval
// Related: specwip.go -- the sibling check for the edit that makes a spec in-progress
package hookruntime

// ze point: planning/spec-metadata-blocking/update-spec-status-at-each-transition
// writeSpecReady judges the edit that makes a spec ready: refused from a
// subagent (fromSubagent), and from the main session unless an owner approval
// of the design matches the proposed text or Thomas is away
// (plan/release/validation/spec-design-needs-owner-approval.md, AC-20, AC-21).
// It is registered and refuses nothing yet: phase 6 of that spec builds it.
func writeSpecReady(_ context) *verdict {
	return nil
}
