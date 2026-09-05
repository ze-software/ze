// Design: docs/architecture/core-design.md -- two generated files, one parse
// Overview: digest.go -- the parse and the core derivation these artifacts render
// Overview: actions.go -- the three actions that run these
//
// artifacts.go renders TRIGGERS.md, CORE.md, and the payload report that
// measures both files. The derivation was ported from the retired rules tool.
//
// Both artifacts come from ONE parse. A second source would drift, and a routed
// section must read identically to the digest section it replaces.
//
// The generated headers name `./le rules condensed-update`, the sole producer
// of both artifacts.

package rules

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// condensedPointers are the bare cross-reference openers that end a paragraph
// and are never directive material themselves.
var condensedPointers = [...]string{wordRationale, wordPrinciple, "see", "further reading"}

// condenseBody answers one rule's directives, dropped down to what a session
// must carry.
//
//	KEEP  every non-denylisted section's headings, list items, table rows and
//	      bold-directive lines, plus the FIRST sentence of each prose paragraph,
//	      which is the rule statement.
//	DROP  denylisted sections, fenced code blocks, HTML comments, and the bare
//	      `Rationale:` / `See:` pointer lines.
func condenseBody(body []string) []string {
	var out []string
	var prose []string
	inFence := false
	dropping := false
	keptProse := false

	flush := func() {
		if len(prose) == 0 {
			return
		}
		para := strings.Join(prose, " ")
		prose = prose[:0]
		if !dropping && !keptProse {
			out = append(out, firstSentence(para))
			keptProse = true
		}
	}

	for _, line := range body {
		s := strings.TrimSpace(line)
		if strings.HasPrefix(s, "```") || strings.HasPrefix(s, "~~~") {
			flush()
			inFence = !inFence
			continue
		}
		if inFence || strings.HasPrefix(s, "<!--") {
			continue
		}
		if marker, text, ok := headingLevel(s); ok {
			flush()
			name := normHeading(text)
			first, _, _ := strings.Cut(name, " ")
			dropping = denyFirstWords[first]
			keptProse = false
			if !dropping {
				var tb textbuf.Buffer
				out = append(out, tb.Str(marker).Byte(' ').Str(strings.TrimSpace(text)).String())
			}
			continue
		}
		if dropping {
			continue
		}
		if s == "" {
			flush()
			continue
		}
		if startsWithWordAny(s, condensedPointers[:]) {
			flush()
			continue
		}
		if isListItem(line) || isTableRow(line) || strings.HasPrefix(s, "**") {
			flush()
			out = append(out, s)
			continue
		}
		prose = append(prose, s)
	}
	flush()

	cleaned := make([]string, 0, len(out))
	for _, line := range out {
		if line == "" && (len(cleaned) == 0 || cleaned[len(cleaned)-1] == "") {
			continue
		}
		cleaned = append(cleaned, line)
	}
	for len(cleaned) > 0 && cleaned[len(cleaned)-1] == "" {
		cleaned = cleaned[:len(cleaned)-1]
	}
	return cleaned
}

// normHeading answers a heading reduced to the words a denylist can match:
// parentheticals dropped, lowercased, and everything but letters, digits and
// spaces removed. Matching the FIRST word rather than the whole heading is what
// makes "Why this matters" and "Rationale and background" both drop.
func normHeading(text string) string {
	text = dropParentheticals(text)
	var tb textbuf.Buffer
	for _, r := range strings.ToLower(text) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' {
			tb.WriteRune(r) //nolint:errcheck // textbuf never fails
		}
	}
	return collapseSpaceTrimmed(tb.String())
}

// dropParentheticals removes each `(...)` group. This matches Python's
// non-greedy `\(.*?\)` substitution: it removes the shortest run to the next
// closing bracket. It removes nothing when no closing bracket exists.
func dropParentheticals(text string) string {
	var tb textbuf.Buffer
	at := 0
	for at < len(text) {
		open := strings.IndexByte(text[at:], '(')
		if open < 0 {
			break
		}
		open += at
		close := strings.IndexByte(text[open+1:], ')')
		if close < 0 {
			break
		}
		tb.Str(text[at:open])
		at = open + 1 + close + 1
	}
	return tb.Str(text[at:]).String()
}

// startsWithWordAny reports whether s opens with one of the words, on a word
// boundary. It is Python's `^(A|B|C)\b` with re.IGNORECASE, and the boundary is
// what keeps "Rationales" from reading as "Rationale".
func startsWithWordAny(s string, words []string) bool {
	low := strings.ToLower(s)
	for _, word := range words {
		if !strings.HasPrefix(low, word) {
			continue
		}
		if len(low) == len(word) {
			return true
		}
		r, _ := utf8.DecodeRuneInString(low[len(word):])
		if !isPythonWord(r) {
			return true
		}
	}
	return false
}

