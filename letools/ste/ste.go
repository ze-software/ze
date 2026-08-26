// Design: docs/architecture/core-design.md -- the writing gate, as a command
// Detail: tables.go -- the six habits as data
// Detail: wordlist.go -- how a word list is scanned for
// Detail: pytext.go -- the Python string semantics every predicate is written against
// Detail: extract.go -- what counts as prose on each surface
// Detail: checks.go -- the detectors and the per-document review
// Detail: files.go -- which documents a review reads
// Detail: ratchet.go -- the gate: each changed file against its own HEAD version
// Detail: report.go -- the three answers, as payloads
// Detail: actions.go -- the three actions, as one command
//
// Package ste ports scripts/dev/ste_check.py. It reviews repository prose
// against ASD-STE100 Simplified Technical English, Issue 9.
//
// `ai/rules/writing.md` requires Simplified Technical English for Ze prose. It
// defines six habits that this package detects:
//
//   - Habit 1, synonym-rotation, detects multiple names for one concept (Rules 1.3, 1.11, 9.4).
//   - Habit 2, hedging, detects uncertain or qualified statements (Rule 1.1 and the dictionary).
//   - Habit 3, frozen-verbs, detects an action hidden in a noun (Rule 3.7).
//   - Habit 4, marketing-adjectives, detects praise without a measurement (Rule 1.1).
//   - Habit 5, run-ons, detects long sentences and semicolons (Rules 4.1, 5.1, 6.3, 6.6, 8.1).
//   - Habit 6, phrasal-verbs, detects multiple words for one action (Rule 9.3).
//
// This package is not a full STE conformance checker. Full conformance requires
// the controlled dictionary in part 2 of the standard. That dictionary contains
// approximately 900 approved words and is copyright (c) ASD 2025. This
// repository has no reproduction right, so tables.go contains only Ze's lists.
//
// A ratchet handles legacy prose. It compares each changed file with its OWN
// HEAD version and fails when a habit grows. ratchet.go explains this baseline.
package ste

