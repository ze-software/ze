// Design: docs/contributing/running-commands.md -- `le help ai`, the reference an agent reads
// Related: ../aihelp.go -- ze's own reference, assembled for `ze help ai --json` and MCP
//
// Package reference holds the program-neutral shape of an AI reference: what a
// program is, the sections it answers, and for each section its rows. A program
// fills it from its own registry, and the renderers here print it, so two
// programs that answer `help ai` print one shape. It imports only textbuf: a
// value type that names no registry crosses into any binary.
package reference

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Reference is one program's answer to `help ai`. Sections always lists every
// section the program answers; the other fields hold only the sections the
// reader selected, and are absent from the JSON when not selected.
type Reference struct {
	// Program is the command word the reader types: "le", or "ze le".
	Program string `json:"program"`
	// Summary is the one line under the program name.
	Summary string `json:"summary"`
	// Sections names every section and what it holds.
	Sections []Entry `json:"sections"`
	// Namespaces is every first word several commands share, and what it is
	// for. It is filled with Commands, by the same section.
	Namespaces []Namespace `json:"namespaces,omitempty"`
	// Commands is every command the program registers.
	Commands []Command `json:"commands,omitempty"`
	// Groups is the purpose tags a command declares, and what each means.
	Groups []Entry `json:"groups,omitempty"`
	// Verbs is the program's closed verb vocabulary.
	Verbs []Verb `json:"verbs,omitempty"`
	// ExitCodes is what each exit code means, and who answers another.
	ExitCodes []ExitCode `json:"exit-codes,omitempty"`
	// Launcher is the words a launcher script reads before the program
	// starts (le's ./le), each with its meaning.
	Launcher []Entry `json:"launcher,omitempty"`
}

// Entry is one named row and its one-line meaning.
type Entry struct {
	Name    string `json:"name"`
	Meaning string `json:"meaning"`
}

// Verb is one entry of a closed verb vocabulary: the word, the kind of job it
// does, its one meaning, and the names (commands spelled with a noun rather
// than the verb) that declare they perform it.
type Verb struct {
	Name    string   `json:"name"`
	Kind    string   `json:"kind"`
	Meaning string   `json:"meaning"`
	Names   []string `json:"names,omitempty"`
}

// Namespace is one first word several commands share: its one line and its
// long description, the two texts every level of the command surface carries.
type Namespace struct {
	Name        string `json:"name"`
	ShortHelp   string `json:"short-help"`
	Description string `json:"description"`
}

// Command is one registered command and its whole grammar.
type Command struct {
	// Name is the word, or the words, typed after the program.
	Name string `json:"name"`
	// Group is the command's purpose tag.
	Group string `json:"group"`
	// ShortHelp is the command's one line, at most 60 characters.
	ShortHelp string `json:"short-help"`
	// Description is the command's long account; for a pass-through command
	// it is empty and PassesTo says where its words go.
	Description string `json:"description,omitempty"`
	// Actions is the command's action table. Absent for a pass-through command.
	Actions []Action `json:"actions,omitempty"`
	// PassesTo names the program that reads every word after the name, for a
	// command that passes its words on. Empty for a command le parses itself.
	PassesTo string `json:"passes-to,omitempty"`
	// Hidden says the program starts this command itself and a person never
	// types it, so help pages and completion leave it out; this reference is
	// the one listing that names it.
	Hidden bool `json:"hidden,omitempty"`
}

// Action is one verb of a command and the parameters it takes.
type Action struct {
	Verb string `json:"verb"`
	// ShortHelp is the action's one line, at most 60 characters.
	ShortHelp string `json:"short-help"`
	// Description is the action's long account of what it is for.
	Description string      `json:"description"`
	Writes      bool        `json:"writes"`
	Parameters  []Parameter `json:"parameters,omitempty"`
}

