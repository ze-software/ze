// VALIDATES: AC-22 of spec-verify-release-check-only -- no agent-facing surface
// tells an agent to run a whole-tree verification. Every sentence, table row or
// code line in `ai/`, `AGENTS.md`, `.claude/` and the rest of the corpus below
// that names a whole-tree verb is one a reviewer has read and listed in
// testdata/wholetree-mentions.txt.
// PREVENTS: a skill, a rule, a pattern or a plan that still tells an agent to
// run `./le verify current` or `./le verify worktree`, which the terminal gate
// then refuses, so the agent loses the turn or works around the gate.
//
// The ledger replaces a regex classifier that tried to tell, from the words of
// an English sentence, whether it tells an agent to run a command. That cannot
// be done soundly, and it failed open twice. A reviewed ratchet can: a new
// mention is refused until a reader judges it, and a listed mention that no
// longer occurs is refused too, so the ledger cannot rot.
//
// THE RULE FOR ADDING AN ENTRY: a reviewer reads the sentence in its file, and
// lists it only when it does not tell an agent to run the verb. A sentence that
// bans the run, ties it to Thomas, or describes the release check qualifies.
// One that tells an agent to run the verb, in any wording, is rewritten
// instead: an agent's evidence is `./le verify changed`, and the full run is the
// command the agent hands Thomas.
//
// It runs in package le because the verb list is DERIVED from the action tables
// the blank imports in register.go populate: every action that registers with
// WholeTree set is a verb, and an area whose every action is WholeTree is one as
// a whole (`verify evidence`). A new whole-tree verb is refused here with no
// edit to this file.
package le

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/gitenv"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// agentCorpus is the set of trees this scan judges, and exactly that set: the
// shared rules, patterns, skills and INSTRUCTIONS.md under ai/, the generated
// AGENTS.md, the Claude copies and plans under .claude/, the spec templates
// under plan/templates/, the contributor pages under docs/contributing/, and
// docs/functional-tests.md, the test-running page the tooling door sends an
// agent to. The rest of docs/ (architecture, guide) and the specs under plan/
// are not judged. Only files git tracks are read: a gitignored derived file
// (ai/RFC-REQUIREMENTS.md, rebuilt from internal/le/rfc/carriers.go) is judged
// at its Go source, and a file another session has not committed is not yet a
// surface any agent inherits.
var agentCorpus = []string{"ai", "AGENTS.md", ".claude", "plan/templates", "docs/contributing", "docs/functional-tests.md"}

// mentionLedger is the reviewed list of every whole-tree mention the corpus
// holds, one `<path> :: <normalized text>` entry per line, relative to this
// package. A line opening with # is a comment.
var mentionLedger = filepath.Join("testdata", "wholetree-mentions.txt")

// ledgerSeparator splits an entry's corpus path from its text.
const ledgerSeparator = " :: "

// sentenceEnd ends a sentence: a terminal mark, any closing emphasis, code or
// quote mark, then a space or the end of the paragraph.
var sentenceEnd = regexp.MustCompile("[.!?][*_`\"')\\]]*(\\s|$)")

// paragraphBreak starts a new unit of prose: a list item, a heading or a
// checklist box. A line that matches it never joins the paragraph above.
var paragraphBreak = regexp.MustCompile(`^\s*([-*+]\s|\d+\.\s|#|\[ \])`)

