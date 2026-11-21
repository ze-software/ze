// Design: docs/contributing/spec-workflow.md -- the independent design judge
// Related: approval.go -- the design digest the verdict is bound to
// Related: check.go -- the table the judge's verdict needs to pass
//
// judge.go records the independent judge's verdict on a spec's design, bound
// to the design digest, in tmp/spec-design/judged/<stem>.md
// (spec-design-needs-owner-approval, AC-15, AC-16,
// C-3). The judge is an agent that did not write the spec; the action refuses
// one with no agent identity, and one that has not read the section behind
// every row of the table.
package specdesign

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/doc/doorindex"
	"github.com/ze-software/ze/internal/le/doc/readrecord"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// recordsDir holds every record of this area, untracked, written only by
// `le spec design`.
const recordsDir = "tmp/spec-design"

// judgedDir holds one judge record per spec.
const judgedDir = recordsDir + "/judged"

// The two verdicts a judge records.
const (
	judgeApprove = "approve"
	judgeRefuse  = "refuse"
)

// judgment is one judge record.
type judgment struct {
	Spec     string `json:"spec"`
	Digest   string `json:"digest"`
	Verdict  string `json:"verdict"`
	Sentence string `json:"sentence"`
	Judge    string `json:"judge"`
	Date     string `json:"date"`
}

// judgmentFields are the record's lines, in order, each `<field>: <value>`.
var judgmentFields = []string{"Spec", "Digest", "Verdict", "Sentence", "Judge", "Date"}

// judgedPath is the judge record of stem, from the checkout root.
func judgedPath(root, stem string) string {
	return filepath.Join(root, filepath.FromSlash(judgedDir), stem+".md")
}

// values answers the record's fields in judgmentFields order.
func (j judgment) values() []string {
	return []string{j.Spec, j.Digest, j.Verdict, j.Sentence, j.Judge, j.Date}
}

// readJudgment answers the judge record of stem. A record that does not exist
// answers found false; one that cannot be read or parsed is an error naming
// the file, never an absent record, since an unreadable approval must not
// pass for none and send the spec back silently.
func readJudgment(root, stem string) (record judgment, found bool, err error) {
	file := judgedPath(root, stem)
	body, err := os.ReadFile(file) //nolint:gosec // the record of a validated stem under tmp/spec-design
	if errors.Is(err, os.ErrNotExist) {
		return judgment{}, false, nil
	}
	if err != nil {
		return judgment{}, false, fmt.Errorf("read the judge record %s: %w", file, err)
	}
	fields := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		name, value, ok := strings.Cut(scanner.Text(), ": ")
		if !ok {
			continue
		}
		fields[name] = value
	}
	for _, name := range judgmentFields {
		if fields[name] == "" {
			return judgment{}, false, errors.New("the judge record " + file + " carries no " + name +
				" line; fix: have ze-design-judge judge the spec again")
		}
	}
	record = judgment{Spec: fields["Spec"], Digest: fields["Digest"], Verdict: fields["Verdict"],
		Sentence: fields["Sentence"], Judge: fields["Judge"], Date: fields["Date"]}
	return record, true, nil
}

// writeJudgment replaces the judge record of stem atomically.
func writeJudgment(root, stem string, record judgment) error {
	var tb textbuf.Buffer
	for position, name := range judgmentFields {
		tb.Str(name).Str(": ").Str(record.values()[position]).Byte('\n')
	}
	return replaceRecord(root, judgedDir, stem+".md", tb.String())
}

// judgeApprovalProblem answers why spec has no current judge approval, or ""
// when it has one: a record of this design digest whose verdict is approve.
func judgeApprovalProblem(root string, spec specFile) (string, error) {
	record, found, err := readJudgment(root, spec.stem)
	if err != nil {
		return "", err
	}
	judgeCommand := "have ze-design-judge judge it (./le spec design judge spec " + spec.relative +
		" verdict approve reason \"<sentence>\")"
	if !found {
		return spec.name() + ": no judge has judged this design; fix: " + judgeCommand, nil
	}
	if record.Digest != DesignDigest(spec.text) {
		return spec.name() + ": the judge judged another text (digest " + shortDigest(record.Digest) +
			", now " + shortDigest(DesignDigest(spec.text)) + "); fix: " + judgeCommand, nil
	}
	if record.Verdict != judgeApprove {
		return spec.name() + ": the judge refused this design on " + record.Date + ": " + record.Sentence +
			"; fix: correct the design, then " + judgeCommand, nil
	}
	return "", nil
}

