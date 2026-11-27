// Design: docs/architecture/core-design.md -- advisory readability scoring for prose
// Overview: ste.go -- the writing checker this report belongs to
//
// Readability rates English simplicity for blog posts and documentation using
// pure-Go heuristics (Flesch-Kincaid grade, Flesch reading ease, sentence word
// counts, and comma/subordinator clause density) after stripping YAML front
// matter, fenced code blocks, and inline code spans. It is strictly advisory:
// it exits 0 on valid files so writers can spot dense sentences without forcing
// robotic short prose.
package docste

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

var markdownLinkRe = regexp.MustCompile(`\[([^\]]+)\]\([^)\n]+\)`)

const (
	// readabilityLongWords flags sentences that ask a reader to hold too many
	// words in one breath.
	readabilityLongWords = 30
	// readabilityNestedMinWords avoids flagging short lists such as
	// "Run A, B, C, or D." as deeply nested prose.
	readabilityNestedMinWords = 20
	// readabilityMaxPunctBreaks flags sentences with three or more internal
	// comma/semicolon clause breaks alongside readabilityNestedMinWords.
	readabilityMaxPunctBreaks = 3
	// readabilityMaxSubordinators flags sentences with two or more clause
	// joiners alongside at least two commas and readabilityNestedMinWords.
	readabilityMaxSubordinators = 2

	defaultReadabilityGlob = "website/blog/posts/*.md"
)

// clauseJoiners are subordinating and relative words that commonly signal
// nested clauses when stacked inside a long sentence.
var clauseJoiners = map[string]struct{}{
	"although": {},
	"because":  {},
	"however":  {},
	"since":    {},
	"that":     {},
	"though":   {},
	"unless":   {},
	"whereas":  {},
	"whether":  {},
	"which":    {},
	"while":    {},
	"whilst":   {},
}

// ReadabilityFlag records one dense or overloaded sentence in a document.
type ReadabilityFlag struct {
	File          string `json:"file"`
	Line          int    `json:"line"`
	Reason        string `json:"reason"`
	Words         int    `json:"words"`
	ClauseBreaks  int    `json:"clause-breaks"`
	Subordinators int    `json:"subordinators"`
	Excerpt       string `json:"excerpt"`
}

// ReadabilityDocument holds readability metrics for one prose document.
type ReadabilityDocument struct {
	File                string            `json:"file"`
	Sentences           int               `json:"sentences"`
	Words               int               `json:"words"`
	AvgSentenceWords    float64           `json:"avg-sentence-words"`
	AvgSyllablesPerWord float64           `json:"avg-syllables-per-word"`
	FleschKincaidGrade  float64           `json:"flesch-kincaid-grade"`
	FleschReadingEase   float64           `json:"flesch-reading-ease"`
	Rating              string            `json:"rating"`
	Flagged             []ReadabilityFlag `json:"flagged"`
}

// ReadabilityReport is the answer of `le doc ste readability`.
type ReadabilityReport struct {
	DocumentsReviewed int                   `json:"documents-reviewed"`
	TotalFlagged      int                   `json:"total-flagged"`
	Documents         []ReadabilityDocument `json:"documents"`
}

// Text renders the advisory readability report for a person.
func (r ReadabilityReport) Text() string {
	var tb textbuf.Buffer
	for _, doc := range r.Documents {
		tb.Str(doc.File).Str(": ").Str(doc.Rating).
			Str(" (grade ").Str(formatOneDecimal(doc.FleschKincaidGrade)).
			Str(", ease ").Str(formatOneDecimal(doc.FleschReadingEase)).
			Str(", avg ").Str(formatOneDecimal(doc.AvgSentenceWords)).
			Str(" words/sentence, ").Int(int64(doc.Sentences)).
			Str(" sentences, ").Int(int64(len(doc.Flagged))).Str(" flagged)\n")
		for _, flag := range doc.Flagged {
			tb.Str("  ").Str(flag.File).Byte(':').Int(int64(flag.Line)).
				Str(": [").Str(flag.Reason).Str("] ").
				Int(int64(flag.Words)).Str(" words, ").
				Int(int64(flag.ClauseBreaks)).Str(" breaks, ").
				Int(int64(flag.Subordinators)).Str(" joiners -- ").
				Str(flag.Excerpt).Byte('\n')
		}
	}
	tb.Str("\nste readability: ").Int(int64(r.TotalFlagged)).
		Str(" dense sentence(s) flagged in ").Int(int64(r.DocumentsReviewed)).
		Str(" document(s) (advisory)\n")
	return tb.String()
}