// TestNoAgentSurfaceTellsAnAgentToRunAWholeTreeVerify reads the verb list from
// the registered action tables, walks the corpus, and holds every mention it
// finds against the reviewed ledger: an unlisted mention is refused, and so is
// a listed entry that no longer occurs.
func TestNoAgentSurfaceTellsAnAgentToRunAWholeTreeVerify(t *testing.T) {
	named := wholeTreePattern(wholeTreeVerbs(t))
	ledger, err := os.ReadFile(mentionLedger)
	if err != nil {
		t.Fatalf("read the reviewed ledger: %v", err)
	}
	listed, err := parseMentionLedger(string(ledger))
	if err != nil {
		t.Fatalf("%s: %v", mentionLedger, err)
	}

	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find checkout: %v", err)
	}
	listing := exec.CommandContext(t.Context(), "git", append([]string{"ls-files", "-z", "--"}, agentCorpus...)...)
	listing.Dir = root
	listing.Env = gitenv.ReadOnly(os.Environ())
	output, err := listing.Output()
	if err != nil {
		t.Fatalf("list the tracked agent corpus: %v", err)
	}
	read := 0
	var found []mention
	for relative := range strings.SplitSeq(string(output), "\x00") {
		if !corpusText(relative) {
			continue
		}
		content, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if errors.Is(readErr, fs.ErrNotExist) {
			// Deleted in the working tree and not yet committed.
			continue
		}
		if readErr != nil {
			t.Fatalf("read %s: %v", relative, readErr)
		}
		read++
		found = append(found, corpusMentions(relative, string(content), named)...)
	}
	if read == 0 {
		t.Fatalf("git tracks no markdown under %v, so the scan read nothing", agentCorpus)
	}
	reportJudgement(t, found, listed)
}

// TestWholeTreeMentionsRefusesAnUnlistedMention proves a mention nobody
// reviewed is refused, whatever its wording: an imperative, a sentence that
// names Thomas, a ban, a fenced command and a table row each fail against an
// empty ledger, and each reports the line its verb starts on.
func TestWholeTreeMentionsRefusesAnUnlistedMention(t *testing.T) {
	named := wholeTreePattern([]string{"verify current", "verify worktree", "verify evidence"})
	cases := []struct {
		name string
		text string
		line int
	}{
		{"imperative", "Before closing, run `./le verify worktree`.\n", 1},
		{"names Thomas", "Some prose.\n\nThomas runs `./le\nverify current` at his terminal.\n", 3},
		{"ban", "Never run `./le verify worktree` from a tool call.\n", 1},
		{"fenced command", "Thomas runs this.\n\n```bash\n./le verify current fast\n```\n", 4},
		{"table row", "| Release | `./le verify evidence release-candidate` |\n", 1},
	}
	for _, tc := range cases {
		unlisted, stale := judgeMentions(corpusMentions("ai/x.md", tc.text, named), nil)
		if len(unlisted) != 1 || unlisted[0].line != tc.line {
			t.Errorf("%s: unlisted %v, want one mention on line %d", tc.name, unlisted, tc.line)
		}
		if len(stale) != 0 {
			t.Errorf("%s: stale %v, want none", tc.name, stale)
		}
	}
	scoped := corpusMentions("ai/x.md", "Run `./le verify changed`.\n", named)
	if len(scoped) != 0 {
		t.Errorf("a scoped verb is no whole-tree mention, got %v", scoped)
	}
}

