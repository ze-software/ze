// Design: docs/architecture/README.md -- the page format: where an owner decision lives
// Detail: decisionschange.go -- the change-set check over the rows a commit changes
// Related: register.go -- registers judgeDecisionRows as a change-set check
//
// decisions.go owns the `## Design decisions` section of a design page: its
// parser, the judgment of one row, and the `design-decisions` tree check over
// every page a `// Design:` header names
// (spec-design-needs-owner-approval, AC-8).
package docconsistency

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/ze-software/ze/internal/le/repo/objects"
)

// DecisionsHeading is the heading of the section, a whole line, at most once
// per page.
const DecisionsHeading = "## Design decisions"

// decisionColumns are the section's table columns, in order.
var decisionColumns = []string{"Decision", "It forbids", "Source", "Date"}

// metaDecisions and metaDecisionsSituational are the Sources that name an
// owner-decisions file and a section of it.
const (
	metaDecisions            = "ai/META-DECISIONS.md"
	metaDecisionsSituational = "ai/META-DECISIONS-SITUATIONAL.md"
)

// pageFormat names where the format is defined, for every fix line.
const pageFormat = `docs/architecture/README.md, "Page format"`

// commitID is the spelling of a commit Source: 7 to 40 lowercase hex digits.
var commitID = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// separatorCell is one cell of a table's separator row.
var separatorCell = regexp.MustCompile(`^:?-+:?$`)

// DecisionRow is one row of a page's Design decisions table, as the page
// writes it.
type DecisionRow struct {
	// Line is the row's 1-based line in the page; Number its 1-based row in
	// the table.
	Line   int
	Number int
	// The four cells, trimmed; a cell the row does not hold is "".
	Decision string
	Forbids  string
	Source   string
	Date     string
	// cells is every cell the row holds, so a row of the wrong width is
	// judged as written, and two rows are the same row only when every cell is.
	cells []string
}

// key is the row's identity: every cell, trimmed. A row is the same decision
// on both sides of a commit only when no cell changed.
func (r DecisionRow) key() string {
	return strings.Join(r.cells, "\x00")
}

// DecisionProblem is one way a section or one of its rows breaks the page
// format. Text names what is wrong and the fix.
type DecisionProblem struct {
	Line int
	Text string
}

// DecisionsSection is a page's Design decisions section. A page with no
// heading answers a section whose Headings is empty: it records no decision,
// which is no problem.
type DecisionsSection struct {
	// Headings is the line of every DecisionsHeading the page carries; the
	// first is the section, any other is a problem.
	Headings []int
	Rows     []DecisionRow
	// Problems is what is wrong with the section's shape (the heading, the
	// table's columns, a missing table); a row's cells are JudgeDecisionRow's.
	Problems []DecisionProblem
}

// Present answers whether the page carries the section.
func (s DecisionsSection) Present() bool { return len(s.Headings) != 0 }