// readabilityNamed runs the advisory readability report on named files, or on
// website/blog/posts/*.md when no file is named.
func readabilityNamed(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err,
			"cannot locate the Ze repository root",
			"readability resolves relative paths and the default blog post glob against the checkout root",
			"cd into the Ze checkout and re-run ./le doc ste readability"))
		return nil, 1
	}
	named := args.Values(fileKeyword)
	var files []string
	if len(named) > 0 {
		for _, raw := range named {
			files = append(files, filepath.ToSlash(raw))
		}
	} else {
		matches, globErr := filepath.Glob(filepath.Join(root, filepath.FromSlash(defaultReadabilityGlob)))
		if globErr != nil {
			leaction.ReportError(opererr.Wrap(globErr,
				"cannot list default blog posts under "+defaultReadabilityGlob,
				"the default blog post glob failed while resolving files to score",
				"./le doc ste readability file <path>"))
			return nil, 1
		}
		for _, m := range matches {
			rel, relErr := filepath.Rel(root, m)
			if relErr != nil {
				continue
			}
			files = append(files, filepath.ToSlash(rel))
		}
		slices.Sort(files)
	}

	docs := make([]ReadabilityDocument, 0, len(files))
	totalFlagged := 0
	for _, rel := range files {
		fullPath := rel
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(root, filepath.FromSlash(rel))
		}
		body, readErr := os.ReadFile(fullPath) // #nosec G304 -- explicit user path or blog glob inside checkout
		if readErr != nil {
			var tb textbuf.Buffer
			problem := tb.Str("cannot read ").Str(rel).String()
			leaction.ReportError(opererr.Wrap(readErr,
				problem,
				"readability needs an existing readable Markdown file to score",
				"check the file path and re-run ./le doc ste readability file "+rel))
			return nil, 1
		}
		doc := AnalyzeReadability(rel, string(body))
		totalFlagged += len(doc.Flagged)
		docs = append(docs, doc)
	}
	return ReadabilityReport{
		DocumentsReviewed: len(docs),
		TotalFlagged:      totalFlagged,
		Documents:         docs,
	}, 0
}

// AnalyzeReadability computes advisory English simplicity metrics and dense
// sentence flags for a Markdown document.
func AnalyzeReadability(path, content string) ReadabilityDocument {
	text := blankYAMLFrontMatter(content)
	text = blankHTMLComments(text)
	text = markdownLinkRe.ReplaceAllString(text, "$1")
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	units := unitsMarkdown(lines)

	var (
		sentences int
		words     int
		syllables int
		flagged   []ReadabilityFlag
	)

	for _, u := range units {
		if !u.Paragraph {
			continue
		}
		for _, rawSent := range Sentences(u.Text) {
			sent := strings.TrimSpace(rawSent)
			if sent == "" {
				continue
			}
			wCount, sylCount, breaks, joiners := measureSentence(sent)
			if wCount == 0 {
				continue
			}
			sentences++
			words += wCount
			syllables += sylCount

			reason := flagReason(wCount, breaks, joiners)
			if reason != "" {
				flagged = append(flagged, ReadabilityFlag{
					File:          path,
					Line:          u.Line,
					Reason:        reason,
					Words:         wCount,
					ClauseBreaks:  breaks,
					Subordinators: joiners,
					Excerpt:       runeHead(sent, excerptRunes),
				})
			}
		}
	}

	if flagged == nil {
		flagged = []ReadabilityFlag{}
	}

	doc := ReadabilityDocument{
		File:      path,
		Sentences: sentences,
		Words:     words,
		Rating:    "simple",
		Flagged:   flagged,
	}
	if sentences == 0 {
		return doc
	}
	if words == 0 {
		return doc
	}

	avgWords := float64(words) / float64(sentences)
	avgSyllables := float64(syllables) / float64(words)
	grade := 0.39*avgWords + 11.8*avgSyllables - 15.59
	if grade < 0 {
		grade = 0
	}
	ease := 206.835 - 1.015*avgWords - 84.6*avgSyllables
	if ease < 0 {
		ease = 0
	}
	if ease > 100 {
		ease = 100
	}

	doc.AvgSentenceWords = roundOneDecimal(avgWords)
	doc.AvgSyllablesPerWord = roundTwoDecimals(avgSyllables)
	doc.FleschKincaidGrade = roundOneDecimal(grade)
	doc.FleschReadingEase = roundOneDecimal(ease)
	doc.Rating = readabilityRating(doc.FleschKincaidGrade)
	return doc
}

