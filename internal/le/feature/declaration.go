// Design: docs/contributing/feature-maturity.md -- one declaration per feature, features/<id>.md
// Related: vocabulary.go -- the values a Meta cell may hold
// Related: check.go -- the evidence each parsed field points at
//
// The ONE parser of features/*.md. The check, the report, the rendered
// docs/features.md and the site all read the values this file produces, so no
// two consumers can read one declaration and disagree about what it says.

package feature

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// declarationDir is the directory every declaration lives in, relative to the
// checkout root. Listing it is the registration: a new feature is one new file.
const declarationDir = "features"

// Field names of the `## Meta` table, as a declaration spells them.
const (
	fieldName          = "Name"
	fieldPage          = "Page"
	fieldKind          = "Kind"
	fieldScope         = "Scope"
	fieldScopeGaps     = "Scope gaps"
	fieldLevel         = "Level"
	fieldParts         = "Parts"
	fieldComponents    = "Components"
	fieldRealPathTests = "Real-path tests"
	fieldInterop       = "Interop"
	fieldRFCs          = "RFCs"
	fieldDocs          = "Docs"
	fieldDocReview     = "Doc review"
	fieldDefectReview  = "Defect review"
	fieldStubEvidence  = "Stub evidence"
)

// knownFields is every field a Meta table may carry. A row naming anything else
// is refused by name, because a misspelled field is otherwise a silently
// absent one.
var knownFields = []string{
	fieldName, fieldPage, fieldKind, fieldScope, fieldScopeGaps, fieldLevel,
	fieldParts, fieldComponents, fieldRealPathTests, fieldInterop, fieldRFCs,
	fieldDocs, fieldDocReview, fieldDefectReview, fieldStubEvidence,
}

// listSeparator separates the items of a list-valued cell.
const listSeparator = ", "

// goTestSeparator splits a Go test item into its file and its function, the
// same `file::Func` spelling a discrimination record's unit uses.
const goTestSeparator = "::"

// Declaration is one parsed features/<id>.md. Every value has passed the
// vocabulary and shape checks of Parse; whether the paths it names exist is
// the check's question, answered against a tree.
type Declaration struct {
	ID            string
	Source        string // repository-relative path of the declaration
	Name          string
	Page          string
	Kind          Kind
	Scope         Scope
	ScopeGaps     []string
	Level         Level
	Parts         []string
	Components    []string
	RealPathTests []string
	Interop       []string
	RFCs          []string
	Docs          []string
	DocReview     Attestation
	DefectReview  Attestation
	StubEvidence  []string
	Description   string
}

// Attestation is a dated reader's judgement: the date it was made, and what was
// judged. A date with nothing judged is refused, so a re-dated stamp cannot
// pass for a review (spec R-2).
type Attestation struct {
	Date   time.Time
	Judged string
}

// Present answers whether the declaration carries the attestation at all.
func (a Attestation) Present() bool { return !a.Date.IsZero() }

// attestationLayout is the date form an attestation opens with.
const attestationLayout = "2006-01-02"

// Load parses every declaration under the tree, in id order.
//
// A malformed declaration is collected, not returned on its own: the check
// reports every refusal in one run. The error return is for a tree that cannot
// be read at all.
func Load(tree string) ([]Declaration, []error, error) {
	entries, err := os.ReadDir(filepath.Join(tree, declarationDir))
	if err != nil {
		return nil, nil, err
	}
	var declarations []Declaration
	var problems []error
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		rel := filepath.ToSlash(filepath.Join(declarationDir, entry.Name()))
		text, readErr := os.ReadFile(filepath.Join(tree, rel)) //nolint:gosec // rel is an entry of features/, listed above
		if readErr != nil {
			return nil, nil, readErr
		}
		declaration, parseErr := Parse(string(text), strings.TrimSuffix(entry.Name(), ".md"), rel)
		if parseErr != nil {
			problems = append(problems, parseErr)
			continue
		}
		declarations = append(declarations, declaration)
	}
	slices.SortFunc(declarations, func(a, b Declaration) int { //nolint:gocritic // hugeParam: SortFunc fixes the comparator's value signature
		return strings.Compare(a.ID, b.ID)
	})
	return declarations, problems, nil
}