// isPythonWord reports what Python's `\w` matches for a str pattern: an
// underscore, or any Unicode letter or digit.
func isPythonWord(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// isListItem reports what Python's `^\s*([-*+]|\d+[.)])\s+\S` reports over the
// RAW line, indentation included.
func isListItem(line string) bool {
	rest := strings.TrimLeft(line, asciiSpace)
	if rest == "" {
		return false
	}
	marker := 0
	if rest[0] == '-' || rest[0] == '*' || rest[0] == '+' {
		marker = 1
	} else {
		for marker < len(rest) && rest[marker] >= '0' && rest[marker] <= '9' {
			marker++
		}
		if marker == 0 || marker >= len(rest) || (rest[marker] != '.' && rest[marker] != ')') {
			return false
		}
		marker++
	}
	after := rest[marker:]
	trimmed := strings.TrimLeft(after, asciiSpace)
	return trimmed != after && trimmed != ""
}

// ruleBlock answers one rule's condensed section, identical in every artifact
// that shows it.
func ruleBlock(rule *Rule) string {
	var meta textbuf.Buffer
	for i, pair := range rule.Meta {
		if i > 0 {
			meta.Str(" — ")
		}
		meta.Str("**").Str(pair.Key).Str(":** ").Str(pair.Value)
	}

	var tb textbuf.Buffer
	block := []string{
		tb.Str("## ").Str(rule.Title).String(),
		func() string { tb.Reset(); return tb.Byte('`').Str(rule.Path).Byte('`').String() }(),
	}
	if meta.Len() > 0 {
		block = append(block, meta.String())
	}
	block = append(block, "")
	block = append(block, condenseBody(rule.Body)...)
	return strings.TrimRight(strings.Join(block, "\n"), asciiSpace)
}

// triggerLines answers one routing row per rule: path, severity, trigger. Never
// longer than maxTriggerLine.
//
// A core rule is marked `always-on`, because the index's own header promises
// the reader can tell which bodies are already loaded. Without the marker that
// sentence is a claim the table does not support.
//
// A rule with no trigger still gets a line. Dropping it would make the one
// thing this index promises -- that every rule stays named -- untrue for
// exactly the rules that are hardest to route.
func triggerLines(rules []Rule, core map[string]bool) []string {
	lines := make([]string, 0, len(rules))
	for i := range rules {
		rule := &rules[i]
		severity := rule.Severity
		if !isSeverity(severity) {
			severity = "unclassified"
		}
		var tb textbuf.Buffer
		if core[rule.Name] {
			severity = tb.Str(severity).Str(", always-on").String()
			tb.Reset()
		}
		trigger := collapseSpaceTrimmed(rule.Trigger)
		if trigger == "" {
			trigger = "(no trigger: always read it)"
		}
		prefix := tb.Str("| `").Str(rule.Path).Str("` | ").Str(severity).Str(" | ").String()
		tb.Reset()

		room := maxTriggerLine - runeLen(prefix) - runeLen(" |")
		if runeLen(trigger) > room {
			runes := []rune(trigger)
			cut := lastSpaceBefore(runes, room-3)
			if cut <= 0 {
				cut = room - 3
			}
			trigger = tb.Str(strings.TrimRight(string(runes[:cut]), " ,;:-")).Str("...").String()
			tb.Reset()
		}
		lines = append(lines, tb.Str(prefix).Str(trigger).Str(" |").String())
	}
	return lines
}

// triggersHeader is TRIGGERS.md above its first row.
var triggersHeader = []string{
	"# Ze Rules -- Trigger Index",
	"",
	"<!-- GENERATED by ./le rules condensed-update -- do not edit -->",
	"<!-- Regenerate: ./le rules condensed-update -->",
	"",
	"Every rule under `ai/rules/`, one line each. When a trigger matches the work",
	"in hand, READ that rule's file before acting. A row marked `always-on` is",
	"already loaded in full (`ai/rules/CORE.md`) and needs no read; every other",
	"rule's body is one Read away at the path in its row.",
	"",
	"| Rule | Severity | When to read it |",
	"|------|----------|-----------------|",
}

// coreHeader is CORE.md above its first block, minus the derived reason line.
var coreHeader = []string{
	"# Ze Rules -- Always-On Core",
	"",
	"<!-- GENERATED by ./le rules condensed-update -- do not edit -->",
	"<!-- Regenerate: ./le rules condensed-update -->",
	"",
	"The rules that apply before the shape of a task is known, so they are never",
	"reached through a trigger. Membership is DERIVED, never listed here: the",
	"ladder in `ai/rules/rule-precedence.md` names rungs 1 and 2, and rename a rule",
	"there and this file follows. A rule with no routable trigger lands here too,",
	"rather than being dropped from both artifacts, and so does a blocking rule",
	"that no past task description in `plan/` would surface",
	"(`./le rules router-report` prints that set and the corpus it read).",
	"",
	"Every other rule is named in `ai/rules/TRIGGERS.md`. Read its file when its",
	"trigger matches.",
	"",
}

// buildTriggers answers TRIGGERS.md: every rule named, in one line each.
func buildTriggers(rules []Rule, core map[string]bool) string {
	lines := append(append([]string{}, triggersHeader...), triggerLines(rules, core)...)
	var tb textbuf.Buffer
	return tb.Str(strings.Join(lines, "\n")).Byte('\n').String()
}

// buildCore answers CORE.md: the directives that must reach every session
// unconditionally.
//
// The header states no corpus SIZE or rule COUNT. These derived values caused a
// rewrite when a commit changed no core rule. Such a rewrite left `--check` red
// with no relevant author action. The header keeps only membership, which
// changes when the core changes.
func buildCore(core []Rule) string {
	reasons := make(map[string]bool, len(core))
	for i := range core {
		reasons[core[i].CoreReason] = true
	}
	ordered := make([]string, 0, len(reasons))
	for reason := range reasons {
		ordered = append(ordered, reason)
	}
	sort.Strings(ordered)

	var tb textbuf.Buffer
	header := append(append([]string{}, coreHeader...),
		tb.Str("Core reasons: ").Join(ordered, ", ").Byte('.').String(), "")
	tb.Reset()

	blocks := make([]string, 0, len(core))
	for i := range core {
		blocks = append(blocks, tb.Str(ruleBlock(&core[i])).
			Str("\n\n<!-- always-on: ").Str(core[i].CoreReason).Str(" -->").String())
		tb.Reset()
	}
	return tb.Str(strings.Join(header, "\n")).Str("\n---\n\n").
		Str(strings.Join(blocks, "\n\n---\n\n")).Byte('\n').String()
}

// DigestArtifact is one generated file's verdict.
type DigestArtifact struct {
	File  string `json:"file"`
	Rules int    `json:"rules"`
	Chars int    `json:"chars"`
	Stale bool   `json:"stale"`
}

// DigestReport is what the two condensed actions answer.
type DigestReport struct {
	Written bool `json:"written"`
	// EmptyCorpus says that the reachability derivation read no task description.
	// Thus, it shows no blocking rule as unreachable, and the core loses that
	// derivation. This describes the RUN, not the corpus. The fact belongs in the
	// payload instead of only on stderr.
	EmptyCorpus bool             `json:"empty-corpus"`
	Artifacts   []DigestArtifact `json:"artifacts"`
}

// Failed reports whether either artifact is stale.
func (r DigestReport) Failed() bool {
	for _, artifact := range r.Artifacts {
		if artifact.Stale {
			return true
		}
	}
	return false
}

// Text renders the verdict in the words the script printed.
func (r DigestReport) Text() string {
	var tb textbuf.Buffer
	for _, artifact := range r.Artifacts {
		switch {
		case r.Written:
			tb.Str("wrote ").Str(artifact.File).Str(" (").Int(int64(artifact.Rules)).
				Str(" rules, ").Int(int64(artifact.Chars)).Str(" chars)\n")
		case !artifact.Stale:
			tb.Str("checked ").Int(int64(artifact.Rules)).Str(" rules, ").
				Str(artifact.File).Str(" up to date\n")
		}
	}
	for _, artifact := range r.Artifacts {
		if artifact.Stale {
			tb.Str("WARNING: ").Str(artifact.File).
				Str(" is stale -- run: ./le rules condensed-update\n")
		}
	}
	return tb.String()
}

// Digest builds both artifacts and either compares them against the tree or
// writes them.
func Digest(tree string, check bool) (DigestReport, error) {
	report := DigestReport{Written: !check}
	dir := rulesDir(tree)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		var tb textbuf.Buffer
		return report, errors.New(tb.Str(rulesRel).Str(" not found").String())
	}

	rules, err := loadRules(dir)
	if err != nil {
		return report, err
	}
	tasks, err := loadCorpus(tree)
	if err != nil {
		return report, err
	}
	core, err := coreMembers(rules, taskCorpus{Read: true, Tasks: tasks}, &report.EmptyCorpus)
	if err != nil {
		return report, err
	}

	// One parse, two artifacts. The pair is a table so a third one is a row
	// here and nothing else, which is what ARTIFACTS is in the script.
	built := []struct {
		name    string
		rules   int
		content string
	}{
		{triggersFile, len(rules), buildTriggers(rules, coreNames(core))},
		{coreFile, len(core), buildCore(core)},
	}

	report.Artifacts = make([]DigestArtifact, 0, len(built))
	for _, artifact := range built {
		row := DigestArtifact{
			File:  artifactPath(artifact.name),
			Rules: artifact.rules,
			Chars: runeLen(artifact.content),
		}
		target := filepath.Join(dir, artifact.name)
		if check {
			current, readErr := os.ReadFile(target) // #nosec G304 -- a path derived from the checkout
			// An absent artifact is STALE, not an error. The reader takes the same
			// action as for drift: regenerate the artifact.
			row.Stale = readErr != nil || string(current) != artifact.content
		} else if err := os.WriteFile(target, []byte(artifact.content), 0o600); err != nil {
			return report, err
		}
		report.Artifacts = append(report.Artifacts, row)
	}
	return report, nil
}

