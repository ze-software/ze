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

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// isHelpArg reports whether the word asks for usage rather than naming a
// command. The spellings are declared once, by the package that also reads
// them after a verb (leaction.IsHelpArg).
func isHelpArg(word string) bool { return leaction.IsHelpArg(word) }

// Commands answers le's commands with the metadata from the shared local
// registry. ListLocal sorts by full path, so the stripped names remain sorted.
func Commands() []registry.RootCommand {
	all := command.ListLocal()
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

// Usage writes the root page to stderr, beside a refusal: the reader typed a
// command le does not hold, and the page names every one it does. program
// supplies the page name, "le" for the binary and "ze le" for a ze_le build, so
// every line a reader copies is valid for the active program.
//
// The page is the manifest's own (manifest.go), so the page beside a refusal,
// the page `le help` answers and the payload a bare invocation answers name one
// command set.
func Usage(program string) {
	page := manifestOf(program).page()
	page.Width = leaction.PageWidth(os.Stderr)
	page.WriteErr()
}

// helpRoot writes the root page to stdout: `le help` and `le --help` asked a
// question, so the answer goes where a bare `le` puts it, byte for byte.
func helpRoot(program string) int {
	var tb textbuf.Buffer
	tb.Str(manifestOf(program).Text()).StdOut() //nolint:errcheck // CLI output
	return 0
}

// commandWordsMax bounds how many argv words the lookup may consume. le's
// deepest registered path is one namespace and one member, so two words after
// `le` is the whole grammar.
//
// The bound is load-bearing. LookupLocalData matches the longest registered
// prefix of whatever it is handed, so an unbounded span would let a value
// further along the line be read as a command word: `le job run label x
// command le go lint` offers nine.
const commandWordsMax = 2

// resolve answers the registered command among the leading words of argv: the
// name as it was typed, its handler, and the words the tool itself receives.
//
// The pipe word ends the candidate span, so `le verify list | json` offers the
// matcher `verify list` and never the chain.
func resolve(args []string) (string, command.LocalDataHandler, []string) {
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

	handler, trailing := command.LookupLocalData(words)
	if handler == nil {
		return "", nil, nil
	}
	consumed := span - len(trailing)

	var tb textbuf.Buffer
	return tb.Join(args[:consumed], " ").String(), handler, args[consumed:]
}

// members answers the commands registered under a namespace token, with the
// token stripped: `spec` answers citation, claim, current, status and the rest.
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
	checkRegistrations()
	leaction.SetProgram(program)
	// The root names no command, so it answers what le holds, as a payload
	// through Run: `le | json` renders the manifest, and a bare `le` prints the
	// page a reader has always seen. `le` asked a question rather than making a
	// mistake, so the answer goes to stdout and the code is 0.
	if own, _ := splitChain(args); len(own) == 0 {
		// A person at a terminal who typed nothing at all gets the menu over
		// the same tree (D-Q4); `le | json` and every pipe still get the page.
		if len(args) == 0 && interactive() {
			return openMenu(program)
		}
		return Run("", func(command.ValidatedArgs) (any, int) { return manifestOf(program), 0 }, args, os.Stdout, os.Stderr)
	}
	if isHelpArg(args[0]) {
		if len(args) == 1 {
			return helpRoot(program)
		}
		// `ai` is the one reserved word after the help word: the reference an
		// agent reads. The ai namespace's own page is `le ai help`.
		if args[1] == aiWord {
			return helpAI(program, args[2:])
		}
		return helpAsked(program, args[1:])
	}
	return dispatchLine(program, args, false)
}

// dispatchLine runs one command line that is not the root and not a help
// question. At a terminal, a missing Required value is asked for first
// (fillLine); fromMenu says the reader built the line at the menu, so it is
// echoed even when nothing was missing.
func dispatchLine(program string, args []string, fromMenu bool) int {
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
		if interactive() {
			line, err := fillLine(program, name, toolArgs, fromMenu)
			if err != nil {
				return canceled(program, err)
			}
			toolArgs = line.args
		}
		return Run(name, handler, toolArgs, os.Stdout, os.Stderr)
	}

	// A namespace token is not an unknown command, it is a question: what does
	// this namespace hold. It is answered with the members and their summaries
	// and exit 0 (owner decision, 2026-09-24), as a bare `le` is. A word typed
	// after the token that names no member is a mistake, and is refused.
	if held := members(args[0]); len(held) != 0 {
		if len(args) == 1 {
			return helpPage(program, args[0])
		}
		if len(args) == 2 && isHelpArg(args[1]) {
			return helpPage(program, args[0])
		}
		var tb textbuf.Buffer
		message := tb.Str("error: ").Str(args[0]).
			Str(" is a namespace; it needs one of: ").Join(held, " | ").String()
		fmt.Fprintln(os.Stderr, message) //nolint:errcheck // CLI output
		return 2
	}

	var tb textbuf.Buffer
	tb.Str("unknown command: ").Str(args[0]).Byte('\n').StdErr() //nolint:errcheck // CLI output
	Usage(program)
	return 2
}

