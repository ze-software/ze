// Design: docs/contributing/rule-authoring.md -- the agent rule set the payload report measures
// Related: ../artifacts.go -- the payload report that measures these parts
//
// Package ruleset reads the parts of a rule set from the checkout: the files a
// session loads, and the agent rule set a phase agent starts with in their
// place. It is the one reader of those parts. The payload report
// (internal/le/ai/rules) measures every part, and the subagent start hook
// (internal/le/hookruntime) sends the hard bans read by HardBans and names the
// every-agent decisions by the line range AgentDecisions answers, so the two
// never disagree on where a part begins or ends.
//
// It is its own package so the hook source seal (hookSourcePackages in
// internal/le/ai/hooks/fixtures.go) watches exactly what decides an omitting
// agent's start text, and not the rules generator around it.

package ruleset

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The files a rule set is read from, as repository paths.
const (
	InstructionsFile = "ai/INSTRUCTIONS.md"
	DecisionsFile    = "ai/META-DECISIONS.md"
	TriggersFile     = "ai/rules/TRIGGERS.md"
	CoreFile         = "ai/rules/CORE.md"
)

// AgentDefinitionsDir is the repository directory the harness reads custom
// agent definitions from: the mirror `./le ai sync update` writes from
// ai/agents (internal/le/ai/sync writes there through this constant). The
// subagent start hook reads a definition's frontmatter from it.
const AgentDefinitionsDir = ".claude/agents"

// HardBansHeading is the heading text of the section of ai/INSTRUCTIONS.md
// every agent carries.
const HardBansHeading = "Hard Bans"

// The two part headings of ai/META-DECISIONS.md, matched by their text at any
// heading level. The every-agent part comes first and ends where the
// main-session part begins.
const (
	DecisionsAgentPartHeading = "Rules for every agent"
	DecisionsMainPartHeading  = "Rules for the main session"
)

// Part is one part of a rule set as read from the checkout: the file
// it comes from, the heading text that opens it (empty for a whole file), its
// text with every line's newline, and the lines it spans in that file,
// counted from 1, both included. An empty file spans no line: Last is
// First - 1. The int64 fields are what textbuf.Buffer.Int takes.
//
// Missing says the file or the part is absent, and Reason then names what is
// absent; Text is empty and First and Last are not a range. A caller that
// joins the texts MUST check Missing, because an absent part is not a shorter
// one.
type Part struct {
	File    string
	Heading string
	Text    string
	First   int64
	Last    int64
	Missing bool
	Reason  string
}

// missingPartError says a file holds no part where one was looked for: a
// heading that is absent, repeated, or out of order.
type missingPartError struct{ reason string }

func (e *missingPartError) Error() string { return e.reason }

// Reader reads one part of a rule set from the checkout at tree.
type Reader func(tree string) (Part, error)

// markdownLine is one line of a file, its newline kept, and its heading level
// and text when it is a heading outside a code fence. Level 0 is not a heading.
type markdownLine struct {
	raw     string
	level   int
	heading string
}

// HardBans reads the "Hard Bans" section of ai/INSTRUCTIONS.md, its heading
// line included, up to the next heading of the same or a higher level. A file
// that cannot be read, or that holds the heading other than once, is an error;
// the part answered with an error still names its File and Heading.
func HardBans(tree string) (Part, error) {
	part := Part{File: InstructionsFile, Heading: HardBansHeading}
	lines, err := readMarkdownLines(tree, InstructionsFile)
	if err != nil {
		return part, err
	}
	start, err := onlyHeading(lines, InstructionsFile, HardBansHeading)
	if err != nil {
		return part, err
	}
	end := len(lines)
	for index := start + 1; index < len(lines); index++ {
		if lines[index].level == 0 {
			continue
		}
		if lines[index].level <= lines[start].level {
			end = index
			break
		}
	}
	return spanned(part, lines, start, end), nil
}

// AgentDecisions reads the every-agent part of ai/META-DECISIONS.md: from its
// heading to the line before the main-session heading. Each heading must
// appear once and the every-agent one first; anything else is an error naming
// what is missing or misordered, so no caller acts on a guessed range. The
// part answered with an error still names its File and Heading.
func AgentDecisions(tree string) (Part, error) {
	part := Part{File: DecisionsFile, Heading: DecisionsAgentPartHeading}
	lines, err := readMarkdownLines(tree, DecisionsFile)
	if err != nil {
		return part, err
	}
	agent, err := onlyHeading(lines, DecisionsFile, DecisionsAgentPartHeading)
	if err != nil {
		return part, err
	}
	session, err := onlyHeading(lines, DecisionsFile, DecisionsMainPartHeading)
	if err != nil {
		return part, err
	}
	if session < agent {
		var tb textbuf.Buffer
		tb.Str(DecisionsFile).Str(" puts the heading ").Quoted(DecisionsMainPartHeading).
			Str(" before ").Quoted(DecisionsAgentPartHeading)
		return part, &missingPartError{tb.String()}
	}
	return spanned(part, lines, agent, session), nil
}

