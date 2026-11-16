// Design: docs/architecture/web-interface.md -- reuse the CLI editor producers.

package web

import (
	"strings"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/core/configtext"
)

// executeTerminalEditorCommand keeps display and supplied-content loading in
// the CLI owner. Both browser transports render this result.
func executeTerminalEditorCommand(mgr *EditorManager, username string, path []string, cmd cliCommand) (terminalCommandResult, bool) {
	answer := terminalCommandResult{path: path, mode: terminalModeConfig}
	switch cmd.Verb {
	case verbShow, verbErrors, verbHistory, verbOption, verbLoad, verbCompare, verbInsert, verbWho, verbSave, verbRollback, verbDisconnect, verbUp, verbEdit, verbTop, verbSet, verbDelete, verbRename, verbCopy, verbDeactivate, verbActivate:
	default:
		return answer, false
	}
	line := cmd.words.String()
	if line == "" {
		parts := []string{cmd.Verb}
		for _, word := range cmd.Args {
			parts = append(parts, configtext.QuoteCommandWord(word))
		}
		line = strings.Join(parts, " ")
	}
	if cmd.Verb == verbCompare {
		line = "show | compare"
		for _, argument := range cmd.Args {
			line += " " + configtext.QuoteCommandWord(argument)
		}
	}
	result, err := mgr.ConfigCommand(username, contract.ConfigCommandRequest{
		Line: line, Path: path, Operator: cmd.Operator, Content: cmd.content,
	})
	if err != nil {
		answer.output = "error: " + err.Error()
		return answer, true
	}
	answer.output = result.Output
	if result.Feedback != "" {
		answer.output = result.Feedback
		switch cmd.Verb {
		case verbShow, verbCompare, verbOption, verbLoad:
			answer.output += "\n" + result.Output
		}
	}
	answer.input = result.Input
	answer.path = result.Path
	answer.navigated = result.Navigated
	answer.auditSuccess = result.Input == ""
	answer.outputComplete = result.Input == ""
	if result.Input != "" {
		answer.output = "Select a file or paste configuration to finish load " + result.Input + "."
	}
	return answer, true
}
