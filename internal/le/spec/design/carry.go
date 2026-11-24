// Design: docs/contributing/spec-workflow.md -- an approval carries over to a text that only writes in Thomas's own decisions
// Related: approval.go -- the one lookup that reads the carried record and its trailer
// Related: approve.go -- the approval record a carried record copies Thomas's words from
// Related: check.go -- the check and the judge approval the carried text still owes
//
// carry.go is `le spec design carry`. Thomas's approval binds a design digest,
// so any change of the four sections stales it. One kind of change is not put
// back to him (ai/META-DECISIONS.md, "How agents work"): a change that only
// writes in a choice he made himself. This file proves a change is of that
// kind, mechanically, against the text he approved: only rows of ## Design
// documents change, each added row gives a verdict on a row of a page's
// Design decisions section, each changed row follows a rename on the page it
// quotes, and nothing else moves. The independent judge's approval of the new
// text is still owed. On a pass it writes tmp/spec-design/carried/<stem>.md,
// which the one lookup reads as his approval of the new text, and the next
// commit carrying the spec records the chain in its Design-approved: line.
package specdesign

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/doc/readrecord"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// carriedDir holds one carried approval per spec: an approval record whose
// Digest is the later text and whose Carried from is the text Thomas approved.
const carriedDir = recordsDir + "/carried"

// carryRepair is the repair of a carried record that cannot be used.
const carryRepair = "; fix: run ./le spec design carry spec <path> again, or have Thomas approve the spec (./le spec design approve)"

// carriedPath is the carried approval record of stem, from the checkout root.
func carriedPath(root, stem string) string {
	return filepath.Join(root, filepath.FromSlash(carriedDir), stem+".md")
}

// approvedTextName is the file, beside an approval or carried record, that
// holds the spec text the record approves. It never ends in .md, so no reader
// of the record directories takes it for a record.
func approvedTextName(stem string) string { return stem + ".text" }

// localApproval answers the local record that approves digest for stem:
// Thomas's approval record, else the record carry wrote. found false means
// neither names this digest. It is the only reader of the two for a digest, so
// the lookup and the owed trailer cannot disagree on what a checkout approves.
func localApproval(root, stem, digest string) (record approvalRecord, found bool, err error) {
	record, found, err = readApprovalFile(approvedPath(root, stem))
	if err != nil {
		return approvalRecord{}, false, err
	}
	if found {
		if record.digest == digest {
			return record, true, nil
		}
	}
	record, found, err = readApprovalRecord(carriedPath(root, stem), carryRepair)
	if err != nil {
		return approvalRecord{}, false, err
	}
	if !found {
		return approvalRecord{}, false, nil
	}
	if record.digest != digest {
		return approvalRecord{}, false, nil
	}
	return record, true, nil
}

// approvedBase is one approval of Thomas's a carry may start from: where it
// was found, and how its text is recovered.
type approvedBase struct {
	approval Approval
	// where names the approval for a person.
	where string
	// text answers the spec text the approval approved. kept false means no
	// copy of that text exists, with why.
	text func() (text string, kept bool, why string, err error)
}

// approvedBases answers every approval of Thomas's of the spec stem names,
// most recent source first: the carried record, his approval record, then each
// Design-approved: line reachable from HEAD, newest first. An away admission
// is no approval of his, so it is no base. A digest is listed once: a record
// written before approve kept the text, or on another machine, has no copy
// beside it, so the newest commit carrying the same digest supplies its text.
func (l *approvalLookup) approvedBases(stem string) ([]approvedBase, error) {
	var bases []approvedBase
	// listed is each digest's position in bases, and inHistory the digests a
	// commit's text already stands behind.
	listed := map[string]int{}
	inHistory := map[string]bool{}
	local := []struct{ file, dir, repair string }{
		{carriedPath(l.root, stem), carriedDir, carryRepair},
		{approvedPath(l.root, stem), approvedDir, approveRepair},
	}
	for _, source := range local {
		record, found, err := readApprovalRecord(source.file, source.repair)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		if _, twice := listed[record.digest]; twice {
			continue
		}
		listed[record.digest] = len(bases)
		bases = append(bases, approvedBase{
			approval: record.approval(stem),
			where:    "the record " + source.dir + "/" + stem + ".md",
			text:     l.recordText(source.dir, stem, record.digest),
		})
	}
	trailers, err := l.load()
	if err != nil {
		return nil, err
	}
	for _, trailer := range trailers {
		if trailer.stem != stem {
			continue
		}
		if strings.HasPrefix(trailer.value, awayWord) {
			continue
		}
		if inHistory[trailer.digest] {
			continue
		}
		inHistory[trailer.digest] = true
		if position, local := listed[trailer.digest]; local {
			bases[position].text = firstKept(bases[position].text, l.commitText(trailer))
			continue
		}
		listed[trailer.digest] = len(bases)
		approval, _, err := l.trailerApproval(trailer)
		if err != nil {
			return nil, err
		}
		bases = append(bases, approvedBase{
			approval: approval,
			where:    "commit " + shortDigest(trailer.commit),
			text:     l.commitText(trailer),
		})
	}
	return bases, nil
}

