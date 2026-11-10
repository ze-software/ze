// Design: docs/architecture/testing/test-health.md -- owner approval of a test weakening
// Related: testweakened.go -- pairing parsed rows with HEAD/worktree findings.
package testweakened

import (
	"errors"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/rfc"
)

var separatorCellPattern = regexp.MustCompile(`^:?-{2,}:?$`)

// Row is one owner approval and its reason in an approval file.
type Row struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
	Target string `json:"target,omitempty"`
	// Base and Current are the digests of the unit's content the verdict
	// judged, before and after the change (rfc.ApprovalContentCell).
	Base    string `json:"base"`
	Current string `json:"current"`
	Line    int    `json:"line"`
}

// ledgerHeaderCells is the one table header every approval file opens with
// (rfc.ApprovalTableHeader).
var ledgerHeaderCells = []string{"Test", "Reason", "Target", "Content"}

func parseLedger(contents, path string) ([]Row, []string) {
	lines := strings.Split(contents, "\n")
	starts := make([]int, 0, 1)
	for index, line := range lines {
		cells, tableRow := tableCells(line)
		if tableRow && slices.Equal(cells, ledgerHeaderCells) {
			starts = append(starts, index)
		}
	}
	var message textbuf.Buffer
	if len(starts) == 0 {
		return nil, []string{message.Str(path).
			Str(" has no `| Test | Reason | Target | Content |` table header, so no row in it can be read; ").
			Str("record each approval again with the command that wrote it").String()}
	}

	problems := make([]string, 0)
	if len(starts) > 1 {
		problems = append(problems, message.Reset().Str(path).Str(" has ").Int(int64(len(starts))).
			Str(" `| Test | Reason | Target | Content |` tables; keep one, or the gate has two answers").String())
	}
	rows := make([]Row, 0)
	seen := make(map[string]int)
	for index := starts[0] + 1; index < len(lines); index++ {
		cells, tableRow := tableCells(lines[index])
		if !tableRow {
			break
		}
		line := index + 1
		header, _ := tableCells(lines[starts[0]])
		if len(cells) != len(header) {
			problems = append(problems, message.Reset().Str(path).Byte(':').Int(int64(line)).
				Str(" has ").Int(int64(len(cells))).Str(" cells; a row is `").
				Str(strings.TrimSpace(lines[starts[0]])).Byte('`').String())
			continue
		}
		name, reason := cells[0], cells[1]
		if separatorRow(cells) {
			continue
		}
		target := cells[2]
		base, current, judged := rfc.ParseApprovalContentCell(cells[3])
		if !judged {
			problems = append(problems, message.Reset().Str(path).Byte(':').Int(int64(line)).
				Str(" names no judged content for ").Str(name).
				Str("; only the approve command writes a row, so record the approval again with it").String())
			continue
		}
		if target != "" && !rfc.ValidApprovalTarget(target) {
			problems = append(problems, path+": approval target must be a canonical full commit SHA: "+target)
			continue
		}
		if name == "" {
			problems = append(problems,
				message.Reset().Str(path).Byte(':').Int(int64(line)).Str(" names no test").String())
			continue
		}
		if reason == "" {
			problems = append(problems, message.Reset().Str(path).Byte(':').Int(int64(line)).
				Str(" gives no reason for ").Str(name).
				Str("; a row with no reason accepts nothing").String())
			continue
		}
		key := target + "\x00" + name
		if previous, duplicate := seen[key]; duplicate {
			problems = append(problems, message.Reset().Str(path).Byte(':').Int(int64(line)).
				Str(" names ").Str(name).Str(" again (already on line ").Int(int64(previous)).
				Str("); one test, one reason").String())
			continue
		}
		seen[key] = line
		rows = append(rows, Row{Name: name, Reason: reason, Target: target, Base: base, Current: current, Line: line})
	}
	return rows, problems
}

// ParseLedger parses an owner approval file, the weakening one or the RFC
// one, with the canonical table and optional exact-target column.
func ParseLedger(contents, path string) ([]Row, []string) {
	return parseLedger(contents, path)
}

