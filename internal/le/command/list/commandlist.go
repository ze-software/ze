// Design: docs/architecture/api/commands.md -- the command verb taxonomy
//
// Package commandlist is the `ze-command-list` gate: every command ze
// registers, classified by verb, read from the live handlers and the YANG
// command tree rather than parsed out of source.
//
// It blank-imports the product's composition root, which is what makes the
// answer accurate: the RPCs and the streaming prefixes come from the same
// registrations the daemon runs. That import is allowed in exactly this
// direction (docs/architecture/core-design.md): le may link ze to
// introspect it, ze never links le, and le never RUNS a product command
// (internal/le/leroot/dispatch.go).
//
// The tool answers Commands (report.go) rather than printing one. That is what
// lets `| json` feed a script and `| match show` keep one verb's rows, and it
// is why nothing here writes a line of JSON, YAML or table code.

package commandlist

import (
	"fmt"
	"os"
	"sort"
	"strings"

	// The blank import triggers every plugin and RPC registration, so the
	// inventory reports the product rather than a subset of it. plugin/all is
	// used rather than a hand-picked list so the import set matches the
	// runtime's exactly.
	_ "github.com/ze-software/ze/internal/component/plugin/all"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/yang"
	pluginregistry "github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/le/leroot"
)

// dashboardPath is the one command reached through neither an RPC nor a
// streaming prefix: the BGP dashboard is a TUI the CLI owns. It is listed so
// the inventory covers what an operator can type, not only what the wire
// carries.
const dashboardPath = "monitor bgp"

// Collect answers every registered command, sorted by verb and then by path.
//
// The error is the YANG loader's. The script discarded it, and a discarded
// loader error is not a smaller answer: WireMethodToPath over a nil loader
// answers an empty map, so EVERY command's Path silently falls back to its wire
// method, under a header saying the output is always accurate. It is reported
// here instead.
func Collect() (Commands, error) {
	loader, err := yang.DefaultLoader()
	if err != nil {
		return nil, fmt.Errorf("load the YANG command tree: %w", err)
	}
	wireToPath := yang.WireMethodToPath(loader)

	var commands Commands

	// Builtin RPCs.
	for _, rpc := range pluginserver.AllBuiltinRPCs() {
		path := wireToPath[rpc.WireMethod]
		if path == "" {
			// No YANG mapping, so the wire method is the only name this
			// command has.
			path = rpc.WireMethod
		}
		commands = append(commands, Command{
			Verb:       classifyVerb(path),
			Path:       path,
			WireMethod: rpc.WireMethod,
			Source:     "builtin",
		})
	}

	// Streaming handlers, minus the ones a builtin RPC already named.
	for _, prefix := range pluginserver.StreamingPrefixes() {
		if hasPath(commands, prefix, strings.EqualFold) {
			continue
		}
		commands = append(commands, Command{
			Verb:   classifyVerb(prefix),
			Path:   prefix,
			Source: "streaming",
		})
	}

	// TUI-only commands, added only when nothing above already named them.
	if !hasPath(commands, dashboardPath, func(a, b string) bool { return a == b }) {
		commands = append(commands, Command{
			Verb:   "monitor",
			Path:   dashboardPath,
			Source: "cli",
		})
	}

	// Commands a plugin declares on its registration. A plugin's command is
	// dispatched through the plugin rather than through a builtin RPC or a
	// streaming prefix, so nothing above names one and the inventory reported
	// none of them.
	//
	// A HIDDEN declaration is included. The inventory answers what ze
	// registers, and a hidden command is registered; it is kept out of
	// completion, which is a different question.
	for _, registration := range pluginregistry.All() {
		for index := range registration.Commands {
			path := registration.Commands[index].Name
			if path == "" || hasPath(commands, path, strings.EqualFold) {
				continue
			}
			commands = append(commands, Command{
				Verb:   classifyVerb(path),
				Path:   path,
				Source: "plugin",
			})
		}
	}

	// What each command DECLARES about its answer. One reader answers for both
	// declaration channels: the registries an in-tree package writes from
	// init(), and the registration a plugin carries
	// (internal/component/command, DeclaredForCommand). `ze help command
	// --json` and the wiki catalog read that same function, so the three
	// inventories cannot disagree about one command.
	for index := range commands {
		entry := &commands[index]
		declared := command.DeclaredForCommand(entry.Path)
		if declared.ShapeDeclared {
			entry.Shape = declared.Shape.String()
		}
		entry.ColumnOrders = command.ColumnNames(declared.Columns)
		entry.AddressFields = declared.AddressFields
		entry.Aliases = aliasesFor(declared)
	}

	sort.Slice(commands, func(i, j int) bool {
		if commands[i].Verb != commands[j].Verb {
			return commands[i].Verb < commands[j].Verb
		}
		return commands[i].Path < commands[j].Path
	})
	return commands, nil
}

// aliasesFor answers the pipe chains a command names, from the declaration the
// inventory already resolved for it.
//
// It carries a registered plugin's aliases beside the alias registry's, because
// this process starts no plugin and so receives no Stage 1 message. An alias
// declared by a plugin that is not in this composition root, an EXTERNAL plugin,
// stays a running daemon's answer (internal/plugins/meta/cmd/help.go,
// commandHelp).
func aliasesFor(declared command.Declared) []Alias {
	if len(declared.Aliases) == 0 {
		return nil
	}
	aliases := make([]Alias, 0, len(declared.Aliases))
	for _, alias := range declared.Aliases {
		aliases = append(aliases, Alias{
			Name: alias.Name, Description: alias.Description, Expansion: alias.Expansion,
		})
	}
	return aliases
}

// hasPath reports whether any collected command already carries path, compared
// with the caller's equality. The two callers differ: a streaming prefix is
// matched case-insensitively, and the dashboard path exactly.
func hasPath(commands Commands, path string, equal func(a, b string) bool) bool {
	for _, entry := range commands {
		if equal(entry.Path, path) {
			return true
		}
	}
	return false
}

// classifyVerb answers the taxonomy verb of a CLI path, which is its first word
// when that word is one of the five, and "-" when it is not.
func classifyVerb(path string) string {
	first, _, _ := strings.Cut(path, " ")
	switch strings.ToLower(first) {
	case "show", "set", "delete", "update", "monitor":
		return strings.ToLower(first)
	}
	return "-"
}

// Answer is the `le command list` command. It takes no arguments: the registry
// is the product's own, and the rendering is the operator's to choose with a
// pipe operator (ai/rules/cli.md).
func Answer(args []string) (any, int) {
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "error: command-list takes no arguments, got %q\n", args[0]) //nolint:errcheck // CLI output
		fmt.Fprintln(os.Stderr, "usage: le command list [| json | yaml | table]")           //nolint:errcheck // CLI output
		return nil, 1
	}

	commands, err := Collect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err) //nolint:errcheck // CLI output
		return nil, 1
	}
	return commands, 0
}

// Commands answers a rendering of itself, so the bare command prints the table
// the gate has always printed.
var _ leroot.Prose = Commands(nil)