// firstKept answers the text first keeps, else the one second keeps.
func firstKept(first, second func() (string, bool, string, error)) func() (string, bool, string, error) {
	return func() (string, bool, string, error) {
		text, kept, why, err := first()
		if err != nil {
			return "", false, "", err
		}
		if kept {
			return text, true, why, nil
		}
		return second()
	}
}

// recordText recovers the text a local record approves from the copy written
// beside it. A copy of another digest is no copy of the approved text.
func (l *approvalLookup) recordText(dir, stem, digest string) func() (string, bool, string, error) {
	return func() (string, bool, string, error) {
		name := dir + "/" + approvedTextName(stem)
		body, err := os.ReadFile(filepath.Join(l.root, filepath.FromSlash(name))) //nolint:gosec // the copy of a validated stem under tmp/spec-design
		if errors.Is(err, os.ErrNotExist) {
			return "", false, name + " does not exist, and no commit carries this approval", nil
		}
		if err != nil {
			return "", false, "", errors.New("read " + name + ": " + err.Error())
		}
		if DesignDigest(string(body)) != digest {
			return "", false, name + " holds another text than the record approves", nil
		}
		return string(body), true, "", nil
	}
}

// commitText recovers the text a Design-approved: line approves from the spec
// its commit carries.
func (l *approvalLookup) commitText(trailer approvalTrailer) func() (string, bool, string, error) {
	return func() (string, bool, string, error) {
		text, err := l.specTextAt(trailer.commit, trailer.stem)
		if err != nil {
			return "", false, "", err
		}
		if DesignDigest(text) != trailer.digest {
			return "", false, "the spec commit " + shortDigest(trailer.commit) + " carries has another design digest than its Design-approved: line", nil
		}
		return text, true, "", nil
	}
}

// carryDelta is what a carried text changes in the text Thomas approved.
type carryDelta struct {
	// added counts the rows the table gained, each on a decision of his.
	added int
	// renamed counts the rows whose quote follows a rename on its page.
	renamed int
	// problems names each change that goes back to Thomas; empty when the
	// approval carries over.
	problems []string
}

// carriedSections are the sections a carried text keeps byte for byte, as the
// digest covers them. ## Design documents is judged row by row instead.
var carriedSections = []string{"Task", "Acceptance Criteria", presentationHeading}