// blankYAMLFrontMatter replaces leading `--- ... ---` front matter with empty
// lines so line numbers of body paragraphs remain exact.
func blankYAMLFrontMatter(content string) string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) < 3 {
		return normalized
	}
	if strings.TrimSpace(lines[0]) != "---" {
		return normalized
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return normalized
	}
	for i := 0; i <= end; i++ {
		lines[i] = ""
	}
	return strings.Join(lines, "\n")
}

func measureSentence(sentence string) (words, syllables, breaks, joiners int) {
	breaks = strings.Count(sentence, ",") + strings.Count(sentence, ";")
	for _, raw := range strings.Fields(sentence) {
		if raw == "CODE" {
			words++
			syllables++
			continue
		}
		if raw == "URL" {
			words++
			syllables++
			continue
		}
		cleaned := cleanAlphaToken(raw)
		if cleaned == "" {
			if strings.ContainsFunc(raw, isWordLike) {
				words++
				syllables++
			}
			continue
		}
		words++
		syllables += countSyllables(cleaned)
		if _, ok := clauseJoiners[cleaned]; ok {
			joiners++
		}
	}
	return words, syllables, breaks, joiners
}

func flagReason(words, breaks, joiners int) string {
	isLong := words > readabilityLongWords
	isNested := words >= readabilityNestedMinWords &&
		(breaks >= readabilityMaxPunctBreaks || (joiners >= readabilityMaxSubordinators && breaks >= 2))
	switch {
	case isLong && isNested:
		return "long-and-nested"
	case isLong:
		return "long-sentence"
	case isNested:
		return "nested-clauses"
	default:
		return ""
	}
}

func readabilityRating(grade float64) string {
	switch {
	case grade <= 9.0:
		return "simple"
	case grade <= 11.5:
		return "clear"
	case grade <= 14.0:
		return "dense"
	default:
		return "complex"
	}
}

func cleanAlphaToken(raw string) string {
	var tb textbuf.Buffer
	for i := range len(raw) {
		c := raw[i]
		if c >= 'A' && c <= 'Z' {
			tb.Byte(c + ('a' - 'A'))
			continue
		}
		if c >= 'a' && c <= 'z' {
			tb.Byte(c)
		}
	}
	return tb.String()
}

// countSyllables estimates English syllables for a lowercase ASCII word.
func countSyllables(word string) int {
	if len(word) <= 3 {
		return 1
	}
	w := word
	// Trim common silent endings while preserving words like "table" or "simple".
	if strings.HasSuffix(w, "e") && !strings.HasSuffix(w, "le") {
		w = w[:len(w)-1]
	} else if strings.HasSuffix(w, "le") && len(w) > 2 && isVowel(w[len(w)-3]) {
		w = w[:len(w)-1]
	}
	if strings.HasSuffix(w, "ed") && len(w) > 3 {
		prev := w[len(w)-3]
		if prev != 't' && prev != 'd' {
			w = w[:len(w)-2]
		}
	}

	count := 0
	inVowel := false
	for i := range len(w) {
		v := isVowel(w[i])
		if v && !inVowel {
			count++
		}
		inVowel = v
	}
	// Common English hiatus pairs (e.g. "di-ag-nos-tics", "pri-or-i-ty") split
	// across two syllables, except "-tion"/"-sion" which pronounce "io" as one.
	for i := 0; i+1 < len(w); i++ {
		switch w[i : i+2] {
		case "ia", "eo", "ua":
			count++
		case "io":
			if i == 0 || (w[i-1] != 't' && w[i-1] != 's') {
				count++
			}
		}
	}
	if count < 1 {
		return 1
	}
	return count
}

func isVowel(c byte) bool {
	switch c {
	case 'a', 'e', 'i', 'o', 'u', 'y':
		return true
	default:
		return false
	}
}

func roundOneDecimal(v float64) float64 {
	return math.Round(v*10) / 10
}

func roundTwoDecimals(v float64) float64 {
	return math.Round(v*100) / 100
}

func formatOneDecimal(v float64) string {
	return strconv.FormatFloat(v, 'f', 1, 64)
}
