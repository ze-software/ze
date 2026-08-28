// Design: docs/architecture/core-design.md -- the greppable map of the rule set
// Overview: rules.go -- the corpus predicate this index names
// Overview: actions.go -- the two actions that run this
//
// index.go ports `rules_index.py`. It generates one `ai/rules/INDEX.md` line for
// each rule. The line contains the rule title, its trigger, and its severity.
//
// Each summary comes from the rule. Selection uses this priority: an explicit
// `**When:**` line, a `**BLOCKING:**` directive, then the first prose paragraph.
// The check names a rule with no derivable summary. Thus, every new rule must
// have a discoverable trigger.
//
// It shares only the corpus predicate with this package. The digest reads rule
// metadata and body text, while the index reads paragraphs. The two parses
// answer different questions about one file.

package rules

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// maxSummary is the width one index row's summary is cut to at a word
// boundary. The path on the same line makes the full clause one read away.
const maxSummary = 200

// indexFile is the artifact this generator owns.
const indexFile = "INDEX.md"

// noSummary is what a rule with no derivable trigger renders as. The row still
// exists: dropping it would hide exactly the rules that are hardest to route.
const noSummary = "_(no summary -- add a `**When:**` line)_"

// indexPointers are the bare cross-reference openers that end a paragraph and
// are never summary material.
var indexPointers = [...]string{wordRationale, wordPrinciple, "structural template"}

// indexProseOpeners are markup prefixes for non-prose paragraphs. They include
// directives, tables, quotes, lists, links, and comments.
var indexProseOpeners = [...]string{"**", "|", ">", "-", "*", "[", "<!--"}

// indexProseWords are lowercase prefixes that cannot open a prose paragraph.
// This uses a plain prefix instead of a word boundary, unlike indexPointers.
var indexProseWords = [...]string{wordRationale, wordPrinciple, "structural template", "see "}

// IndexRow is one line of the generated index.
type IndexRow struct {
	Rule     string `json:"rule"`
	Summary  string `json:"summary"`
	Severity string `json:"severity"`
	File     string `json:"file"`
}

// IndexReport is what the two index actions answer.
type IndexReport struct {
	File    string     `json:"file"`
	Rules   int        `json:"rules"`
	Written bool       `json:"written"`
	Stale   bool       `json:"stale"`
	Missing []string   `json:"missing"`
	Rows    []IndexRow `json:"rows"`
}

// Failed reports whether the check found a stale index or a rule with no
// derivable summary. The update action answers 0 whatever it found, because it
// has just written the answer.
func (r IndexReport) Failed() bool {
	return !r.Written && (r.Stale || len(r.Missing) > 0)
}

// Text renders the verdict in the words the script printed.
func (r IndexReport) Text() string {
	var tb textbuf.Buffer
	if r.Written {
		tb.Str("wrote ").Str(r.File).Str(" (").Int(int64(r.Rules)).Str(" rules)\n")
		if len(r.Missing) > 0 {
			tb.Str("WARNING: ").Int(int64(len(r.Missing))).
				Str(" rule(s) lack a derivable summary -- add a `**When:**` line to: ").
				Join(r.Missing, ", ").Byte('\n')
		}
		return tb.String()
	}
	if !r.Failed() {
		return tb.Str("checked ").Int(int64(r.Rules)).Str(" rules, ").Str(r.File).
			Str(" up to date\n").String()
	}
	if len(r.Missing) > 0 {
		tb.Str("WARNING: rules missing a derivable summary (add a `**When:**` line): ").
			Join(r.Missing, ", ").Byte('\n')
	}
	if r.Stale {
		tb.Str("WARNING: ").Str(r.File).Str(" is stale -- run: ./le rules index-update\n")
	}
	return tb.String()
}

