// Design: docs/contributing/rule-authoring.md -- the agent rule set the payload report measures
// Related: ../artifacts.go -- the payload report that measures these parts
//
// Package ruleset reads the parts of a rule set from the checkout: the files a
// session loads, and the agent rule set a phase agent starts with in their
// place. It is the one reader of those parts. The payload report
// (internal/le/ai/rules) measures every part, and the subagent start hook
// (internal/le/hookruntime) sends the hard bans read by HardBans, names the
// every-agent decisions by the line range AgentDecisions answers, and lists
// each section an agent reads only before one kind of work by the heading,
// situation and range SituationalDecisions answers, so the two never disagree
// on where a part begins or ends.
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
	InstructionsFile         = "ai/INSTRUCTIONS.md"
	DecisionsFile            = "ai/META-DECISIONS.md"
	SituationalDecisionsFile = "ai/META-DECISIONS-SITUATIONAL.md"
	TriggersFile             = "ai/rules/TRIGGERS.md"
	CoreFile                 = "ai/rules/CORE.md"
)

// AgentDefinitionsDir is the repository directory the harness reads custom
// agent definitions from: the mirror `./le ai sync update` writes from
// ai/agents (internal/le/ai/sync writes there through this constant). The
// subagent start hook reads a definition's frontmatter from it.
const AgentDefinitionsDir = ".claude/agents"

// HardBansHeading is the heading text of the section of ai/INSTRUCTIONS.md
// every agent carries.
const HardBansHeading = "Hard Bans"

// The three part headings of ai/META-DECISIONS.md, matched by their text at
// any heading level. The every-agent part comes first and ends where the
// main-session part begins. The part for one kind of work comes after the
// main-session part, so the every-agent range never holds it.
const (
	DecisionsAgentPartHeading       = "Rules for every agent"
	DecisionsMainPartHeading        = "Rules for the main session"
	DecisionsSituationalPartHeading = "Rules for one kind of work"
)

// SituationPrefix opens the first paragraph of each section of the part for
// one kind of work: the rest of that paragraph names the work, and an agent
// reads the section before that work.
const SituationPrefix = "Read before: "

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
	return spanned(part, lines, start, sectionEnd(lines, start)), nil
}

// sectionEnd answers the 0-based index of the first heading after start of
// the same or a higher level than the heading at start, or len(lines) when
// none follows: the end, excluded, of the section that heading opens.
func sectionEnd(lines []markdownLine, start int) int {
	for index := start + 1; index < len(lines); index++ {
		if lines[index].level == 0 {
			continue
		}
		if lines[index].level <= lines[start].level {
			return index
		}
	}
	return len(lines)
}

// Situational is one section of the part of ai/META-DECISIONS.md for one kind
// of work: the section as a Part, whose Heading is the section's heading, and
// the situation its first paragraph names after SituationPrefix, its lines
// joined by a space. An agent reads the section before work the situation
// names.
type Situational struct {
	Part
	Situation string
}

