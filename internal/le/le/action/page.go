// Design: docs/architecture/core-design.md -- le's help at every depth
// Overview: leaction.go -- the action table these pages render
// Related: parameter.go -- the typed keyword each keyword line describes
//
// Every le page an area's table answers is built here as one helpfmt.Page and
// rendered by helpfmt: the area page, one action's page, one keyword's page,
// and the usage a refusal prints. leroot builds the root and namespace pages
// the same way, so a reader meets one layout at every depth.

package leaction

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/term"

	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// pipeWidth is the column a page stops at when stdout is not a terminal. A
// fixed width keeps a page's bytes the same on every host, so a golden read
// through a pipe cannot depend on the terminal of the machine that ran it.
const pipeWidth = 100

// Renderings is the pipe-operator suffix every usage line ends with. The bar
// is quoted because it is typed quoted: a shell takes a bare `|` as its own
// pipe and runs `json` as a program, so le never sees it. The bare bars
// between the renderings separate the choices.
const Renderings = " ['|' json|yaml|table]"

// passedWords names the words a forwarding row or a pass-through area hands to
// its program, in the usage line and in the pass-through section, so both say
// it the same way.
const passedWords = "[<words>...]"

// program holds the words the running process was started as. Safe for
// concurrent use.
var program struct {
	sync.RWMutex
	name string
}

// InvocationName answers the words this process was started as: "le" for the
// binary named le, and "ze le" for a tagged ze build reached through its le
// root. The file name decides it because cmd/ze picks its personality the same
// way.
func InvocationName() string {
	name := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	if name == "le" {
		return "le"
	}
	return "ze le"
}

// SetProgram records the program name every page and refusal prints. The
// dispatcher MUST call it before any area answers, with the name it was
// started as (leroot.Dispatch).
func SetProgram(name string) {
	program.Lock()
	defer program.Unlock()
	program.name = name
}

// Program answers the name SetProgram recorded. A process that never
// dispatched, a Go test calling an area directly, is reaching le's own table,
// so it answers "le", the name the table is documented under.
func Program() string {
	program.RLock()
	defer program.RUnlock()
	// refuse-by-default: guards SetProgram -- nothing dispatched, so the table is reached as le itself
	if program.name == "" {
		return "le"
	}
	return program.name
}

// PageWidth answers the column a page written to file stops at: the terminal's
// width when file is a terminal that reports one, else pipeWidth. A terminal
// that cannot report its size gets pipeWidth too, which is a page that still
// fits a conventional terminal.
func PageWidth(file *os.File) int {
	descriptor := int(file.Fd()) //nolint:gosec // a file descriptor fits an int
	if !term.IsTerminal(descriptor) {
		return pipeWidth
	}
	width, _, err := term.GetSize(descriptor)
	if err != nil {
		return pipeWidth
	}
	if width <= 0 {
		return pipeWidth
	}
	return width
}

// Page answers the area page: the area's usage and one line per action, the
// writes marker in brackets, then the action's declared Summary. A
// pass-through area's usage takes its program's words, not an action, and an
// area with a default action shows the action as optional.
func (l List) Page(program string) helpfmt.Page {
	// The area word alone names a verbless listing's action, so the area's
	// page is that action's page (Action.Verb).
	if l.Verbless() {
		page, _ := l.ActionPage(program, "")
		return page
	}
	var tb textbuf.Buffer
	command := tb.Str(program).Byte(' ').Str(l.Area).String()
	tb.Reset()
	tb.Str(command)
	switch _, hasDefault := l.defaultVerb(); {
	case l.PassesWords:
		tb.Byte(' ').Str(passedWords)
	case hasDefault:
		// A line that names no action runs the default (Action.Default), so
		// the usage shows the action as optional.
		tb.Str(" [<action>] [<keyword> <value>...]")
	default:
		tb.Str(" <action> [<keyword> <value>...]")
	}
	return helpfmt.Page{
		Command:  command,
		Usage:    []string{tb.Str(Renderings).String()},
		Sections: []helpfmt.HelpSection{l.Section()},
	}
}

