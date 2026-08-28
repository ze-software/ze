// Design: docs/architecture/core-design.md -- the rule format, checked
// Overview: rules.go -- the corpus predicate and the Python spellings this uses
// Overview: actions.go -- the action that runs this
//
// lint.go ports internal/le/rules/lint.go. It answers only two corpus questions.
//
// A RENDERED rule must contain the canonical metadata block. The block must be
// HONEST. `**When:**` names a task situation, and `**Severity:**` agrees with
// the body. A trigger that repeats a directive matches every task and routes
// nothing. An `advisory` rule with a BLOCKING body makes severity meaningless.
//
// A `directive` POINT file must state its obligation with RFC 2119 keywords. Its
// `level:` must name the body's strongest tier. Otherwise, readers must infer
// the instruction weight from tone and can reach different answers.
//
// One behavior differs from the script. The script succeeds on an empty corpus.
// A checkout without rule files or an ai/rules/points/ tree prints
// "0 rule file(s) conform" and exits 0. This port refuses both cases. That
// matches the rules_points.py answer from render_all and report_gate_map. See
// plan/journal/zero-value-as-valid-answer.md.

package rules

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// severities is the closed set that `**Severity:**` can name.
var severities = [...]string{severityAdvisory, severityBlocking}

// The two severities, spelled once. Four files test a rule's severity against
// one of them, and a fifth renders it.
const (
	severityAdvisory = "advisory"
	severityBlocking = "blocking"
)

// canonKeys contains the required spelling and ORDER of metadata keys. Consumers
// match `**When:**`, `**Severity:**`, and `**Related:**` case-sensitively. Thus,
// a case error is a violation, not an alias. Otherwise, digest artifacts would
// receive an unparsed key.
var canonKeys = [...]string{"When", "Severity", "Related"}

// openers contains the permitted temporal starts for a trigger. This small,
// closed set makes the routing column uniform and easy to scan. Each addition
// creates another form that readers must understand.
var openers = [...]string{
	"when ", "whenever ", "while ", "before ", "after ", "during ", "if ",
	"unless ", "once ", "upon ", "on ", "at ", "any time ", "every time ",
	"each time ", "prior to ", "as soon as ",
}

// notGerund holds the words that end in -ing and are not gerunds, so they
// cannot open a trigger.
var notGerund = map[string]bool{
	"nothing": true, "something": true, "anything": true,
	"everything": true, "string": true, "thing": true,
}

// truncatedTail contains the trailing characters removed from a wrapped body
// line. The script lists both "--" and "-", although "-" already covers both.
// Keep both to preserve the corpus record.
var truncatedTail = [...]string{",", ";", ":", "-", "--"}

// danglingLastWord holds the words no English clause ends on. A trigger
// stopping on one is syntactically a fine metadata line and useless as a
// routing key: planning.md shipped "...and is enforced by" for months.
var danglingLastWord = map[string]bool{
	"a": true, "an": true, "and": true, "any": true, "are": true, "as": true,
	"at": true, "be": true, "been": true, "before": true, "but": true,
	"by": true, "each": true, "every": true, "for": true, "from": true,
	"in": true, "into": true, "is": true, "its": true, "not": true,
	"of": true, "on": true, "or": true, "the": true, "their": true,
	"then": true, "these": true, "this": true, "those": true, "to": true,
	"was": true, "were": true, "which": true, "with": true,
}

// These five values are RFC 2119 obligation levels. levelCanon maps each keyword
// synonym to one value, so readers never compare REQUIRED with MUST.
const (
	levelMust      = "MUST"
	levelMustNot   = "MUST NOT"
	levelShould    = "SHOULD"
	levelShouldNot = "SHOULD NOT"
	levelMay       = "MAY"
)