// Parameter is one keyword, or the positional value, an action takes.
type Parameter struct {
	// Keyword is the word that introduces the value; empty for the positional.
	Keyword string `json:"keyword,omitempty"`
	// Value names the value in usage, `<value>`.
	Value string `json:"value,omitempty"`
	// Kind is the value's kind: switch, string, enum, uint and the rest.
	Kind string `json:"kind"`
	// Values is the closed set an enum or a closed list accepts.
	Values []string `json:"values,omitempty"`
	// ValueHelp is the line that says what each of Values does, keyed by value.
	ValueHelp map[string]string `json:"value-help,omitempty"`
	// Pair names the second value of a keyword that takes two.
	Pair     string `json:"pair,omitempty"`
	Required bool   `json:"required"`
	Repeat   bool   `json:"repeat"`
	Help     string `json:"help"`
}

// ExitCode is one exit code and its meaning.
type ExitCode struct {
	Code    int    `json:"code"`
	Meaning string `json:"meaning"`
	// Exempt names the commands that answer another program's code instead.
	Exempt []string `json:"exempt,omitempty"`
}

// Text renders the reference for a reader: the section index when no section
// was selected, else each selected section in turn. It satisfies le's Prose,
// so `le help ai` prints this and `le help ai | json` prints the value.
func (r Reference) Text() string {
	var tb textbuf.Buffer
	tb.Str("# ").Str(r.Program).Str(" AI reference\n# Generated from code -- always matches this binary.\n\n")
	if len(r.Commands) == 0 && len(r.Groups) == 0 && len(r.Verbs) == 0 && len(r.ExitCodes) == 0 && len(r.Launcher) == 0 {
		r.writeIndex(&tb)
		return tb.String()
	}
	r.writeNamespaces(&tb)
	r.writeCommands(&tb)
	writeEntries(&tb, "Groups", r.Groups)
	writeVerbs(&tb, r.Verbs)
	r.writeExitCodes(&tb)
	writeEntries(&tb, "Launcher", r.Launcher)
	return tb.String()
}

// writeIndex lists the sections and the two ways to read one.
func (r Reference) writeIndex(tb *textbuf.Buffer) {
	tb.Str(r.Program).Str(" - ").Str(r.Summary).Str("\n\n")
	tb.Str("## Sections (").Str(r.Program).Str(" help ai <section>)\n\n")
	width := 0
	for _, section := range r.Sections {
		width = max(width, len(section.Name))
	}
	for _, section := range r.Sections {
		tb.Str("  ").PadRight(section.Name, width).Str("  ").Str(section.Meaning).Byte('\n')
	}
	tb.Str("\n## Machine-readable\n\n")
	tb.Str("  ").Str(r.Program).Str(" help ai | json          every section's index as JSON\n")
	tb.Str("  ").Str(r.Program).Str(" help ai all | json      every section as JSON (| yaml for YAML)\n")
	tb.Str("  ").Str(r.Program).Str(" help ai commands | json every command, action and typed parameter\n")
}

// writeNamespaces prints one line per namespace with its one-line summary,
// then its long description indented under it.
func (r Reference) writeNamespaces(tb *textbuf.Buffer) {
	if len(r.Namespaces) == 0 {
		return
	}
	tb.Str("## Namespaces\n\n")
	width := 0
	for _, namespace := range r.Namespaces {
		width = max(width, len(namespace.Name))
	}
	for _, namespace := range r.Namespaces {
		tb.Str("  ").PadRight(namespace.Name, width).Str("  ").Str(namespace.ShortHelp).Byte('\n')
		writeDescription(tb, "      ", namespace.Description)
	}
	tb.Byte('\n')
}

// writeDescription prints a long description on one line under the item it
// describes, at indent; an empty one prints nothing.
func writeDescription(tb *textbuf.Buffer, indent, description string) {
	if description == "" {
		return
	}
	tb.Str(indent).Str(strings.Join(strings.Fields(description), " ")).Byte('\n')
}