// Index builds the rule index and either compares it against the tree or writes
// it.
func Index(tree string, check bool) (IndexReport, error) {
	report := IndexReport{File: artifactPath(indexFile), Written: !check}
	dir := rulesDir(tree)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		var tb textbuf.Buffer
		return report, errors.New(tb.Str(rulesRel).Str(" not found").String())
	}

	rows, missing, err := indexRows(dir)
	if err != nil {
		return report, err
	}
	report.Rows = rows
	report.Rules = len(rows)
	report.Missing = missing

	content := renderIndex(rows)
	target := filepath.Join(dir, indexFile)
	if check {
		current, readErr := os.ReadFile(target) // #nosec G304 -- a path derived from the checkout
		report.Stale = readErr != nil || string(current) != content
		// A missing index is STALE, not an error. The reader takes the same action
		// as for drift: regenerate it.
		return report, nil //nolint:nilerr // an unreadable index is the stale verdict, not a failure to reach one
	}
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		return report, err
	}
	return report, nil
}

// indexRows answers one row for each rule. It also answers the rules that have
// no derivable summary.
func indexRows(rulesDir string) ([]IndexRow, []string, error) {
	files, err := ruleFiles(rulesDir)
	if err != nil {
		return nil, nil, err
	}
	if len(files) == 0 {
		var tb textbuf.Buffer
		return nil, nil, errors.New(tb.Str("no rule file under ").Str(rulesRel).
			Str("/; the index read nothing and must not report success").String())
	}

	rows := make([]IndexRow, 0, len(files))
	missing := []string{}
	for _, path := range files {
		raw, err := os.ReadFile(path) // #nosec G304 -- a path this tool derived from the checkout
		if err != nil {
			return nil, nil, err
		}
		name := filepath.Base(path)
		lines := splitLines(string(raw))
		title := indexTitle(lines, strings.TrimSuffix(name, ".md"))
		summary := indexSummary(lines)
		if summary == "" {
			missing = append(missing, name)
			summary = noSummary
		}
		severity := "-"
		if value := indexMeta(lines)["Severity"]; value != "" {
			severity = value
		}
		var tb textbuf.Buffer
		rows = append(rows, IndexRow{
			Rule:     title,
			Summary:  summary,
			Severity: severity,
			File:     tb.Byte('`').Str(rulesRel).Byte('/').Str(name).Byte('`').String(),
		})
	}
	return rows, missing, nil
}

// indexHeader is INDEX.md above its first row. Its producer instruction names
// the native action that renders these bytes.
var indexHeader = []string{
	"# Ze Rules Index",
	"",
	"<!-- GENERATED by internal/le/rules/index.go -- do not edit -->",
	"<!-- Regenerate: ./le rules index-update -->",
	"",
	"One-line overview of every rule under `ai/rules/`. Read the listed file in",
	"full before acting on a topic it covers.",
	"",
	"| Rule | When to read | Severity | File |",
	"|------|--------------|----------|------|",
}

