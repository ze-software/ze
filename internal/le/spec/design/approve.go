// Design: docs/contributing/spec-workflow.md -- Thomas approves a design from his own terminal
// Related: approval.go -- the design digest the record is bound to, and the one approval lookup
// Related: check.go -- the judgment the approval re-runs
// Related: records.go -- the atomic replace and the record reader
//
// approve.go is `le spec design approve`: from Thomas's terminal only, it
// re-runs the check, requires the judge's current approval, and writes
// tmp/spec-design/approved/<stem>.md
// (spec-design-needs-owner-approval, AC-17 to
// AC-19, "The records").
package specdesign

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	"github.com/ze-software/ze/internal/le/le/human"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// approvedDir holds one approval record per spec.
const approvedDir = recordsDir + "/approved"

// The fields of an approval record, in order. Departure repeats, one line per
// departs row.
const (
	fieldSpec       = "Spec"
	fieldDigest     = "Digest"
	fieldDate       = "Date"
	fieldMachine    = "Machine"
	fieldOwnerWords = "Owner words"
	fieldJudge      = "Judge"
	fieldDeparture  = "Departure"
)

// approvalFields are the single fields every approval record carries.
var approvalFields = []string{fieldSpec, fieldDigest, fieldDate, fieldMachine, fieldOwnerWords, fieldJudge}

// Departure is one `departs` row Thomas approved: the page, the section, and
// the statement the row quotes.
type Departure struct {
	Page      string `json:"page"`
	Section   string `json:"section"`
	Statement string `json:"statement"`
}

// line renders the departure as its record value: three tab-separated cells,
// since a table cell never holds a tab.
func (d Departure) line() string {
	return d.Page + "\t" + d.Section + "\t" + d.Statement
}

// parseDeparture reads one Departure value back.
func parseDeparture(value string) (Departure, bool) {
	cells := strings.SplitN(value, "\t", 3)
	if len(cells) != 3 {
		return Departure{}, false
	}
	return Departure{Page: cells[0], Section: cells[1], Statement: cells[2]}, true
}

// names reports whether the departure is from decision row decision of page:
// the row on that page, in its Design decisions section, whose statement
// carries the decision, by the rule the check holds a table to
// (coversDecision).
func (d Departure) names(page, decision string) bool {
	if d.Page != page {
		return false
	}
	if d.Section != decisionsSection {
		return false
	}
	return strings.Contains(normalized(d.Statement), normalized(decision))
}

// approvalRecord is one record under tmp/spec-design/approved/.
type approvalRecord struct {
	spec       string
	digest     string
	date       string
	machine    string
	words      string
	judge      string
	departures []Departure
}

// approvedPath is the approval record of stem, from the checkout root.
func approvedPath(root, stem string) string {
	return filepath.Join(root, filepath.FromSlash(approvedDir), stem+".md")
}

// render answers the record's text.
func (a approvalRecord) render() string {
	var tb textbuf.Buffer
	for _, pair := range [][2]string{
		{fieldSpec, a.spec}, {fieldDigest, a.digest}, {fieldDate, a.date},
		{fieldMachine, a.machine}, {fieldOwnerWords, a.words}, {fieldJudge, a.judge},
	} {
		tb.Str(pair[0]).Str(": ").Str(pair[1]).Byte('\n')
	}
	for _, departure := range a.departures {
		tb.Str(fieldDeparture).Str(": ").Str(departure.line()).Byte('\n')
	}
	return tb.String()
}

// readApprovalFile reads the approval record at file. A missing file answers
// found false; one that cannot be read or lacks a field is an error naming
// the file and the repair (AC-26).
func readApprovalFile(file string) (record approvalRecord, found bool, err error) {
	lines, found, err := readRecord(file, fieldDeparture)
	if err != nil || !found {
		return approvalRecord{}, found, err
	}
	repair := "; fix: have Thomas approve the spec again (./le spec design approve spec <path> words \"<what he said>\")"
	if missing := lines.missingField(approvalFields); missing != "" {
		return approvalRecord{}, false, &recordError{file: file, problem: "carries no " + missing + " line" + repair}
	}
	record = approvalRecord{
		spec: lines.single[fieldSpec], digest: lines.single[fieldDigest], date: lines.single[fieldDate],
		machine: lines.single[fieldMachine], words: lines.single[fieldOwnerWords], judge: lines.single[fieldJudge],
	}
	for _, value := range lines.repeated[fieldDeparture] {
		departure, ok := parseDeparture(value)
		if !ok {
			return approvalRecord{}, false, &recordError{file: file, problem: "carries a Departure line that is not three cells" + repair}
		}
		record.departures = append(record.departures, departure)
	}
	return record, true, nil
}