// The two files this generator owns. Adding one here is the only edit a new
// artifact needs.
const (
	triggersFile = "TRIGGERS.md"
	coreFile     = "CORE.md"
)

// artifactPath names a generated artifact the way every message names it:
// relative to the checkout.
func artifactPath(name string) string {
	var tb textbuf.Buffer
	return tb.Str(rulesRel).Byte('/').Str(name).String()
}

// PayloadPart is one file of the always-loaded payload.
type PayloadPart struct {
	File   string `json:"file"`
	Chars  int    `json:"chars"`
	Tokens int    `json:"tokens"`
	// Missing says that the file was absent. The script counts an absent file as
	// zero characters. Thus, a payload without CORE.md appears further under
	// budget than a complete payload.
	Missing bool `json:"missing"`
}

// PayloadReport describes what a session loads: the instructions, the routing
// index, and the always-on core.
type PayloadReport struct {
	Parts    []PayloadPart `json:"parts"`
	Chars    int           `json:"chars"`
	Tokens   int           `json:"tokens"`
	Budget   int           `json:"budget"`
	Met      bool          `json:"met"`
	Headroom float64       `json:"headroom-percent"`
	// Missing names every absent part. A reader can use the verdict only when it
	// measures a complete payload.
	Missing []string `json:"missing"`
}