// RowMatches reports whether an approval row's name covers one package/test
// pair.
func RowMatches(rowName, packageName, testName string) bool {
	return rowMatches(rowName, Finding{Package: packageName, Name: testName})
}

// UnitSite is one changed test an approval could name: its unit,
// `<package>.<TestName>`, and the checkout-relative slash path of the file
// that carries it.
type UnitSite struct {
	Unit string
	Path string
}

// SharedUnits answers, for each unit the sites carry from more than one
// directory, those directories in sorted order. A unit keeps only the base
// name of its directory, so tests in a/cmd and b/cmd are both `cmd.<Test>`,
// and an approval of that unit cannot say which of them the owner approved.
// The commit gate refuses such a commit and the audit lets no approval admit
// either test; the edit hook judges one file per edit, so it never meets two
// directories.
func SharedUnits(sites []UnitSite) map[string][]string {
	directories := make(map[string][]string)
	for _, site := range sites {
		directory := path.Dir(site.Path)
		if !slices.Contains(directories[site.Unit], directory) {
			directories[site.Unit] = append(directories[site.Unit], directory)
		}
	}
	shared := make(map[string][]string)
	for unit, held := range directories {
		if len(held) > 1 {
			slices.Sort(held)
			shared[unit] = held
		}
	}
	return shared
}

// SharedUnitProblem says why an approval of unit, which the commit changes in
// each of directories, approves nothing, and how the author fixes it.
func SharedUnitProblem(unit string, directories []string) string {
	var text textbuf.Buffer
	text.Str("the approval unit ").Str(unit).Str(" names a test in each of ").
		Str(strings.Join(directories, ", ")).
		Str(" in this one commit, so no approval of it can say which test the judge approved: ").
		Str("split the commit so each carries the change to one of them, and record one approval for each commit")
	return text.String()
}

// SharedUnitProblems is the commit gate's refusal for every unit SharedUnits
// finds among sites, one problem per unit in sorted order.
func SharedUnitProblems(sites []UnitSite) []string {
	shared := SharedUnits(sites)
	units := make([]string, 0, len(shared))
	for unit := range shared {
		units = append(units, unit)
	}
	slices.Sort(units)
	problems := make([]string, 0, len(units))
	for _, unit := range units {
		problems = append(problems, "Refused: "+SharedUnitProblem(unit, shared[unit])+".")
	}
	return problems
}

// ApprovalSource names one owner approval file and how its rows become
// commit message lines: the weakening file (WeakeningApprovals) or the RFC
// one (RFCApprovals).
type ApprovalSource struct {
	Path        string
	TrailerLine func(unit, reason string) string
	// Command renders the command that records an approval of unit, for a
	// refusal to print.
	Command func(unit string) string
	// Bindings admits exact historical target rows only in the RFC source.
	Bindings bool
}

// WeakeningApprovals is the source of one commit session's weakening
// approval file.
func WeakeningApprovals(session string) ApprovalSource {
	return ApprovalSource{Path: ApprovalPath(session), TrailerLine: ApprovalTrailerLine, Command: ApproveCommand}
}

// RFCApprovals is the source of one commit session's RFC approval file.
func RFCApprovals(session string) ApprovalSource {
	return ApprovalSource{Path: rfc.ApprovalPath(session), TrailerLine: rfc.ApprovalTrailerLine, Command: rfc.ApproveCommand, Bindings: true}
}

// ApprovalRow is one parsed approval row, the exact file line that holds it,
// its trailer, and the commit that already carries that trailer. Carrier is
// empty for a live row; a row with a carrier did its work and approves
// nothing further.
type ApprovalRow struct {
	Row
	Text    string
	Trailer string
	Carrier string
	// Stale says why a live ordinary row no longer judges its unit: the unit
	// changed from another base, or into other content, after the verdict
	// (rfc.ApprovalContentAdmits). A stale row approves nothing.
	Stale string
}

