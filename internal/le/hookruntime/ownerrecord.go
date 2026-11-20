// Design: docs/contributing/running-commands.md -- owner authorization requires a terminal
// Related: bash.go -- the Bash guards this pair sits beside
package hookruntime

// ze point: none -- the rule that only Thomas's terminal records an owner authorization lives in ai/META-DECISIONS.md, "Test changes", outside the rule corpus
// writeOwnerRecord refuses a Write, Edit or MultiEdit whose path lies under
// tmp/spec-design/, where only `./le spec design` writes Thomas's approvals
// (plan/release/validation/spec-design-needs-owner-approval.md, AC-34). It is
// registered and refuses nothing yet: phase 5 of that spec builds it.
func writeOwnerRecord(_ context) *verdict {
	return nil
}

// ze point: none -- the rule that only Thomas's terminal records an owner authorization lives in ai/META-DECISIONS.md, "Test changes", outside the rule corpus
// bashOwnerRecord refuses a Bash line that names tmp/spec-design/ and is not
// made only of readers (AC-34). It is registered and refuses nothing yet:
// phase 5 of that spec builds it.
func bashOwnerRecord(_ context) *verdict {
	return nil
}
