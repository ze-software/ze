// Design: docs/architecture/web-interface.md -- browser equivalents of CLI controls.
// Related: cli_terminal.go -- authenticated execution and transport completion

package web

import (
	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// executeTerminalControl handles the same position-independent Enter controls
// as the CLI model. In config mode, top changes neither their position nor
// their meaning. Every iteration consumes one bounded command token.
func executeTerminalControl(schema *config.Schema, tree *config.Tree, mgr *EditorManager, username string, path []string, mode string, cmd cliCommand) (terminalCommandResult, bool) {
	if mode == terminalModeConfig {
		for {
			inner, underTop := commandBehindTop(cmd)
			if !underTop {
				break
			}
			cmd = inner
		}
	}
	line := cmd.Verb
	if len(cmd.Args) > 0 {
		line = ""
	}
	if mode == terminalModeConfig {
		if cmd.Verb == verbRun {
			if cmd.words.Tail().String() == "clear" {
				line = "clear"
			}
		}
	}
	answer := terminalCommandResult{path: path, mode: mode}
	if canonical, lifecycle := contract.LifecycleCommand(line); lifecycle {
		if !cmd.confirmed {
			answer.confirmation = "This will " + line + " the daemon for all connected users. Continue?"
			return answer, true
		}
		answer.operational = canonical
		return answer, true
	}
	switch line {
	case "clear":
		if mode == terminalModeConfig {
			answer.output = serializeTreeAtPath(displayTree(tree, mgr, username), schema, path)
			answer.outputComplete = true
		}
		return answer, true
	case verbExit, verbQuit:
		if !cmd.confirmed {
			if mgr.ChangeCount(username) > 0 {
				answer.confirmation = "Pending changes will stay in your candidate. Leave this CLI mode?"
				return answer, true
			}
		}
		if mode == terminalModeConfig {
			if line == verbExit {
				answer.mode = terminalModeOperational
				return answer, true
			}
		}
		answer.navigate = "/show/"
		return answer, true
	}
	return answer, false
}

// terminalEditorHelp renders the shared CLI catalog in both browser modes.
// The catalog remains the authority for command forms and their descriptions.
func terminalEditorHelp() string {
	var output textbuf.Buffer
	output.Str("commands:\n")
	for _, entry := range cli.EditorHelpEntries() {
		output.Str("  ").Str(entry.Name).Str("  ").Str(entry.Desc).Byte('\n')
	}
	return output.String()
}