// rfcLevels contains RFC 2119 and RFC 8174 keywords. Longest values come first,
// so MUST NOT wins over MUST at one position.
//
// Keep this array and rfcKeyword synchronized with
// .claude/hooks/pretool-writeedit.py (c_rule_point_rfc_language). The hook checks
// writes, and this pass checks the gate. They must agree on every keyword.
var rfcLevels = [...]string{
	levelMustNot, "SHALL NOT", levelShouldNot, "NOT RECOMMENDED",
	levelMust, "SHALL", "REQUIRED", levelShould, "RECOMMENDED", levelMay, "OPTIONAL",
}

// levelCanon maps each synonym to one obligation level. Thus, `level:` has one
// spelling for each level, and readers need not compare REQUIRED with MUST.
var levelCanon = map[string]string{
	levelMust: levelMust, "SHALL": levelMust, "REQUIRED": levelMust,
	levelMustNot: levelMustNot, "SHALL NOT": levelMustNot,
	levelShould: levelShould, "RECOMMENDED": levelShould,
	levelShouldNot: levelShouldNot, "NOT RECOMMENDED": levelShouldNot,
	levelMay: levelMay, "OPTIONAL": levelMay,
}

// levelRank is the closed set that a `level:` field can name.
var levelRank = [...]string{levelMay, levelShouldNot, levelShould, levelMustNot, levelMust}

// levelTiers ranks obligation by STRENGTH, weakest tier first.
//
// RFC 2119 does not rank MUST against MUST NOT. They share one tier with
// opposite polarity, as do SHOULD and SHOULD NOT. `level:` names the strongest
// TIER in the body, and either polarity is valid. An ordering would make a
// prohibition declare MUST when another sentence contains a positive
// obligation. The prohibition would then be unrecorded.
var levelTiers = [][]string{
	{levelMay},
	{levelShould, levelShouldNot},
	{levelMust, levelMustNot},
}

// lowerModalWords contains lowercase obligation words that a directive cannot
// use. They resemble obligation keywords, but they do not carry their force.
// ai/rules/writing.md also bans this hedge.
var lowerModalWords = [...]string{"must", "shall", "should", "may"}

var (
	lintMetaLine   = regexp.MustCompile(`^\*\*([A-Za-z]+):\*\*[ \t\n\r\f\v]*(.*)$`)
	lintH1         = regexp.MustCompile(`^#[ \t\n\r\f\v]+(\S.*)$`)
	relatedSlug    = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	codeSpan       = regexp.MustCompile("`[^`]*`")
	severityNote   = regexp.MustCompile(`<!--[ \t\n\r\f\v]*severity-note:`)
	rfcKeyword     = regexp.MustCompile(`\b(?:` + strings.Join(rfcLevels[:], "|") + `)\b`)
	pointMatter    = regexp.MustCompile(`(?s)\A---\n(.*?)\n---\n(.*)\z`)
	pointKindLine  = regexp.MustCompile(`(?m)^kind:[ \t\n\r\f\v]*(\S*)`)
	pointLevelLine = regexp.MustCompile(`(?m)^level:[^\S\n]*(.*)$`)
	fenceOpen      = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})([^\n]*)$")
	blockquote     = regexp.MustCompile(`(?m)^[ \t]{0,3}>.*$`)
)

// LintProblems is one file and everything wrong with it.
type LintProblems struct {
	File     string   `json:"file"`
	Problems []string `json:"problems"`
}

// LintReport is what `le rules lint` answers.
//
// The report keeps both passes separate because they run separately. A
// rule-format violation stops the run before its RFC 2119 pass reads anything.
// Thus, Points is 0 when RuleViolations is nonempty. The payload states this
// control flow instead of hiding it in a merged list.
type LintReport struct {
	Rules           int            `json:"rules"`
	RuleViolations  []LintProblems `json:"rule-violations"`
	Points          int            `json:"points"`
	PointViolations []LintProblems `json:"point-violations"`
	// Empty names each population the lint read nothing from. It is the port's
	// one departure from the script, which reports success over an empty
	// corpus.
	Empty []string `json:"empty"`
}

// Failed reports whether the lint found anything. It is the exit code's only
// input.
func (r LintReport) Failed() bool {
	return len(r.Empty) > 0 || len(r.RuleViolations) > 0 || len(r.PointViolations) > 0
}