// ParseDecisions reads the Design decisions section of a page's text. The
// section runs from its heading to the next heading of level one or two; a
// fenced code block is not read, so a page that shows the format in a fence
// does not carry it. The loop is bounded by the page's line count.
func ParseDecisions(text string) DecisionsSection {
	var section DecisionsSection
	inFence := false
	inSection := false
	tableLines := 0
	for index, raw := range strings.Split(text, "\n") {
		line := strings.TrimRight(raw, " \t\r")
		number := index + 1
		if fenceLine(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if line == DecisionsHeading {
			section.Headings = append(section.Headings, number)
			inSection = len(section.Headings) == 1
			if !inSection {
				section.Problems = append(section.Problems, DecisionProblem{Line: number,
					Text: "carries a second " + DecisionsHeading + " heading; fix: move its rows into the first section, which is the page's only one"})
			}
			continue
		}
		if !inSection {
			continue
		}
		if sectionEnds(line) {
			inSection = false
			continue
		}
		if !strings.HasPrefix(line, "|") {
			continue
		}
		tableLines++
		cells := tableCells(line)
		if tableLines == 1 {
			if !slices.Equal(cells, decisionColumns) {
				section.Problems = append(section.Problems, DecisionProblem{Line: number,
					Text: "has the columns " + strings.Join(cells, ", ") + "; fix: write the columns " +
						strings.Join(decisionColumns, ", ") + " (" + pageFormat + ")"})
			}
			continue
		}
		if tableLines == 2 {
			if !separatorRow(cells) {
				section.Problems = append(section.Problems, DecisionProblem{Line: number,
					Text: "has no separator row under the table's columns; fix: write |---|---|---|---| on the line after the columns"})
			}
			continue
		}
		section.Rows = append(section.Rows, rowFromCells(number, len(section.Rows)+1, cells))
	}
	if section.Present() && len(section.Rows) == 0 {
		section.Problems = append(section.Problems, DecisionProblem{Line: section.Headings[0],
			Text: "holds no decision row, which claims that no decision binds the page; fix: remove the section, " +
				"since a page with no recorded decision carries none (" + pageFormat + ")"})
	}
	return section
}

// rowFromCells builds a DecisionRow from one table line's cells, whatever
// their number.
func rowFromCells(line, number int, cells []string) DecisionRow {
	row := DecisionRow{Line: line, Number: number, cells: cells}
	fields := []*string{&row.Decision, &row.Forbids, &row.Source, &row.Date}
	for position, field := range fields {
		if position < len(cells) {
			*field = cells[position]
		}
	}
	return row
}

// fenceLine answers whether line opens or closes a fenced code block.
func fenceLine(line string) bool {
	trimmed := strings.TrimLeft(line, " ")
	if strings.HasPrefix(trimmed, "```") {
		return true
	}
	return strings.HasPrefix(trimmed, "~~~")
}

// sectionEnds answers whether line is a heading of level one or two, which
// ends the section. A `###` subsection stays inside it.
func sectionEnds(line string) bool {
	if strings.HasPrefix(line, "# ") {
		return true
	}
	return strings.HasPrefix(line, "## ")
}

// tableCells splits one table line into its trimmed cells. An escaped `\|`
// stays inside its cell. The loop is bounded by the line's length.
func tableCells(line string) []string {
	inner := strings.TrimPrefix(strings.TrimSpace(line), "|")
	inner = strings.TrimSuffix(inner, "|")
	var cells []string
	var cell strings.Builder
	escaped := false
	for _, char := range inner {
		if escaped {
			cell.WriteRune(char)
			escaped = false
			continue
		}
		if char == '\\' {
			cell.WriteRune(char)
			escaped = true
			continue
		}
		if char == '|' {
			cells = append(cells, strings.TrimSpace(cell.String()))
			cell.Reset()
			continue
		}
		cell.WriteRune(char)
	}
	return append(cells, strings.TrimSpace(cell.String()))
}

// separatorRow answers whether cells are a table's separator row.
func separatorRow(cells []string) bool {
	for _, cell := range cells {
		if !separatorCell.MatchString(cell) {
			return false
		}
	}
	return true
}

// JudgeDecisionRow answers every way one row breaks the page format: a cell
// it lacks, an empty Decision, It forbids or Source, a Date that is not
// YYYY-MM-DD, and a Source entry that resolves to no file of the checkout at
// root, no commit of its repository and no ai/META-DECISIONS.md section.
func JudgeDecisionRow(root string, row DecisionRow) []DecisionProblem {
	label := "row " + strconv.Itoa(row.Number) + " "
	problem := func(text string) DecisionProblem { return DecisionProblem{Line: row.Line, Text: label + text} }
	if len(row.cells) != len(decisionColumns) {
		return []DecisionProblem{problem("holds " + strconv.Itoa(len(row.cells)) + " cells; fix: write the four cells " +
			strings.Join(decisionColumns, ", ") + " (" + pageFormat + ")")}
	}
	var problems []DecisionProblem
	if row.Decision == "" {
		problems = append(problems, problem("names no Decision; fix: write the decision as Thomas stated it"))
	}
	if row.Forbids == "" {
		problems = append(problems, problem("names nothing in It forbids; fix: write what a design must not do because of it"))
	}
	if row.Source == "" {
		problems = append(problems, problem("names no Source; fix: name the record that holds Thomas's words, "+
			"a commit, or "+metaDecisions+` "<section>"`))
	}
	for _, entry := range sourceEntries(row.Source) {
		if text := sourceProblem(root, entry); text != "" {
			problems = append(problems, problem("Source "+strconv.Quote(entry)+" "+text))
		}
	}
	if _, err := time.Parse(time.DateOnly, row.Date); err != nil {
		problems = append(problems, problem("Date "+strconv.Quote(row.Date)+" is not YYYY-MM-DD; fix: write the day Thomas made the decision"))
	}
	return problems
}

// sourceEntries splits a Source cell into its entries: comma-separated, code
// spans unwrapped, a comma inside a double-quoted section name kept. The loop
// is bounded by the cell's length.
func sourceEntries(source string) []string {
	unwrapped := strings.ReplaceAll(source, "`", "")
	var entries []string
	var entry strings.Builder
	quoted := false
	for _, char := range unwrapped {
		if char == '"' {
			quoted = !quoted
		}
		if char == ',' && !quoted {
			entries = appendEntry(entries, entry.String())
			entry.Reset()
			continue
		}
		entry.WriteRune(char)
	}
	return appendEntry(entries, entry.String())
}

// appendEntry adds one trimmed Source entry; an empty one adds nothing.
func appendEntry(entries []string, entry string) []string {
	trimmed := strings.TrimSpace(entry)
	if trimmed == "" {
		return entries
	}
	return append(entries, trimmed)
}

// sourceProblem answers what is wrong with one Source entry, or "" when it
// resolves: ai/META-DECISIONS.md or ai/META-DECISIONS-SITUATIONAL.md with a
// quoted section it holds, a commit this repository holds, or a file inside
// the checkout.
func sourceProblem(root, entry string) string {
	if section, isSituational := strings.CutPrefix(entry, metaDecisionsSituational); isSituational {
		return metaSectionProblem(root, metaDecisionsSituational, strings.TrimSpace(section))
	}
	if section, isMeta := strings.CutPrefix(entry, metaDecisions); isMeta {
		return metaSectionProblem(root, metaDecisions, strings.TrimSpace(section))
	}
	if commitID.MatchString(entry) {
		if _, err := objects.Resolve(context.Background(), root, entry); err != nil {
			return "is no commit this repository holds (" + err.Error() + "); fix: name the full id of the commit that records the decision"
		}
		return ""
	}
	return pathProblem(root, entry)
}

// metaSectionProblem answers whether sourceFile (or, for ai/META-DECISIONS.md,
// its situational companion ai/META-DECISIONS-SITUATIONAL.md) holds the quoted
// heading, or what is wrong.
func metaSectionProblem(root, sourceFile, quoted string) string {
	heading, err := strconv.Unquote(quoted)
	if err != nil {
		return `names no section; fix: write ` + sourceFile + ` "<the heading of the section that holds the rule>"`
	}
	found, err := fileHasHeading(root, sourceFile, heading)
	if err != nil {
		return "cannot be checked, because " + sourceFile + " cannot be read (" + err.Error() + ")"
	}
	if found {
		return ""
	}
	if sourceFile == metaDecisions {
		situationalFound, sitErr := fileHasHeading(root, metaDecisionsSituational, heading)
		if sitErr == nil && situationalFound {
			return ""
		}
	}
	return "names a section " + sourceFile + " does not hold; fix: quote one of its headings exactly"
}

func fileHasHeading(root, rel, heading string) (bool, error) {
	text, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) //nolint:gosec // a constant tracked file under the checkout root
	if err != nil {
		return false, err
	}
	for line := range strings.SplitSeq(string(text), "\n") {
		if !strings.HasPrefix(line, "#") {
			continue
		}
		if strings.TrimSpace(strings.TrimLeft(line, "#")) == heading {
			return true, nil
		}
	}
	return false, nil
}