// SituationalDecisions reads each section of ai/META-DECISIONS-SITUATIONAL.md
// under DecisionsSituationalPartHeading: every heading one level below the part
// heading, up to the next heading of its own or a higher level. The sections
// are found by heading level alone, so a section added to the file is answered
// with no edit here.
//
// It answers an error, never an empty list, when the part cannot tell an agent
// what to read: the part heading absent or repeated, a part with no section, or
// a section whose first paragraph does not open with SituationPrefix and a
// situation. An empty list would read as "nothing to read", while the caller
// can still send the whole file.
//
// It also refuses a part that leaves a rule outside every section, because no
// agent would be told to read it: a part that does not run to the end of the
// file (a later heading of its own level ends it), and a bullet or a deeper
// heading before its first section.
func SituationalDecisions(tree string) ([]Situational, error) {
	lines, err := readMarkdownLines(tree, SituationalDecisionsFile)
	if err != nil {
		return nil, err
	}
	start, err := onlyHeading(lines, SituationalDecisionsFile, DecisionsSituationalPartHeading)
	if err != nil {
		return nil, err
	}
	end := sectionEnd(lines, start)
	if end != len(lines) {
		var tb textbuf.Buffer
		tb.Str(SituationalDecisionsFile).Str(" ends the part ").Quoted(DecisionsSituationalPartHeading).
			Str(" at the heading ").Quoted(lines[end].heading).
			Str(", so the rules after it sit in no section an agent is told to read")
		return nil, &missingPartError{tb.String()}
	}
	if stray, found := strayBeforeSection(lines[start+1:end], lines[start].level); found {
		var tb textbuf.Buffer
		tb.Str(SituationalDecisionsFile).Str(" holds ").Quoted(stray).Str(" under the heading ").
			Quoted(DecisionsSituationalPartHeading).
			Str(" before its first section, so no agent is told to read it")
		return nil, &missingPartError{tb.String()}
	}
	var sections []Situational
	for index := start + 1; index < end; index++ {
		if lines[index].level != lines[start].level+1 {
			continue
		}
		last := sectionEnd(lines, index)
		situation, found := situationOf(lines[index+1 : last])
		if !found {
			var tb textbuf.Buffer
			tb.Str(SituationalDecisionsFile).Str(" section ").Quoted(lines[index].heading).
				Str(" names no work: its first paragraph must be ").Quoted(SituationPrefix + "<the work it binds>").
				Str(", so no agent can tell when to read it")
			return nil, &missingPartError{tb.String()}
		}
		part := spanned(Part{File: SituationalDecisionsFile, Heading: lines[index].heading}, lines, index, last)
		sections = append(sections, Situational{Part: part, Situation: situation})
	}
	if len(sections) == 0 {
		var tb textbuf.Buffer
		tb.Str(SituationalDecisionsFile).Str(" holds no section under the heading ").Quoted(DecisionsSituationalPartHeading)
		return nil, &missingPartError{tb.String()}
	}
	return sections, nil
}

// strayBeforeSection answers the first line of body, the lines of the part
// for one kind of work after its heading, that sits before the part's first
// section and carries a rule or a heading: a bullet, or a heading deeper than
// a section's. The part's opening prose is allowed there; a fenced line is
// never a heading.
func strayBeforeSection(body []markdownLine, partLevel int) (string, bool) {
	for _, line := range body {
		if line.level == partLevel+1 {
			return "", false
		}
		bare := strings.TrimSpace(line.raw)
		if line.level != 0 {
			return bare, true
		}
		if strings.HasPrefix(bare, "- ") {
			return bare, true
		}
	}
	return "", false
}

// situationMarker is SituationPrefix without its trailing space: the words a
// situation paragraph opens with, whether the situation follows on the same
// line or wraps onto the next.
var situationMarker = strings.TrimSpace(SituationPrefix)

// situationOf answers the situation the first paragraph of a section's body
// names: the paragraph's lines, joined by a space, after SituationPrefix. The
// situation may start on the marker's line or on the next one. The paragraph
// ends at a blank line, a heading or a bullet, so a section written with no
// blank line before its first bullet keeps that bullet out of the situation.
// It answers false when the first line that is not blank does not open with
// the marker, or the paragraph names nothing after it.
func situationOf(body []markdownLine) (string, bool) {
	var words []string
	for _, line := range body {
		bare := strings.TrimSpace(line.raw)
		if len(words) == 0 {
			if bare == "" {
				continue
			}
			if !strings.HasPrefix(bare, situationMarker) {
				return "", false
			}
			words = append(words, bare)
			continue
		}
		if bare == "" {
			break
		}
		if line.level != 0 {
			break
		}
		if strings.HasPrefix(bare, "- ") {
			break
		}
		words = append(words, bare)
	}
	if len(words) == 0 {
		return "", false
	}
	situation := strings.TrimSpace(strings.TrimPrefix(strings.Join(words, " "), situationMarker))
	if situation == "" {
		return "", false
	}
	return situation, true
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
		if err != nil {
			reason, absent := MissingReason(err, part.File)
			if !absent {
				return nil, err
			}
			part.Missing, part.Reason = true, reason
		}
		parts = append(parts, part)
	}
	return parts, nil
}

// MissingReason answers why a part read from file is absent when err says it
// is: the file's path for a file that does not exist, the reader's own words
// for a heading or a section it cannot place. Any other error, an unreadable
// file for one, is not an absence and answers false, because the set cannot
// be measured.
func MissingReason(err error, file string) (string, bool) {
	if errors.Is(err, fs.ErrNotExist) {
		return file, true
	}
	if missing, found := errors.AsType[*missingPartError](err); found {
		return missing.Error(), true
	}
	return "", false
}