// carryChanges compares current, the text to carry the approval to, with base,
// the text Thomas approved, and answers what changed and which changes he did
// not decide. derived is current's derivation, and pages reads the pages its
// rows quote. An error is a page that could not be read, never a pass.
func carryChanges(base, current specFile, derived derivation, pages *pageCache) (carryDelta, error) {
	var delta carryDelta
	for _, heading := range carriedSections {
		before, hadBefore := base.digestBody(heading)
		after, hasAfter := current.digestBody(heading)
		if hadBefore != hasAfter {
			delta.problems = append(delta.problems, "## "+heading+" was added or removed since the approval")
			continue
		}
		if before != after {
			delta.problems = append(delta.problems, "## "+heading+" changed since the approval")
		}
	}
	frameBefore, hadFrame := documentsFrame(base)
	frameAfter, hasFrame := documentsFrame(current)
	if hadFrame != hasFrame {
		delta.problems = append(delta.problems, "## "+documentsHeading+" was added or removed since the approval")
	}
	if frameBefore != frameAfter {
		delta.problems = append(delta.problems, "## "+documentsHeading+" changed outside its table rows since the approval")
	}
	baseRows, _, _ := base.documentsTable()
	currentRows, _, _ := current.documentsTable()
	taken := make([]bool, len(currentRows))
	// Rows that stand as approved are paired first, so a row whose quote
	// changed is never paired with an unchanged row that shares its other cells.
	var moved []tableRow
	for _, row := range baseRows {
		position := unpairedRow(currentRows, taken, func(candidate tableRow) bool { return sameRow(row, candidate) })
		if position < 0 {
			moved = append(moved, row)
			continue
		}
		taken[position] = true
	}
	for _, row := range moved {
		position := unpairedRow(currentRows, taken, func(candidate tableRow) bool { return sameRowButStatement(row, candidate) })
		if position < 0 {
			delta.problems = append(delta.problems, "the approved "+row.where()+
				" was removed, or changed in more than its quoted statement")
			continue
		}
		taken[position] = true
		problem, err := renameProblem(row, currentRows[position], pages)
		if err != nil {
			return carryDelta{}, err
		}
		if problem != "" {
			delta.problems = append(delta.problems, problem)
			continue
		}
		delta.renamed++
	}
	for position, row := range currentRows {
		if taken[position] {
			continue
		}
		if problem := addedRowProblem(row, derived); problem != "" {
			delta.problems = append(delta.problems, problem)
			continue
		}
		delta.added++
	}
	return delta, nil
}

// documentsFrame answers ## Design documents without its table's data rows:
// the prose, the header and the separator, which a carried text keeps. found
// false means the spec has no such section.
func documentsFrame(spec specFile) (frame string, found bool) {
	section, found := spec.section(documentsHeading)
	if !found {
		return "", false
	}
	var kept []string
	tableLines := 0
	for line := range strings.SplitSeq(section.Text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "|") {
			tableLines++
			if tableLines > 2 {
				continue
			}
		}
		kept = append(kept, line)
	}
	return strings.TrimRight(strings.Join(kept, "\n"), " \t\r\n"), true
}

// unpairedRow answers the position of the first row of rows not yet taken
// that matches, or -1.
func unpairedRow(rows []tableRow, taken []bool, matches func(tableRow) bool) int {
	for position, row := range rows {
		if taken[position] {
			continue
		}
		if matches(row) {
			return position
		}
	}
	return -1
}

// sameRowButStatement reports whether two rows agree in every cell but the
// binding statement.
func sameRowButStatement(a, b tableRow) bool {
	return a.document == b.document && a.section == b.section && a.derived == b.derived &&
		a.verdict == b.verdict && a.stands == b.stands
}

// sameRow reports whether two rows agree in every cell.
func sameRow(a, b tableRow) bool {
	return sameRowButStatement(a, b) && a.statement == b.statement
}

// renameProblem answers why the changed quote of a row is no rename on its
// page, or "": the page's section no longer holds the approved quote and holds
// the new one, and the row does not depart.
func renameProblem(approved, changed tableRow, pages *pageCache) (string, error) {
	where := "the " + changed.where()
	if changed.verdict == verdictDeparts {
		return where + " departs, and its quoted statement changed since the approval", nil
	}
	before := normalized(unquoted(approved.statement))
	after := normalized(unquoted(changed.statement))
	if before == "" {
		return where + " quoted nothing when it was approved, and quotes a statement now", nil
	}
	if after == "" {
		return where + " quoted a statement when it was approved, and quotes nothing now", nil
	}
	page, err := pages.page(changed.document)
	if err != nil {
		return "", err
	}
	if page.missing {
		return where + " names a page the checkout does not hold", nil
	}
	section, found := readrecord.Find(page.sections, changed.section)
	if found {
		return renamedQuoteProblem(where, changed.document, normalized(section.Text), before, after), nil
	}
	return where + " names a section its page does not hold", nil
}

// renamedQuoteProblem answers why a quote changed from before to after is no
// rename in text, the section it quotes on document, or "".
func renamedQuoteProblem(where, document, text, before, after string) string {
	if !strings.Contains(text, after) {
		return where + " changed its quoted statement to words " + document + " does not hold"
	}
	if strings.Contains(text, before) {
		return where + " changed its quoted statement while " + document +
			" still holds the approved one, so the change is no rename on the page"
	}
	return ""
}