// Text renders the verdict with the script's words. leroot uses this Prose form
// for the bare command, while pipe operators bypass it.
//
// It prints one report, never three. The script exits after the first failing
// pass. Thus, a rule-format violation fills the page, and no RFC 2119 count
// follows.
func (r LintReport) Text() string {
	var tb textbuf.Buffer
	switch {
	case len(r.Empty) > 0:
		for _, line := range r.Empty {
			tb.Str("rules_lint: ").Str(line).Byte('\n')
		}
	case len(r.RuleViolations) > 0:
		tb.Str("rules_lint: ").Int(int64(len(r.RuleViolations))).Byte('/').
			Int(int64(r.Rules)).Str(" rule file(s) violate the format\n\n")
		writeViolations(&tb, r.RuleViolations)
		tb.Str("\nFormat spec: ai/rules/rule-format.md\n")
	case len(r.PointViolations) > 0:
		tb.Str("rules_lint: ").Int(int64(len(r.PointViolations))).Byte('/').
			Int(int64(r.Points)).Str(" rule point(s) do not state their obligation in RFC 2119 language\n\n")
		writeViolations(&tb, r.PointViolations)
		tb.Str("\nFormat spec: ai/rules/rule-format.md 'Every directive states a level'\n")
	default:
		tb.Str("rules_lint: ").Int(int64(r.Rules)).Str(" rule file(s) conform to ai/rules/rule-format.md\n")
		tb.Str("rules_lint: ").Int(int64(r.Points)).Str(" rule point(s) state an RFC 2119 level\n")
	}
	return tb.String()
}

// writeViolations renders one pass's failing files, in the shape the script
// printed: the path, then one indented dash per problem.
func writeViolations(tb *textbuf.Buffer, violations []LintProblems) {
	for _, entry := range violations {
		tb.Str("  ").Str(entry.File).Byte('\n')
		for _, problem := range entry.Problems {
			tb.Str("      - ").Str(problem).Byte('\n')
		}
	}
}

// Lint checks the rendered rules and the point files of one checkout.
func Lint(tree string) (LintReport, error) {
	var report LintReport
	rulesDir := filepath.Join(tree, filepath.FromSlash(rulesRel))
	info, err := os.Stat(rulesDir)
	if err != nil || !info.IsDir() {
		var tb textbuf.Buffer
		return report, errors.New(tb.Str(rulesRel).Str(" not found").String())
	}

	files, err := ruleFiles(rulesDir)
	if err != nil {
		return report, err
	}
	report.Rules = len(files)
	if len(files) == 0 {
		report.Empty = append(report.Empty,
			"no rule file under ai/rules/; the lint read nothing and must not report success")
	}
	for _, path := range files {
		problems, err := checkRuleFile(path)
		if err != nil {
			return report, err
		}
		if len(problems) > 0 {
			report.RuleViolations = append(report.RuleViolations,
				LintProblems{File: relTo(tree, path), Problems: problems})
		}
	}
	// A malformed rule makes the script exit before its RFC 2119 pass reads a
	// point. Preserve this behavior. The second pass must not parse points whose
	// rendered rule the first pass rejected.
	if len(report.RuleViolations) > 0 {
		return report, nil
	}

	pointsDir := filepath.Join(tree, filepath.FromSlash(pointsRel))
	count, violations, err := checkPointFiles(tree, pointsDir)
	if err != nil {
		return report, err
	}
	report.Points = count
	report.PointViolations = violations
	if count == 0 {
		report.Empty = append(report.Empty,
			"no rule point file under ai/rules/points/; the RFC 2119 pass read nothing and must not report success")
	}
	return report, nil
}

// checkRuleFile answers every violation of one rendered rule.
func checkRuleFile(path string) ([]string, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- a path this tool derived from the checkout
	if err != nil {
		return nil, err
	}
	return checkRule(string(raw)), nil
}

