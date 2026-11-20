// Design: docs/contributing/spec-workflow.md -- the spec's Design documents and Design presentation sections
// Related: documents.go -- the rows the check holds the table to
// Related: judge.go -- the judge approval the check owes
//
// check.go judges a spec's ## Design documents against the derivation and its
// ## Design presentation against the subsections it owes
// (plan/release/validation/spec-design-needs-owner-approval.md, AC-14). It
// judges completeness and form only: whether a verdict is true is the judge's
// and Thomas's to say.
package specdesign

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/doc/readrecord"
)

// presentationSubsections are the subsections ## Design presentation owes,
// each with content.
var presentationSubsections = []string{
	"What changes", "Before and after", "Alternatives set aside", "Documents departed from", choicesHeading,
}

// departedHeading is the subsection that names every page a departs row
// departs from.
const departedHeading = "Documents departed from"

// checkAnswer is the answer of `le spec design check`.
type checkAnswer struct {
	Spec     string   `json:"spec"`
	Status   string   `json:"status"`
	Problems []string `json:"problems,omitempty"`
}

// readyToPresent is the status of a spec that owes nothing.
const readyToPresent = "ready to present"

// Text renders the check for a person: one line per problem, then the command
// that lists the owed rows.
func (a checkAnswer) Text() string {
	var tb textbuf.Buffer
	if len(a.Problems) == 0 {
		return tb.Str(a.Spec).Str(": ").Str(readyToPresent).Byte('\n').String()
	}
	for _, problem := range a.Problems {
		tb.Str(problem).Byte('\n')
	}
	return tb.String()
}

// listCommand is the last line of every failing check.
func listCommand(spec specFile) string {
	return "run ./le spec design documents spec " + spec.relative + " to list the owed rows"
}

// judgeStep says whether a judgment asks for the judge's approval: the judge
// itself runs the check without it, since its record is what it writes.
type judgeStep int

const (
	judgeStepUnspecified judgeStep = iota
	judgeStepOwed
	judgeStepBeingTaken
)

// checkSpec answers every problem of spec against derived, in the table's
// order, then the presentation's, then the judge's. It answers an error only
// when a page or a record cannot be read, never a pass.
func checkSpec(root string, spec specFile, derived derivation, step judgeStep) ([]string, error) {
	var problems []string
	rows, shape, found := spec.documentsTable()
	if !found {
		problems = append(problems, spec.name()+": has no ## "+documentsHeading+" section; fix: add it after ## Existing Mechanisms "+
			"(plan/templates/TEMPLATE.md) with a row for each page below")
	}
	problems = append(problems, shape...)
	problems = append(problems, missingRows(spec, derived, rows)...)
	pages := pageCache{root: root, texts: map[string]pageText{}}
	for _, row := range rows {
		rowProblems, err := judgeRow(spec, derived, row, &pages)
		if err != nil {
			return nil, err
		}
		problems = append(problems, rowProblems...)
	}
	problems = append(problems, presentationProblems(spec, rows)...)
	if step == judgeStepOwed {
		judgeProblem, err := judgeApprovalProblem(root, spec)
		if err != nil {
			return nil, err
		}
		if judgeProblem != "" {
			problems = append(problems, judgeProblem)
		}
	}
	return problems, nil
}

// missingRows names each derived page and decision the table gives no row.
func missingRows(spec specFile, derived derivation, rows []tableRow) []string {
	var missing []string
	for _, page := range derived.Pages {
		if slices.ContainsFunc(rows, func(row tableRow) bool { return row.document == page.Document }) {
			continue
		}
		line := "  - " + page.Document + "  (" + strings.Join(page.DerivedFrom, "; ") + ")"
		if page.Missing {
			line += ": the page does not exist; fix: correct the header that names it"
		}
		missing = append(missing, line)
	}
	for _, decision := range derived.Decisions {
		if slices.ContainsFunc(rows, func(row tableRow) bool { return coversDecision(row, decision) }) {
			continue
		}
		missing = append(missing, "  - "+decision.Document+", "+decisionsSection+", decision of "+decision.Date+
			": \""+decision.Decision+"\"  ("+strings.Join(decision.DerivedFrom, "; ")+")")
	}
	if len(missing) == 0 {
		return nil
	}
	head := spec.name() + ": ## " + documentsHeading + " is missing " + itoaRows(len(missing)) + " owed by its files"
	tail := []string{
		"  why: a spec never departs from the design documentation without a discussion with Thomas, " +
			"so it gives a verdict on every design page its files link and every decision on them",
		"  fix: add a row with a verdict for each; " + listCommand(spec),
	}
	return append(append([]string{head}, missing...), tail...)
}

// itoaRows spells a row count.
func itoaRows(count int) string {
	if count == 1 {
		return "1 row"
	}
	return strconv.Itoa(count) + " rows"
}

// coversDecision reports whether row gives decision its verdict: a row on the
// decision's page, in its Design decisions section, whose binding statement
// carries the decision.
func coversDecision(row tableRow, decision owedDecision) bool {
	if row.document != decision.Document {
		return false
	}
	if row.section != decisionsSection {
		return false
	}
	return strings.Contains(normalized(row.statement), normalized(decision.Decision))
}

// pageText is one page as read for the check: its sections, or that the
// checkout does not hold it.
type pageText struct {
	missing  bool
	sections []readrecord.Section
}

