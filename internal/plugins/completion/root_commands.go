// Design: docs/architecture/api/commands.md -- shell root command inventory
// Overview: main.go -- completion dispatch
// Related: bash.go, zsh.go, fish.go, nushell.go -- shell generators consuming root list

package completion

import (
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

type rootEntry struct {
	Name        string
	Description string
}

func shellRootCommands() []rootEntry {
	commands := registry.ListRoot()
	entries := make([]rootEntry, 0, len(commands)+1)

	for _, cmd := range commands {
		if !shellVisible(cmd) {
			continue
		}
		entries = append(entries, rootEntry{
			Name:        cmd.Name,
			Description: cmd.Meta.ShortHelp,
		})
	}

	entries = append(entries, rootEntry{
		Name:        command.VerbShow,
		Description: "Show daemon state (read-only commands)",
	})

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}

func bashRootNames() string {
	roots := shellRootCommands()
	names := make([]string, len(roots))
	for i, r := range roots {
		names[i] = r.Name
	}
	return strings.Join(names, " ")
}

func zshRootArray() string {
	roots := shellRootCommands()
	var tb textbuf.Buffer
	for _, r := range roots {
		tb.Str("        '").Str(r.Name).Byte(':').Str(escapeSingleQuote(r.Description)).Str("'\n")
	}
	return tb.String()
}

func fishRootLines() string {
	roots := shellRootCommands()
	var tb textbuf.Buffer
	for _, r := range roots {
		tb.Str("complete -c ze -n __ze_needs_command -a ").Str(r.Name).Str(" -d '").Str(escapeSingleQuote(r.Description)).Str("'\n")
	}
	return tb.String()
}

// shellRootsWithSubVerbs answers the shell-visible registered roots whose Subs
// names at least one second word (rootSubVerbs), sorted by name. A generator
// emits a second-word arm for each one, so a root that registers its verbs
// completes them with no edit to any generator.
func shellRootsWithSubVerbs() []string {
	var names []string
	for _, cmd := range registry.ListRoot() {
		if !shellVisible(cmd) {
			continue
		}
		if len(rootSubVerbs(cmd.Meta.ResolveSubs())) == 0 {
			continue
		}
		names = append(names, cmd.Name)
	}
	slices.Sort(names)
	return names
}

// fishRootSubVerbLines emits one fish rule per root with sub-verbs. The words
// come from `ze completion words <root>` at tab time. Fish drops a candidate
// it already holds, so a root that also has a hand-written rule below (bgp,
// signal) gains only the verbs that rule does not list.
func fishRootSubVerbLines() string {
	var tb textbuf.Buffer
	for _, name := range shellRootsWithSubVerbs() {
		tb.Str("complete -c ze -n 'test (__ze_depth ").Str(name).Str(") = 0' -a '(__ze_complete_dynamic ").Str(name).Str(")'\n")
	}
	return tb.String()
}

func escapeSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", "'\\''")
}

func nushellRootCompleter() string {
	roots := shellRootCommands()
	var tb textbuf.Buffer
	tb.Str("def \"nu-complete ze commands\" [] {\n    [\n")
	for _, r := range roots {
		tb.Str("        { value: \"").Str(r.Name).Str(" \", description: \"").Str(r.Description).Str("\" }\n")
	}
	tb.Str("    ]\n}\n")
	return tb.String()
}

func shellVisible(cmd registry.RootCommand) bool {
	if strings.HasPrefix(cmd.Name, "-") {
		return false
	}
	if cmd.Meta.Section == registry.SectionTest {
		return false
	}
	if cmd.Meta.Mode == "setup" {
		return false
	}
	return true
}
