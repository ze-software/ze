// Design: docs/architecture/site-facts.md -- the committed-facts pattern, its second instance
// Related: file.go -- the history file carries the section list this table declares
// Related: days.go -- which commit measures each recorded day
//
// The section table of the size history. Every repository path falls in
// exactly one section, by the first rule that matches it, and a path no rule
// matches falls in the last section. The table is declared here once: the
// history file carries each section's identifier and label, so the page and
// its legend read the file and never a second list.

package sitehistory

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"

	reporewrite "github.com/ze-software/ze/internal/le/repo/rewrite"
)

// CountingRulesVersion names the line rule the measurement applies to every
// blob, and it is raised whenever that rule changes, so the rules identifier of
// a history measured under the old rule no longer matches the tool's.
//
// The rule (Transformation Path step 4 of
// plan/post-release/engineering/spec-activity-codebase-growth.md): a blob with
// a NUL byte among its first 8,000 bytes, offsets 0 to 7,999, is binary and
// counts as a file with zero lines, a Go file included. Any other blob counts
// one line per newline, plus one for an unterminated last line. A Go blob that
// is not binary is also split into code, blank and comment lines by the shared
// classifier, whose own version is reporewrite.GoLineRulesVersion.
const CountingRulesVersion = 1

// Section is one part of the repository the history measures.
type Section struct {
	// ID is the stable identifier the file keys a day's values by: lower case,
	// words joined by hyphens.
	ID string
	// Label is the name a reader of the page sees.
	Label string
	// GoSplit says the section's lines are also split into code, blank and
	// comment lines. Only the three Go sections carry it.
	GoSplit bool
}

// rule is one row of the section table: the section a path falls in when
// every constraint the row declares holds for it.
//
// A rule is data and not a function, so the rules identifier digests what
// decides a path's section rather than a description beside it.
type rule struct {
	// section is where a matching path falls.
	section Section
	// thirdParty says the path lies under a third-party tree named from the
	// repository root, as reporewrite.ThirdPartyPath decides. The trees are
	// declared there once, beside the Go inventory that takes the same rule.
	thirdParty bool
	// suffixes holds the endings a matching path has one of.
	suffixes []string
	// directories holds the root directories, each with its trailing slash, a
	// matching path lies under one of.
	directories []string
	// rootNames holds the names a matching file at the root has one of.
	rootNames []string
	// rootSuffixes holds the endings a matching file at the root has one of.
	rootSuffixes []string
}

// sectionTable is the ordered rule list and the section of every path no rule
// matches. The catch-all is a field of its own, so the table cannot lack one
// and cannot hold it anywhere but last.
type sectionTable struct {
	// rules is tried in order, and the first match wins.
	rules []rule
	// rest is the section of every path that no rule matches.
	rest Section
}

// sections is THE section table of plan/post-release/engineering/
// spec-activity-codebase-growth.md, "Sections", in its order. An edit here
// changes the rules identifier, which TestRulesIdentifierPinsTheSectionTable
// pins, so it is made on purpose and followed by a whole-history run.
var sections = sectionTable{
	rules: []rule{
		{section: Section{ID: "third-party-go", Label: "Third-party Go", GoSplit: true}, thirdParty: true, suffixes: []string{".go"}},
		{section: Section{ID: "third-party-other", Label: "Third-party other"}, thirdParty: true},
		{section: Section{ID: "go-tests", Label: "Go tests", GoSplit: true}, suffixes: []string{"_test.go"}},
		{section: Section{ID: "go-production", Label: "Go production", GoSplit: true}, suffixes: []string{".go"}},
		{section: Section{ID: "functional-tests", Label: "Functional tests"}, directories: []string{"test/"}},
		{section: Section{ID: "rfc-corpus", Label: "RFC corpus"}, directories: []string{"rfc/", "reference/"}},
		{section: Section{ID: "plans", Label: "Plans"}, directories: []string{"plan/"}},
		{section: Section{ID: "agent-rules", Label: "Agent rules"}, directories: []string{"ai/", ".claude/"}, rootNames: []string{"AGENTS.md", "CLAUDE.md", "GEMINI.md"}},
		{section: Section{ID: "documentation", Label: "Documentation"}, directories: []string{"docs/", "features/"}, rootSuffixes: []string{".md"}},
		{section: Section{ID: "website", Label: "Website"}, directories: []string{"website/", "demos/"}},
		{section: Section{ID: "schemas", Label: "Schemas"}, suffixes: []string{".yang"}},
	},
	rest: Section{ID: "other", Label: "Other"},
}