// writeCommands prints one block per command: its actions, each with its
// parameters and their kinds.
func (r Reference) writeCommands(tb *textbuf.Buffer) {
	if len(r.Commands) == 0 {
		return
	}
	tb.Str("## Commands\n")
	for _, command := range r.Commands {
		tb.Str("\n").Str(r.Program).Byte(' ').Str(command.Name).Str("  [").Str(command.Group).Str("]  ")
		if command.Hidden {
			tb.Str("(hidden) ")
		}
		tb.Str(command.ShortHelp).Byte('\n')
		writeDescription(tb, "  ", command.Description)
		if command.PassesTo != "" {
			tb.Str("  <words>...  passed to ").Str(command.PassesTo).Byte('\n')
		}
		for _, action := range command.Actions {
			// A verbless action (an area's sole action, run by the bare
			// area line) has no verb word to print.
			tb.Str("  ")
			if action.Verb != "" {
				tb.Str(action.Verb).Str("  ")
			}
			tb.Str(action.ShortHelp).Byte('\n')
			writeDescription(tb, "      ", action.Description)
			for _, parameter := range action.Parameters {
				writeParameter(tb, parameter)
			}
		}
	}
	tb.Byte('\n')
}

// writeParameter prints one parameter line: its spelling, kind, values and
// whether it is required or repeats, then one line per declared value saying
// what it does.
func writeParameter(tb *textbuf.Buffer, p Parameter) {
	tb.Str("    ")
	if p.Keyword != "" {
		tb.Str(p.Keyword)
		if p.Value != "" {
			tb.Byte(' ')
		}
	}
	if p.Value != "" {
		tb.Byte('<').Str(p.Value).Byte('>')
	}
	if p.Pair != "" {
		tb.Str(" <").Str(p.Pair).Byte('>')
	}
	tb.Str("  ").Str(p.Kind)
	if len(p.Values) != 0 {
		tb.Str(" ").Join(p.Values, "|")
	}
	if p.Required {
		tb.Str(", required")
	}
	if p.Repeat {
		tb.Str(", repeats")
	}
	if p.Help != "" {
		tb.Str("  ").Str(p.Help)
	}
	tb.Byte('\n')
	for _, value := range p.Values {
		if help, held := p.ValueHelp[value]; held {
			tb.Str("      ").Str(value).Str("  ").Str(help).Byte('\n')
		}
	}
}

// writeEntries prints a titled list of named rows.
func writeEntries(tb *textbuf.Buffer, title string, entries []Entry) {
	if len(entries) == 0 {
		return
	}
	width := 0
	for _, entry := range entries {
		width = max(width, len(entry.Name))
	}
	tb.Str("## ").Str(title).Str("\n\n")
	for _, entry := range entries {
		tb.Str("  ").PadRight(entry.Name, width).Str("  ").Str(entry.Meaning).Byte('\n')
	}
	tb.Byte('\n')
}

// writeVerbs prints each verb with its kind and meaning, then the names that
// declare it on one indented line.
func writeVerbs(tb *textbuf.Buffer, verbs []Verb) {
	if len(verbs) == 0 {
		return
	}
	width, kindWidth := 0, 0
	for _, verb := range verbs {
		width = max(width, len(verb.Name))
		kindWidth = max(kindWidth, len(verb.Kind))
	}
	tb.Str("## Verbs\n\n")
	for _, verb := range verbs {
		tb.Str("  ").PadRight(verb.Name, width).Str("  ").PadRight(verb.Kind, kindWidth).Str("  ").Str(verb.Meaning).Byte('\n')
		if len(verb.Names) != 0 {
			tb.Str("      declared by: ").Join(verb.Names, ", ").Byte('\n')
		}
	}
	tb.Byte('\n')
}

// writeExitCodes prints each code, its meaning and its exemptions.
func (r Reference) writeExitCodes(tb *textbuf.Buffer) {
	if len(r.ExitCodes) == 0 {
		return
	}
	tb.Str("## Exit codes\n\n")
	for _, code := range r.ExitCodes {
		tb.Str("  ").Int(int64(code.Code)).Str("  ").Str(code.Meaning).Byte('\n')
		for _, exempt := range code.Exempt {
			tb.Str("     except: ").Str(exempt).Byte('\n')
		}
	}
	tb.Byte('\n')
}