// checkRule answers every violation of one rendered rule's text.
func checkRule(text string) []string {
	var problems []string
	lines := splitLines(text)

	index := 0
	for index < len(lines) && strings.TrimSpace(lines[index]) == "" {
		index++
	}
	var title string
	if index < len(lines) {
		if found := lintH1.FindStringSubmatch(strings.TrimSpace(lines[index])); found != nil {
			title = found[1]
		}
	}
	if title == "" {
		return []string{"first non-blank line must be a single '# Title'"}
	}
	index++

	for index < len(lines) && strings.TrimSpace(lines[index]) == "" {
		index++
	}

	meta := map[string]string{}
	var order []string
	for index < len(lines) && strings.TrimSpace(lines[index]) != "" {
		found := lintMetaLine.FindStringSubmatch(strings.TrimSpace(lines[index]))
		if found == nil {
			break
		}
		key := found[1]
		if !isCanonKey(key) {
			var tb textbuf.Buffer
			problems = append(problems, tb.Str("metadata key '**").Str(key).
				Str(":**' must be one of ").Join(canonKeys[:], "/").
				Str(" (exact case)").String())
			break
		}
		meta[key] = strings.TrimSpace(found[2])
		order = append(order, key)
		index++
	}

	switch when, ok := meta["When"]; {
	case !ok:
		problems = append(problems, "missing required '**When:** <trigger>' line")
	case when == "":
		problems = append(problems, "'**When:**' line is empty")
	default:
		problems = append(problems, checkTrigger(when)...)
	}

	severity, hasSeverity := meta["Severity"]
	switch {
	case !hasSeverity:
		problems = append(problems, "missing required '**Severity:** blocking|advisory' line")
	case !isSeverity(severity):
		var tb textbuf.Buffer
		problems = append(problems, tb.Str("'**Severity:**' must be one of ").
			Str(pyListRepr(severities[:])).Str(", got '").Str(severity).Byte('\'').String())
	default:
		problems = append(problems, checkSeverityAgrees(severity, title, lines, index)...)
	}

	var canon, present []string
	for _, key := range canonKeys {
		if _, ok := meta[key]; ok {
			canon = append(canon, key)
		}
	}
	for _, key := range order {
		if isCanonKey(key) {
			present = append(present, key)
		}
	}
	if strings.Join(present, ",") != strings.Join(canon, ",") {
		var tb textbuf.Buffer
		problems = append(problems, tb.Str("metadata keys must be ordered When, Severity, Related (found ").
			Join(present, ", ").Byte(')').String())
	}

	if related := meta["Related"]; related != "" {
		for slug := range strings.SplitSeq(related, ",") {
			slug = strings.TrimSpace(slug)
			if slug == "" || relatedSlug.MatchString(slug) {
				continue
			}
			var tb textbuf.Buffer
			problems = append(problems, tb.Str("'**Related:**' entry '").Str(slug).
				Str("' must be a bare rule slug (filename without .md, no path)").String())
		}
	}

	_, hasWhen := meta["When"]
	if !hasWhen && !hasSeverity && len(problems) == 0 {
		problems = append(problems, "no metadata block found directly after the title")
	}
	return problems
}