// outsideCheckout is the problem of a Source path that leaves the checkout.
const outsideCheckout = "lies outside the checkout; fix: name the record by its path from the repository root"

// pathProblem answers whether entry names a file inside the checkout at root,
// or what is wrong. A path under tmp/ is refused: no commit carries it.
func pathProblem(root, entry string) string {
	cleaned := path.Clean(entry)
	if path.IsAbs(cleaned) {
		return outsideCheckout
	}
	if strings.HasPrefix(cleaned+"/", "../") {
		return outsideCheckout
	}
	if strings.HasPrefix(cleaned+"/", "tmp/") {
		return "lies under tmp/, which no commit carries; fix: name a committed record"
	}
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(cleaned)))
	if err != nil {
		return "resolves to no file of the checkout, no commit and no " + metaDecisions +
			" section; fix: name the record that holds Thomas's words by its path from the repository root"
	}
	if info.IsDir() {
		return "is a directory; fix: name the record that holds Thomas's words"
	}
	return ""
}

// designHeaderPrefix starts every `// Design:` line of a header.
const designHeaderPrefix = "// Design:"

// DesignLink is one page a `// Design:` line of a file's header names: the
// page, the topic the line gives it, the line's number counting from 1, and the
// line as written, which a refusal quotes. Every page of one line shares that
// line's topic.
type DesignLink struct {
	Page   string
	Topic  string
	Number int
	Line   string
}