// recordError is a record that exists and cannot be used.
type recordError struct {
	file    string
	problem string
}

func (e *recordError) Error() string { return "the record " + e.file + " " + e.problem }

// approveAnswer is the answer of a recorded approval.
type approveAnswer struct {
	Spec      string `json:"spec"`
	Digest    string `json:"digest"`
	Agrees    int    `json:"agrees"`
	Departs   int    `json:"departs"`
	Silent    int    `json:"silent"`
	JudgeDate string `json:"judge-date"`
	Record    string `json:"record"`
}

// Text renders the approval for Thomas, and the next step.
func (a approveAnswer) Text() string {
	var tb textbuf.Buffer
	rows := a.Agrees + a.Departs + a.Silent
	return tb.Str("approved ").Str(a.Spec).Str(" (digest ").Str(shortDigest(a.Digest)).Str("): design documents ").
		Str(strconv.Itoa(rows)).Str(" (").Str(strconv.Itoa(a.Agrees)).Str(" agrees, ").Str(strconv.Itoa(a.Departs)).
		Str(" departs, ").Str(strconv.Itoa(a.Silent)).Str(" silent), judged approve on ").Str(a.JudgeDate).Byte('\n').
		Str("next: the main session sets Status to ready; the next commit carrying the spec records Design-approved\n").
		String()
}

// answerApprove is `le spec design approve spec <path> words "<text>"`. The
// terminal gate runs before anything is read or written (AC-17).
func answerApprove(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		return nil, reportError(err)
	}
	relative := args.One(specParameter.Keyword)
	words := args.One(wordsParameter.Keyword)
	operands := []string{specParameter.Keyword, relative, wordsParameter.Keyword, words}
	if err := human.RequireAuthorization(root, "", area+" approve", operands); err != nil {
		return nil, refuseAction(err.Error())
	}
	if problem := wordsProblem(words); problem != "" {
		return nil, refuseAction("refused: " + problem)
	}
	spec, err := loadSpec(root, relative)
	if err != nil {
		return nil, refuseAction("refused: " + err.Error())
	}
	return approve(root, spec, words)
}

// approve judges spec as the check does, with the judge's approval owed, and
// on a pass replaces the spec's approval record (AC-18, AC-19).
func approve(root string, spec specFile, words string) (any, int) {
	derived, err := derive(root, spec)
	if err != nil {
		return nil, refuseAction("refused: " + err.Error())
	}
	problems, err := checkSpec(root, spec, derived, judgeStepOwed)
	if err != nil {
		return nil, refuseAction("refused: " + err.Error())
	}
	if len(problems) != 0 {
		lines := append([]string{"refused: " + spec.name() + " is not ready for approval"}, problems...)
		lines = append(lines, "  fix: repair each line above, then run ./le spec design approve again")
		return nil, refuseAction(lines...)
	}
	// checkSpec passed with the judge's step owed, so the record exists, is
	// this digest's and says approve: it is read here for its fields.
	verdict, found, err := readJudgment(root, spec.stem)
	if err != nil {
		return nil, refuseAction("refused: " + err.Error())
	}
	if !found {
		return nil, refuseAction("refused: the judge record of " + spec.name() + " disappeared during the approval; fix: run the approval again")
	}
	machine, err := os.Hostname()
	if err != nil {
		return nil, refuseAction("refused: the machine name cannot be read: "+err.Error(), "  fix: run the approval again on a host that answers its name")
	}
	rows, _, _ := spec.documentsTable()
	answer := approveAnswer{Spec: spec.name(), Digest: DesignDigest(spec.text), JudgeDate: verdict.Date,
		Record: approvedDir + "/" + spec.stem + ".md"}
	record := approvalRecord{
		spec: spec.name(), digest: answer.Digest, date: time.Now().Format(time.DateOnly), machine: machine, words: words,
		judge: verdict.Digest + " " + verdict.Date + " " + verdict.Sentence,
	}
	for _, row := range rows {
		switch row.verdict {
		case verdictAgrees:
			answer.Agrees++
		case verdictDeparts:
			answer.Departs++
			record.departures = append(record.departures, Departure{Page: row.document, Section: row.section, Statement: row.statement})
		case verdictSilent:
			answer.Silent++
		}
	}
	if err := replaceRecord(root, approvedDir, spec.stem+".md", record.render()); err != nil {
		return nil, refuseAction("refused: " + err.Error())
	}
	return answer, 0
}