// WholeFile reads one file of a rule set whole.
func WholeFile(file string) Reader {
	return func(tree string) (Part, error) {
		part := Part{File: file}
		lines, err := readMarkdownLines(tree, file)
		if err != nil {
			return part, err
		}
		return spanned(part, lines, 0, len(lines)), nil
	}
}

// readMarkdownLines reads a file of the checkout as lines, each with its
// newline, marking the headings that sit outside a code fence. A `#` line
// inside a fenced block is a shell comment, not a heading.
func readMarkdownLines(tree, file string) ([]markdownLine, error) {
	raw, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(file))) // #nosec G304 -- a fixed repository path under the checkout
	if err != nil {
		return nil, err
	}
	var lines []markdownLine
	var fence Fence
	for line := range strings.SplitAfterSeq(string(raw), "\n") {
		if line == "" {
			continue
		}
		bare := strings.TrimRight(line, "\r\n")
		if fence.Covers(bare) {
			lines = append(lines, markdownLine{raw: line})
			continue
		}
		level, heading := markdownHeading(bare)
		lines = append(lines, markdownLine{raw: line, level: level, heading: heading})
	}
	return lines, nil
}

// fenceOpen matches a line that opens a fenced block: at most three spaces,
// then a run of three or more backticks or tildes, then the info string.
var fenceOpen = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})([^\n]*)$")

// Fence follows the fenced blocks of a Markdown file line by line. It is the
// one fence rule of the rule set: this reader and the rules lint
// (internal/le/ai/rules, stripFences) both use it, so a block one reads as
// code the other never reads as text. The zero value is outside a block.
// Not safe for concurrent use.
type Fence struct {
	char   byte
	length int
}

// Covers reports whether line, without its line ending, is a fence marker or
// sits inside a fenced block, and moves past it. A block closes only on the
// marker character it opened with, in a run at least as long as the opening
// one and indented at most three spaces. An opening backtick run whose info
// string carries a backtick is inline code, not a fence.
func (f *Fence) Covers(line string) bool {
	if f.char != 0 {
		candidate := strings.TrimLeft(line, " ")
		indent := len(line) - len(candidate)
		marker := strings.TrimRight(candidate, " \t")
		if indent <= 3 && len(marker) >= f.length && marker == strings.Repeat(string(f.char), len(marker)) {
			f.char, f.length = 0, 0
		}
		return true
	}
	opened := fenceOpen.FindStringSubmatch(line)
	if opened == nil {
		return false
	}
	mark, info := opened[1], opened[2]
	if mark[0] == '`' && strings.Contains(info, "`") {
		return false
	}
	f.char, f.length = mark[0], len(mark)
	return true
}

// markdownHeading returns a line's heading level and text, or level 0 for a
// line that is not a heading: one or more '#' at its start, then a space.
func markdownHeading(line string) (int, string) {
	text := strings.TrimLeft(line, "#")
	level := len(line) - len(text)
	if level == 0 {
		return 0, ""
	}
	if !strings.HasPrefix(text, " ") {
		return 0, ""
	}
	return level, strings.TrimSpace(text)
}

// onlyHeading returns the 0-based index of the one heading, at any level,
// whose text is heading, or an error naming how many headings file holds.
func onlyHeading(lines []markdownLine, file, heading string) (int, error) {
	found, count := 0, 0
	for index, line := range lines {
		if line.level != 0 && line.heading == heading {
			found = index
			count++
		}
	}
	if count != 1 {
		var tb textbuf.Buffer
		tb.Str(file).Str(" holds ").Int(int64(count)).Str(" headings ").Quoted(heading).Str(", want exactly one")
		return 0, &missingPartError{tb.String()}
	}
	return found, nil
}

// spanned fills part with lines[start:end]: their text, and their line
// numbers counted from 1.
func spanned(part Part, lines []markdownLine, start, end int) Part {
	var tb textbuf.Buffer
	for _, line := range lines[start:end] {
		tb.Str(line.raw)
	}
	part.Text = tb.String()
	part.First = int64(start) + 1
	part.Last = int64(end)
	return part
}

// Read reads each part from the checkout at tree. A part whose file or
// heading is absent is answered Missing, with the reason; any other failure
// stops the read, because the set cannot be measured.
func Read(tree string, readers []Reader) ([]Part, error) {
	parts := make([]Part, 0, len(readers))
	for _, read := range readers {
		part, err := read(tree)
		var missing *missingPartError
		switch {
		case err == nil:
		case errors.Is(err, fs.ErrNotExist):
			part.Missing, part.Reason = true, part.File
		case errors.As(err, &missing):
			part.Missing, part.Reason = true, missing.Error()
		default:
			return nil, err
		}
		parts = append(parts, part)
	}
	return parts, nil
}
