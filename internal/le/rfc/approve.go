// Design: docs/contributing/rfc-implementation-guide.md -- owner approval for RFC-tagged test changes
// Related: internal/le/test/weakened/proposed.go -- the edit-time hook that reads the file.
// Related: internal/le/commit/rfcchange.go -- the commit gate that reads it and writes the trailer.
package rfc

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/le/action"
	"github.com/ze-software/ze/internal/le/le/path"
)

// ApprovalTrailer is the key of the commit message line that records one
// approval for good: `RFC-approved: <package>.<TestName>: <reason>`.
const ApprovalTrailer = "RFC-approved: "

// approvalHeader opens a session file. The table is the ledger row grammar
// testweakened.ParseLedger reads, so the hook and the gate parse the file
// with the reader the weakening ledger already has.
const approvalHeader = "# RFC-tagged test changes the owner approved\n\n| Test | Reason |\n|------|--------|\n"

// approvalUnitPattern is `<package>.<TestName>`: a package directory name, one
// dot, and a Go test name or a `.ci` file stem. A path, a space, a pipe and a
// second dot are refused, so a unit is exactly what the gate reports and
// what the trailer line carries.
var approvalUnitPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$`)

// ApprovalPath answers the file one commit session's approvals live in, under
// tmp/ where git never reads it. The commit that used a row is the record.
func ApprovalPath(session string) string {
	return "tmp/commit-rfc-approved-" + session + ".md"
}

// ApproveCommand renders the command an author runs once the owner has
// answered, for the hook and the gate to print beside a refusal.
func ApproveCommand(unit string) string {
	return "./le rfc approve unit " + unit + " reason \"<the owner's words>\""
}

// ApprovalTrailerLine renders the commit message line for one used row.
func ApprovalTrailerLine(unit, reason string) string {
	return ApprovalTrailer + unit + ": " + reason
}

// ApprovalReport is what one `./le rfc approve` call did.
type ApprovalReport struct {
	Path     string `json:"path"`
	Unit     string `json:"unit"`
	Reason   string `json:"reason"`
	Replaced bool   `json:"replaced"`
}

// Approve writes the owner's approval of a change to one tagged unit into the
// session file, creating it, and replaces the reason of a row that already
// names the unit rather than adding a second row. A unit that is not
// `<package>.<TestName>`, an empty reason, and a reason that would not fit on
// one trailer line are refused before anything is written.
func Approve(root, session, unit, reason string) (ApprovalReport, error) {
	report := ApprovalReport{Path: ApprovalPath(session), Unit: unit, Reason: strings.TrimSpace(reason)}
	if !approvalUnitPattern.MatchString(unit) {
		return report, errors.New("rfc approve: unit must be <package>.<TestName>, got " + unit)
	}
	if report.Reason == "" {
		return report, errors.New("rfc approve: reason is required: the owner's words, in quotes")
	}
	if strings.ContainsAny(report.Reason, "\r\n|") {
		return report, errors.New("rfc approve: reason must be one line and carry no '|': it becomes one trailer line and one ledger cell")
	}
	full := filepath.Join(root, filepath.FromSlash(report.Path))
	content, err := os.ReadFile(full) //nolint:gosec // the path is this session's own tmp/ file under the checkout root
	if errors.Is(err, os.ErrNotExist) {
		content = []byte(approvalHeader)
		err = nil
	}
	if err != nil {
		return report, err
	}
	row := "| " + unit + " | " + report.Reason + " |"
	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	var page textbuf.Buffer
	for _, line := range lines {
		if rowNames(line, unit) {
			report.Replaced = true
			page.Str(row).Byte('\n')
			continue
		}
		page.Str(line).Byte('\n')
	}
	if !report.Replaced {
		page.Str(row).Byte('\n')
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return report, err
	}
	return report, os.WriteFile(full, []byte(page.String()), 0o600)
}

// rowNames reports whether one ledger line is the row for unit: its first
// cell, under the `| Test | Reason |` grammar, is the unit.
func rowNames(line, unit string) bool {
	body := strings.TrimSpace(line)
	if !strings.HasPrefix(body, "|") {
		return false
	}
	cells := strings.SplitN(body[1:], "|", 2)
	return strings.TrimSpace(cells[0]) == unit
}

// approveAnswer is the `le rfc approve` command body.
func approveAnswer(args leaction.Arguments) (any, int) {
	if !args.Has(keyUnit) || !args.Has(keyReason) {
		leaction.ReportError(errors.New("rfc approve requires unit <package>.<TestName> reason \"<the owner's words>\""))
		return nil, 2
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	session, err := lepath.CommitSession(root, "")
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
