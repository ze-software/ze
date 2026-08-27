// Design: docs/architecture/core-design.md -- the always-on payload, derived from one parse
// Overview: rules.go -- the corpus predicate this parse reads
// Detail: artifacts.go -- the two files this parse renders, and the payload it measures
// Detail: router.go -- the report that scores the same triggers with this tokenizer
//
// digest.go ports the parse and core derivation from `rules_condensed.py`. One
// read of `ai/rules/*.md` produces TRIGGERS.md and CORE.md. A second source
// would drift, and a routed section must match the digest section it replaces.
//
// Core membership is DERIVED. `ai/rules/rule-precedence.md` defines the ladder.
// Rungs 1 and 2 apply before the task type is known. This file parses the table
// instead of listing names. A renamed rule then changes the core when it changes
// the ladder.
//
// Every text predicate here is spelled by hand rather than by regexp. Python's
// `\s` is Unicode and Go's RE2 `\s` is the five ASCII bytes, so a shared
// pattern would be two different predicates wearing one spelling. The corpus
// carries no non-ASCII whitespace today (measured 2026-08-26 over ai/rules and
// plan), which is what makes the two halves comparable at all rather than what
// makes the port correct.

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

// rules_condensed.py defines these bounds. The Python comparison checks each
// value because output comparison cannot detect a shared value change.
const (
	// maxTriggerLine is the width one routing row is cut to. The index is a
	// routing key and the path on the same line makes the full clause one read
	// away.
	maxTriggerLine = 200
	// tokenBudget is what the always-loaded payload must come in under.
	tokenBudget = 40000
	// bytesPerToken is the reading the payload report estimates with.
	bytesPerToken = 4
	// maxTriggerDF is the document frequency at which a trigger word has no
	// routing signal. Such words are vocabulary that every rule shares.
	maxTriggerDF = 8
	// minHits is the distinctive trigger-word count that surfaces a rule. One
	// routes everything. Three requires more overlap than a real task
	// description contains.
	minHits = 2
	// maxProse is the longest first sentence a condensed section keeps.
	maxProse = 220
)

// stopwordsText is the vocabulary that carries no routing signal, spelled as
// rules_condensed.py spells it. A trigger is unroutable when nothing but these
// survive, which is the fail-closed case that puts the rule in the core.
const stopwordsText = `a an and any are as at be been before being but by can do does for from has have
if in into is it its more most no not of on once only or other over own same so some
such than that the their them then there these they this those to under until up upon
was were what when whenever whether which while who whom why will with within without
you your yourself each every after during unless prior soon time about above across
against all also am because both cannot could did doing done down else ever few
further had he her here hers him his how i just me my nor now off our ours out same
she should since still sub then thus too very we would`

// stopwords is stopwordsText as the set the tokenizer tests against.
var stopwords = func() map[string]bool {
	set := make(map[string]bool)
	for word := range strings.FieldsSeq(stopwordsText) {
		set[word] = true
	}
	return set
}()

// denyFirstWords names sections that a condensed rule drops. It matches the
// heading's FIRST word, so it drops "Why this matters", "Rationale and
// background", and "Example: the good case".
//
// Generic headings such as Notes, Reference, History, and Appendix are absent.
// They can contain real directives. Keeping unrelated text is safer than
// silently dropping a directive.
var denyFirstWords = map[string]bool{
	wordRationale: true,
	"why":         true,
	"background":  true,
	"example":     true,
	"examples":    true,
}

// The two cross-reference openers that appear in more than one vocabulary. They
// are one word each, spelled once, so a rename cannot reach one list and miss
// the other.
const (
	wordRationale = "rationale"
	wordPrinciple = "principle"
)