// renderIndex answers the whole file.
func renderIndex(rows []IndexRow) string {
	lines := append([]string{}, indexHeader...)
	var tb textbuf.Buffer
	for _, row := range rows {
		lines = append(lines, tb.Str("| ").Str(row.Rule).Str(" | ").Str(row.Summary).
			Str(" | ").Str(row.Severity).Str(" | ").Str(row.File).Str(" |").String())
		tb.Reset()
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

// indexTitle answers the rule's own `# ` heading, or the file stem when it has
// none.
func indexTitle(lines []string, fallback string) string {
	for _, line := range lines {
		if head, ok := headingOne(strings.TrimSpace(line)); ok {
			return head
		}
	}
	return fallback
}

// indexMeta answers the rule's metadata block as a map.
//
// The block is contiguous by construction (ai/rules/rule-format.md), so a
// paragraph-level match would swallow Severity and Related into the When text.
// That is exactly what happened before this existed: every index row read
// "<trigger> Severity: blocking Related: ...".
func indexMeta(lines []string) map[string]string {
	meta := map[string]string{}
	seenTitle := false
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if !seenTitle {
			seenTitle = strings.HasPrefix(s, "# ")
			continue
		}
		if s == "" {
			if len(meta) > 0 {
				break
			}
			continue
		}
		key, value, ok := metaLine(s)
		if !ok {
			break
		}
		meta[key] = strings.TrimSpace(value)
	}
	return meta
}

// indexSummary answers the When-to-read summary for a rule, or "" when none can
// be derived.
//
// An explicit `**When:**` trigger wins, then a `**BLOCKING:**` directive, then
// the first plain prose paragraph. A bold heading like `**When to use ...**` is
// not a trigger, so the When branch reads the metadata block and nothing else.
func indexSummary(lines []string) string {
	if when := indexMeta(lines)["When"]; when != "" {
		return cleanSummary(when)
	}
	blocking, prose := "", ""
	for _, para := range indexParagraphs(lines) {
		if blocking == "" && strings.HasPrefix(para, "**BLOCKING") {
			blocking = para
		}
		if prose == "" && isIndexProse(para) {
			prose = para
		}
	}
	chosen := blocking
	if chosen == "" {
		chosen = prose
	}
	if chosen == "" {
		return ""
	}
	return cleanSummary(chosen)
}

// indexParagraphs groups a rule's lines into paragraphs, dropping headings,
// code fences and pointer lines.
func indexParagraphs(lines []string) []string {
	var paras []string
	var cur []string
	inFence := false

	flush := func() {
		if len(cur) == 0 {
			return
		}
		paras = append(paras, strings.Join(cur, " "))
		cur = cur[:0]
	}

	for _, line := range lines {
		s := strings.TrimSpace(line)
		if strings.HasPrefix(s, "```") {
			inFence = !inFence
			flush()
			continue
		}
		if inFence {
			continue
		}
		if strings.HasPrefix(s, "#") || s == "" || startsWithWordAny(s, indexPointers[:]) {
			flush()
			continue
		}
		cur = append(cur, s)
	}
	flush()
	return paras
}

// isIndexProse reports whether a paragraph is the rule statement rather than
// markup or a cross-reference.
func isIndexProse(para string) bool {
	if startsWithAny(para, indexProseOpeners[:]) {
		return false
	}
	return !startsWithAny(strings.ToLower(para), indexProseWords[:])
}

// cleanSummary converts a paragraph to one index cell. It removes bold markup
// outside code spans, collapses whitespace, escapes pipes, and cuts at a word
// boundary. These changes prevent the text from breaking the table.
//
// The cut counts RUNES to match Python len() and slices. A byte bound can split
// an em dash.
func cleanSummary(text string) string {
	text = stripMarkerPrefix(text)
	text = stripBoldOutsideCode(text)
	text = collapseSpaceTrimmed(text)
	text = strings.ReplaceAll(text, "|", `\|`)
	if runeLen(text) <= maxSummary {
		return text
	}
	runes := []rune(text)
	cut := lastSpaceBefore(runes, maxSummary-1)
	if cut <= 0 {
		cut = maxSummary - 1
	}
	var tb textbuf.Buffer
	return tb.Str(strings.TrimRight(string(runes[:cut]), asciiSpace)).Str("...").String()
}

// stripMarkerPrefix removes a leading `**Marker:**` or `**Marker.**` and the
// whitespace after it, which is Python's `^\*\*[A-Za-z]+[:.]?\*\*\s*`.
func stripMarkerPrefix(text string) string {
	if !strings.HasPrefix(text, "**") {
		return text
	}
	at := 2
	for at < len(text) && isASCIILetter(rune(text[at])) {
		at++
	}
	if at == 2 {
		return text
	}
	if at < len(text) && (text[at] == ':' || text[at] == '.') {
		at++
	}
	if !strings.HasPrefix(text[at:], "**") {
		return text
	}
	return strings.TrimLeft(text[at+2:], asciiSpace)
}

// stripBoldOutsideCode drops bold markers, leaving code spans alone.
//
// A blanket replace corrupts globs: the trigger of testing.md carries
// `test/**/*.ci`, which rendered as `test//*.ci` in the index until the code
// spans were protected.
func stripBoldOutsideCode(text string) string {
	var tb textbuf.Buffer
	at := 0
	for at < len(text) {
		open := strings.IndexByte(text[at:], '`')
		if open < 0 {
			break
		}
		open += at
		end := strings.IndexByte(text[open+1:], '`')
		if end < 0 {
			break
		}
		end += open + 1
		tb.Str(strings.ReplaceAll(text[at:open], "**", ""))
		tb.Str(text[open : end+1])
		at = end + 1
	}
	return tb.Str(strings.ReplaceAll(text[at:], "**", "")).String()
}