// Parse reads one declaration. where names it in every refusal.
func Parse(text, id, where string) (Declaration, error) {
	cells, err := metaCells(text, where)
	if err != nil {
		return Declaration{}, err
	}
	declaration := Declaration{ID: id, Source: where}
	if err := declaration.admit(cells, where); err != nil {
		return Declaration{}, err
	}
	description, err := descriptionOf(text, where)
	if err != nil {
		return Declaration{}, err
	}
	declaration.Description = description
	if err := declaration.requireFields(where); err != nil {
		return Declaration{}, err
	}
	return declaration, nil
}

// admit converts each cell into its typed field.
func (d *Declaration) admit(cells map[string]string, where string) error {
	var err error
	d.Name = cells[fieldName]
	d.Page = cells[fieldPage]
	if d.Kind, err = vocabularyValue(kindNames, cells, fieldKind, where); err != nil {
		return err
	}
	if d.Scope, err = vocabularyValue(scopeNames, cells, fieldScope, where); err != nil {
		return err
	}
	if d.Level, err = vocabularyValue(levelNames, cells, fieldLevel, where); err != nil {
		return err
	}
	d.ScopeGaps = list(cells[fieldScopeGaps])
	d.Parts = list(cells[fieldParts])
	d.Components = list(cells[fieldComponents])
	d.RealPathTests = list(cells[fieldRealPathTests])
	d.Interop = list(cells[fieldInterop])
	d.RFCs = list(cells[fieldRFCs])
	d.Docs = list(cells[fieldDocs])
	d.StubEvidence = list(cells[fieldStubEvidence])
	if d.DocReview, err = attestation(cells, fieldDocReview, where); err != nil {
		return err
	}
	d.DefectReview, err = attestation(cells, fieldDefectReview, where)
	return err
}

// requireFields refuses a declaration missing a field its Kind and Scope
// require, or carrying one they forbid (AC-1).
func (d *Declaration) requireFields(where string) error {
	if d.Name == "" {
		return refusal(where, fieldName, "is required")
	}
	if d.Kind == KindUnspecified {
		return refusal(where, fieldKind, "is required")
	}
	if d.Scope == ScopeUnspecified {
		return refusal(where, fieldScope, "is required")
	}
	if d.Scope.Implemented() {
		if d.Level == LevelUnspecified {
			return refusal(where, fieldLevel, "is required when Scope is complete or partial")
		}
	}
	if !d.Scope.Implemented() {
		if d.Level != LevelUnspecified {
			return refusal(where, fieldLevel, "must be absent when Scope is future or rejected")
		}
	}
	if d.Scope == ScopePartial {
		if len(d.ScopeGaps) == 0 {
			return refusal(where, fieldScopeGaps, "is required when Scope is partial")
		}
	}
	if d.Scope != ScopePartial {
		if len(d.ScopeGaps) != 0 {
			return refusal(where, fieldScopeGaps, "is allowed only when Scope is partial")
		}
	}
	if d.Kind == KindUmbrella {
		if len(d.Parts) == 0 {
			return refusal(where, fieldParts, "is required when Kind is umbrella")
		}
		if len(d.Components) != 0 {
			return refusal(where, fieldComponents, "is not allowed when Kind is umbrella: an umbrella's evidence is its Parts")
		}
		return nil
	}
	if len(d.Parts) != 0 {
		return refusal(where, fieldParts, "is allowed only when Kind is umbrella")
	}
	if d.Scope.Implemented() {
		if len(d.Components) == 0 {
			return refusal(where, fieldComponents, "is required for an implemented feature that is not an umbrella")
		}
	}
	return nil
}