// Section answers the listing of the area's actions, which the area page and
// leroot's help page for the area both render. A listing with a default action
// names it on the heading line.
//
// A pass-through listing (PassesWords) holds no action, so its section is the
// one line that says where the words go, under the name its usage gives them.
func (l List) Section() helpfmt.HelpSection {
	if l.PassesWords {
		return helpfmt.HelpSection{Title: "Words", Entries: []helpfmt.HelpEntry{{Name: passedWords, Desc: l.PassesLine()}}}
	}
	entries := make([]helpfmt.HelpEntry, 0, len(l.Actions))
	for _, row := range l.Actions {
		// "writes" and "checks" are the whole reason a reader can pick an
		// action without opening the code behind it.
		mark := "[checks] "
		if row.Writes {
			mark = "[writes] "
		}
		entries = append(entries, helpfmt.HelpEntry{Name: row.Verb, Desc: mark + row.Summary})
	}
	section := helpfmt.HelpSection{Title: "Actions", Entries: entries}
	// A default action (Action.Default) also owns every line that names no
	// action, the bare one included, so the heading says which: help covers
	// every word le accepts, and that line is one.
	if verb, defaulted := l.defaultVerb(); defaulted {
		var tb textbuf.Buffer
		section.Summary = tb.Str("a line that names no action runs ").Str(verb).String()
	}
	return section
}

// Text renders the area page for a person on stdout, which is where the Prose
// rendering of a bare `le <area>` goes.
func (l List) Text() string {
	page := l.Page(Program())
	page.Width = PageWidth(os.Stdout)

	var out strings.Builder
	page.WriteTo(&out, slogutil.UseColor(os.Stdout))
	return out.String()
}

// row answers the listing row for verb, and whether the listing holds it.
func (l List) row(verb string) (Row, bool) {
	for _, row := range l.Actions {
		if row.Verb == verb {
			return row, true
		}
	}
	return Row{}, false
}

// usageLine renders the invocation one action takes, from its declared
// grammar, under the program name the reader typed.
func (l List) usageLine(program string, row Row) string {
	var tb textbuf.Buffer
	tb.Str(program).Byte(' ').Str(l.Area)
	if grammar := rowGrammar(row); grammar != "" {
		tb.Byte(' ').Str(grammar)
	}
	return tb.Str(Renderings).String()
}

// commandOf answers the words that name one action under the program name:
// the area, then the verb, which a verbless action has none of (Action.Verb).
func (l List) commandOf(program string, row Row) string {
	var tb textbuf.Buffer
	tb.Str(program).Byte(' ').Str(l.Area)
	if row.Verb != "" {
		tb.Byte(' ').Str(row.Verb)
	}
	return tb.String()
}

// Grammar answers the line one action takes after the area's name, its verb
// then each keyword in the form usage renders: `run suite <suite> [test
// <selector>]`. It answers false for a verb this listing does not hold. An area
// whose help hint names the grammar derives it here, so the hint cannot
// disagree with the table.
func (l List) Grammar(verb string) (string, bool) {
	row, held := l.row(verb)
	if !held {
		return "", false
	}
	return rowGrammar(row), true
}

// rowGrammar renders a row's verb and its keyword forms. A verbless row
// (Action.Verb) starts at its first keyword form, and one that takes nothing
// renders empty.
func rowGrammar(row Row) string {
	words := make([]string, 0, len(row.Parameters)+2)
	if row.Verb != "" {
		words = append(words, row.Verb)
	}
	for _, parameter := range row.Parameters {
		words = append(words, parameterForm(parameter))
	}
	if row.Forwards {
		words = append(words, passedWords)
	}
	var tb textbuf.Buffer
	return tb.Join(words, " ").String()
}

// ActionPage answers one action's page: its whole purpose, its usage line, and
// one line per keyword with the facts its declaration carries. It answers false
// for a verb this listing does not hold.
//
// The page is built from the LISTING rather than from the action table, so the
// dispatcher renders it from the listing an area registered without calling
// that area's handler (internal/le/le/root).
func (l List) ActionPage(program, verb string) (helpfmt.Page, bool) {
	row, held := l.row(verb)
	if !held {
		return helpfmt.Page{}, false
	}
	page := helpfmt.Page{
		Command:     l.commandOf(program, row),
		ShortHelp:   row.Summary,
		Description: row.Why,
		Usage:       []string{l.usageLine(program, row)},
	}
	if keywords := keywordSection(row.Parameters); len(keywords.Entries) != 0 {
		page.Sections = []helpfmt.HelpSection{keywords}
	}
	return page, true
}