// trailerStart is the start of a line a commit reads as a trailer: the judge's
// sentence never starts with one, so it cannot pose as a trailer wherever it
// is carried.
var trailerStart = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*:`)

// reasonProblem answers what is wrong with the judge's sentence, or "".
func reasonProblem(reason string) string {
	if strings.TrimSpace(reason) == "" {
		return "the reason is empty; fix: give one sentence saying why the verdict holds"
	}
	if strings.ContainsAny(reason, "\r\n") {
		return "the reason spans more than one line; fix: give one sentence on one line"
	}
	if trailerStart.MatchString(reason) {
		return "the reason starts like a trailer key; fix: start the sentence with a word, not `Key:`"
	}
	return ""
}

// uncoveredRow is one table row whose section this agent has not read.
type uncoveredRow struct {
	document string
	section  readrecord.Section
	door     bool
}

// line renders the row for the refusal: the Read that clears it.
func (u uncoveredRow) line() string {
	if u.door {
		return "  - " + u.document + ": Read the whole door"
	}
	return "  - " + u.document + ", " + u.section.Heading + ": Read it with offset " + strconv.Itoa(u.section.Start) +
		" and limit " + strconv.Itoa(u.section.ReadLimit())
}

// uncoveredRows answers every row of rows whose section the agent of
// fingerprint holds no current read record of, once per page and section. A
// door row is covered by the whole-read marker the door gate keeps; any other
// row by a section read record carrying the section's current digest. Both
// are looked for in every store the Read hook can have written them to for
// the checkout at root (readrecord.Stores), so a judge working in a worktree
// finds the reads its session keeps in the holding checkout. A row whose page
// or section the checkout does not hold is the check's to report.
func uncoveredRows(root, fingerprint string, rows []tableRow, pages *pageCache) ([]uncoveredRow, error) {
	stores := readrecord.Stores(root)
	seen := map[string]bool{}
	var uncovered []uncoveredRow
	for _, row := range rows {
		key := row.document + "\x00" + row.section
		if seen[key] {
			continue
		}
		seen[key] = true
		if strings.HasPrefix(row.document, doorindex.Dir+"/") {
			if !doorRead(stores, row.document, fingerprint) {
				uncovered = append(uncovered, uncoveredRow{document: row.document, door: true})
			}
			continue
		}
		page, err := pages.page(row.document)
		if err != nil {
			return nil, err
		}
		if page.missing {
			continue
		}
		section, found := readrecord.Find(page.sections, row.section)
		if !found {
			continue
		}
		read, err := sectionRead(stores, row.document, fingerprint, section)
		if err != nil {
			return nil, err
		}
		if !read {
			uncovered = append(uncovered, uncoveredRow{document: row.document, section: section})
		}
	}
	return uncovered, nil
}

// doorRead reports whether some store holds the whole-read marker of door for
// fingerprint. A marker that cannot be stat'ed is no record, so the door stays
// owed and the refusal names the Read that clears it.
func doorRead(stores []readrecord.Store, door, fingerprint string) bool {
	for _, store := range stores {
		if _, err := os.Stat(readrecord.WholeReadMarker(store.Dir, store.Key(door), fingerprint)); err == nil {
			return true
		}
	}
	return false
}

// sectionRead reports whether some store records fingerprint's read of
// section of page as it stands. A record that cannot be read is an error,
// never "not read" (readrecord.SectionRead).
func sectionRead(stores []readrecord.Store, page, fingerprint string, section readrecord.Section) (bool, error) {
	for _, store := range stores {
		read, err := readrecord.SectionRead(readrecord.SectionMarker(store.Dir, store.Key(page), fingerprint), section)
		if err != nil {
			return false, err
		}
		if read {
			return true, nil
		}
	}
	return false, nil
}

// judgeAnswer is the answer of a recorded verdict.
type judgeAnswer struct {
	judgment
	Rows int `json:"rows"`
}

// Text renders the recorded verdict for a person.
func (a judgeAnswer) Text() string {
	var tb textbuf.Buffer
	return tb.Str("judged ").Str(a.Spec).Str(" (digest ").Str(shortDigest(a.Digest)).Str("): ").Str(a.Verdict).
		Str(", ").Str(strconv.Itoa(a.Rows)).Str(" rows, each section read by this agent\n").String()
}

// judge records verdict and reason on the spec at relative, for the agent this
// process runs for.
func judge(root, relative, verdict, reason string) (any, int) {
	spec, err := loadSpec(root, relative)
	if err != nil {
		return nil, refuseAction("refused: " + err.Error())
	}
	agent, err := lepath.Agent()
	if err != nil {
		return nil, refuseAction("refused: "+err.Error(), "  fix: run the judge from an agent whose commands carry its own ZE_AGENT_ID")
	}
	if agent == "" {
		return nil, refuseAction("refused: this process has no agent identity, so no judge's read records can be named",
			"  why: the judge is an independent agent that did not write the spec, and the main thread is the author's",
			"  fix: have a ze-design-judge agent run the command with its identity line (ZE_AGENT_ID) in front")
	}
	session, err := lepath.ResolveSessionID(root)
	if err != nil {
		return nil, refuseAction("refused: "+err.Error(), "  fix: run the judge inside a harness session")
	}
	fingerprint, err := lepath.AgentFingerprint(session, agent)
	if err != nil {
		return nil, refuseAction("refused: " + err.Error())
	}
	if problem := reasonProblem(reason); problem != "" {
		return nil, refuseAction("refused: " + problem)
	}
	return judgeSpec(root, spec, fingerprint, verdict, reason)
}

// judgeSpec judges the coverage and, for an approval, the table, then writes
// the record.
func judgeSpec(root string, spec specFile, fingerprint, verdict, reason string) (any, int) {
	derived, err := derive(root, spec)
	if err != nil {
		return nil, refuseAction("refused: " + err.Error())
	}
	rows, _, _ := spec.documentsTable()
	pages := pageCache{root: root, texts: map[string]pageText{}}
	uncovered, err := uncoveredRows(root, fingerprint, rows, &pages)
	if err != nil {
		return nil, refuseAction("refused: " + err.Error())
	}
	var refusal []string
	if len(uncovered) != 0 {
		refusal = append(refusal, "refused: this agent has not read "+strconv.Itoa(len(uncovered))+
			" section(s) the table gives a verdict on")
		for _, row := range uncovered {
			refusal = append(refusal, row.line())
		}
		refusal = append(refusal,
			"  why: a judge vouches only for what it read, so each row's section is read before the verdict",
			"  fix: read each listed range with the Read tool, then run the judge again")
	}
	if verdict == judgeApprove {
		problems, err := checkSpec(root, spec, derived, judgeStepBeingTaken)
		if err != nil {
			return nil, refuseAction("refused: " + err.Error())
		}
		if len(problems) != 0 {
			refusal = append(refusal, "refused: the table does not pass ./le spec design check, so no approval is recorded")
			refusal = append(refusal, problems...)
			refusal = append(refusal, "  fix: hand the problems to the spec's author, or record refuse naming the row")
		}
	}
	if len(refusal) != 0 {
		return nil, refuseAction(refusal...)
	}
	record := judgment{
		Spec: spec.name(), Digest: DesignDigest(spec.text), Verdict: verdict, Sentence: reason,
		Judge: fingerprint, Date: time.Now().Format(time.DateOnly),
	}
	if err := writeJudgment(root, spec.stem, record); err != nil {
		return nil, refuseAction("refused: " + err.Error())
	}
	return judgeAnswer{judgment: record, Rows: len(rows)}, 0
}
