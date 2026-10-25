// VALIDATES: AC-22 of spec-verify-release-check-only -- no agent-facing surface
// tells an agent to run a whole-tree verification. Every sentence in `ai/`,
// `AGENTS.md` and `.claude/` that names a whole-tree verb either says the run is
// Thomas's or bans running it.
// PREVENTS: a skill, a rule, a pattern or a plan that still tells an agent to
// run `./le verify current` or `./le verify worktree`, which the terminal gate
// then refuses, so the agent loses the turn or works around the gate.
//
// It runs in package le because the verb list is DERIVED from the action tables
// the blank imports in register.go populate: every action that registers with
// WholeTree set is a verb, and an area whose every action is WholeTree is one as
// a whole (`verify evidence`). A new whole-tree verb is refused here with no
// edit to this file.
package le

import (
	"errors"
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

// ownedOrBanned is what a prose sentence naming a whole-tree verb must carry: a
// form that ties the run to Thomas, or a form that bans an agent running it. A
// bare "Thomas" or a bare ban word is not enough, because "Thomas says run it"
// and "run it; do not skip it" are both instructions to run.
var ownedOrBanned = regexp.MustCompile("(?i)" +
	`\b(give|gives|hand|hands)\s+thomas\b` +
	`|\bthomas\s+(runs|starts)\b` +
	`|\bthomas's\s+release\s+check\b` +
	`|\bnever\s+(run|runs|start|starts)\b` +
	`|\b(must|do)\s+not\s+(run|start)\b` +
	`|\b(run|runs)\s+no\b` +
	`|\brefuses\b`)

// sentenceEnd ends a sentence: a terminal mark, any closing emphasis, code or
// quote mark, then a space or the end of the paragraph.
var sentenceEnd = regexp.MustCompile("[.!?][*_`\"')\\]]*(\\s|$)")

// paragraphBreak starts a new unit of prose: a list item, a heading or a
// checklist box. A line that matches it never joins the paragraph above.
var paragraphBreak = regexp.MustCompile(`^\s*([-*+]\s|\d+\.\s|#|\[ \])`)

// TestNoAgentSurfaceTellsAnAgentToRunAWholeTreeVerify reads the verb list from
// the registered action tables, then walks the corpus. Prose is judged one
// sentence at a time, after the wrapped lines of its paragraph are joined, so a
// sentence that names Thomas on its next line is judged whole. A code-block line
// and a table row are each a unit of their own: a command in a fence is an
// instruction, whatever the prose around it says.
func TestNoAgentSurfaceTellsAnAgentToRunAWholeTreeVerify(t *testing.T) {
	verbs := wholeTreeVerbs(t)
	named := wholeTreePattern(verbs)

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
		for _, hit := range unbannedRuns(string(content), named) {
			t.Errorf("%s:%d tells an agent to run %q, a whole-tree verification only Thomas starts at a terminal: "+
				"name `./le verify changed` as the agent's evidence, and give the full run as the command the agent hands Thomas",
				relative, hit.line, hit.phrase)
		}
	}
	if read == 0 {
		t.Fatalf("git tracks no markdown under %v, so the scan read nothing", agentCorpus)
	}
}

// TestUnbannedRunsJudgesTheSentence holds the scanner to its answers, so the
// corpus test above cannot pass by reading nothing: a bare instruction is
// refused; one that ties the run to Thomas or bans it is not; a sentence that
// merely names Thomas, or bans something other than the run, is refused; and a
// fenced command is refused even when the paragraph above names Thomas or its
// own comment says never.
func TestUnbannedRunsJudgesTheSentence(t *testing.T) {
	named := wholeTreePattern([]string{"verify current", "verify worktree", "verify evidence"})
	cases := []struct {
		name string
		text string
		want []int
	}{
		{"fenced instruction", "Thomas runs the release check.\n\n```bash\n./le verify current fast   # in place\n```\n", []int{4}},
		{"wrapped sentence names Thomas", "The full run (`./le verify current`) is a\nrelease check Thomas runs.\n", nil},
		{"ban", "Never run `./le verify worktree` from a tool call.\n", nil},
		{"no list", "You run no command that can take longer: no `./le verify worktree`.\n", nil},
		{"imperative", "Before closing, run `./le verify worktree`.\nThomas reads the report.\n", []int{1}},
		{"area verb", "- Run `./le verify evidence release-candidate`.\n", []int{1}},
		{"scoped verb", "Run `./le verify changed`.\n", nil},
		{"Thomas named but not the runner", "Thomas says run ./le verify worktree before you commit.\n", []int{1}},
		{"ban word on another verb", "Run ./le verify current now; do not skip it.\n", []int{1}},
		{"fenced comment is no ban", "```bash\n./le verify current   # never skip this\n```\n", []int{2}},
		{"agent never runs", "The release check is `./le verify current`, which an agent never runs.\n", nil},
	}
	for _, tc := range cases {
		var got []int
		for _, hit := range unbannedRuns(tc.text, named) {
			got = append(got, hit.line)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: refused lines %v, want %v", tc.name, got, tc.want)
		}
	}
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

// runHit is one sentence that names a whole-tree verb and neither names Thomas
// nor bans the run.
type runHit struct {
	line   int
	phrase string
}

// unbannedRuns answers each whole-tree verb in text that a code line names, or
// whose prose sentence carries no ownedOrBanned form, with the line the verb
// starts on. A code line is never exempt: a comment beside a command does not
// stop an agent copying the command.
func unbannedRuns(text string, named *regexp.Regexp) []runHit {
	var hits []runHit
	for _, unit := range units(text) {
		for _, match := range named.FindAllStringSubmatchIndex(unit.text, -1) {
			if !unit.code && ownedOrBanned.MatchString(sentenceAround(unit.text, match[0], match[1])) {
				continue
			}
			hits = append(hits, runHit{
				line:   unit.line + strings.Count(unit.text[:match[0]], "\n"),
				phrase: "le " + strings.Join(strings.Fields(unit.text[match[2]:match[3]]), " "),
			})
		}
	}
	return hits
}

// textUnit is a paragraph of prose with its wrapped lines joined, or one
// code-block line or table row, with the line it starts on. code marks a line
// inside a fence.
type textUnit struct {
	line int
	text string
	code bool
}

// units splits markdown into the units unbannedRuns judges.
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
			out = append(out, textUnit{line: number, text: line, code: fenced})
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