// metaCells reads the `## Meta` table into field -> value, refusing an unknown
// or repeated field by name.
func metaCells(text, where string) (map[string]string, error) {
	section, found := sectionBody(text, "## Meta")
	if !found {
		return nil, errors.New(where + ": no `## Meta` section")
	}
	cells := map[string]string{}
	for line := range strings.SplitSeq(section, "\n") {
		field, value, ok := tableRow(line)
		if !ok {
			continue
		}
		if field == "Field" {
			continue
		}
		if !slices.Contains(knownFields, field) {
			var tb textbuf.Buffer
			return nil, errors.New(tb.Str(where).Str(": unknown Meta field '").Str(field).
				Str("'; the fields are: ").Str(strings.Join(knownFields, ", ")).String())
		}
		if _, repeated := cells[field]; repeated {
			return nil, refusal(where, field, "appears twice")
		}
		cells[field] = value
	}
	return cells, nil
}

// tableRow splits `| a | b |` into its two cells. A separator row and any line
// that is not a two-cell row answer false.
func tableRow(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "|") {
		return "", "", false
	}
	if !strings.HasSuffix(line, "|") {
		return "", "", false
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	field, value, ok := strings.Cut(inner, "|")
	if !ok {
		return "", "", false
	}
	field = strings.TrimSpace(field)
	if strings.Trim(field, "-: ") == "" {
		return "", "", false
	}
	value = strings.TrimSpace(value)
	if value == "-" {
		value = ""
	}
	return field, value, true
}

// descriptionOf answers the `## Description` prose: the public row text,
// one paragraph, with no unescaped table pipe.
func descriptionOf(text, where string) (string, error) {
	section, found := sectionBody(text, "## Description")
	if !found {
		return "", errors.New(where + ": no `## Description` section")
	}
	description := strings.Join(strings.Fields(section), " ")
	if description == "" {
		return "", errors.New(where + ": the Description section is empty")
	}
	if unescapedPipe(description) {
		return "", errors.New(where + ": the Description holds an unescaped '|', which would split the rendered table row; write '\\|'")
	}
	return description, nil
}

func unescapedPipe(text string) bool {
	for i := range len(text) {
		if text[i] != '|' {
			continue
		}
		if i == 0 {
			return true
		}
		if text[i-1] != '\\' {
			return true
		}
	}
	return false
}

// sectionBody answers the lines after heading up to the next `## ` heading.
func sectionBody(text, heading string) (string, bool) {
	start := -1
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == heading {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return "", false
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n"), true
}

func vocabularyValue[T comparable](table []namedValue[T], cells map[string]string, field, where string) (T, error) {
	var zero T
	cell := cells[field]
	if cell == "" {
		return zero, nil
	}
	value, ok := lookup(table, cell)
	if !ok {
		var tb textbuf.Buffer
		return zero, errors.New(tb.Str(where).Str(": ").Str(field).Str(" '").Str(cell).
			Str("' is not one of: ").Str(strings.Join(names(table), ", ")).String())
	}
	return value, nil
}

// attestation reads `YYYY-MM-DD: what was judged`.
func attestation(cells map[string]string, field, where string) (Attestation, error) {
	cell := cells[field]
	if cell == "" {
		return Attestation{}, nil
	}
	date, judged, _ := strings.Cut(cell, ":")
	parsed, err := time.Parse(attestationLayout, strings.TrimSpace(date))
	if err != nil {
		return Attestation{}, refusal(where, field, "must open with a YYYY-MM-DD date")
	}
	judged = strings.TrimSpace(judged)
	if judged == "" {
		return Attestation{}, refusal(where, field, "names no claim judged after its date; a date alone attests nothing")
	}
	return Attestation{Date: parsed, Judged: judged}, nil
}

func list(cell string) []string {
	if cell == "" {
		return nil
	}
	items := strings.Split(cell, listSeparator)
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.Trim(strings.TrimSpace(item), "`")
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func refusal(where, field, reason string) error {
	var tb textbuf.Buffer
	return errors.New(tb.Str(where).Str(": ").Str(field).Byte(' ').Str(reason).String())
}
