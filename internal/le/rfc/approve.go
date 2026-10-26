// Design: docs/contributing/rfc-implementation-guide.md -- owner approval for RFC-tagged test changes
// Related: internal/le/test/weakened/proposed.go -- the edit-time hook that reads the file.
// Related: internal/le/commit/rfcchange.go -- the commit gate that reads it and writes the trailer.
package rfc

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// ApprovalTrailer is the key of the commit message line that records one
// approval for good: `RFC-approved: <package>.<TestName>: <reason>`.
const ApprovalTrailer = "RFC-approved: "

// approvalHeader opens a session file. The table is the row grammar
// testweakened.ParseLedger reads, so the hook and the gate parse every
// approval file with one reader.
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

// ApproveCommand renders the command the ze-test-judge agent runs once it has
// approved, for the hook and the gate to print beside a refusal.
func ApproveCommand(unit string) string {
	return "./le rfc approve unit " + unit + " reason \"<the judge's verdict>\""
}

// ApprovalTrailerLine renders the commit message line for one used row.
func ApprovalTrailerLine(unit, reason string) string {
	return ApprovalTrailer + unit + ": " + reason
}

// ApprovalReport is what one approve call did.
type ApprovalReport struct {
	Path     string `json:"path"`
	Unit     string `json:"unit"`
	Reason   string `json:"reason"`
	Replaced bool   `json:"replaced"`
}

// ApprovalFile names one owner approval file and how it is written: its path,
// the header that opens it, and the command that writes it, which prefixes
// each refusal. The RFC approvals and the test weakening approvals are two
// such files with one row grammar, so one writer serves both.
type ApprovalFile struct {
	Path    string
	Header  string
	Command string
}

// ValidApprovalUnit reports whether unit is `<package>.<TestName>`, the one
// grammar every owner approval names.
func ValidApprovalUnit(unit string) bool {
	return approvalUnitPattern.MatchString(unit)
}

// Approve writes the judge's approval of a change to one tagged unit into the
// session's RFC approval file (ApproveInto).
func Approve(root, session, unit, reason string) (ApprovalReport, error) {
	return ApproveInto(root, ApprovalFile{Path: ApprovalPath(session), Header: approvalHeader, Command: "rfc approve"}, unit, reason)
}

// ApproveInto writes the judge's approval of one unit into file, creating it,
// and replaces the reason of a row that already names the unit rather than
// adding a second row. A unit that is not `<package>.<TestName>`, an empty
// reason, and a reason that would not fit on one trailer line are refused
// before anything is written.
func ApproveInto(root string, file ApprovalFile, unit, reason string) (ApprovalReport, error) {
	report := ApprovalReport{Path: file.Path, Unit: unit, Reason: strings.TrimSpace(reason)}
	if !ValidApprovalUnit(unit) {
		return report, errors.New(file.Command + ": unit must be <package>.<TestName>, got " + unit)
	}
	if report.Reason == "" {
		return report, errors.New(file.Command + ": reason is required: the judge's verdict, in quotes")
	}
	if strings.ContainsAny(report.Reason, "\r\n|") {
		return report, errors.New(file.Command + ": reason must be one line and carry no '|': it becomes one trailer line and one ledger cell")
	}
	full := filepath.Join(root, filepath.FromSlash(report.Path))
	content, err := os.ReadFile(full) //nolint:gosec // the path is this session's own tmp/ file under the checkout root
	if errors.Is(err, os.ErrNotExist) {
		content = []byte(file.Header)
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

// TrailerLanded reports whether a commit in this repository already carries
// one trailer line, which is how a gate knows an approval row was used, and
// names that commit as `<short hash> <subject>` for the refusal.
//
// `git log --grep` matches a substring, so a landed trailer whose reason
// EXTENDS this one would answer for it. It only narrows the candidates: the
// verdict is a whole message line equal to the trailer.
func TrailerLanded(root, trailer string) (carrier string, landed bool, problem string) {
	command := exec.CommandContext(context.Background(), "git", "log", "--fixed-strings", "--grep="+trailer, "--format=%x1e%h %s%x1f%B") // #nosec G204 -- fixed Git query; the trailer is one argv operand.
	command.Dir = root
	var stdout bytes.Buffer
	var complaint bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &complaint
	if err := command.Run(); err != nil {
		// A tree with no commits, or one no repository holds at all (an edit
		// hook fixture tree), has no commit that could carry the trailer, so
		// "not landed" is the answer rather than a default. Any other git
		// failure leaves the question open, and the caller fails closed.
		if _, ok := errors.AsType[*exec.ExitError](err); ok && strings.Contains(complaint.String(), "does not have any commits") {
			return "", false, ""
		}
		if _, ok := errors.AsType[*exec.ExitError](err); ok &&
			strings.HasPrefix(complaint.String(), "fatal: not a git repository (or any of the parent") {
			return "", false, ""
		}
		return "", false, "git log --grep failed: " + strings.TrimSpace(complaint.String())
	}
	for record := range strings.SplitSeq(stdout.String(), "\x1e") {
		name, body, found := strings.Cut(record, "\x1f")
		if !found {
			continue
		}
		for line := range strings.SplitSeq(body, "\n") {
			if strings.TrimSpace(line) == trailer {
				return strings.TrimSpace(name), true, ""
			}
		}
	}
	return "", false, ""
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
		leaction.ReportError(errors.New("rfc approve requires unit <package>.<TestName> reason \"<the judge's verdict>\""))
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
