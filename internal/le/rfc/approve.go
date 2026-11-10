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
	"github.com/ze-software/ze/internal/le/le/human"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// ApprovalTrailer is the key of the commit message line that records one
// approval for good: `RFC-approved: <package>.<TestName>: <reason>`.
const ApprovalTrailer = "RFC-approved: "

// ApprovalForTrailer binds a same-carrier ordinary verdict to one exact change.
const ApprovalForTrailer = "RFC-approved-for: "

// approvalHeader opens a session file. The table is the row grammar
// testweakened.ParseLedger reads, so the hook and the gate parse every
// approval file with one reader.
const approvalHeader = "# RFC-tagged test changes the owner approved\n\n" + ApprovalTableHeader

// ApprovalTableHeader opens the one table every owner approval file holds, the
// RFC one and the weakening one: the unit, the judge's verdict, the exact
// commit a historical verdict binds (empty for an ordinary row), and the
// unit's content digests the verdict judged (ApprovalContentCell).
const ApprovalTableHeader = "| Test | Reason | Target | Content |\n|------|--------|--------|---------|\n"

const keyCommit = "commit"

// approvalUnitPattern is `<package>.<TestName>`: a package directory name, a
// dot, and a Go test name or a `.ci` file stem. The directory name may itself
// hold dots (`test/exabgp-compat/old-5.0/` gives `old-5.0.<stem>`), so the
// name is the part after the last dot (SplitApprovalUnit), and neither a Go
// test name nor a `.ci` stem holds one. A path, a space, a pipe, an empty
// dot-separated segment (`..`, a leading or trailing dot) and a bare name are
// refused, so a unit is exactly what the gate reports and what the trailer
// line carries.
var approvalUnitPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*\.[A-Za-z0-9_-]+$`)

// SplitApprovalUnit answers the directory key and the test name of a unit
// that ValidApprovalUnit accepted, cut at the last dot: the directory key
// may hold dots, the test name never does.
func SplitApprovalUnit(unit string) (packageName, name string) {
	cut := strings.LastIndexByte(unit, '.')
	if cut < 0 {
		return "", unit
	}
	return unit[:cut], unit[cut+1:]
}

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

// ApprovalForTrailerLine renders the exact change/unit companion of a verdict.
func ApprovalForTrailerLine(target, unit string) string {
	return ApprovalForTrailer + target + " " + unit
}

// ParseApprovalForTrailer reads one canonical full-SHA/unit companion. Callers
// retain their existing message-line whitespace normalization.
func ParseApprovalForTrailer(line string) (target, unit string, valid bool) {
	value, found := strings.CutPrefix(line, ApprovalForTrailer)
	if !found {
		return "", "", false
	}
	target, unit, split := strings.Cut(value, " ")
	if !split {
		return "", "", false
	}
	if !ValidApprovalTarget(target) {
		return "", "", false
	}
	if !ValidApprovalUnit(unit) {
		return "", "", false
	}
	return target, unit, true
}

// ApprovalReport is what one approve call did.
type ApprovalReport struct {
	Path     string `json:"path"`
	Unit     string `json:"unit"`
	Reason   string `json:"reason"`
	Target   string `json:"target,omitempty"`
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
	Target  string
}

// ValidApprovalUnit reports whether unit is `<package>.<TestName>`, the one
// grammar every owner approval names.
func ValidApprovalUnit(unit string) bool {
	return approvalUnitPattern.MatchString(unit)
}

// Approve writes the judge's approval into the session's RFC approval file.
// A reason citing an owner ruling MUST be issued from the owner's terminal.
func Approve(root, session, unit, reason string) (ApprovalReport, error) {
	return approveTarget(root, session, "", unit, reason)
}

func approveTarget(root, session, target, unit, reason string) (ApprovalReport, error) {
	if err := requireRulingTerminal(root, target, unit, reason); err != nil {
		return ApprovalReport{}, err
	}
	return ApproveInto(root, ApprovalFile{
		Path: ApprovalPath(session), Header: approvalHeader, Command: "rfc approve", Target: target,
	}, unit, reason)
}

// requireRulingTerminal uses the same predicate as the recorded-ruling consumer.
// Ordinary judge verdicts and the shared weakening writer remain agent-usable.
func requireRulingTerminal(root, target, unit, reason string) error {
	operands := []string{keyUnit, unit, keyReason, reason}
	if target != "" {
		operands = append([]string{keyCommit, target}, operands...)
	}
	return requireRulingAuthorization(root, reason, operands)
}

func requireRulingAuthorization(root, reason string, operands []string) error {
	if ownerRuling(reason) == "" {
		return nil
	}
	return human.RequireAuthorization(root, "", "rfc approve", operands)
}

// ApproveInto writes the judge's approval of one unit into file, creating it,
// replacing only the exact (target, unit) row. Ordinary rows have no target.
// Every row's unit is resolved to its one carrier first (ResolveApprovalUnit),
// so a name no consumer would ever match (a Go package clause in place of the
// directory key, an unknown or ambiguous unit) is refused before anything is
// written, and the row records the digests of the unit's content the verdict
// judged, so it cannot admit a later different change of the same unit
// (ApprovalContentAdmits).
func ApproveInto(root string, file ApprovalFile, unit, reason string) (ApprovalReport, error) {
	report := ApprovalReport{Path: file.Path, Unit: unit, Reason: strings.TrimSpace(reason), Target: file.Target}
	if !ValidApprovalUnit(unit) {
		return report, errors.New(file.Command + ": unit must be <package>.<TestName>, got " + unit)
	}
	if report.Reason == "" {
		return report, errors.New(file.Command + ": reason is required: the judge's verdict, in quotes")
	}
	if strings.ContainsAny(report.Reason, "\r\n|") {
		return report, errors.New(file.Command + ": reason must be one line and carry no '|': it becomes one trailer line and one ledger cell")
	}
	judged, err := ResolveApprovalUnitContent(root, file.Target, unit)
	if err != nil {
		return report, err
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
	row := "| " + unit + " | " + report.Reason + " | " + file.Target + " | " + ApprovalContentCell(judged) + " |"
	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	var page textbuf.Buffer
	for _, line := range lines {
		if rowNames(line, file.Target, unit) {
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
	return approvalLanded(root, trailer, "")
}

// BoundApprovalLanded finds a unique verdict and its exact target binding in one
// carrier. The tuple is spent regardless of later wording of the judge's reason.
func BoundApprovalLanded(root, trailer, target, unit string) (carrier string, landed bool, problem string) {
	return approvalLanded(root, trailer, ApprovalForTrailerLine(target, unit))
}

func approvalLanded(root, trailer, binding string) (carrier string, landed bool, problem string) {
	command := exec.CommandContext(context.Background(), "git", "log", "--no-show-signature", "--format=%H") // #nosec G204 -- Fixed metadata-only history query; message bytes never frame carrier IDs.
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
			strings.HasPrefix(complaint.String(), "fatal: not a git repository (") {
			return "", false, ""
		}
		return "", false, "git log approval history failed: " + strings.TrimSpace(complaint.String())
	}
	carrier, matched, problem := approvalHistoryLanded(root, stdout.String(), []approvalHistoryQuery{newApprovalHistoryQuery(trailer, binding)})
	return carrier, matched >= 0, problem
}

// rowNames compares the existing table's exact target/unit key.
func rowNames(line, target, unit string) bool {
	body := strings.TrimSpace(line)
	if !strings.HasPrefix(body, "|") {
		return false
	}
	cells := strings.Split(strings.Trim(body, "|"), "|")
	if len(cells) != 4 {
		return false
	}
	if strings.TrimSpace(cells[0]) != unit {
		return false
	}
	return strings.TrimSpace(cells[2]) == target
}

// approveAnswer admits a parsed ruling before process startup can write.
func approveAnswer(args leaction.Arguments) (leaction.Prepared, int) {
	if !args.Has(keyUnit) || !args.Has(keyReason) {
		leaction.ReportError(errors.New("rfc approve requires unit <package>.<TestName> reason \"<the judge's verdict>\""))
		return leaction.Prepared{}, 2
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return leaction.Prepared{}, 2
	}
	unit, reason, target := args.One(keyUnit), args.One(keyReason), args.One(keyCommit)
	// This boundary MUST admit before SessionCommitSession writes a namespace.
	operands := []string{keyUnit, unit, keyReason, reason}
	if args.Has(keyCommit) {
		operands = append([]string{keyCommit, target}, operands...)
	}
	if err := requireRulingAuthorization(root, reason, operands); err != nil {
		leaction.ReportError(err)
		return leaction.Prepared{}, 2
	}
	if args.Has(keyCommit) && target == "" {
		leaction.ReportError(errors.New("rfc approve: commit requires a canonical full SHA"))
		return leaction.Prepared{}, 2
	}
	if _, err := ResolveApprovalUnit(root, target, unit); err != nil {
		leaction.ReportError(err)
		return leaction.Prepared{}, 2
	}
	if strings.TrimSpace(reason) == "" {
		leaction.ReportError(errors.New("rfc approve: reason is required"))
		return leaction.Prepared{}, 2
	}
	if strings.ContainsAny(reason, "\r\n|") {
		leaction.ReportError(errors.New("rfc approve: reason must be one line and carry no '|'"))
		return leaction.Prepared{}, 2
	}
	return leaction.PreparedAnswer(func() (any, int) {
		return approveAdmitted(root, target, unit, reason)
	}), 0
}

// approveAdmitted retains the parsed ruling across startup. Approve keeps the
// independent lower writer guard for callers that bypass this adapter.
func approveAdmitted(root, target, unit, reason string) (any, int) {
	// The approval admits the main thread's commit, so it is keyed on the
	// main thread's namespace even when the judge agent records it.
	session, err := lepath.SessionCommitSession(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := approveTarget(root, session, target, unit, reason)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}
