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
// command. The spellings are declared once, by the package that also reads
// them after a verb (leaction.IsHelpArg).
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

// Usage writes the root page to stderr, which is where a refusal's reader is
// looking. program supplies the page name: "le" for the binary and "ze le" for
// a ze_le build. Thus, every line that a reader copies is valid for the active
// program.
//
// The page is the manifest's own (manifest.go), so the page beside a refusal
// and the payload a bare invocation answers name one command set. Eighty-six
// commands in one alphabetical list read as one thing. They are five, and the
// groups say which, in the order a person meets them (group.go).
func Usage(program string) {
	page := manifestOf(program).page()
	page.WriteErr()
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
	// The root names no command, so it answers what le holds, as a payload
	// through Run: `le | json` renders the manifest, and a bare `le` prints the
	// page a reader has always seen. `le` asked a question rather than making a
	// mistake, so the answer goes to stdout and the code is 0.
	if own, _ := splitChain(args); len(own) == 0 {
		return Run("", func([]string) (any, int) { return manifestOf(program), 0 }, args, os.Stdout, os.Stderr)
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
		// A help word the reader typed LAST asks what this command takes, and
		// it is answered here rather than by the handler: an area that
		// hand-rolls its dispatch reads the word as a value and starts the work
		// it names. The bare word `help` a declared keyword introduced is not
		// this question, and travels on as that keyword's value.
		own, _ := splitChain(toolArgs)
		if asksForUsage(name, own) {
			return helpTrailing(program, name, own)
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

// asksForUsage reports whether the reader's LAST word is a help word ASKING a
// question, rather than the bare word `help` an area's grammar reads as data.
// The bare word carries no dash, so a keyword can introduce it in any position.
// `le source-rewrite replace file <path> old beta new help` types `help` as the
// text `new` takes.
//
// Four spellings ask the question here, in every area, and the set is closed:
// `help`, `--help`, `-h` and `-help` (leaction.IsHelpArg). `le stress-repro run
// suite -help` therefore renders a page rather than the burn.
//
// Every other option reaches the area. Where the area declared a table, its
// parser refuses the option by name and answers 2 (leaction.parseArguments).
// Where it declared none, the dispatcher has no grammar to read, so the area's
// own parser decides. `le job run label x command bin/ze-test bgp encode
// --list` hands `--list` to the child, which is what `command <argv...>` means,
// and so does `command echo -html=cover.out`.
//
// The area's registered table is what tells a value from a question, so an area
// that registered one is asked. An area that registered none publishes no
// grammar, so the dispatcher guards. To run a probe's work is the worse of the
// two failures, and it is the burn this guard exists to stop. The cost is a
// value spelled like a help word, unreachable in those areas until spec 2 has
// each one declare its table.
func asksForUsage(name string, args []string) bool {
	if len(args) == 0 {
		return false
	}
	if !isHelpArg(args[len(args)-1]) {
		return false
	}
	list, declared := ActionsOf(name)
	if !declared {
		return true
	}
	return !list.TrailingWordIsValue(args)
}

// helpTrailing answers a help word typed at the end of an invocation. It calls
// no handler, so an area whose work starts from its first argument starts
// nothing.
//
// The grammar of one action is rendered from the listing its area registered
// (leroot.RegisterActions). An area that registered none still gets its page,
// because guarding every area is what keeps a probe from running work, and the
// seven that hand-roll their dispatch publish no grammar until
// plan/spec-le-every-area-dispatches-through-one-table.md migrates them.
func helpTrailing(program, name string, own []string) int {
	if len(own) == 1 {
		return helpNode(program, name)
	}
	list, declared := ActionsOf(name)
	if !declared {
		return helpNode(program, name)
	}
	text, rendered := list.UsageText(own[0])
	if !rendered {
		return helpNode(program, name)
	}

	var tb textbuf.Buffer
	tb.Str(text).StdErr() //nolint:errcheck // CLI output
	return 0
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
		LongHelp: meta.LongHelp,
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