// helpAsked answers `le help <command>...`: it resolves the words the reader
// typed after the help word and renders that node, rather than the whole tree
// the bare help word answers. One word after the command names an action and
// answers its page; two name an action and one of its keywords and answer the
// keyword's page.
func helpAsked(program string, words []string) int {
	name, handler, rest := resolve(words)
	if handler == nil {
		// A namespace holds commands without being one, so it has no handler to
		// resolve and its page is still worth printing.
		if len(members(words[0])) == 0 {
			var tb textbuf.Buffer
			tb.Str("unknown command: ").Str(words[0]).Byte('\n').StdErr() //nolint:errcheck // CLI output
			Usage(program)
			return 2
		}
		return helpPage(program, words[0])
	}
	own, _ := splitChain(rest)
	if len(own) == 0 {
		return helpPage(program, name)
	}
	list, declared := ActionsOf(name)
	if !declared {
		return refuseHelp(program, name, "publishes no action table yet, so its own page is the whole help")
	}
	// A pass-through area holds no action to page, and its program owns the
	// grammar, so the program is the one to ask.
	if list.PassesWords {
		var tb textbuf.Buffer
		return refuseHelp(program, name, tb.Str("passes its words to ").Str(list.PassesTo).
			Str(", so `").Str(program).Byte(' ').Str(name).Str(" <words> --help` asks it, and le's page for it is").String())
	}
	switch len(own) {
	case 1:
		return writeActionPage(program, name, list, own[0])
	case 2:
		page, err := list.KeywordPage(program, own[0], own[1])
		if err != nil {
			leaction.ReportError(err)
			return 2
		}
		page.Width = leaction.PageWidth(os.Stdout)
		page.WriteOut()
		return 0
	default:
		return refuseHelp(program, name, "takes at most an action and one of its keywords after its name")
	}
}

// refuseHelp refuses a help question the command cannot answer, naming the
// page that does answer, and returns the usage code.
func refuseHelp(program, name, why string) int {
	var tb textbuf.Buffer
	tb.Str("error: ").Str(name).Byte(' ').Str(why).Str(": ").
		Str(program).Str(" help ").Str(name).Byte('\n').StdErr() //nolint:errcheck // CLI output
	return 2
}

// writeActionPage writes one action's page to stdout, or refuses a verb the
// area does not hold, naming the ones it does.
func writeActionPage(program, name string, list leaction.List, verb string) int {
	page, held := list.ActionPage(program, verb)
	if !held {
		var tb textbuf.Buffer
		tb.Str("error: no such action in ").Str(name).Str(": ").Str(verb).Byte('\n').StdErr() //nolint:errcheck // CLI output
		tb.Reset()
		verbs := make([]string, 0, len(list.Actions))
		for _, row := range list.Actions {
			verbs = append(verbs, row.Verb)
		}
		tb.Str("try one of: ").Join(verbs, " | ").Byte('\n').StdErr() //nolint:errcheck // CLI output
		return 2
	}
	page.Width = leaction.PageWidth(os.Stdout)
	page.WriteOut()
	return 0
}

// asksForUsage reports whether the reader's LAST word is a help word ASKING a
// question, rather than the bare word `help` an area's grammar reads as data.
// The bare word carries no dash, so a keyword can introduce it in any position.
// `le repo rewrite replace file <path> old beta new help` types `help` as the
// text `new` takes.
//
// Four spellings ask the question, and the set is closed: `help`, `--help`,
// `-h` and `-help` (leaction.IsHelpArg). In an area with a table, a dash-leading
// one in a keyword's value slot asks nothing: it is an option where a value is
// expected, so the area's parser refuses it by name and answers 2, and `le test
// stress-repro run suite -help` starts no burn either way.
//
// Every other option reaches the area. Where the area declared a table, its
// parser refuses the option by name and answers 2 (leaction.parseArguments).
// Where it declared none, the dispatcher has no grammar to read, so the area's
// own parser decides. `le job run label x command ./le test bgp encode
// --list` hands `--list` to the child, which is what `command <argv...>` means,
// and so does `command echo -html=cover.out`.
//
// The area's registered table is what tells a value from a question, so an area
// that registered one is asked, and a forwarding row answers that the words
// after its verb are the program's (leaction.List.TrailingWordAsksForHelp). An area
// that registered as forwarding (RegisterForwarding) registered a pass-through
// table, which answers that every word is its program's, the help word
// included, so the program prints its own help. An area
// that registered neither publishes no grammar, so the dispatcher guards. To
// run a probe's work is the worse of the two failures, and it is the burn this
// guard exists to stop. The cost is a value spelled like a help word,
// unreachable in those areas until each one declares its table.
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
	return list.TrailingWordAsksForHelp(args)
}