// UsagePage answers what a refusal of one action prints: its usage line as the
// page's first line, then its keyword lines, and nothing about the area's other
// actions. It answers false for a verb this listing does not hold.
func (l List) UsagePage(program, verb string) (helpfmt.Page, bool) {
	row, held := l.row(verb)
	if !held {
		return helpfmt.Page{}, false
	}
	page := helpfmt.Page{Command: "usage: " + l.usageLine(program, row)}
	if keywords := keywordSection(row.Parameters); len(keywords.Entries) != 0 {
		page.Sections = []helpfmt.HelpSection{keywords}
	}
	return page, true
}

// errNoSuchKeyword is what KeywordPage answers for a keyword the action does
// not declare.
var errNoSuchKeyword = errors.New("no such keyword")

// KeywordPage answers one keyword's page: its kind and what the kind means, its
// values, whether it is required and whether it repeats, and its help. A verb
// the listing does not hold, or a keyword the action does not declare, is
// refused with an error naming what the action does take.
func (l List) KeywordPage(program, verb, keyword string) (helpfmt.Page, error) {
	row, held := l.row(verb)
	if !held {
		var tb textbuf.Buffer
		return helpfmt.Page{}, errors.New(tb.Str(l.Area).Str(" has no action ").Quoted(verb).String())
	}
	command := l.commandOf(program, row)
	for _, parameter := range row.Parameters {
		if parameter.Keyword != keyword {
			continue
		}
		return helpfmt.Page{
			Command:     command + " " + keyword,
			ShortHelp:   parameter.Help,
			Description: keywordDescription(parameter),
			Usage:       []string{slotForm(parameter)},
		}, nil
	}
	names := make([]string, 0, len(row.Parameters))
	for _, parameter := range row.Parameters {
		names = append(names, slotForm(parameter))
	}
	var tb textbuf.Buffer
	tb.Str(command).Str(": ").Err(errNoSuchKeyword).Byte(' ').Quoted(keyword).Str("; it takes: ")
	if len(names) == 0 {
		tb.Str("no keywords")
	}
	tb.Join(names, ", ")
	return helpfmt.Page{}, errors.New(tb.String())
}

// keywordSection answers one line per keyword: the spelling a reader types,
// then the facts its declaration carries.
func keywordSection(parameters []Parameter) helpfmt.HelpSection {
	entries := make([]helpfmt.HelpEntry, 0, len(parameters))
	for _, parameter := range parameters {
		entries = append(entries, helpfmt.HelpEntry{Name: slotForm(parameter), Desc: keywordFacts(parameter) + ": " + parameter.Help})
	}
	return helpfmt.HelpSection{Title: "Keywords", Entries: entries}
}

// keywordFacts renders the declaration's facts on one keyword line: kind, then
// required or optional, then repeatable when it is.
func keywordFacts(parameter Parameter) string {
	var tb textbuf.Buffer
	tb.Str(parameter.Kind.String()).Str(", ")
	if parameter.Requirement == Required {
		tb.Str("required")
	} else {
		tb.Str("optional")
	}
	if parameter.Repeat {
		tb.Str(", repeatable")
	}
	return tb.String()
}

// keywordDescription is the body of a keyword's page, one fact per line, and
// each declared value on its own line with what it does.
func keywordDescription(parameter Parameter) string {
	var tb textbuf.Buffer
	tb.Str("kind: ").Str(parameter.Kind.String()).Str(" (").Str(parameter.Kind.Meaning()).Str(")\n")
	if len(parameter.Values) != 0 {
		tb.Str("values:\n")
		for _, candidate := range parameter.ValueCandidates() {
			tb.Str("  ").Str(candidate.Value).Str(": ").Str(candidate.Summary).Byte('\n')
		}
	}
	tb.Str("required: ")
	if parameter.Requirement == Required {
		tb.Str("yes")
	} else {
		tb.Str("no")
	}
	tb.Str("\nrepeat: ")
	if parameter.Repeat {
		tb.Str("yes")
	} else {
		tb.Str("no")
	}
	return tb.String()
}
