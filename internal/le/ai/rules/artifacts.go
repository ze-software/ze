// Design: docs/architecture/core-design.md -- two generated files, one parse
// Overview: digest.go -- the parse and the core derivation these artifacts render
// Overview: actions.go -- the three actions that run these
//
// artifacts.go renders TRIGGERS.md, CORE.md, and the payload report that
// measures both files. The derivation was ported from the retired rules tool.
//
// Both artifacts come from ONE parse. A second source would drift, and a routed
// section must read identically to the digest section it replaces. The one
// exception is a core member holding a point that declares `applies`: its block
// is condensed from its points, through RenderText, because the field lives in
// the points and never reaches the rendered rule.
//
// The generated headers name `./le ai rules update condensed`, the sole producer
// of both artifacts.

package airules

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/ai/rules/ruleset"
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
// that shows it. A rule whose points name situations ends its block with one
// line naming the rule file and each situation, so a reader who meets one of
// them knows the rest of the rule is one Read away.
func ruleBlock(rule *Rule, situations []string) string {
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
	if len(situations) > 0 {
		block = append(block, "", situationsLine(rule.Path, situations))
	}
	return strings.TrimRight(strings.Join(block, "\n"), asciiSpace)
}

// maxSituationsLine bounds the line a core block ends with when points of its
// rule declare `applies`. Every session loads it, like a trigger row, but it
// names one situation per marked point, so its room is two trigger rows
// (maxTriggerLine) rather than one. A line over it is refused, never cut: a
// cut situation is a pointer an agent cannot match.
const maxSituationsLine = 2 * maxTriggerLine

// situationsLine answers the line naming a rule file and the situations its
// marked points apply in.
func situationsLine(path string, situations []string) string {
	var tb textbuf.Buffer
	return tb.Str("**Read `").Str(path).Str("` before acting when:** ").Join(situations, "; ").String()
}

// coreView is what the core shows of one always-on rule whose points name
// situations: the rule body without those points, and the situations.
type coreView struct {
	body       []string
	situations []string
}

// coreViews answers the view of every always-on rule that holds a point
// declaring `applies`, keyed by the rule's file name. A rule absent from the
// answer is shown whole, from its rendered file, exactly as it was before the
// field existed.
//
// The body is rendered from the points rather than cut out of the rendered
// file, because the points are where the field lives and RenderText is the one
// function that turns points into rule text. Every rule's points are read, so
// `applies` on a routed rule's point is refused rather than ignored.
func coreViews(pointsRoot string, core map[string]bool) (map[string]coreView, error) {
	views := map[string]coreView{}
	for _, dir := range pointDirs(pointsRoot) {
		header, sections, err := readRulePoints(dir)
		if err != nil {
			return nil, err
		}
		stem := filepath.Base(dir)
		var tb textbuf.Buffer
		name := tb.Str(stem).Str(".md").String()
		kept, situations, err := situate(stem, sections, core[name])
		if err != nil {
			return nil, err
		}
		if len(situations) == 0 {
			continue
		}
		tb.Reset()
		path := tb.Str(rulesRel).Byte('/').Str(name).String()
		if length := runeLen(situationsLine(path, situations)); length > maxSituationsLine {
			tb.Reset()
			return nil, errors.New(tb.Str(name).Str(": the core line naming the situations its points apply in is ").
				Int(int64(length)).Str(" characters, over the ").Int(maxSituationsLine).
				Str("-character bound. Fix: shorten the `applies` situations of its points, or remove an `applies` line").String())
		}
		_, _, body := parseRule(splitLines(RenderText(header, kept)))
		views[name] = coreView{body: body, situations: situations}
	}
	return views, nil
}

// triggerLines answers one routing row per rule: path, severity, trigger. A row
// never exceeds maxTriggerLine, and a trigger that does not fit is refused, never
// cut: a cut trigger drops the situations past the cut, so the index agents
// route by would answer a narrower trigger than the rule declares, with nothing
// to say so. The author shortens the trigger at its source instead.
//
// A core rule is marked `always-on`, because the index's own header promises
// the reader can tell which bodies are already loaded. Without the marker that
// sentence is a claim the table does not support.
//
// A rule with no trigger still gets a line. Dropping it would make the one
// thing this index promises -- that every rule stays named -- untrue for
// exactly the rules that are hardest to route.
func triggerLines(rules []Rule, core map[string]bool) ([]string, error) {
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
			return nil, triggerTooLong(rule.Stem, runeLen(trigger), room)
		}
		lines = append(lines, tb.Str(prefix).Str(trigger).Str(" |").String())
	}
	return lines, nil
}