import (
	"path"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The four writing surfaces INSIDE this repository. The website and the wiki
// are outside it: their copy is marketing, is written in the owner's voice, and
// keeps UK English.
const (
	SurfaceMarkdown = "markdown"
	SurfaceGo       = "go-comments"
	SurfaceYANG     = "yang-descriptions"
	SurfaceStdin    = "stdin"
)

// surfaces is the column order every count table prints.
var surfaces = []string{SurfaceMarkdown, SurfaceGo, SurfaceYANG, SurfaceStdin}

// habitNames is the six habits by number, which is the order every report
// groups by.
var habitNames = []string{
	"synonym-rotation",
	"hedging",
	"frozen-verbs",
	"marketing-adjectives",
	"run-ons",
	"phrasal-verbs",
}

// habitNumber answers the number a habit slug carries, which is what a reader
// looks up in `ai/rules/writing.md`. It answers 0 for a slug no habit has,
// which no caller in this package can produce: every detector names one of the
// six above.
func habitNumber(slug string) int {
	for index, name := range habitNames {
		if name == slug {
			return index + 1
		}
	}
	return 0
}

// surfaceOf answers the surface a file suffix is reviewed as, and whether the
// suffix has one at all.
func surfaceOf(name string) (string, bool) {
	switch path.Ext(name) {
	case ".md":
		return SurfaceMarkdown, true
	case ".go":
		return SurfaceGo, true
	case ".yang":
		return SurfaceYANG, true
	default:
		return "", false
	}
}

// defaultGlobs is every writing surface the whole-tree review reads.
var defaultGlobs = []string{
	"*.md",
	"docs/**/*.md",
	"ai/**/*.md",
	"plan/**/*.md",
	"internal/**/*.go",
	"cmd/**/*.go",
	"pkg/**/*.go",
	"internal/**/*.yang",
}

// excludeDirs contains trees that reviews ignore. `rfc/` has external normative
// text that must remain verbatim. Other entries are vendor, generated, or
// scratch trees.
//
// `ai/rules/points/` contains source fragments for rendered rule files. Reviewing
// both copies counts every sentence twice. Of the first 2417 ratchet lines after
// points appeared, 951 repeated rendered-rule findings. The rendered rules stay
// in scope, so the review still reads every sentence.
//
// Deferral and known-failure shards leave the tree when their rows resolve, like
// the specs in excludeGlobs.
var excludeDirs = []string{
	"rfc/",
	"tmp/",
	"vendor/",
	"third_party/",
	"gokrazy/",
	".git/",
	"backups/",
	"ai/rules/points/",
	"plan/deferrals/",
	"plan/known-failures/",
}

// excludeGlobs contains documents that closure DELETES. An STE edit there has no
// lasting value (owner directive, 2026-08-10). A spec exists for one work item
// and `git rm` removes it in commit B. Durable `plan/journal/`,
// `plan/learned/`, and `plan/TEMPLATE.md` files stay in scope because later
// sessions read them.
var excludeGlobs = []string{"plan/spec-*.md"}

// generatedMarkers identify a file whose prose belongs to its producer.
// Detected by marker, so a new generated document needs no wiring.
//
// "DO NOT EDIT" is deliberately NOT a marker. `ai/INSTRUCTIONS.md` opens with
// the banner it writes INTO its generated outputs, so that string skipped the
// one document every session reads.
var generatedMarkers = []string{"GENERATED by", "<!-- generated", "Code generated"}

// ignoreFileRe is the whole-document opt-out, for a document that must quote
// non-STE text at length. The reason is mandatory: an opt-out with no reason is
// a silent exemption, and those accumulate.
//
// Anchored to the start of a line, because a document that DOCUMENTS the escape
// hatch must not exempt itself. The writing rule names the marker in its own
// Enforcement section, and an unanchored pattern switched that rule file off.
var ignoreFileRe = mustPattern(`(?m)^{SP}*(?:<!--|//|#){SP}*ste:{SP}*ignore-file{SP}+(?P<reason>.+?){SP}*(?:-->|$)`)

// ignoreLineRe skips the line that FOLLOWS it.
var ignoreLineRe = mustPattern(`(?m)^{SP}*(?:<!--|//|#){SP}*ste:{SP}*ignore{SP}*(?:-->|$)`)

// Finding is one habit found at one place.
type Finding struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Surface string `json:"surface"`
	Habit   string `json:"habit"`
	Number  int    `json:"habit-number"`
	Detail  string `json:"detail"`
	Fix     string `json:"fix"`
	Excerpt string `json:"excerpt"`
}

// String renders one finding the way the report prints it.
func (f Finding) String() string {
	var tb textbuf.Buffer
	return tb.Str(f.File).Byte(':').Int(int64(f.Line)).Str(": [").Str(f.Habit).
		Str("] ").Str(f.Detail).Str(" -> ").Str(f.Fix).String()
}

// unit is a prose run and its starting line. It can be a paragraph, list item,
// table cell, comment block, or YANG description. Table cells remain separate
// because a wide row is a table, not a run-on sentence.
type unit struct {
	Text string
	Line int
	// Procedural marks a numbered step, which STE Rule 5.1 holds to a shorter
	// sentence than a description.
	Procedural bool
	// Paragraph says that Rule 6.6's sentence cap applies. Table cells, headings,
	// and list items are not paragraphs. Limiting a reference-table cell with
	// eight short facts would encourage fewer, longer sentences.
	Paragraph bool
}

// excluded reports whether a repository-relative path is outside every review.
func excluded(rel string) bool {
	for _, dir := range excludeDirs {
		if strings.HasPrefix(rel, dir) {
			return true
		}
	}
	for _, pattern := range excludeGlobs {
		if ok, err := path.Match(pattern, rel); err == nil && ok {
			return true
		}
	}
	return false
}
