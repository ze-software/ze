// Design: docs/contributing/spec-workflow.md -- the spec's Design documents and Design presentation sections
// Related: documents.go -- the rows a spec owes, derived from its files
// Related: check.go -- the judgment of the table against them
//
// specfile.go reads one spec: its path, its stem, its sections and its Design
// documents table, as the spec text writes them.
package specdesign

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/le/doc/readrecord"
)

// The headings of the two sections a spec carries for its design.
const (
	documentsHeading    = "Design documents"
	presentationHeading = "Design presentation"
)

// decisionsSection is the Section cell of a row about a decision row: the
// heading of the page's Design decisions section, without its #s.
const decisionsSection = "Design decisions"

// documentsColumns are the Design documents table's columns, in order.
var documentsColumns = []string{
	"Document", "Section", "Derived from", "Binding statement", "Verdict", "How this design stands against it",
}

// The three verdicts a row may give.
const (
	verdictAgrees  = "agrees"
	verdictDeparts = "departs"
	verdictSilent  = "silent"
)

// specStem is the spelling of a spec's file name without .md: it names the
// records under tmp/spec-design/, so it MUST be one safe path component.
var specStem = regexp.MustCompile(`^spec-[a-z0-9][a-z0-9.-]*$`)

// errNotUnderPlan is the refusal of a spec path that does not name a file
// under plan/ from the repository root.
const errNotUnderPlan = " is not a path under plan/; fix: name the spec from the repository root, such as plan/release/fixes/spec-x.md"

// errNotASpec is the refusal of a path under plan/ whose file name is not a
// spec's.
const errNotASpec = " is not a spec file; fix: name a plan/<bucket>/spec-<name>.md file"

// specFile is one spec as read from the checkout.
type specFile struct {
	// relative is the spec's checkout-relative slash path, cleaned.
	relative string
	// stem is the file name without .md.
	stem     string
	text     string
	sections []readrecord.Section
}

// name is the spec's file name.
func (s specFile) name() string { return s.stem + ".md" }

// loadSpec reads the spec at relative, a path under plan/ from root. A path
// outside plan/, one that climbs out of it, or one whose file name is not a
// spec's is refused before anything is read.
func loadSpec(root, relative string) (specFile, error) {
	cleaned := path.Clean(filepath.ToSlash(relative))
	if path.IsAbs(cleaned) {
		return specFile{}, errors.New(relative + errNotUnderPlan)
	}
	if !strings.HasPrefix(cleaned, "plan/") {
		return specFile{}, errors.New(relative + errNotUnderPlan)
	}
	stem, isMarkdown := strings.CutSuffix(path.Base(cleaned), ".md")
	if !isMarkdown {
		return specFile{}, errors.New(relative + errNotASpec)
	}
	if !specStem.MatchString(stem) {
		return specFile{}, errors.New(relative + errNotASpec)
	}
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(cleaned))) //nolint:gosec // a spec under plan/, checked above
	if errors.Is(err, os.ErrNotExist) {
		return specFile{}, errors.New(cleaned + " does not exist; fix: name the spec by its path from the repository root (./le spec status lists them)")
	}
	if err != nil {
		return specFile{}, fmt.Errorf("read %s: %w", cleaned, err)
	}
	text := string(body)
	return specFile{relative: cleaned, stem: stem, text: text, sections: readrecord.Sections(text)}, nil
}

// section answers the spec's level-two section headed heading.
func (s specFile) section(heading string) (readrecord.Section, bool) {
	for _, section := range s.sections {
		if section.Level == 2 && section.Heading == heading {
			return section, true
		}
	}
	return readrecord.Section{}, false
}

// subsections answers the level-three sections inside parent, in order.
func (s specFile) subsections(parent readrecord.Section) []readrecord.Section {
	var inside []readrecord.Section
	for _, section := range s.sections {
		if section.Level != 3 {
			continue
		}
		if section.Start <= parent.Start {
			continue
		}
		if section.Start > parent.End {
			continue
		}
		inside = append(inside, section)
	}
	return inside
}

// bulletPath is the first back-quoted path of a list item.
var bulletPath = regexp.MustCompile("^\\s*-\\s+`([^`]+)`")

// modifiedPages answers the markdown pages the spec's Files to Modify bullets
// name: the pages the spec itself changes.
func (s specFile) modifiedPages() []string {
	section, found := s.section("Files to Modify")
	if !found {
		return nil
	}
	var pages []string
	for line := range strings.SplitSeq(section.Text, "\n") {
		match := bulletPath.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if strings.HasSuffix(match[1], ".md") {
			pages = append(pages, match[1])
		}
	}
	return pages
}

// tableRow is one data row of the spec's Design documents table.
type tableRow struct {
	// line is the row's 1-based line in the spec.
	line      int
	document  string
	section   string
	derived   string
	statement string
	verdict   string
	stands    string
}

// where names the row for a problem line.
func (r tableRow) where() string {
	return "row at line " + strconv.Itoa(r.line) + " (" + r.document + ", " + r.section + ")"
}

// documentsTable parses the Design documents table. It answers the rows and
// what is wrong with the table's shape; a spec with no section answers found
// false.
func (s specFile) documentsTable() (rows []tableRow, problems []string, found bool) {
	section, found := s.section(documentsHeading)
	if !found {
		return nil, nil, false
	}
	tableLines := 0
	for offset, line := range strings.Split(section.Text, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		tableLines++
		cells := tableCells(line)
		number := strconv.Itoa(section.Start + offset)
		if tableLines == 1 {
			if !slices.Equal(cells, documentsColumns) {
				problems = append(problems, "## "+documentsHeading+" line "+number+" has the columns "+
					strings.Join(cells, ", ")+"; fix: write the columns "+strings.Join(documentsColumns, ", "))
			}
			continue
		}
		if tableLines == 2 {
			continue
		}
		if len(cells) != len(documentsColumns) {
			problems = append(problems, "## "+documentsHeading+" line "+number+" has "+strconv.Itoa(len(cells))+
				" cells; fix: give the row the columns "+strings.Join(documentsColumns, ", "))
			continue
		}
		rows = append(rows, tableRow{
			line: section.Start + offset, document: strings.Trim(cells[0], "`"), section: cells[1],
			derived: cells[2], statement: cells[3], verdict: cells[4], stands: cells[5],
		})
	}
	return rows, problems, true
}

// tableCells splits one table line into its trimmed cells. A pipe inside back
// quotes or escaped as \| belongs to the cell. The loop is bounded by the
// line's length.
func tableCells(line string) []string {
	trimmed := strings.TrimPrefix(strings.TrimSpace(line), "|")
	var cells []string
	var cell strings.Builder
	inCode := false
	escaped := false
	for _, char := range trimmed {
		if escaped {
			cell.WriteRune(char)
			escaped = false
			continue
		}
		if char == '\\' {
			escaped = true
			cell.WriteRune(char)
			continue
		}
		if char == '`' {
			inCode = !inCode
		}
		if char != '|' {
			cell.WriteRune(char)
			continue
		}
		if inCode {
			cell.WriteRune(char)
			continue
		}
		cells = append(cells, strings.TrimSpace(cell.String()))
		cell.Reset()
	}
	if rest := strings.TrimSpace(cell.String()); rest != "" {
		cells = append(cells, rest)
	}
	return cells
}