// triggerTooLong names the rule whose trigger does not fit its routing row, by
// how much, and where to shorten it.
func triggerTooLong(stem string, length, room int) error {
	var tb textbuf.Buffer
	return errors.New(tb.Str("the trigger of rule ").Str(stem).Str(" is ").Int(int64(length)).
		Str(" characters, and its TRIGGERS.md row has room for ").Int(int64(room)).
		Str(": the index refuses to cut it, because a cut trigger drops the situations past the cut.").
		Str(" Fix: shorten the `when:` line in ").Str(rulesRel).Str("/points/").Str(stem).
		Str("/manifest.md, keeping every situation it names, then run ./le ai rules update condensed").String())
}

// triggersHeader is TRIGGERS.md above its first row.
var triggersHeader = []string{
	"# Ze Rules -- Trigger Index",
	"",
	"<!-- GENERATED by ./le ai rules update condensed -- do not edit -->",
	"<!-- Regenerate: ./le ai rules update condensed -->",
	"",
	"Every rule under `ai/rules/`, one line each. When a trigger matches the work",
	"in hand, READ that rule's file before acting. A row marked `always-on` is",
	"already loaded (`ai/rules/CORE.md`) and needs no read, except that its core",
	"block may end with a line naming situations: the points for those situations",
	"are in the rule file, so read it when one of them matches. Every other rule's",
	"body is one Read away at the path in its row.",
	"",
	"| Rule | Severity | When to read it |",
	"|------|----------|-----------------|",
}

// coreHeader is CORE.md above its first block, minus the derived reason line.
var coreHeader = []string{
	"# Ze Rules -- Always-On Core",
	"",
	"<!-- GENERATED by ./le ai rules update condensed -- do not edit -->",
	"<!-- Regenerate: ./le ai rules update condensed -->",
	"",
	"The rules that apply before the shape of a task is known, so they are never",
	"reached through a trigger. Membership is DERIVED, never listed here: the",
	"ladder in `ai/rules/rule-precedence.md` names rungs 1 and 2, and rename a rule",
	"there and this file follows. A rule with no routable trigger lands here too,",
	"rather than being dropped from both artifacts, and so does a blocking rule",
	"that no past task description in `plan/` would surface",
	"(`./le ai rules report router` prints that set and the corpus it read).",
	"",
	"A block shows every point of its rule except a point that declares the",
	"situation it applies in. Such a block ends with a line naming the rule file",
	"and each situation: when one matches, read that file before acting.",
	"",
	"Every other rule is named in `ai/rules/TRIGGERS.md`. Read its file when its",
	"trigger matches.",
	"",
}

// buildTriggers answers TRIGGERS.md: every rule named, in one line each.
func buildTriggers(rules []Rule, core map[string]bool) (string, error) {
	rows, err := triggerLines(rules, core)
	if err != nil {
		return "", err
	}
	lines := append(append([]string{}, triggersHeader...), rows...)
	var tb textbuf.Buffer
	return tb.Str(strings.Join(lines, "\n")).Byte('\n').String(), nil
}