// pageCache reads each page a row names once.
type pageCache struct {
	root  string
	texts map[string]pageText
}

// page answers document as the checkout holds it.
func (c *pageCache) page(document string) (pageText, error) {
	if cached, found := c.texts[document]; found {
		return cached, nil
	}
	body, err := os.ReadFile(filepath.Join(c.root, filepath.FromSlash(document))) //nolint:gosec // a checkout-relative page a row names
	if errors.Is(err, os.ErrNotExist) {
		c.texts[document] = pageText{missing: true}
		return c.texts[document], nil
	}
	if err != nil {
		return pageText{}, errors.New("read " + document + ": " + err.Error())
	}
	c.texts[document] = pageText{sections: readrecord.Sections(string(body))}
	return c.texts[document], nil
}

// judgeRow answers what is wrong with one table row: its verdict word, the
// page and section it names, its quote, and a silent verdict the row may not
// give.
func judgeRow(spec specFile, derived derivation, row tableRow, pages *pageCache) ([]string, error) {
	var problems []string
	where := spec.name() + ": " + row.where()
	if !slices.Contains([]string{verdictAgrees, verdictDeparts, verdictSilent}, row.verdict) {
		problems = append(problems, where+": the verdict "+quoted(row.verdict)+" is none of agrees, departs, silent; fix: give one of the three")
	}
	page, err := pages.page(row.document)
	if err != nil {
		return nil, err
	}
	if page.missing {
		return append(problems, where+": "+row.document+" does not exist; fix: name the page by its path from the repository root"), nil
	}
	section, found := readrecord.Find(page.sections, row.section)
	if !found {
		problems = append(problems, where+": "+row.document+" holds no section "+quoted(row.section)+
			"; fix: name the heading of the section the binding statement is quoted from")
	}
	switch row.verdict {
	case verdictAgrees, verdictDeparts:
		problems = append(problems, quoteProblems(where, row, section, found)...)
	case verdictSilent:
		problems = append(problems, silentProblems(where, derived, row)...)
	}
	return problems, nil
}

// quoteProblems judges the binding statement of an agrees or departs row: a
// verbatim span of the section it names, blanks compared as one.
func quoteProblems(where string, row tableRow, section readrecord.Section, found bool) []string {
	quote := normalized(unquoted(row.statement))
	if quote == "" {
		return []string{where + ": the " + row.verdict + " row quotes nothing; fix: quote the sentence of the section the verdict stands against"}
	}
	if !found {
		return nil
	}
	if strings.Contains(normalized(section.Text), quote) {
		return nil
	}
	return []string{where + ": the binding statement is not a verbatim span of " + row.document + ", " + row.section +
		"; fix: quote the section's own words"}
}

// silentProblems judges a silent row: it needs a reason, and neither a page
// the spec modifies nor a decision row may be silent.
func silentProblems(where string, derived derivation, row tableRow) []string {
	var problems []string
	if strings.TrimSpace(row.stands) == "" {
		problems = append(problems, where+": the silent row gives no reason; fix: say why the page says nothing on what the design changes")
	}
	if slices.Contains(derived.modified, row.document) {
		problems = append(problems, where+": the spec modifies "+row.document+", so it cannot be silent on it; fix: give agrees or departs")
	}
	if row.section == decisionsSection {
		problems = append(problems, where+": a decision row cannot be silent; fix: give agrees or departs")
	}
	return problems
}

// presentationProblems judges ## Design presentation: each owed subsection
// present with content, and every departs page named under Documents
// departed from.
func presentationProblems(spec specFile, rows []tableRow) []string {
	presentation, found := spec.section(presentationHeading)
	if !found {
		return []string{spec.name() + ": has no ## " + presentationHeading + " section; fix: add it before ## Implementation Steps " +
			"with the subsections " + strings.Join(presentationSubsections, ", ")}
	}
	var problems []string
	subsections := spec.subsections(presentation)
	for _, heading := range presentationSubsections {
		sub, present := readrecord.Find(subsections, heading)
		if !present {
			problems = append(problems, spec.name()+": ## "+presentationHeading+" has no ### "+heading+"; fix: write it")
			continue
		}
		if sub.End == sub.Start {
			problems = append(problems, spec.name()+": ### "+heading+" is empty; fix: write it")
		}
	}
	departed, _ := readrecord.Find(subsections, departedHeading)
	for _, row := range rows {
		if row.verdict != verdictDeparts {
			continue
		}
		if strings.Contains(departed.Text, row.document) {
			continue
		}
		problems = append(problems, spec.name()+": "+row.where()+" departs, and ### "+departedHeading+
			" does not name "+row.document+"; fix: present the departure there with its before and after")
	}
	return problems
}

// normalized collapses every run of blanks to one space, so a quote wrapped
// across lines still matches.
func normalized(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// unquoted removes one pair of double quotes around a binding statement.
func unquoted(statement string) string {
	trimmed := strings.TrimSpace(statement)
	for _, pair := range [][2]string{{"\"", "\""}, {"“", "”"}} {
		inner, opened := strings.CutPrefix(trimmed, pair[0])
		if !opened {
			continue
		}
		if closedInner, closed := strings.CutSuffix(inner, pair[1]); closed {
			return closedInner
		}
	}
	return trimmed
}

// quoted wraps a cell for a problem line.
func quoted(cell string) string { return "\"" + cell + "\"" }
