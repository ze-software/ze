// Design: docs/architecture/core-design.md -- how a program dispatches le's commands
// Overview: leroot.go -- the registration adapter every le tool joins through
//
// The loop resolves the canonical `le <tool>` local-data path. The same
// handler and renderer therefore serve a standalone binary named le and the
// explicit `ze le` root in a tagged ze build.
//
// Tools that inspect ze load product packages into the le process. Filtering
// the shared local registry by the `le ` prefix keeps product commands outside
// this surface without a second ownership table.

package leroot

import (
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
)

// isHelpArg reports whether the word asks for usage rather than naming a
// command. The three spellings are declared once, by the package that also
// reads them after a verb (leaction.IsHelpArg).
func isHelpArg(word string) bool { return leaction.IsHelpArg(word) }

// Commands answers le's commands with the metadata from the shared local
// registry. ListLocal sorts by full path, so the stripped names remain sorted.
func Commands() []registry.RootCommand {
	all := registry.ListLocal()
	mine := make([]registry.RootCommand, 0, len(all))
	for _, entry := range all {
		if len(entry.Path) <= len(pathPrefix) || entry.Path[:len(pathPrefix)] != pathPrefix {
			continue
		}
		mine = append(mine, registry.RootCommand{
			Name: entry.Path[len(pathPrefix):],
			Meta: entry.Meta,
		})
	}
	return mine
}

// Usage lists every registered le tool under the group it declared. program
// supplies the page name: "le" for the binary and "ze le" for a ze_le build.
// Thus, every line that a reader copies is valid for the active program.
//
// Eighty-six commands in one alphabetical list read as one thing. They are
// five. The groups say which, in the order a person meets them (group.go).
func Usage(program string) {
	var tb textbuf.Buffer
	page := helpfmt.Page{
		Command:  program,
		Summary:  "the Ze repository and development entry point",
		Usage:    []string{tb.Str(program).Str(" <command> [options] [| json | yaml | table]").String()},
		Sections: usageSections(Commands()),
	}
	page.WriteErr()
}

// usageSections splits the commands into one help section for each group, in
// render order.
//
// A command whose group is unknown still prints. It goes in a final section of
// its own. A help page that hides a command is worse than one that files a
// command badly. Only a registration that bypassed Register can produce such a
// command.
func usageSections(roots []registry.RootCommand) []helpfmt.HelpSection {
	byGroup := make(map[Group][]helpfmt.HelpEntry, len(groupOrder))
	ungrouped := make([]helpfmt.HelpEntry, 0)
	for _, rc := range roots {
		entry := helpfmt.HelpEntry{Name: rc.Name, Desc: rc.Meta.Description}
		group, ok := GroupOf(rc.Name)
		if !ok {
			ungrouped = append(ungrouped, entry)
			continue
		}
		byGroup[group] = append(byGroup[group], entry)
	}

	sections := make([]helpfmt.HelpSection, 0, len(groupOrder)+1)
	for _, group := range groupOrder {
		entries := byGroup[group]
		if len(entries) == 0 {
			continue
		}
		sections = append(sections, helpfmt.HelpSection{Title: GroupTitle(group), Entries: entries})
	}
	if len(ungrouped) != 0 {
		sections = append(sections, helpfmt.HelpSection{Title: "Ungrouped", Entries: ungrouped})
	}
	return sections
}

// commandWordsMax bounds how many argv words the lookup may consume. le's
// deepest registered path is one namespace and one member, so two words after
// `le` is the whole grammar.
//
// The bound is load-bearing. LookupLocalData matches the longest registered
// prefix of whatever it is handed, so an unbounded span would let a value
// further along the line be read as a command word: `le job run label x
// command le verify lint` offers nine.
const commandWordsMax = 2

// resolve answers the registered command among the leading words of argv: the
// name as it was typed, its handler, and the words the tool itself receives.
//
// The pipe word ends the candidate span, so `le verify list | json` offers the
// matcher `verify list` and never the chain.
func resolve(args []string) (string, registry.LocalDataHandler, []string) {
	span := min(len(args), commandWordsMax)
	for index := range args[:span] {
		if args[index] == pipeWord {
			span = index
			break
		}
	}
	if span == 0 {
		return "", nil, nil
	}

	words := make([]string, 0, span+1)
	words = append(words, "le")
	words = append(words, args[:span]...)

	handler, trailing := registry.LookupLocalData(words)
	if handler == nil {
		return "", nil, nil
	}
	consumed := span - len(trailing)

	var tb textbuf.Buffer
	return tb.Join(args[:consumed], " ").String(), handler, args[consumed:]
}