// MetaPair is one line of a rule's metadata block, in file order. The order is
// what CORE.md renders, so a map would lose it.
type MetaPair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Rule is one parsed rule file: the single parse every artifact reads.
type Rule struct {
	Name     string     `json:"name"`
	Stem     string     `json:"stem"`
	Path     string     `json:"path"`
	Title    string     `json:"title"`
	Meta     []MetaPair `json:"meta"`
	Body     []string   `json:"-"`
	Trigger  string     `json:"trigger"`
	Severity string     `json:"severity"`
	// CoreReason says WHY this rule is always-on, and is empty for a routed
	// rule. It is set by CoreMembers and by nothing else.
	CoreReason string `json:"core-reason,omitempty"`
}

// LoadRules answers every rule under rulesDir, parsed once.
//
// It refuses a directory that contains no rules. Python returns an empty list
// and relies on artifact comparison. That works only while the artifacts have
// rows. A fresh checkout of both sources would agree on nothing and pass.
func LoadRules(rulesDir string) ([]Rule, error) {
	files, err := RuleFiles(rulesDir)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("no rule file under ").Str(rulesRel).
			Str("/; the digest read nothing and must not report success").String())
	}

	rules := make([]Rule, 0, len(files))
	for _, path := range files {
		raw, err := os.ReadFile(path) // #nosec G304 -- a path this tool derived from the checkout
		if err != nil {
			return nil, err
		}
		name := filepath.Base(path)
		stem := strings.TrimSuffix(name, ".md")
		title, meta, body := parseRule(splitLines(string(raw)))
		if title == "" {
			title = stem
		}
		var tb textbuf.Buffer
		rule := Rule{
			Name:  name,
			Stem:  stem,
			Path:  tb.Str(rulesRel).Byte('/').Str(name).String(),
			Title: title,
			Meta:  meta,
			Body:  body,
		}
		for _, pair := range meta {
			switch pair.Key {
			case "When":
				rule.Trigger = strings.TrimSpace(pair.Value)
			case "Severity":
				rule.Severity = strings.ToLower(strings.TrimSpace(pair.Value))
			}
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// parseRule splits one rule file into its title, its metadata block and its
// body.
//
// The metadata block is contiguous by construction (ai/rules/rule-format.md),
// so the first line that is blank or is not a canonical key ends it. A
// paragraph-level match would swallow Severity and Related into the When text.
func parseRule(raw []string) (title string, meta []MetaPair, body []string) {
	at := 0
	for at < len(raw) && strings.TrimSpace(raw[at]) == "" {
		at++
	}
	if at < len(raw) {
		if head, ok := headingOne(strings.TrimSpace(raw[at])); ok {
			title = head
			at++
		}
	}
	for at < len(raw) && strings.TrimSpace(raw[at]) == "" {
		at++
	}
	for at < len(raw) {
		line := strings.TrimSpace(raw[at])
		if line == "" {
			break
		}
		key, value, ok := metaLine(line)
		if !ok {
			break
		}
		meta = append(meta, MetaPair{Key: key, Value: strings.TrimSpace(value)})
		at++
	}
	return title, meta, raw[at:]
}

// headingOne answers the text of a `# ` heading. A `## ` heading is not one
// because its marker is not a single hash followed by whitespace.
//
// A whitespace-only heading answers false instead of Python's empty title. That
// shape never reaches a generator. ze-rules-lint first requires the initial
// nonblank line to contain `# ` and a nonspace character (lint.go, lintH1).
func headingOne(line string) (string, bool) {
	if !strings.HasPrefix(line, "#") {
		return "", false
	}
	rest := line[1:]
	trimmed := strings.TrimLeft(rest, asciiSpace)
	if trimmed == rest || trimmed == "" {
		return "", false
	}
	return strings.TrimRight(trimmed, asciiSpace), true
}

// headingLevel answers the marker and the text of a `##` to `######` heading.
func headingLevel(line string) (marker, text string, ok bool) {
	hashes := 0
	for hashes < len(line) && line[hashes] == '#' {
		hashes++
	}
	if hashes < 2 || hashes > 6 || hashes >= len(line) {
		return "", "", false
	}
	rest := line[hashes:]
	trimmed := strings.TrimLeft(rest, asciiSpace)
	if trimmed == rest {
		return "", "", false
	}
	return line[:hashes], trimmed, true
}

// metaLine answers one `**Key:** value` line of the metadata block, for the
// three canonical keys and no other.
func metaLine(line string) (key, value string, ok bool) {
	if !strings.HasPrefix(line, "**") {
		return "", "", false
	}
	close := strings.Index(line[2:], ":**")
	if close < 0 {
		return "", "", false
	}
	key = line[2 : 2+close]
	if !isCanonKey(key) {
		return "", "", false
	}
	return key, strings.TrimLeft(line[2+close+len(":**"):], asciiSpace), true
}

// asciiSpace is the whitespace class for this port. Python's `\s` also contains
// vertical tab and Unicode whitespace. The corpus contains none of those extra
// characters.
const asciiSpace = " \t\n\v\f\r"

// significantTerms answers the lowercase content words of a trigger or a task
// description.
//
// A compound yields itself and its parts. For example, `wire-encoding` also
// emits `wire` and `encoding`. Thus, hyphenated and spaced forms of a phrase
// match. Without the split, a trigger hyphen would prevent each match.
func significantTerms(text string) map[string]bool {
	terms := make(map[string]bool)
	for _, word := range termWords(strings.ToLower(text)) {
		word = strings.Trim(word, ".-_/")
		add := func(part string) {
			if len(part) > 2 && !stopwords[part] {
				terms[part] = true
			}
		}
		add(word)
		for _, part := range strings.FieldsFunc(word, isTermSeparator) {
			add(part)
		}
	}
	return terms
}

// termWords answers what Python's findall(r"[a-z0-9][a-z0-9.\-_/]*") answers
// over already-lowercased text.
func termWords(lower string) []string {
	var out []string
	at := 0
	for at < len(lower) {
		if !isTermHead(lower[at]) {
			at++
			continue
		}
		end := at + 1
		for end < len(lower) && isTermTail(lower[end]) {
			end++
		}
		out = append(out, lower[at:end])
		at = end
	}
	return out
}

func isTermHead(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}

func isTermTail(c byte) bool {
	return isTermHead(c) || c == '.' || c == '-' || c == '_' || c == '/'
}

func isTermSeparator(r rune) bool {
	return r == '-' || r == '_' || r == '/' || r == '.'
}

// distinctiveTerms answers each rule's trigger terms, minus the vocabulary the
// triggers all share.
//
// "code", "test" and "any" appear in dozens of triggers and separate nothing.
// "gokrazy", "nlri" and "qdisc" separate a great deal.
func distinctiveTerms(rules []Rule) map[string]map[string]bool {
	frequency := make(map[string]int)
	perRule := make(map[string]map[string]bool, len(rules))
	for i := range rules {
		rule := &rules[i]
		terms := significantTerms(rule.Trigger)
		perRule[rule.Name] = terms
		for term := range terms {
			frequency[term]++
		}
	}
	out := make(map[string]map[string]bool, len(rules))
	for name, terms := range perRule {
		kept := make(map[string]bool)
		for term := range terms {
			if frequency[term] <= maxTriggerDF {
				kept[term] = true
			}
		}
		out[name] = kept
	}
	return out
}

// overlap answers how many of a rule's distinctive terms a task carries.
func overlap(ruleTerms, taskTerms map[string]bool) int {
	hits := 0
	for term := range ruleTerms {
		if taskTerms[term] {
			hits++
		}
	}
	return hits
}

// surfacedBy answers the rules a trigger index would surface for one task.
func surfacedBy(rules []Rule, taskText string, terms map[string]map[string]bool) map[string]bool {
	taskTerms := significantTerms(taskText)
	out := make(map[string]bool)
	for i := range rules {
		if overlap(terms[rules[i].Name], taskTerms) >= minHits {
			out[rules[i].Name] = true
		}
	}
	return out
}

// Task is one past task description, the input the core's reachability test
// reads.
type Task struct {
	Source string `json:"source"`
	Text   string `json:"-"`
}

// Corpus is that input and records whether any caller READ it.
//
// The zero value says that the caller did not request reachability. The router
// report does this intentionally. A read Corpus with no task is different: the
// caller requested a derivation that is absent. Both contain no task, so Read
// states the difference.
type Corpus struct {
	Read  bool
	Tasks []Task
}

// unreachableBlocking answers the blocking rules that NO corpus task surfaces.
// Routing puts this population at risk. Therefore, the corpus is a generator
// input rather than an operator report.
//
// A nil corpus means the caller did not ask for this derivation, which is what
// the router report does on purpose. An EMPTY corpus is a different fact: the
// caller DID ask and the read returned nothing. Both answer the same set, so
// the empty case is reported rather than passing for a real result.
func unreachableBlocking(rules []Rule, corpus Corpus, empty *bool) map[string]bool {
	if !corpus.Read {
		return nil
	}
	if len(corpus.Tasks) == 0 {
		if empty != nil {
			*empty = true
		}
		return nil
	}
	terms := distinctiveTerms(rules)
	reached := make(map[string]bool)
	for _, entry := range corpus.Tasks {
		for name := range surfacedBy(rules, entry.Text, terms) {
			reached[name] = true
		}
	}
	out := make(map[string]bool)
	for i := range rules {
		if rules[i].Severity == severityBlocking && !reached[rules[i].Name] {
			out[rules[i].Name] = true
		}
	}
	return out
}

// LadderError says that the precedence ladder was unreadable. Thus, the core
// cannot be derived.
//
// The function returns this error instead of an empty set. Empty and unreadable
// ladders otherwise produce the same value. Only the layer that detected the
// missing ladder can report it.
type LadderError struct {
	Reason string
}

func (e *LadderError) Error() string { return e.Reason }

// ladderRungs contains the two rungs that derive the always-on core:
// irreversible action and correctness owed outside this repository.
var ladderRungs = [...]int{1, 2}

// precedenceRungSlugs answers the rule file names on rungs 1 and 2 of
// rule-precedence.md.
//
// The parser locates markdown table columns by HEADER text, not index. Thus,
// column reordering cannot silently empty the core. But a renamed header or a
// list representation makes the parser skip every row. Each empty parse returns
// a LadderError that names its cause.
func precedenceRungSlugs(rules []Rule) (map[string]bool, *Rule, error) {
	var source *Rule
	for i := range rules {
		if rules[i].Stem == "rule-precedence" {
			source = &rules[i]
			break
		}
	}
	if source == nil {
		return nil, nil, &LadderError{Reason: "ai/rules/rule-precedence.md is absent: the always-on core is derived " +
			"from its ladder, so there is nothing left to derive it from"}
	}

	known := make(map[string]bool, len(rules))
	for i := range rules {
		known[rules[i].Stem] = true
	}

	slugs := make(map[string]bool)
	rungCol, rulesCol := -1, -1
	sawHeader, sawRungRow := false, false
	for _, line := range source.Body {
		if !isTableRow(line) {
			// A markdown table cannot have gaps, so any non-row line ends it.
			// Carrying the column indexes past the table let a LATER table's
			// rows be read with this one's layout.
			rungCol, rulesCol = -1, -1
			continue
		}
		cells := tableCells(line)
		if at, other, ok := headerColumns(cells); ok {
			rungCol, rulesCol = at, other
			sawHeader = true
			continue
		}
		if rungCol < 0 || rungCol >= len(cells) || rulesCol >= len(cells) {
			continue
		}
		rung, ok := asciiDigits(cells[rungCol])
		if !ok || !isLadderRung(rung) {
			continue
		}
		sawRungRow = true
		for _, token := range backtickTokens(cells[rulesCol]) {
			stem := strings.TrimSuffix(strings.TrimSpace(token), ".md")
			if known[stem] {
				var tb textbuf.Buffer
				slugs[tb.Str(stem).Str(".md").String()] = true
			}
		}
	}

	var tb textbuf.Buffer
	switch {
	case !sawHeader:
		return nil, nil, &LadderError{Reason: "no table in ai/rules/rule-precedence.md has both a `Rung` and a " +
			"`Rules` column: the columns are found by header text, so renaming " +
			"either one, or rewriting the ladder as a list, empties the core"}
	case !sawRungRow:
		return nil, nil, &LadderError{Reason: tb.Str("the ladder in ai/rules/rule-precedence.md has no rung ").
			Str(rungWord()).
			Str(" row: those rungs are what the always-on core is derived from").String()}
	case len(slugs) == 0:
		return nil, nil, &LadderError{Reason: tb.Str("rung ").Str(rungWord()).
			Str(" of the ladder in ai/rules/rule-precedence.md names no ").
			Str("rule under ai/rules/: the core would lose every destructive-action ").
			Str("and outside-facing-correctness guard").String()}
	}
	return slugs, source, nil
}

// rungWord spells the rung pair the way the Python messages spell it.
func rungWord() string {
	var tb textbuf.Buffer
	for i, rung := range ladderRungs {
		if i > 0 {
			tb.Byte('/')
		}
		tb.Int(int64(rung))
	}
	return tb.String()
}

func isLadderRung(n int) bool {
	for _, rung := range ladderRungs {
		if n == rung {
			return true
		}
	}
	return false
}

// isTableRow reports what Python's `^\s*\|` reports.
func isTableRow(line string) bool {
	return strings.HasPrefix(strings.TrimLeft(line, asciiSpace), "|")
}

// tableCells answers one markdown row's cells, trimmed, which is what
// `line.strip().strip("|").split("|")` answers.
func tableCells(line string) []string {
	inner := strings.Trim(strings.TrimSpace(line), "|")
	cells := strings.Split(inner, "|")
	for i, cell := range cells {
		cells[i] = strings.TrimSpace(cell)
	}
	return cells
}

// headerColumns answers the index of the `Rung` and `Rules` columns, when the
// row carries both.
func headerColumns(cells []string) (rung, rules int, ok bool) {
	rung, rules = -1, -1
	for i, cell := range cells {
		switch strings.ToLower(cell) {
		case "rung":
			if rung < 0 {
				rung = i
			}
		case "rules":
			if rules < 0 {
				rules = i
			}
		}
	}
	return rung, rules, rung >= 0 && rules >= 0
}

// asciiDigits answers an ASCII value that Python's str.isdigit() would accept.
// A Unicode digit in a rung is a typo. If read as a rung, it CAN put a rule in
// the core by accident.
func asciiDigits(cell string) (int, bool) {
	if cell == "" {
		return 0, false
	}
	value := 0
	for i := range len(cell) {
		if cell[i] < '0' || cell[i] > '9' {
			return 0, false
		}
		value = value*10 + int(cell[i]-'0')
	}
	return value, true
}

// backtickTokens answers every `code span`'s content, which is what
// findall("`([^`]+)`") answers.
//
// The scan is bounded by the line. Each pass moves `at` past a closing
// backtick. Thus, each pass advances by at least two bytes. The scan ends when
// it finds no pair.
func backtickTokens(text string) []string {
	var out []string
	at := 0
	for {
		open := strings.IndexByte(text[at:], '`')
		if open < 0 {
			return out
		}
		open += at
		close := strings.IndexByte(text[open+1:], '`')
		if close < 0 {
			return out
		}
		close += open + 1
		if close > open+1 {
			out = append(out, text[open+1:close])
		}
		at = close + 1
	}
}

// CoreMembers answers the always-on set, derived. Each member carries WHY it is
// eager.
//
// Four derivations, no list:
//
//   - The precedence ladder's rungs 1 and 2, parsed from the ladder table.
//   - The ladder itself, which resolves every conflict.
//   - Fail-closed: no trigger, no severity, or no term to match.
//   - Blocking rules that no past task description would surface (needs a corpus).
//
// Order follows the rule list, so CORE.md reads in the same order as the
// trigger index and a reader can diff the two.
func CoreMembers(rules []Rule, corpus Corpus, emptyCorpus *bool) ([]Rule, error) {
	ladder, source, err := precedenceRungSlugs(rules)
	if err != nil {
		return nil, err
	}
	unreachable := unreachableBlocking(rules, corpus, emptyCorpus)

	var core []Rule
	for i := range rules {
		reason, eager := coreReason(&rules[i], ladder, source, unreachable)
		if !eager {
			continue
		}
		member := rules[i]
		member.CoreReason = reason
		core = append(core, member)
	}
	return core, nil
}

// coreReason answers WHY one rule is always-on, and whether it is.
//
// The order is the derivation's: the ladder first, then the ladder's own file,
// then the three fail-closed cases, then reachability. A rule matching two of
// them takes the first, so the reason a reader sees is the strongest one.
func coreReason(rule *Rule, ladder map[string]bool, source *Rule, unreachable map[string]bool) (string, bool) {
	switch {
	case ladder[rule.Name]:
		var tb textbuf.Buffer
		return tb.Str("precedence rung ").Str(rungWord()).String(), true
	case source != nil && rule.Name == source.Name:
		return "the ladder itself", true
	case rule.Trigger == "":
		return "no trigger to route on", true
	case !isSeverity(rule.Severity):
		return "no severity to route on", true
	case rule.Severity == severityBlocking && len(significantTerms(rule.Trigger)) == 0:
		return "trigger carries no term to match", true
	case unreachable[rule.Name]:
		return "no past task would surface it", true
	default:
		return "", false
	}
}

// coreNames answers the member set as names.
func coreNames(core []Rule) map[string]bool {
	out := make(map[string]bool, len(core))
	for i := range core {
		out[core[i].Name] = true
	}
	return out
}

// firstSentence answers a paragraph's rule statement: its first sentence, or a
// word-boundary cut at limit runes.
//
// The limit counts RUNES, because Python's len() and slice do. A byte bound
// cuts an em dash in half, and the corpus is full of them.
func firstSentence(text string) string {
	const limit = maxProse
	runes := []rune(collapseSpaceTrimmed(text))
	for i, r := range runes {
		if r != '.' {
			continue
		}
		if i+1 < len(runes) && !unicode.IsSpace(runes[i+1]) {
			continue
		}
		if i > limit {
			break
		}
		return string(runes[:i+1])
	}
	if len(runes) <= limit {
		return string(runes)
	}
	cut := lastSpaceBefore(runes, limit-1)
	if cut <= 0 {
		cut = limit - 1
	}
	var tb textbuf.Buffer
	return tb.Str(strings.TrimRight(string(runes[:cut]), asciiSpace)).Str("...").String()
}

// collapseSpaceTrimmed answers what Python's `WS.sub(" ", text).strip()`
// answers. It replaces each whitespace run with one space and removes the
// leading and trailing runs.
//
// The substitution and the strip use one pass. Every caller strips, so emitted
// outer runs would contain bytes that no caller observes.
func collapseSpaceTrimmed(text string) string {
	var tb textbuf.Buffer
	space := false
	for i := range len(text) {
		if strings.IndexByte(asciiSpace, text[i]) >= 0 {
			space = true
			continue
		}
		if space && tb.Len() > 0 {
			tb.Byte(' ')
		}
		space = false
		tb.Byte(text[i])
	}
	return tb.String()
}

// lastSpaceBefore answers what Python's rfind(" ", 0, stop) answers over runes.
func lastSpaceBefore(runes []rune, stop int) int {
	if stop > len(runes) {
		stop = len(runes)
	}
	for i := stop - 1; i >= 0; i-- {
		if runes[i] == ' ' {
			return i
		}
	}
	return -1
}

// runeLen answers what Python's len() answers for a str.
func runeLen(s string) int { return utf8.RuneCountInString(s) }

// sortedNames answers a name set, sorted.
func sortedNames(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