// TestWholeTreeMentionsRefusesAStaleEntry proves a listed entry the corpus no
// longer holds is refused, so a rewritten or deleted sentence takes its entry
// with it.
func TestWholeTreeMentionsRefusesAStaleEntry(t *testing.T) {
	named := wholeTreePattern([]string{"verify current"})
	listed, err := parseMentionLedger("# reviewed\n\nai/x.md :: Thomas runs `./le verify current`.\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	unlisted, stale := judgeMentions(corpusMentions("ai/x.md", "Nothing names the verb now.\n", named), listed)
	if len(unlisted) != 0 {
		t.Errorf("unlisted %v, want none", unlisted)
	}
	if !slices.Equal(stale, []string{"ai/x.md :: Thomas runs `./le verify current`."}) {
		t.Errorf("stale %v, want the one listed entry", stale)
	}
}

// TestWholeTreeMentionsPassesAListedMention proves a reviewed mention passes:
// a wrapped sentence is listed once with its whitespace normalized, and a
// fenced line by its own text. A duplicate or malformed ledger line is refused.
func TestWholeTreeMentionsPassesAListedMention(t *testing.T) {
	named := wholeTreePattern([]string{"verify current"})
	text := "The release check is `./le verify\ncurrent`, which an agent never runs.\n\n```sh\n./le verify current\n```\n"
	listed, err := parseMentionLedger("ai/x.md :: The release check is `./le verify current`, which an agent never runs.\n" +
		"ai/x.md :: ./le verify current\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	unlisted, stale := judgeMentions(corpusMentions("ai/x.md", text, named), listed)
	if len(unlisted) != 0 || len(stale) != 0 {
		t.Errorf("unlisted %v, stale %v, want neither", unlisted, stale)
	}
	if _, err := parseMentionLedger("ai/x.md :: one\nai/x.md :: one\n"); err == nil {
		t.Error("a duplicate entry was accepted")
	}
	if _, err := parseMentionLedger("no separator here\n"); err == nil {
		t.Error("an entry with no separator was accepted")
	}
}

// reportJudgement fails the test once per unlisted mention and once per stale
// entry, each with the line a reviewer acts on.
func reportJudgement(t *testing.T, found []mention, listed []string) {
	t.Helper()
	unlisted, stale := judgeMentions(found, listed)
	for _, m := range unlisted {
		t.Errorf("%s:%d names %q in text the reviewed ledger %s does not list. "+
			"If it tells an agent to run the verb, rewrite it: the agent's evidence is `./le verify changed`, "+
			"and the full run is the command it hands Thomas. If it bans the run, ties it to Thomas or "+
			"describes the release check, a reviewer adds this entry:\n%s",
			m.path, m.line, m.verb, mentionLedger, m.entry())
	}
	for _, entry := range stale {
		t.Errorf("%s lists an entry the corpus no longer holds; remove it:\n%s", mentionLedger, entry)
	}
}

// mention is one sentence, table row or code line of the corpus that names a
// whole-tree verb, with the line the verb starts on.
type mention struct {
	path string
	line int
	verb string
	text string
}

// entry is the ledger line that lists this mention.
func (m mention) entry() string {
	return m.path + ledgerSeparator + m.text
}

// String renders a mention for a test failure.
func (m mention) String() string {
	return fmt.Sprintf("%s:%d %q", m.path, m.line, m.text)
}

// parseMentionLedger answers the entries of a ledger, refusing a line with no
// path or no text and an entry listed twice: two copies of one fact would let
// one go stale behind the other.
func parseMentionLedger(content string) ([]string, error) {
	var entries []string
	for index, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		path, text, found := strings.Cut(trimmed, ledgerSeparator)
		if !found {
			return nil, fmt.Errorf("line %d has no %q between path and text: %s", index+1, ledgerSeparator, trimmed)
		}
		if strings.TrimSpace(path) == "" {
			return nil, fmt.Errorf("line %d names no path: %s", index+1, trimmed)
		}
		if strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("line %d carries no text: %s", index+1, trimmed)
		}
		if slices.Contains(entries, trimmed) {
			return nil, fmt.Errorf("line %d lists an entry twice: %s", index+1, trimmed)
		}
		entries = append(entries, trimmed)
	}
	return entries, nil
}

// judgeMentions answers each distinct mention whose entry the ledger does not
// list, and each listed entry no mention matches.
func judgeMentions(found []mention, listed []string) (unlisted []mention, stale []string) {
	occurs := map[string]bool{}
	for _, m := range found {
		entry := m.entry()
		if occurs[entry] {
			continue
		}
		occurs[entry] = true
		if !slices.Contains(listed, entry) {
			unlisted = append(unlisted, m)
		}
	}
	for _, entry := range listed {
		if !occurs[entry] {
			stale = append(stale, entry)
		}
	}
	return unlisted, stale
}

// wholeTreeVerbs answers every registered whole-tree verb as the words after
// `le`: each WholeTree action as "<area> <verb>", and an area whose every action
// is WholeTree as "<area>".
func wholeTreeVerbs(t *testing.T) []string {
	t.Helper()
	var verbs []string
	for _, tool := range commandsAtStart {
		list, declared := leroot.ActionsOf(tool.Name)
		if !declared || len(list.Actions) == 0 {
			continue
		}
		every := true
		for _, row := range list.Actions {
			if !row.WholeTree {
				every = false
				continue
			}
			verbs = append(verbs, list.Area+" "+row.Verb)
		}
		if every {
			verbs = append(verbs, list.Area)
		}
	}
	if len(verbs) == 0 {
		t.Fatal("no registered action is WholeTree, so the corpus scan would refuse nothing")
	}
	return verbs
}