// members answers the commands registered under a namespace token, with the
// token stripped: `spec` answers citation, session and status.
//
// The listing is derived from the registry rather than declared, so the help
// page, a bare token's answer and a refusal cannot disagree about what a
// namespace holds.
func members(token string) []string {
	var tb textbuf.Buffer
	prefix := tb.Str(token).Byte(' ').String()

	found := make([]string, 0, 4)
	for _, command := range Commands() {
		if name, ok := strings.CutPrefix(command.Name, prefix); ok {
			found = append(found, name)
		}
	}
	return found
}

// Dispatch resolves argv through the canonical local-data path and answers the
// tool's exit code. It calls Run directly so a nonzero verdict can still carry
// a structured payload to stdout.
//
// program supplies the page name for help and refusals. It does not change the
// registered command set.
func Dispatch(program string, args []string) int {
	if len(args) == 0 {
		Usage(program)
		return 1
	}
	if isHelpArg(args[0]) {
		if len(args) == 1 {
			Usage(program)
			return 0
		}
		return helpAsked(program, args[1:])
	}

	name, handler, toolArgs := resolve(args)
	if handler != nil {
		// One help word and nothing else asks about the command itself. A help
		// word after an action belongs to that action's grammar, which only the
		// area holds, so it travels on to the handler.
		if len(toolArgs) == 1 && isHelpArg(toolArgs[0]) {
			return helpNode(program, name)
		}
		return Run(name, Answer(handler), toolArgs, os.Stdout, os.Stderr)
	}

	// A namespace token is not an unknown command, it is an incomplete one.
	// Naming the members it holds is the difference between a typo and a
	// command the reader has half typed.
	if held := members(args[0]); len(held) != 0 {
		if len(args) == 2 && isHelpArg(args[1]) {
			return helpNode(program, args[0])
		}
		var tb textbuf.Buffer
		message := tb.Str("error: ").Str(args[0]).
			Str(" is a namespace; it needs one of: ").Join(held, " | ").String()
		fmt.Fprintln(os.Stderr, message) //nolint:errcheck // CLI output
		return 1
	}

	var tb textbuf.Buffer
	tb.Str("unknown command: ").Str(args[0]).Byte('\n').StdErr() //nolint:errcheck // CLI output
	Usage(program)
	return 1
}

// helpAsked answers `le help <command>...`: it resolves the words the reader
// typed after the help word and renders that node, rather than the whole tree
// the bare help word answers.
func helpAsked(program string, words []string) int {
	name, handler, _ := resolve(words)
	if handler == nil {
		// A namespace holds commands without being one, so it has no handler to
		// resolve and its page is still worth printing.
		if len(members(words[0])) == 0 {
			var tb textbuf.Buffer
			tb.Str("unknown command: ").Str(words[0]).Byte('\n').StdErr() //nolint:errcheck // CLI output
			Usage(program)
			return 1
		}
		name = words[0]
	}
	return helpNode(program, name)
}

// helpNode renders what the registry knows about one command: its summary, the
// actions it declares, and the commands registered under it.
//
// It never calls the command's own handler. A single-action area answers a bare
// invocation by RUNNING its gate, so re-entering the handler to print a page
// would scan the tree, write a file, or start a build on `--help`.
func helpNode(program, name string) int {
	var meta registry.Meta
	for _, command := range Commands() {
		if command.Name == name {
			meta = command.Meta
			break
		}
	}

	var tb textbuf.Buffer
	command := tb.Str(program).Byte(' ').Str(name).String()
	tb.Reset()

	sections := make([]helpfmt.HelpSection, 0, 2)
	actions := meta.ResolveSubs()
	if actions != "" {
		entries := make([]helpfmt.HelpEntry, 0, 4)
		for action := range strings.SplitSeq(actions, " | ") {
			entries = append(entries, helpfmt.HelpEntry{Name: action})
		}
		sections = append(sections, helpfmt.HelpSection{Title: "Actions", Entries: entries})
	}
	if held := childEntries(name); len(held) != 0 {
		sections = append(sections, helpfmt.HelpSection{Title: "Commands", Entries: held})
	}

	pattern := tb.Str(command).Str(" [| json | yaml | table]").String()
	if actions != "" {
		tb.Reset()
		pattern = tb.Str(command).Str(" <action> [| json | yaml | table]").String()
	}
	page := helpfmt.Page{
		Command:  command,
		Summary:  meta.Description,
		Help:     meta.LongHelp,
		Usage:    []string{pattern},
		Sections: sections,
	}
	page.WriteErr()
	return 0
}

// childEntries answers the commands registered under a name, each with the
// summary it registered, so a namespace page and a refusal name the same set.
func childEntries(name string) []helpfmt.HelpEntry {
	var tb textbuf.Buffer
	prefix := tb.Str(name).Byte(' ').String()

	entries := make([]helpfmt.HelpEntry, 0, 4)
	for _, command := range Commands() {
		if child, ok := strings.CutPrefix(command.Name, prefix); ok {
			entries = append(entries, helpfmt.HelpEntry{Name: child, Desc: command.Meta.Description})
		}
	}
	return entries
}