// checkTrigger answers every violation of a `**When:**` value that cannot route
// a task.
func checkTrigger(trigger string) []string {
	var problems []string
	bare := stripMarkup(trigger)
	if bare == "" {
		return []string{"'**When:**' has no text once markup is stripped"}
	}

	// An ODD number of '**' outside code spans means one marker lost its
	// partner, which is what truncating a wrapped bold body line produces.
	// Balanced pairs are legitimate emphasis, and a glob inside backticks is
	// not a marker at all.
	if strings.Count(codeSpan.ReplaceAllString(trigger, ""), "**")%2 != 0 {
		problems = append(problems, "'**When:**' has an unbalanced '**' -- it was copied out of a wrapped "+
			"bold body line, not written as a trigger")
	}
	if endsWithAny(bare, truncatedTail[:]) {
		var tb textbuf.Buffer
		problems = append(problems, tb.Str("'**When:**' ends with ").Str(pyRepr(lastRune(bare))).
			Str(", so it is a truncated sentence rather than a complete situation").String())
	}
	words := strings.Fields(bare)
	last := ""
	if len(words) > 0 {
		trimmed := strings.Fields(strings.TrimRight(bare, "."))
		if len(trimmed) > 0 {
			last = strings.ToLower(trimmed[len(trimmed)-1])
		}
	}
	if danglingLastWord[last] {
		var tb textbuf.Buffer
		problems = append(problems, tb.Str("'**When:**' ends on ").Str(pyRepr(last)).
			Str(", so the situation is cut off mid-clause").String())
	}

	lowered := strings.ToLower(bare)
	loweredWords := strings.Fields(lowered)
	first := ""
	if len(loweredWords) > 0 {
		first = strings.Trim(loweredWords[0], ",;:.")
	}
	isGerund := strings.HasSuffix(first, "ing") &&
		utf8.RuneCountInString(first) >= 5 && !notGerund[first]
	if !isGerund && !startsWithAny(lowered, openers[:]) {
		var tb textbuf.Buffer
		problems = append(problems, tb.Str("'**When:**' must name a situation, not a directive: start it with a ").
			Str("temporal opener (when/whenever/before/after/while/if/once/during) or ").
			Str("a gerund (writing/adding/reviewing/...). Got ").Str(pyRepr(words[0])).
			Str(". See ai/rules/rule-format.md 'The trigger is a routing key'").String())
	}
	return problems
}

// checkSeverityAgrees answers every violation where the declared severity
// contradicts the prose.
//
// Table rows and quoted lines are exempt: a reference rule tabulates OTHER
// rules' severities, and those cells say nothing about this rule's own weight.
// A prose line describing another artifact's severity says so with a trailing
// severity-note comment.
func checkSeverityAgrees(severity, title string, lines []string, bodyStart int) []string {
	var problems []string
	if strings.Contains(title, "BLOCKING") {
		problems = append(problems, "the title must not say BLOCKING -- '**Severity:** blocking' carries "+
			"that, and a title marker cannot be read by tooling")
	}
	if severity != "advisory" {
		return problems
	}
	for offset, line := range lines[bodyStart:] {
		stripped := strings.TrimSpace(line)
		if strings.HasPrefix(stripped, "|") || strings.HasPrefix(stripped, ">") {
			continue
		}
		if !strings.Contains(stripped, "BLOCKING") || severityNote.MatchString(stripped) {
			continue
		}
		var tb textbuf.Buffer
		problems = append(problems, tb.Str("declares '**Severity:** advisory' but line ").
			Int(int64(bodyStart+offset+1)).Str(" says BLOCKING (").
			Str(pyRepr(firstRunes(stripped, 60))).
			Str(") -- raise the severity, drop the word, or mark the line ").
			Str("<!-- severity-note: whose severity this is -->").String())
		break
	}
	return problems
}

