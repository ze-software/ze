// Design: docs/architecture/api/commands.md — help command and help ai registration
// Related: help_command.go — the command catalog
// Related: help_ai.go — the agent reference
// codegen:skip -- wired via cmd/ze/ze_core_dispatch.go; imports cli/client which cycles with plugin/all.

// Package help owns `help command` and `help ai`: the command catalog and the
// agent reference this binary builds from its own registries. Both are
// local-data commands at the offline root `help`, which is not a verb, so no
// YANG node sits at their own paths: the daemon serves the same handlers as
// `show command catalog` and `show command reference`, the paths
// ze-help-cmd.yang declares and each registration names (registry.Meta.DaemonPath).
package help

import (
	"io"
	"os"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	_ "github.com/ze-software/ze/internal/plugins/help/yang" // the daemon paths the registrations name
)

const (
	// flagJSON asks either command for its JSON form on the local route.
	flagJSON = "--json"
	// keywordFilter is the filter leaf ze-help-cmd.yang declares.
	keywordFilter = "filter"
	// modeOffline is the registry mode of a command that needs no daemon.
	modeOffline = "offline"
	// keyCatalog is the envelope key of the `help command` answer.
	keyCatalog = "command-catalog"
)

func init() {
	// `help command` answers its catalog rows as DATA, so `| json` reaches
	// them on every route; drawCatalog gives the daemon's text route the table
	// the local route prints. `help ai` answers a Markdown document (a Blob,
	// as `show yang doc` is), or the reference as data under the local
	// `--json` flag.
	command.MustRegisterTextRenderer(keyCatalog, drawCatalog)
	command.MustRegisterLocalData("help command", answerHelpCommand, registry.Meta{
		ShortHelp: "List every command this binary carries with its summary.",
		Description: "A filter word keeps the commands whose path holds it, and the answer renders as " +
			"JSON for a program to read.",
		Mode:       modeOffline,
		DaemonPath: "show command catalog",
	}, render)
	command.MustRegisterLocalData("help ai", answerAIHelp, registry.Meta{
		ShortHelp: "Print the agent reference this binary builds from its own registries.",
		Description: "The sections are cli, api, mcp, dispatch and all, and the answer renders as JSON " +
			"for a program to read.",
		Mode:       modeOffline,
		DaemonPath: "show command reference",
	}, render)
	// The shape belongs to the answer, so it is declared on both paths that
	// return it: the local root and the daemon path.
	command.RegisterShape([]string{"help command", "show command catalog"}, command.ShapeMap)
	command.RegisterShape([]string{"help ai", "show command reference"}, command.ShapeBlob)
}

// render prints a help answer on the local `ze help` route.
//
// It replaces command.RenderLocalAnswer for these two commands because that
// renderer drops a failed write, and `ze help ai | head` closing the pipe
// must exit non-zero rather than report a truncated page as whole.
func render(path string, payload any) int {
	return renderTo(os.Stdout, os.Stderr, path, payload)
}

// renderTo writes a help answer to w, its diagnostics to diag, and answers 1
// when a write to w fails. A payload these commands never answer goes through
// command.RenderLocalAnswer.
func renderTo(w, diag io.Writer, path string, payload any) int {
	switch answer := payload.(type) {
	case catalogAnswer:
		return printCatalog(w, diag, answer)
	case referenceAnswer:
		return printReference(w, diag, &answer)
	case command.Blob:
		if _, err := w.Write(answer); err != nil {
			return 1
		}
		return 0
	}
	return command.RenderLocalAnswer(path, payload)
}