// addedRowProblem answers why a row the table gained is not a verdict on a
// decision of Thomas's, or "": a row on a page's Design decisions section,
// agrees or silent, whose statement carries a decision the derivation lists.
func addedRowProblem(row tableRow, derived derivation) string {
	where := "the added " + row.where()
	if row.section != decisionsSection {
		return where + " is not about a row of a page's ## " + decisionsSection + " section"
	}
	if !slices.Contains([]string{verdictAgrees, verdictSilent}, row.verdict) {
		return where + " gives the verdict " + quoted(row.verdict) + ", and only an agrees or silent row is added without Thomas"
	}
	covers := slices.ContainsFunc(derived.Decisions, func(decision owedDecision) bool { return coversDecision(row, decision) })
	if !covers {
		return where + " quotes no decision that ./le spec design documents derives for this spec"
	}
	return ""
}

// carryAnswer is the answer of `le spec design carry`.
type carryAnswer struct {
	Spec string `json:"spec"`
	// Status is carried, or already approved when the text needs no carry.
	Status   string `json:"status"`
	Digest   string `json:"digest"`
	From     string `json:"carried-from,omitempty"`
	Approved string `json:"approved"`
	Words    string `json:"owner-words"`
	Added    int    `json:"rows-added"`
	Renamed  int    `json:"quotes-renamed"`
	Judge    string `json:"judge-sentence,omitempty"`
	Record   string `json:"record,omitempty"`
}

// The two statuses a carry answers.
const (
	carryStatusCarried  = "carried"
	carryStatusApproved = "already approved"
)

// Text renders the carry for a person, and the next step.
func (a carryAnswer) Text() string {
	var tb textbuf.Buffer
	if a.Status == carryStatusApproved {
		return tb.Str(a.Spec).Str(" (digest ").Str(shortDigest(a.Digest)).Str("): already approved on ").Str(a.Approved).
			Str(` "`).Str(a.Words).Str("\"; nothing to carry\n").String()
	}
	return tb.Str("carried ").Str(a.Spec).Str(": Thomas's approval of ").Str(a.Approved).Str(` "`).Str(a.Words).
		Str(`" (digest `).Str(shortDigest(a.From)).Str(") now covers digest ").Str(shortDigest(a.Digest)).Str(": ").
		Str(strconv.Itoa(a.Added)).Str(" decision rows added, ").Str(strconv.Itoa(a.Renamed)).
		Str(" quotes follow a rename on their page; the judge: ").Str(a.Judge).Byte('\n').
		Str("next: the next commit carrying the spec records Design-approved with this chain\n").String()
}

// answerCarry is `le spec design carry spec <path>`.
func answerCarry(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		return nil, reportError(err)
	}
	spec, err := loadSpec(root, args.One(specParameter.Keyword))
	if err != nil {
		return nil, refuseAction("refused: " + err.Error())
	}
	return carry(root, spec)
}

// approveFix is the two closing lines of a carry refusal: the route back to
// Thomas.
func approveFix(spec specFile) []string {
	return []string{
		"  fix: present the design to Thomas; he runs, from his own terminal:",
		"    ./le spec design approve spec " + spec.relative + ` words "<what he said>"`,
	}
}

// carryWhy is why a carry refuses a change.
const carryWhy = "  why: an approval carries over only to a text that writes in decisions Thomas made himself; " +
	`anything else goes back to him (ai/META-DECISIONS.md, "How agents work")`