// LoadApprovals reads one owner approval file the way the edit hook, the
// commit gate and the audit all read it, so the three cannot disagree on
// which rows are live. An absent file holds no row. An unreadable or
// malformed file, a row whose unit is not `<package>.<TestName>`, and a git
// history that cannot be searched are each a problem, so a caller fails
// closed rather than admitting nothing in silence. Beside a refused unit the
// well-formed rows are still answered, so the hook can name what each
// changed unit still needs; no caller admits anything while a problem
// stands.
func LoadApprovals(root string, source ApprovalSource) ([]ApprovalRow, []string) {
	content, err := readRepositoryFile(root, source.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	var text textbuf.Buffer
	if err != nil {
		return nil, []string{text.Str(cannotRunPrefix).Str("cannot read ").Str(source.Path).
			Str(": ").Err(err).String()}
	}
	parsed, problems := parseLedger(validUTF8(content), source.Path)
	if len(problems) != 0 {
		return nil, problems
	}
	valid := make([]Row, 0, len(parsed))
	for _, row := range parsed {
		if row.Target != "" && !source.Bindings {
			problems = append(problems, source.Path+": targeted rows belong only in the RFC approval table")
			continue
		}
		if rfc.ValidApprovalUnit(row.Name) {
			valid = append(valid, row)
		} else {
			problems = append(problems, text.Reset().Str(source.Path).Byte(':').Int(int64(row.Line)).
				Str(" names ").Str(row.Name).
				Str(", which is not <package>.<TestName>, so it approves nothing.\n").
				Str("    Remove the row, and record a ze-test-judge approval of the qualified unit:\n    ").
				Str(source.Command("<package>.<TestName>")).String())
		}
	}
	lines := strings.Split(string(content), "\n")
	rows := make([]ApprovalRow, 0, len(valid))
	for _, row := range valid {
		held := ApprovalRow{Row: row, Text: lines[row.Line-1], Trailer: source.TrailerLine(row.Name, row.Reason)}
		var carrier, problem string
		var landed bool
		if row.Target == "" {
			carrier, landed, problem = rfc.TrailerLanded(root, held.Trailer)
		} else {
			carrier, landed, problem = rfc.BoundApprovalLanded(root, held.Trailer, row.Target, row.Name)
		}
		if problem != "" {
			return nil, []string{text.Reset().Str(cannotRunPrefix).Str(problem).String()}
		}
		if landed {
			held.Carrier = carrier
		}
		if !landed && row.Target == "" {
			held.Stale = staleApproval(root, row)
		}
		rows = append(rows, held)
	}
	return rows, problems
}

// staleApproval answers why an ordinary row's verdict no longer judges its
// unit, or empty when it still does. The unit is read as it stands now, HEAD
// before the change and the working tree after it, which is the content the
// commit gate selects and the base the edit hook's change starts from. A unit
// that no longer resolves to one carrier is stale too: the verdict judged a
// carrier that is gone or no longer unique.
func staleApproval(root string, row Row) string {
	now, err := rfc.ResolveApprovalUnitContent(root, "", row.Name)
	if err != nil {
		return "its unit no longer resolves: " + err.Error()
	}
	if rfc.ApprovalContentAdmits(row.Base, row.Current, now) {
		return ""
	}
	return "the unit changed after the verdict, so the verdict judged another change"
}

func tableCells(line string) ([]string, bool) {
	body := strings.TrimSpace(line)
	if !strings.HasPrefix(body, "|") {
		return nil, false
	}
	parts := strings.Split(body, "|")
	if len(parts) != 0 && strings.TrimSpace(parts[0]) == "" {
		parts = parts[1:]
	}
	if len(parts) != 0 && strings.TrimSpace(parts[len(parts)-1]) == "" {
		parts = parts[:len(parts)-1]
	}
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	return parts, true
}

func separatorRow(cells []string) bool {
	filled := 0
	for _, cell := range cells {
		if cell == "" {
			continue
		}
		filled++
		if !separatorCellPattern.MatchString(cell) {
			return false
		}
	}
	return filled != 0
}
