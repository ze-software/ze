// Design: docs/architecture/api/commands.md -- shell root command inventory
// Overview: main.go -- completion dispatch
// Related: bash.go, zsh.go, fish.go, nushell.go -- shell generators consuming root list

package completion

import (
	"sort"
	"strings"

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
			Description: cmd.Meta.Description,
		})
	}

	entries = append(entries, rootEntry{
		Name:        verbShow,
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