// carry judges spec as the check does, with the judge's approval of this text
// owed, then looks for an approval of Thomas's whose text differs from spec
// only in what carryChanges allows, and on a pass replaces the spec's carried
// record. The bases are tried most recent first, and the refusal names what
// the most recent one does not cover.
func carry(root string, spec specFile) (any, int) {
	derived, err := derive(root, spec)
	if err != nil {
		return nil, refuseAction("refused: " + err.Error())
	}
	problems, err := checkSpec(root, spec, derived, judgeStepOwed)
	if err != nil {
		return nil, refuseAction("refused: " + err.Error())
	}
	if len(problems) != 0 {
		lines := append([]string{"refused: " + spec.name() + " is not ready to carry an approval"}, problems...)
		lines = append(lines,
			"  why: a carried approval still owes the check and the independent judge's approval of this text",
			"  fix: repair each line above, then run",
			"    ./le spec design carry spec "+spec.relative)
		return nil, refuseAction(lines...)
	}
	digest := DesignDigest(spec.text)
	lookup := approvalLookup{root: root}
	standing, found, err := lookup.approved(spec.stem, spec.text)
	if err != nil {
		return nil, refuseAction("refused: " + err.Error())
	}
	if found {
		if !standing.Away {
			return carryAnswer{Spec: spec.name(), Status: carryStatusApproved, Digest: digest, From: standing.CarriedFrom,
				Approved: standing.Date, Words: standing.Words}, 0
		}
	}
	bases, err := lookup.approvedBases(spec.stem)
	if err != nil {
		return nil, refuseAction("refused: " + err.Error())
	}
	if len(bases) == 0 {
		return nil, refuseAction(append([]string{"refused: Thomas approved no text of " + spec.name() + ", so there is no approval to carry",
			"  why: an approval is carried from a text he approved to one that adds only his own decisions"}, approveFix(spec)...)...)
	}
	var refusal []string
	pages := pageCache{root: root, texts: map[string]pageText{}}
	for i := range bases {
		base := &bases[i]
		text, kept, why, err := base.text()
		if err != nil {
			return nil, refuseAction("refused: " + err.Error())
		}
		named := "his approval of " + base.approval.Date + " (digest " + shortDigest(base.approval.Digest) + ", " + base.where + ")"
		if !kept {
			if refusal == nil {
				refusal = []string{"refused: " + named + " cannot be carried to " + spec.name() + ": the text he approved is not kept",
					"  - " + why}
			}
			continue
		}
		approved := specFile{relative: spec.relative, stem: spec.stem, text: text, sections: readrecord.Sections(text)}
		delta, err := carryChanges(approved, spec, derived, &pages)
		if err != nil {
			return nil, refuseAction("refused: " + err.Error())
		}
		if len(delta.problems) == 0 {
			return recordCarry(root, spec, base.approval, delta)
		}
		if refusal == nil {
			refusal = []string{"refused: " + named + " does not carry over to " + spec.name() + " (digest " + shortDigest(digest) +
				"); these changes go back to Thomas:"}
			for _, problem := range delta.problems {
				refusal = append(refusal, "  - "+problem)
			}
		}
	}
	if len(bases) > 1 {
		refusal = append(refusal, "  his "+strconv.Itoa(len(bases)-1)+" earlier approval(s) of this spec do not cover this text either")
	}
	refusal = append(refusal, carryWhy)
	return nil, refuseAction(append(refusal, approveFix(spec)...)...)
}

// recordCarry writes the carried record of spec from base, Thomas's approval
// of the earlier text: his date and words, the digest carried from, and the
// judge's verdict on this text. The text goes first, so a later carry can
// start from this one before a commit holds it.
func recordCarry(root string, spec specFile, base Approval, delta carryDelta) (any, int) {
	// checkSpec passed with the judge's step owed, so the record exists, is
	// this digest's and says approve: it is read here for its fields.
	verdict, found, err := readJudgment(root, spec.stem)
	if err != nil {
		return nil, refuseAction("refused: " + err.Error())
	}
	if !found {
		return nil, refuseAction("refused: the judge record of "+spec.name()+" disappeared during the carry",
			"  fix: run the carry again:", "    ./le spec design carry spec "+spec.relative)
	}
	machine, err := os.Hostname()
	if err != nil {
		return nil, refuseAction("refused: the machine name cannot be read: "+err.Error(),
			"  fix: run the carry again on a host that answers its name")
	}
	record := approvalRecord{
		spec: spec.name(), digest: DesignDigest(spec.text), date: base.Date, machine: machine, words: base.Words,
		judge:       verdict.Digest + " " + verdict.Date + " " + verdict.Sentence,
		carriedFrom: base.Digest, carriedOn: time.Now().Format(time.DateOnly),
		departures: departsRows(spec),
	}
	if err := replaceRecord(root, carriedDir, approvedTextName(spec.stem), spec.text); err != nil {
		return nil, refuseAction("refused: " + err.Error())
	}
	if err := replaceRecord(root, carriedDir, spec.stem+".md", record.render()); err != nil {
		return nil, refuseAction("refused: " + err.Error())
	}
	return carryAnswer{Spec: spec.name(), Status: carryStatusCarried, Digest: record.digest, From: base.Digest,
		Approved: base.Date, Words: base.Words, Added: delta.added, Renamed: delta.renamed, Judge: verdict.Sentence,
		Record: carriedDir + "/" + spec.stem + ".md"}, 0
}