// Failed reports whether the measurement is missing one of the files it
// measures.
//
// The script always answers 0, and an absent file contributes zero characters.
// Thus, deletion of CORE.md makes the budget look MET. An absent payload is not
// a smaller payload.
func (r PayloadReport) Failed() bool { return len(r.Missing) > 0 }

// Text renders the report in the shape the script printed.
func (r PayloadReport) Text() string {
	var tb textbuf.Buffer
	tb.Str("always-loaded payload:\n")
	for _, part := range r.Parts {
		tb.Str("  ").Str(part.File).Str(": ").Int(int64(part.Chars)).Str(" chars (").
			Int(int64(part.Tokens)).Str(" tokens)\n")
	}
	tb.Str("  TOTAL: ").Int(int64(r.Chars)).Str(" chars (").Int(int64(r.Tokens)).Str(" tokens)\n")
	verdict := "EXCEEDED"
	if r.Met {
		verdict = "MET"
	}
	tb.Str("  budget: ").Int(int64(r.Budget)).Str(" tokens -- ").Str(verdict).Byte('\n')
	tb.Str("  headroom: ").Float(r.Headroom, 1).Str("%\n")
	if len(r.Missing) > 0 {
		tb.Str("WARNING: the payload is incomplete, so the total above measures a file set that "+
			"is not what a session loads: ").Join(r.Missing, ", ").Byte('\n')
	}
	return tb.String()
}

// payloadParts names the three files a session loads before it reads anything
// else.
var payloadParts = [...]string{"ai/INSTRUCTIONS.md", "ai/rules/TRIGGERS.md", "ai/rules/CORE.md"}

// Payload measures what the always-loaded payload costs against its budget.
func Payload(tree string) (PayloadReport, error) {
	report := PayloadReport{Budget: tokenBudget, Parts: make([]PayloadPart, 0, len(payloadParts))}
	for _, rel := range payloadParts {
		path := filepath.Join(tree, filepath.FromSlash(rel))
		part := PayloadPart{File: rel}
		raw, err := os.ReadFile(path) // #nosec G304 -- a path derived from the checkout
		switch {
		case err != nil && os.IsNotExist(err):
			part.Missing = true
			report.Missing = append(report.Missing, rel)
		case err != nil:
			return report, err
		case !utf8.Valid(raw):
			var tb textbuf.Buffer
			return report, errors.New(tb.Str(rel).Str(" is not valid UTF-8, so its size in characters cannot be counted").String())
		default:
			part.Chars = runeLen(string(raw))
			part.Tokens = part.Chars / bytesPerToken
		}
		report.Chars += part.Chars
		report.Parts = append(report.Parts, part)
	}
	report.Tokens = report.Chars / bytesPerToken
	report.Met = report.Tokens < tokenBudget
	report.Headroom = 100.0 * float64(tokenBudget-report.Tokens) / float64(tokenBudget)
	return report, nil
}
