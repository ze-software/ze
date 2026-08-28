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

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// isHelpArg reports whether the word asks for the command list rather than
// naming a command.
func isHelpArg(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

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
		Usage(program)
		return 0
	}

	words := [2]string{"le", args[0]}
	handler, trailing := registry.LookupLocalData(words[:])
	if handler != nil && len(trailing) == 0 {
		return Run(args[0], Answer(handler), args[1:], os.Stdout, os.Stderr)
	}

	fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0]) //nolint:errcheck // CLI output
	Usage(program)
	return 1
}
