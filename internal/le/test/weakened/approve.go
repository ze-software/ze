// Design: docs/contributing/testing.md -- when a test must be weakened
// Related: proposed.go -- the edit-time hook that reads the file.
// Related: testweakened.go -- the commit gate that reads it.
// Related: internal/le/rfc/approve.go -- the one approval writer.
package testweakened

import (
	"errors"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/rfc"
)

// ApprovalTrailer is the key of the commit message line that records one
// ze-test-judge approval of a test weakening for good:
// `Weakening-approved: <package>.<TestName>: <the judge's verdict>`.
const ApprovalTrailer = "Weakening-approved: "

// approvalHeader opens a session's weakening approval file, in the row
// grammar ParseLedger reads.
const approvalHeader = "# Test weakenings the owner approved\n\n| Test | Reason |\n|------|--------|\n"

const (
	keyUnit   = "unit"
	keyReason = "reason"
)

// ApprovalPath answers the file one commit session's weakening approvals live
// in, under tmp/ where git never reads it. The commit that used a row carries
// its trailer, and that commit is the record.
func ApprovalPath(session string) string {
	return "tmp/commit-weakening-approved-" + session + ".md"
}

// ApproveCommand renders the command the ze-test-judge agent runs once it has
// approved, for the hook and the gate to print beside a refusal.
func ApproveCommand(unit string) string {
	return "./le test weakened approve unit " + unit + " reason \"<the judge's verdict>\""
}

// ApprovalTrailerLine renders the commit message line for one used row.
func ApprovalTrailerLine(unit, reason string) string {
	return ApprovalTrailer + unit + ": " + reason
}

// Approve writes the judge's approval of one test weakening into the
// session's weakening approval file, with the writer and the refusals the RFC
// approval file has (rfc.ApproveInto).
func Approve(root, session, unit, reason string) (rfc.ApprovalReport, error) {
	return rfc.ApproveInto(root, rfc.ApprovalFile{
		Path: ApprovalPath(session), Header: approvalHeader, Command: "test weakened approve",
	}, unit, reason)
}

// approveAnswer is the `le test weakened approve` command body.
func approveAnswer(args leaction.Arguments) (any, int) {
	if !args.Has(keyUnit) || !args.Has(keyReason) {
		leaction.ReportError(errors.New("test weakened approve requires unit <package>.<TestName> reason \"<the judge's verdict>\""))
		return nil, 2
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	// The approval admits the main thread's commit, so it is keyed on the
	// main thread's namespace even when the judge agent records it.
	session, err := lepath.SessionCommitSession(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := Approve(root, session, args.One(keyUnit), args.One(keyReason))
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}