// Sections answers every section in table order, the catch-all last. The
// answer is a copy, so a caller cannot edit the table through it.
func Sections() []Section {
	out := make([]Section, 0, len(sections.rules)+1)
	for _, row := range sections.rules {
		out = append(out, row.section)
	}
	return append(out, sections.rest)
}

// SectionOf answers the section of one repository-relative slash path, as a
// git tree listing names it: the first rule that matches, or the catch-all.
func SectionOf(path string) Section {
	for _, row := range sections.rules {
		if row.matches(path) {
			return row.section
		}
	}
	return sections.rest
}

// matches reports whether every constraint the rule declares holds for path.
// A rule that declares no constraint matches nothing: only the table's rest
// field takes every path, so a row whose author forgot its constraint cannot
// swallow the rows after it.
func (r rule) matches(path string) bool {
	if r.thirdParty && !reporewrite.ThirdPartyPath(path) {
		return false
	}
	if len(r.suffixes) > 0 && !hasAnySuffix(path, r.suffixes) {
		return false
	}
	if r.placed() {
		return r.holds(path)
	}
	return r.thirdParty || len(r.suffixes) > 0
}

// placed reports whether the rule constrains where a path lies.
func (r rule) placed() bool {
	return len(r.directories)+len(r.rootNames)+len(r.rootSuffixes) > 0
}

// holds reports whether path lies under one of the rule's directories, or is a
// root file with one of its names or endings.
func (r rule) holds(path string) bool {
	for _, directory := range r.directories {
		if strings.HasPrefix(path, directory) {
			return true
		}
	}
	if strings.Contains(path, "/") {
		return false
	}
	if slices.Contains(r.rootNames, path) {
		return true
	}
	return hasAnySuffix(path, r.rootSuffixes)
}

// hasAnySuffix reports whether path ends with one of suffixes.
func hasAnySuffix(path string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

// RulesIdentifier answers the hex SHA-256 digest of the section table as
// declared, every rule's section and constraints in order and the catch-all
// last, of the third-party trees reporewrite.ThirdPartyTrees names, and of the
// two counting-rule versions. A history file records it, and
// a file whose identifier differs was measured under other rules.
func RulesIdentifier() string {
	digest := sha256.Sum256([]byte(rulesText()))
	return hex.EncodeToString(digest[:])
}

// rulesText is the canonical text RulesIdentifier digests: one line per rule,
// its fields tab-separated, then the catch-all, the third-party trees and the
// two versions.
func rulesText() string {
	var text strings.Builder
	for _, row := range sections.rules {
		writeSection(&text, row.section)
		text.WriteString("\tthird-party=")
		text.WriteString(strconv.FormatBool(row.thirdParty))
		writeList(&text, "suffixes", row.suffixes)
		writeList(&text, "directories", row.directories)
		writeList(&text, "root-names", row.rootNames)
		writeList(&text, "root-suffixes", row.rootSuffixes)
		text.WriteByte('\n')
	}
	writeSection(&text, sections.rest)
	text.WriteString("\teverything\n")
	// The third-party rows match through reporewrite.ThirdPartyPath, so the
	// trees it names decide those rows as much as the table does.
	writeList(&text, "third-party-trees", reporewrite.ThirdPartyTrees())
	text.WriteString("\ncounting-rules-version=")
	text.WriteString(strconv.Itoa(CountingRulesVersion))
	text.WriteString("\ngo-line-rules-version=")
	text.WriteString(strconv.Itoa(reporewrite.GoLineRulesVersion))
	text.WriteByte('\n')
	return text.String()
}

// writeSection writes a section's three fields for rulesText.
func writeSection(text *strings.Builder, section Section) {
	text.WriteString(section.ID)
	text.WriteByte('\t')
	text.WriteString(section.Label)
	text.WriteString("\tgo-split=")
	text.WriteString(strconv.FormatBool(section.GoSplit))
}

// writeList writes one named list of a rule for rulesText, its items joined
// by commas, so an item moved between lists changes the text.
func writeList(text *strings.Builder, name string, items []string) {
	text.WriteByte('\t')
	text.WriteString(name)
	text.WriteByte('=')
	text.WriteString(strings.Join(items, ","))
}
