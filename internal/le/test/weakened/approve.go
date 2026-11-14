// Design: docs/contributing/testing.md -- when a test must be weakened
// Related: proposed.go -- the edit-time hook that reads the file.
// Related: testweakened.go -- the commit gate that reads it.
// Related: internal/le/rfc/approve.go -- the one approval writer.
package testweakened

import (
	"errors"
	"fmt"
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
	keyFile   = "file"
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
	return ApproveIn(root, session, unit, "", reason)
}

// ApproveIn is Approve for the unit in the one file carrierPath, which an
// approval needs when more than one file of the unit's directory declares its
// name; an empty carrierPath is Approve. The row records the file, so the
// edit hook and the commit gate admit the change to that file's unit alone.
func ApproveIn(root, session, unit, carrierPath, reason string) (rfc.ApprovalReport, error) {
	if carrierPath != "" {
		if err := resolveApproval(root, unit, carrierPath); err != nil {
			return rfc.ApprovalReport{}, err
		}
	}
	return rfc.ApproveInto(root, rfc.ApprovalFile{
		Path: ApprovalPath(session), Header: approvalHeader, Command: "test weakened approve",
		Carrier: carrierPath,
	}, unit, reason)
}

// errUnneededQualifier refuses a file qualifier on a unit the bare name
// already resolves to one carrier.
var errUnneededQualifier = errors.New("the unit resolves to one carrier without a file qualifier")

// resolveApproval admits unit for an approval, qualified with carrierPath when
// it is not empty. The bare unit is resolved first. A qualifier is accepted
// only when the bare unit is ambiguous (rfc.AmbiguousCarrierError), because
// the qualified resolution reads that one file alone: on a unit that resolves
// without it, the qualifier would drop the carrier the unit had at HEAD under
// another path (a move), and the staleness check would stop following the
// text the judge read.
func resolveApproval(root, unit, carrierPath string) error {
	_, bare := rfc.ResolveApprovalUnitIn(root, "", unit, "")
	if carrierPath == "" {
		return bare
	}
	if _, ambiguous := errors.AsType[*rfc.AmbiguousCarrierError](bare); !ambiguous {
		if bare != nil {
			return bare
		}
		return fmt.Errorf("approval unit %s: %w", unit, errUnneededQualifier)
	}
	_, err := rfc.ResolveApprovalUnitIn(root, "", unit, carrierPath)
	return err
}

// ambiguousUnitFix renders the commands that record an approval of unit in
// each file that declares it, for the refusal of an ambiguous unit.
func ambiguousUnitFix(unit string, ambiguous *rfc.AmbiguousCarrierError) string {
	var text strings.Builder
	text.WriteString("name the file the judge read, one of:")
	for _, path := range ambiguous.Paths {
		text.WriteString("\n    ./le test weakened approve unit " + unit + " file " + path + " reason \"<the judge's verdict>\"")
	}
	return text.String()
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
	carrierPath := args.One(keyFile)
	if args.Has(keyFile) && !rfc.ValidApprovalCarrierPath(carrierPath) {
		leaction.ReportError(opererr.New("test weakened approve: file "+carrierPath+" is not a clean repository-relative path",
			"the file qualifier names the one test file that declares the unit, as git lists it",
			"./le test weakened approve unit "+unit+" file <directory>/<name>_test.go reason \"<the judge's verdict>\""))
		return leaction.Prepared{}, 2
	}
	// A refusal the arguments alone decide comes before any git read.
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
	// This boundary MUST admit before SessionCommitSession writes a namespace.
	if err := resolveApproval(root, unit, carrierPath); err != nil {
		if errors.Is(err, errUnneededQualifier) {
			leaction.ReportError(opererr.New("test weakened approve: unit "+unit+" is declared in one file, so it takes no file qualifier",
				"a file binds the approval to that file alone and drops the unit's committed side, so a move would no longer be tracked",
				"./le test weakened approve unit "+unit+" reason \"<the judge's verdict>\""))
			return leaction.Prepared{}, 2
		}
		if ambiguous, found := errors.AsType[*rfc.AmbiguousCarrierError](err); found {
			leaction.ReportError(opererr.Wrap(err, "test weakened approve: unit "+unit+" is declared in more than one file",
				"an approval admits one test unit, and the name alone does not say which file's unit the judge read",
				ambiguousUnitFix(unit, ambiguous)))
			return leaction.Prepared{}, 2
		}
		leaction.ReportError(opererr.Wrap(err, "test weakened approve: unit "+unit+" names no one changed test",
			"an approval admits one test unit, <package>.<TestName>, that the working tree changes against HEAD",
			"run ./le test weakened check to list the weakened units, then ./le test weakened approve unit <package>.<TestName> reason \"<the judge's verdict>\""))
		return leaction.Prepared{}, 2
	}
	return leaction.PreparedAnswer(func() (any, int) {
		return approveAdmitted(root, unit, carrierPath, reason)
	}), 0
}

// approveAdmitted retains the admitted request across startup. Approve keeps
// the writer's own guard for callers that bypass the registered action.
func approveAdmitted(root, unit, carrierPath, reason string) (any, int) {
	// The approval admits the main thread's commit, so it is keyed on the
	// main thread's namespace even when the judge agent records it.
	session, err := lepath.SessionCommitSession(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := ApproveIn(root, session, unit, carrierPath, reason)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}