// wholeTreePattern matches any verb after `le`, the longest first, so a line
// naming `verify evidence release-candidate` reports the whole verb.
func wholeTreePattern(verbs []string) *regexp.Regexp {
	ordered := slices.Clone(verbs)
	slices.SortFunc(ordered, func(a, b string) int { return len(b) - len(a) })
	alternatives := make([]string, 0, len(ordered))
	for _, verb := range ordered {
		words := strings.Fields(verb)
		for i, word := range words {
			words[i] = regexp.QuoteMeta(word)
		}
		alternatives = append(alternatives, strings.Join(words, `\s+`))
	}
	return regexp.MustCompile(`\ble\s+(` + strings.Join(alternatives, "|") + `)\b`)
}

// corpusText answers whether the file is prose an agent reads: markdown, or a
// file with no extension (the .claude/plan files).
func corpusText(path string) bool {
	extension := filepath.Ext(path)
	if path == "" {
		// The NUL that ends the last git ls-files entry leaves an empty field.
		return false
	}
	return extension == ".md" || extension == ""
}

// corpusMentions answers each whole-tree verb in text with the unit that names
// it: the whole sentence for prose, after the wrapped lines of its paragraph
// are joined, and the whole line for a code-block line or a table row. The
// text is whitespace-normalized, so rewrapping a paragraph does not stale its
// entry.
func corpusMentions(path, text string, named *regexp.Regexp) []mention {
	var out []mention
	for _, unit := range units(text) {
		for _, match := range named.FindAllStringSubmatchIndex(unit.text, -1) {
			span := unit.text
			if !unit.whole {
				span = sentenceAround(unit.text, match[0], match[1])
			}
			out = append(out, mention{
				path: path,
				line: unit.line + strings.Count(unit.text[:match[0]], "\n"),
				verb: "le " + strings.Join(strings.Fields(unit.text[match[2]:match[3]]), " "),
				text: strings.Join(strings.Fields(span), " "),
			})
		}
	}
	return out
}

// textUnit is a paragraph of prose with its wrapped lines joined, or one
// code-block line or table row, with the line it starts on. whole marks a unit
// that is listed as one line rather than split into sentences.
type textUnit struct {
	line  int
	text  string
	whole bool
}

// units splits markdown into the units corpusMentions reads.
func units(text string) []textUnit {
	var out []textUnit
	var paragraph []string
	start := 0
	flush := func() {
		if len(paragraph) > 0 {
			out = append(out, textUnit{line: start, text: strings.Join(paragraph, "\n")})
			paragraph = nil
		}
	}
	fenced := false
	for index, line := range strings.Split(text, "\n") {
		number := index + 1
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			flush()
			fenced = !fenced
			continue
		}
		if fenced || strings.HasPrefix(trimmed, "|") {
			flush()
			out = append(out, textUnit{line: number, text: line, whole: true})
			continue
		}
		if trimmed == "" {
			flush()
			continue
		}
		if paragraphBreak.MatchString(line) {
			flush()
		}
		if len(paragraph) == 0 {
			start = number
		}
		paragraph = append(paragraph, line)
	}
	flush()
	return out
}

// sentenceAround answers the sentence of paragraph holding [from, to): from the
// end of the sentence before it to the end of its own.
func sentenceAround(paragraph string, from, to int) string {
	begin := 0
	for _, end := range sentenceEnd.FindAllStringIndex(paragraph[:from], -1) {
		begin = end[1]
	}
	finish := len(paragraph)
	if next := sentenceEnd.FindStringIndex(paragraph[to:]); next != nil {
		finish = to + next[1]
	}
	return paragraph[begin:finish]
}