// helpTrailing answers a help word typed at the end of an invocation. It calls
// no handler, so an area whose work starts from its first argument starts
// nothing.
//
// The page of one action is rendered from the listing its area registered
// (leroot.RegisterActions). Dispatch refuses to start while any area holds none
// (requireEveryAreaHasATable), so the not-declared branches below answer the
// area's page only for a name no area registered.
func helpTrailing(program, name string, own []string) int {
	if len(own) == 1 {
		return helpPage(program, name)
	}
	list, declared := ActionsOf(name)
	if !declared {
		return helpPage(program, name)
	}
	if _, held := list.ActionPage(program, own[0]); !held {
		return helpPage(program, name)
	}
	return writeActionPage(program, name, list, own[0])
}

// helpPage writes what the registry knows about one command to stdout: its
// summary, its usage, the actions its table declares, and the commands
// registered under it. It answers 0, because the reader asked a question.
//
// It never calls the command's own handler. A single-action area answers a bare
// invocation by RUNNING its gate, so re-entering the handler to print a page
// would scan the tree, write a file, or start a build on `--help`.
func helpPage(program, name string) int {
	page := areaPage(program, name)
	page.Width = leaction.PageWidth(os.Stdout)
	page.WriteOut()
	return 0
}

// areaPage builds one command's page. An area with an action table renders
// the table's own listing (leaction.List.Section), so `le <area>` and
// `le <area> --help` show one set of lines. An area without one publishes only
// its declared grammar (registry.Meta.Subs), and each alternative of it is a
// usage line. A pass-through area (leaction.PassesWords) keeps that declared
// grammar, which names its program's subcommands, and adds the section saying
// where its words go.
func areaPage(program, name string) helpfmt.Page {
	var meta registry.Meta
	for _, command := range Commands() {
		if command.Name == name {
			meta = command.Meta
			break
		}
	}
	// A namespace is no command, so its line is the summary it registered.
	if summary, named := NamespaceSummary(name); named {
		meta.ShortHelp = summary
	}

	var tb textbuf.Buffer
	command := tb.Str(program).Byte(' ').Str(name).String()
	page := helpfmt.Page{Command: command, ShortHelp: meta.ShortHelp, Description: meta.Description}
	list, declared := ActionsOf(name)
	switch {
	case declared && !list.PassesWords:
		page.Usage = list.Page(program).Usage
		page.Sections = append(page.Sections, list.Section())
	case meta.ResolveSubs() != "":
		for grammar := range strings.SplitSeq(meta.ResolveSubs(), " | ") {
			tb.Reset()
			page.Usage = append(page.Usage, tb.Str(command).Byte(' ').Str(grammar).Str(" [| json | yaml | table]").String())
		}
	case len(members(name)) != 0:
		tb.Reset()
		page.Usage = []string{tb.Str(command).Str(" <command> ...").String()}
	default:
		tb.Reset()
		page.Usage = []string{tb.Str(command).Str(" [| json | yaml | table]").String()}
	}
	// A pass-through area keeps the usage its registration declares, which
	// names the program's own subcommands, and says where the words go.
	if declared && list.PassesWords {
		page.Sections = append(page.Sections, list.Section())
	}
	if held := childEntries(name); len(held) != 0 {
		page.Sections = append(page.Sections, helpfmt.HelpSection{Title: "Commands", Entries: held})
	}
	return page
}

// childEntries answers the commands registered under a name, each with its
// purpose tag and the first sentence of its summary, so a namespace page and
// the root page describe a command in the same words.
func childEntries(name string) []helpfmt.HelpEntry {
	var tb textbuf.Buffer
	prefix := tb.Str(name).Byte(' ').String()

	entries := make([]helpfmt.HelpEntry, 0, 4)
	for _, command := range Commands() {
		if child, ok := strings.CutPrefix(command.Name, prefix); ok {
			entries = append(entries, helpfmt.HelpEntry{Name: child, Desc: entryText(command.Name, command.Meta.ShortHelp)})
		}
	}
	return entries
}

// entryText is one command's line in a listing: its purpose in brackets, then
// its one-line summary. A command registered without a group says so rather
// than borrowing one.
func entryText(name, summary string) string {
	tag := ungroupedTag
	if group, declared := GroupOf(name); declared {
		tag = string(group)
	}
	var tb textbuf.Buffer
	return tb.Byte('[').Str(tag).Str("] ").Str(summary).String()
}