// checkPointFiles answers the point-file count and all RFC 2119 violations.
//
// manifest.md is not a point. A file with an all-caps stem is also not a point
// because RETIRED.md is the ledger.
func checkPointFiles(tree, pointsDir string) (int, []LintProblems, error) {
	// An ABSENT point tree gives zero points, which Lint refuses. An UNREADABLE
	// tree is an error, not a zero. The script conflates the cases. This port
	// prevents a permission failure from reporting "no points".
	info, err := os.Stat(pointsDir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return 0, nil, nil
	case err != nil:
		return 0, nil, err
	case !info.IsDir():
		return 0, nil, nil
	}

	var paths []string
	err = filepath.WalkDir(pointsDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		name := filepath.Base(path)
		if name == "manifest.md" || isUpperStem(strings.TrimSuffix(name, ".md")) {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return 0, nil, err
	}
	sort.Strings(paths)

	var violations []LintProblems
	for _, path := range paths {
		raw, err := os.ReadFile(path) // #nosec G304 -- a path this tool derived from the checkout
		if err != nil {
			return 0, nil, err
		}
		if problems := checkPoint(string(raw)); len(problems) > 0 {
			violations = append(violations, LintProblems{File: relTo(tree, path), Problems: problems})
		}
	}
	return len(paths), violations, nil
}

// checkPoint answers all violations in one point file.
//
// The pass intentionally applies only to `kind: directive`. Tables usually
// provide lookups, and notes usually provide context. Forcing MUST into a
// glossary would add a word but no obligation.
func checkPoint(text string) []string {
	found := pointMatter.FindStringSubmatch(text)
	if found == nil {
		return []string{"no '---' frontmatter block: see ai/rules/rule-format.md"}
	}
	matter, body := found[1], found[2]
	kind := pointKindLine.FindStringSubmatch(matter)
	if len(kind) < 2 || kind[1] != kindDirective {
		return nil
	}

	var problems []string
	visible := stripQuoted(body)
	tier := strongestTier(body)
	if len(tier) == 0 {
		var tb textbuf.Buffer
		problems = append(problems, tb.Str("a directive MUST state its obligation in RFC 2119 language: no ").
			Str("capitalised keyword (").Join(rfcLevels[:6], ", ").
			Str(", ...) appears in the body").String())
	}
	if lower := lowerModals(visible); len(lower) > 0 {
		quoted := make([]string, 0, len(lower))
		for _, word := range lower {
			quoted = append(quoted, pyRepr(word))
		}
		var tb textbuf.Buffer
		problems = append(problems, tb.Str("lowercase obligation word(s) ").Join(quoted, ", ").
			Str(": capitalise the RFC 2119 keyword, or rewrite the sentence so it ").
			Str("carries no modal (ai/rules/writing.md bans the hedging spelling)").String())
	}

	declared := ""
	if level := pointLevelLine.FindStringSubmatch(matter); level != nil {
		declared = strings.TrimSpace(level[1])
	}
	switch {
	case declared != "" && !isLevelRank(declared):
		var tb textbuf.Buffer
		problems = append(problems, tb.Str("'level: ").Str(declared).Str("' is not one of ").
			Join(levelRank[:], ", ").String())
	case len(tier) > 0 && !slices.Contains(tier, declared):
		shown := declared
		if shown == "" {
			shown = "(empty)"
		}
		var tb textbuf.Buffer
		problems = append(problems, tb.Str("'level: ").Str(shown).
			Str("' disagrees with the body, whose strongest obligation is ").
			Join(tier, " or ").String())
	}
	return problems
}

// strongestTier answers the levels of the strongest tier a body states, or none.
//
// A slice rather than one level: the tier is the answer, and either polarity in
// it is a true `level:`.
func strongestTier(body string) []string {
	found := map[string]bool{}
	for _, keyword := range rfcKeyword.FindAllString(stripQuoted(body), -1) {
		found[levelCanon[keyword]] = true
	}
	for _, tier := range slices.Backward(levelTiers) {
		var hit []string
		for _, level := range tier {
			if found[level] {
				hit = append(hit, level)
			}
		}
		if len(hit) > 0 {
			return hit
		}
	}
	return nil
}

// lowerModals answers sorted, unique lowercase obligation words from a body.
//
// Python uses lookbehind and lookahead, which RE2 lacks. This code instead reads
// adjacent characters. The preceding and following characters must not be word
// characters or hyphens.
func lowerModals(text string) []string {
	seen := map[string]bool{}
	for _, word := range lowerModalWords {
		for offset := 0; ; {
			at := strings.Index(text[offset:], word)
			if at < 0 {
				break
			}
			start := offset + at
			end := start + len(word)
			offset = start + 1
			if isWordOrHyphen(runeBefore(text, start)) || isWordOrHyphen(runeAt(text, end)) {
				continue
			}
			seen[word] = true
			break
		}
	}
	return sortedUnique(seen)
}

// stripQuoted drops the Markdown a point QUOTES from the obligations it states.
// A rule that reproduces another artifact does not state that artifact's
// obligations.
func stripQuoted(body string) string {
	body = stripFences(body)
	body = blockquote.ReplaceAllString(body, "")
	return codeSpan.ReplaceAllString(body, "")
}

// stripFences removes Markdown fenced blocks, fence markers included.
//
// A fence closes only on the marker character it opened with, and only on a run
// at least as long as the opening one. An opening line whose info string
// carries a backtick is not a fence at all: it is inline code.
func stripFences(text string) string {
	var out textbuf.Buffer
	fenceChar := byte(0)
	fenceLength := 0
	for _, line := range linesKeepingEnds(text) {
		bare := strings.TrimRight(line, "\r\n")
		if fenceChar != 0 {
			candidate := strings.TrimLeft(bare, " ")
			indent := len(bare) - len(candidate)
			marker := strings.TrimRight(candidate, " \t")
			if indent <= 3 && len(marker) >= fenceLength && marker == strings.Repeat(string(fenceChar), len(marker)) {
				fenceChar = 0
				fenceLength = 0
			}
			continue
		}
		if opened := fenceOpen.FindStringSubmatch(bare); opened != nil {
			mark, info := opened[1], opened[2]
			if mark[0] != '`' || !strings.Contains(info, "`") {
				fenceChar = mark[0]
				fenceLength = len(mark)
				continue
			}
		}
		out.Str(line)
	}
	return out.String()
}

// stripMarkup removes markup that a trigger can contain and keeps its words.
func stripMarkup(text string) string {
	text = strings.ReplaceAll(text, "**", "")
	text = strings.ReplaceAll(text, "`", "")
	text = strings.ReplaceAll(text, "*", "")
	return strings.TrimSpace(text)
}

// isCanonKey reports whether key is one of the three metadata keys, in the
// exact case the consumers parse.
func isCanonKey(key string) bool {
	for _, canon := range canonKeys {
		if key == canon {
			return true
		}
	}
	return false
}

// isSeverity reports whether value is one of the two severities.
func isSeverity(value string) bool {
	for _, known := range severities {
		if value == known {
			return true
		}
	}
	return false
}

// isLevelRank reports whether value is one of the five `level:` spellings.
func isLevelRank(value string) bool {
	for _, known := range levelRank {
		if value == known {
			return true
		}
	}
	return false
}

// startsWithAny reports whether text opens with any of the prefixes.
func startsWithAny(text string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

// endsWithAny reports whether text closes with any of the suffixes.
func endsWithAny(text string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(text, suffix) {
			return true
		}
	}
	return false
}

// isWordOrHyphen reports whether r is a word character or a hyphen, which is
// what the lowercase-modal pattern refuses on either side of a match.
func isWordOrHyphen(r rune) bool {
	return r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// runeBefore answers the rune ending at index, or -1 at the start of the text.
func runeBefore(text string, index int) rune {
	if index <= 0 {
		return -1
	}
	r, _ := utf8.DecodeLastRuneInString(text[:index])
	return r
}

// runeAt answers the rune starting at index, or -1 at the end of the text.
func runeAt(text string, index int) rune {
	if index >= len(text) {
		return -1
	}
	r, _ := utf8.DecodeRuneInString(text[index:])
	return r
}

// splitLines answers the text's lines for the shapes this corpus carries. This
// matches Python's str.splitlines(). A trailing newline adds no empty final
// element. An empty text has no lines.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	text = strings.TrimSuffix(text, "\n")
	return strings.Split(text, "\n")
}

// linesKeepingEnds answers the text's lines with their terminators attached,
// which is what Python's str.splitlines(keepends=True) answers.
func linesKeepingEnds(text string) []string {
	var out []string
	for text != "" {
		at := strings.IndexByte(text, '\n')
		if at < 0 {
			return append(out, text)
		}
		out = append(out, text[:at+1])
		text = text[at+1:]
	}
	return out
}
