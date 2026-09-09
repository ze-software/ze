// Design: docs/architecture/api/process-protocol.md — plugin CLI dispatch
//
// Package plugin provides the ze plugin subcommand.
package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/suggest"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// Run executes the plugin subcommand with the given arguments.
// Returns exit code.
func Run(args []string) int {
	if len(args) < 1 {
		usage()
		return 1
	}

	switch args[0] {
	case "test":
		// Test is a debugging tool, not a real plugin.
		return cmdPluginTest(args[1:])
	case "help", "-h", "--help": //nolint:goconst // consistent with main.go
		usage()
		return 0
	}

	// Look up in registry.
	reg := registry.Lookup(args[0])
	if reg == nil {
		fmt.Fprintf(os.Stderr, "unknown plugin subcommand: %s\n", args[0])
		if s := suggest.Command(args[0], append(registry.Names(), "test", "help")); s != "" {
			fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", s)
		}
		usage()
		return 1
	}
	// Query mode is answered here, from the registration this process already
	// holds, and never by the plugin. No plugin code runs at all, so a query
	// cannot initialize data, open a connection, bind a socket, or start a
	// timer (docs/architecture/cli/plugin-modes.md).
	if sdk.QueryModeRequested() {
		return answerQuery(os.Stdout, reg)
	}
	return reg.CLIHandler(args[1:])
}

// answerQuery writes the plugin's Stage 1 declaration to out as one
// newline-framed request line, and returns the exit code. The line itself is
// rpc.WriteDeclaration's, the same writer the SDK gives a plugin binary ze does
// not carry, so both answers are one message.
//
// Commands and pipes are the declaration fields the compiled-in registration
// carries. The Stage 1 fields with no compiled-in twin are not answered
// (spec-plugin-declaration-fields-on-registration).
func answerQuery(out io.Writer, reg *registry.Registration) int {
	declaration := rpc.DeclareRegistrationInput{Commands: reg.Commands, Pipes: reg.Pipes}

	if err := rpc.WriteDeclaration(out, &declaration); err != nil {
		writeError(os.Stderr, "error: plugin '%s' declaration: %v", reg.Name, err)
		return 1
	}
	return 0
}

func usage() {
	// Build plugin entries dynamically from the registry.
	regs := registry.All()
	pluginEntries := make([]helpfmt.HelpEntry, 0, len(regs)+2)
	for _, reg := range regs {
		desc := reg.Description
		if len(reg.RFCs) > 0 {
			desc += " (RFC " + textbuf.Join(reg.RFCs, ", ") + ")"
		}
		pluginEntries = append(pluginEntries, helpfmt.HelpEntry{Name: reg.Name, Desc: desc})
	}
	pluginEntries = append(pluginEntries,
		helpfmt.HelpEntry{Name: "test", Desc: "Test plugin YANG schema and config delivery (debugging)"},
		helpfmt.HelpEntry{Name: "help", Desc: "Show this help"},
	)

	p := helpfmt.Page{
		Command: "ze plugin",
		Summary: "Plugin subcommands",
		Usage:   []string{"ze plugin <subcommand>"},
		Sections: []helpfmt.HelpSection{
			{Title: "Plugin Subcommands", Entries: pluginEntries},
		},
		Examples: []string{
			`ze plugin test --plugin ze.hostname --schema config.conf`,
			`ze plugin test --plugin ze.hostname --tree config.conf`,
			`ze plugin test --plugin ze.hostname --json config.conf`,
		},
		SeeAlso: []string{
			"Plugins run as API processes spawned by the router via plugin configuration.",
		},
	}
	p.WriteErr()
}