// DesignHeader answers every page the `// Design:` lines of text's header
// name, in header order, each line's pages and topic as designTargets reads
// them. The header is the leading run of comment and blank lines: it ends at
// the first other line, `package` in Go and templ, `module` in YANG, the first
// statement elsewhere, so a `// Design:` line further down the file names no
// page. It is the one header rule `./le doc consistency`, the spec citation
// audit, the edit gate and the ai/DOCS-TO-CODE.md generator read a file by.
func DesignHeader(text string) []DesignLink {
	var links []DesignLink
	number := 0
	for line := range strings.Lines(text) {
		number++
		line = strings.TrimRight(line, "\r\n")
		if !headerLine(line) {
			break
		}
		if !strings.HasPrefix(line, designHeaderPrefix) {
			continue
		}
		pages, topic := designTargets(line)
		for _, page := range pages {
			links = append(links, DesignLink{Page: page, Topic: topic, Number: number, Line: line})
		}
	}
	return links
}

// headerLine reports whether line belongs to a file's leading header: a blank
// line or a `//` comment, `//go:build` included.
func headerLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}
	return strings.HasPrefix(trimmed, "//")
}

// headerSeparators are the words that end the page list of a `// Design:`
// line and start its description.
var headerSeparators = []string{"--", "—", "-"}

// topicSeparators are the leading separators removed from a topic, longest
// first so `--` is tried before `-`. A colon is one too: `docs/a.md : topic`.
var topicSeparators = [...]string{"—", "--", "-", ":"}

// designTargets answers the `.md` pages one `// Design:` line names and the
// topic it gives them. A page is every word before the separator (` -- ` or a
// dash) that ends in `.md` once the punctuation around it is trimmed. A line
// naming `(none ...)` or a `.go` file names no page. The topic is the text
// after the separator, or, when a word that is not a page comes first, the text
// from that word on, with one leading separator removed.
func designTargets(text string) (pages []string, topic string) {
	rest, _ := strings.CutPrefix(text, designHeaderPrefix)
	topicFound := false
	for rest = strings.TrimLeftFunc(rest, unicode.IsSpace); rest != ""; {
		word, after := headerWord(rest)
		if slices.Contains(headerSeparators, word) {
			if !topicFound {
				topic = strings.TrimSpace(after)
			}
			break
		}
		if page := strings.Trim(word, "`(),.;:"); strings.HasSuffix(page, ".md") {
			pages = append(pages, page)
		} else if !topicFound {
			topic, topicFound = stripTopicSeparator(strings.TrimSpace(rest)), true
		}
		rest = after
	}
	return pages, topic
}

// headerWord splits text, which starts with a non-space, into its first word
// and the text after it with the leading space removed.
func headerWord(text string) (word, after string) {
	end := strings.IndexFunc(text, unicode.IsSpace)
	if end < 0 {
		return text, ""
	}
	return text[:end], strings.TrimLeftFunc(text[end:], unicode.IsSpace)
}

// stripTopicSeparator removes one leading topicSeparators entry from topic.
func stripTopicSeparator(topic string) string {
	for _, separator := range topicSeparators {
		if rest, ok := strings.CutPrefix(topic, separator); ok {
			return strings.TrimSpace(rest)
		}
	}
	return topic
}

// checkDesignDecisions judges the Design decisions section of every page in
// pages (checkDesignRefs answers them): each shape problem and each row
// problem is a design-decisions error naming the page. A page with no section
// is reported as nothing: it records no owner decision.
func (c *checker) checkDesignDecisions(pages []string) {
	for _, page := range pages {
		text, err := os.ReadFile(c.path(page))
		if err != nil {
			c.unreadable(page, 0, "cannot read, so its Design decisions section was not checked", err)
			continue
		}
		section := ParseDecisions(string(text))
		for _, problem := range section.Problems {
			c.report(SeverityError, "design-decisions", page, problem.Line, "the section "+problem.Text)
		}
		for _, row := range section.Rows {
			for _, problem := range JudgeDecisionRow(c.root, row) {
				c.report(SeverityError, "design-decisions", page, problem.Line, problem.Text)
			}
		}
	}
}