// buildCore answers CORE.md: the directives that must reach every session
// unconditionally.
//
// The header states no corpus SIZE or rule COUNT. These derived values caused a
// rewrite when a commit changed no core rule. Such a rewrite left `--check` red
// with no relevant author action. The header keeps only membership, which
// changes when the core changes.
//
// A member named in views is condensed from the view's body and ends with its
// situations; every other member is condensed from its rendered file.
func buildCore(core []Rule, views map[string]coreView) string {
	reasons := make(map[string]bool, len(core))
	for i := range core {
		reasons[core[i].CoreReason] = true
	}
	ordered := make([]string, 0, len(reasons))
	for reason := range reasons {
		ordered = append(ordered, reason)
	}
	slices.Sort(ordered)

	var tb textbuf.Buffer
	header := append(append([]string{}, coreHeader...),
		tb.Str("Core reasons: ").Join(ordered, ", ").Byte('.').String(), "")
	tb.Reset()

	blocks := make([]string, 0, len(core))
	for i := range core {
		member := core[i]
		view, situated := views[member.Name]
		if situated {
			member.Body = view.body
		}
		blocks = append(blocks, tb.Str(ruleBlock(&member, view.situations)).
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
				Str(" is stale -- run: ./le ai rules update condensed\n")
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

	alwaysOn := coreNames(core)
	triggers, err := buildTriggers(rules, alwaysOn)
	if err != nil {
		return report, err
	}
	views, err := coreViews(pointsDir(tree), alwaysOn)
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
		{triggersFile, len(rules), triggers},
		{coreFile, len(core), buildCore(core, views)},
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

// PayloadPart is one file of a measured payload, or one part of a file.
type PayloadPart struct {
	File string `json:"file"`
	// Part is the heading line that opens the measured part of File, or empty
	// when the whole file is measured.
	Part   string `json:"part,omitempty"`
	Chars  int    `json:"chars"`
	Tokens int    `json:"tokens"`
	// Missing says that the file, or the heading that opens Part, was absent.
	// The script counts an absent file as zero characters. Thus, a payload
	// without CORE.md appears further under budget than a complete payload.
	Missing bool `json:"missing"`
}

// PayloadSet is one file set measured against its own budget.
type PayloadSet struct {
	Parts    []PayloadPart `json:"parts"`
	Chars    int           `json:"chars"`
	Tokens   int           `json:"tokens"`
	Budget   int           `json:"budget"`
	Met      bool          `json:"met"`
	Headroom float64       `json:"headroom-percent"`
	// Missing names every absent part. A reader can use the verdict only when it
	// measures a complete set.
	Missing []string `json:"missing"`
}

// PayloadReport describes what a session loads, the instructions, the routing
// index and the always-on core, and beside it the agent rule set a phase agent
// starts with and the decisions sections it reads only before one kind of work.
type PayloadReport struct {
	PayloadSet
	Agent       PayloadSet     `json:"agent-rule-set"`
	Situational SituationalSet `json:"situational-decisions"`
}

// SituationalSet measures the sections of ai/META-DECISIONS.md an agent reads
// only before the work each one names. It has no budget and is no part of the
// agent rule set's total, because no agent always loads it; a main session
// does, inside the whole file the always-loaded payload measures.
type SituationalSet struct {
	Parts  []PayloadPart `json:"parts"`
	Chars  int           `json:"chars"`
	Tokens int           `json:"tokens"`
	// Missing names why the sections cannot be read. The start hook then tells
	// an agent to read the whole decisions file, so the agent rule set's total
	// understates what that agent reads.
	Missing []string `json:"missing"`
}

// Failed reports whether any measurement is missing one of the parts it
// measures.
//
// The script always answers 0, and an absent file contributes zero characters.
// Thus, deletion of CORE.md makes the budget look MET. An absent payload is not
// a smaller payload.
func (r PayloadReport) Failed() bool {
	if len(r.Missing) > 0 {
		return true
	}
	if len(r.Agent.Missing) > 0 {
		return true
	}
	return len(r.Situational.Missing) > 0
}

// Text renders the report: the main payload in the shape the script printed,
// then the agent rule set in the same shape, then the situational sections.
func (r PayloadReport) Text() string {
	var tb textbuf.Buffer
	r.PayloadSet.text(&tb, "always-loaded payload", "what a session loads")
	r.Agent.text(&tb, "agent rule set", "what a phase agent starts with")
	r.Situational.text(&tb)
	return tb.String()
}

// text renders the situational sections, one line each, and their total on a
// line of its own.
func (s SituationalSet) text(tb *textbuf.Buffer) {
	tb.Str("situational decisions (read before the work each names, outside the agent rule set):\n")
	for _, part := range s.Parts {
		tb.Str("  ").Str(part.File).Str(" (").Str(part.Part).Str("): ").
			Int(int64(part.Chars)).Str(" chars (").Int(int64(part.Tokens)).Str(" tokens)\n")
	}
	tb.Str("  TOTAL: ").Int(int64(s.Chars)).Str(" chars (").Int(int64(s.Tokens)).Str(" tokens)\n")
	if len(s.Missing) > 0 {
		tb.Str("WARNING: the situational decisions cannot be read, so an agent is told to read the whole decisions file: ").
			Join(s.Missing, ", ").Byte('\n')
	}
}

// text renders one measured set under its title. What names what the set
// stands for, in the warning an incomplete set prints.
func (s PayloadSet) text(tb *textbuf.Buffer, title, what string) {
	tb.Str(title).Str(":\n")
	for _, part := range s.Parts {
		tb.Str("  ").Str(part.File)
		if part.Part != "" {
			tb.Str(" (").Str(part.Part).Byte(')')
		}
		tb.Str(": ").Int(int64(part.Chars)).Str(" chars (").Int(int64(part.Tokens)).Str(" tokens)\n")
	}
	tb.Str("  TOTAL: ").Int(int64(s.Chars)).Str(" chars (").Int(int64(s.Tokens)).Str(" tokens)\n")
	verdict := "EXCEEDED"
	if s.Met {
		verdict = "MET"
	}
	tb.Str("  budget: ").Int(int64(s.Budget)).Str(" tokens -- ").Str(verdict).Byte('\n')
	tb.Str("  headroom: ").Float(s.Headroom, 1).Str("%\n")
	if len(s.Missing) > 0 {
		tb.Str("WARNING: the ").Str(title).Str(" is incomplete, so the total above measures a set that "+
			"is not ").Str(what).Str(": ").Join(s.Missing, ", ").Byte('\n')
	}
}

// payloadParts names the four files a session loads before it reads anything
// else: ai/INSTRUCTIONS.md, which becomes AGENTS.md, and the three files its
// `@` lines include. A new `@` line in ai/INSTRUCTIONS.md owes an entry here.
var payloadParts = [...]ruleset.Reader{
	ruleset.WholeFile(ruleset.InstructionsFile), ruleset.WholeFile(ruleset.DecisionsFile),
	ruleset.WholeFile(ruleset.TriggersFile), ruleset.WholeFile(ruleset.CoreFile),
}

// agentRuleSetParts names what a phase agent starts with in place of the main
// session's files: the hard bans of ai/INSTRUCTIONS.md, the every-agent part of
// ai/META-DECISIONS.md, the trigger index and the core. It is read from the
// main session's own files, so there is no second copy to keep fresh.
var agentRuleSetParts = [...]ruleset.Reader{
	ruleset.HardBans, ruleset.AgentDecisions,
	ruleset.WholeFile(ruleset.TriggersFile), ruleset.WholeFile(ruleset.CoreFile),
}

// agentTokenBudget is what the agent rule set must come in under: the core at
// its 24KB ceiling (6,000 tokens), the trigger index (about 1,400), the hard
// bans (about 650), and about 3,950 tokens for the every-agent decisions.
const agentTokenBudget = 12000

// readAgentRuleSet reads the agent rule set from the checkout at tree, part by
// part. A part whose file or heading is absent is answered Missing with no
// text, never as an empty part: a caller that joins the texts MUST check
// Missing, because an absent part is not a shorter one.
func readAgentRuleSet(tree string) ([]ruleset.Part, error) {
	return ruleset.Read(tree, agentRuleSetParts[:])
}

// countable refuses a part whose text is not valid UTF-8, because its size in
// characters cannot be counted.
func countable(parts []ruleset.Part) error {
	for _, part := range parts {
		if !utf8.ValidString(part.Text) {
			var tb textbuf.Buffer
			return errors.New(tb.Str(part.File).Str(" is not valid UTF-8, so its size in characters cannot be counted").String())
		}
	}
	return nil
}

// measure counts each part against budget.
func measure(parts []ruleset.Part, budget int) PayloadSet {
	set := PayloadSet{Budget: budget, Parts: make([]PayloadPart, 0, len(parts))}
	for _, read := range parts {
		part := PayloadPart{File: read.File, Part: read.Heading, Missing: read.Missing}
		if read.Missing {
			set.Missing = append(set.Missing, read.Reason)
		}
		part.Chars = runeLen(read.Text)
		part.Tokens = part.Chars / bytesPerToken
		set.Chars += part.Chars
		set.Parts = append(set.Parts, part)
	}
	set.Tokens = set.Chars / bytesPerToken
	set.Met = set.Tokens < budget
	set.Headroom = 100.0 * float64(budget-set.Tokens) / float64(budget)
	return set
}

// Payload measures what the always-loaded payload costs against its budget,
// and what the agent rule set costs against its own.
func Payload(tree string) (PayloadReport, error) {
	session, err := ruleset.Read(tree, payloadParts[:])
	if err != nil {
		return PayloadReport{}, err
	}
	if err := countable(session); err != nil {
		return PayloadReport{}, err
	}
	agent, err := readAgentRuleSet(tree)
	if err != nil {
		return PayloadReport{}, err
	}
	if err := countable(agent); err != nil {
		return PayloadReport{}, err
	}
	situational, err := measureSituational(tree)
	if err != nil {
		return PayloadReport{}, err
	}
	return PayloadReport{
		PayloadSet:  measure(session, tokenBudget),
		Agent:       measure(agent, agentTokenBudget),
		Situational: situational,
	}, nil
}

// measureSituational counts each section ruleset.SituationalDecisions answers.
// A part or section it cannot place is answered Missing with its reason, never
// as an empty set; any other failure stops the report, because the set cannot
// be measured.
func measureSituational(tree string) (SituationalSet, error) {
	sections, err := ruleset.SituationalDecisions(tree)
	if err != nil {
		reason, absent := ruleset.MissingReason(err, ruleset.DecisionsFile)
		if !absent {
			return SituationalSet{}, err
		}
		return SituationalSet{Missing: []string{reason}}, nil
	}
	set := SituationalSet{Parts: make([]PayloadPart, 0, len(sections))}
	for _, section := range sections {
		if err := countable([]ruleset.Part{section.Part}); err != nil {
			return SituationalSet{}, err
		}
		part := PayloadPart{File: section.File, Part: section.Heading, Chars: runeLen(section.Text)}
		part.Tokens = part.Chars / bytesPerToken
		set.Chars += part.Chars
		set.Parts = append(set.Parts, part)
	}
	set.Tokens = set.Chars / bytesPerToken
	return set, nil
}
