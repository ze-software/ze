// Design: docs/contributing/testing.md -- when a test must be weakened
// Related: proposed.go -- the edit-time hook that reads the file.
// Related: testweakened.go -- the commit gate that reads it.
// Related: internal/le/rfc/approve.go -- the one approval writer.
package testweakened

import (
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
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
const approvalHeader = "# Test weakenings the owner approved\n\n" + rfc.ApprovalTableHeader

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
// approval file has (rfc.ApproveInto): the unit is resolved to its one
// carrier by its directory key before anything is written, and the row
// records the unit content the verdict judged.
func Approve(root, session, unit, reason string) (rfc.ApprovalReport, error) {
	return rfc.ApproveInto(root, rfc.ApprovalFile{
		Path: ApprovalPath(session), Header: approvalHeader, Command: "test weakened approve",
	}, unit, reason)
}

// approveAnswer admits a parsed approval before process startup can write:
// the unit resolves to its one carrier and the reason passes the writer's
// checks while nothing has been created, so a refused request leaves no
// commit namespace and no approval file behind.
func approveAnswer(args leaction.Arguments) (leaction.Prepared, int) {
	if !args.Has(keyUnit) || !args.Has(keyReason) {
		leaction.ReportError(errors.New("test weakened approve requires unit <package>.<TestName> reason \"<the judge's verdict>\""))
		return leaction.Prepared{}, 2
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return leaction.Prepared{}, 2
	}
	unit, reason := args.One(keyUnit), strings.TrimSpace(args.One(keyReason))
	// This boundary MUST admit before SessionCommitSession writes a namespace.
	if _, err := rfc.ResolveApprovalUnit(root, "", unit); err != nil {
		leaction.ReportError(opererr.Wrap(err, "test weakened approve: unit "+unit+" names no one changed test",
			"an approval admits one test unit, <package>.<TestName>, that the working tree changes against HEAD",
			"run ./le test weakened check to list the weakened units, then ./le test weakened approve unit <package>.<TestName> reason \"<the judge's verdict>\""))
		return leaction.Prepared{}, 2
	}
	if reason == "" {
		leaction.ReportError(opererr.New("test weakened approve: the reason is empty",
			"the reason is the judge's verdict, which the commit carries as its Weakening-approved trailer",
			"./le test weakened approve unit "+unit+" reason \"<the judge's verdict>\""))
		return leaction.Prepared{}, 2
	}
	if strings.ContainsAny(reason, "\r\n|") {
		leaction.ReportError(opererr.New("test weakened approve: the reason holds a line break or a '|'",
			"the reason becomes one Weakening-approved trailer line and one cell of the approval table",
			"./le test weakened approve unit "+unit+" reason \"<the verdict on one line, with no '|'>\""))
		return leaction.Prepared{}, 2
	}
	return leaction.PreparedAnswer(func() (any, int) {
		return approveAdmitted(root, unit, reason)
	}), 0
}

// approveAdmitted retains the admitted request across startup. Approve keeps
// the writer's own guard for callers that bypass the registered action.
func approveAdmitted(root, unit, reason string) (any, int) {
	// The approval admits the main thread's commit, so it is keyed on the
	// main thread's namespace even when the judge agent records it.
	session, err := lepath.SessionCommitSession(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := Approve(root, session, unit, reason)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}
