// Design: docs/guide/config-editor.md -- browser commands share the TUI producers.
// Related: model_commands_show.go, model_load.go, model_commands_option.go.

package cli

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/component/cli/contract"
)

// ExecuteConfigCommand runs the display, option and supplied-content commands
// through the TUI model. It starts no event loop or workers. The caller MUST
// hold the editor's session lock and authorize a load before calling it.
func (e *Editor) ExecuteConfigCommand(request contract.ConfigCommandRequest) (answer contract.ConfigCommandResult, err error) {
	answer.Path = slices.Clone(request.Path)
	words, err := ParseCommandWords(request.Line)
	if err != nil {
		return answer, err
	}
	tokens := words.Tokens()
	if len(tokens) == 0 {
		return answer, errors.New("the editor command is empty; run help")
	}
	model, err := NewModel(e, FilesystemAuthorityUnknown)
	if err != nil {
		return answer, err
	}
	defer func() { err = errors.Join(err, model.Close()) }()
	model.contextPath = request.Path
	model.topOrigin = request.Operator
	model.underTop = len(request.Operator) > 0
	model.showHints = request.ShowHints
	var result commandResult
	switch tokens[0] {
	case cmdShow, cmdOption, cmdInsert, cmdSave, cmdRollback, cmdDisconnect, cmdUp, cmdEdit, cmdTop, cmdSet, cmdDelete, cmdRename, cmdCopy, cmdDeactivate, cmdActivate:
		result, err = model.dispatchCommandWords(words)
	case cmdWho:
		result, err = model.cmdWho()
	case cmdErrors:
		result, err = model.cmdErrors(tokens[1:])
	case cmdHistory:
		result, err = model.cmdHistory()
	case cmdLoad:
		var source, location, action, path string
		source, location, action, path, err = ParseLoadArgs(tokens[1:])
		if err == nil {
			if request.Content == nil {
				answer.Input = source
				answer.ShowHints = model.showHints
				return answer, nil
			}
			if source == loadSourceTerminal {
				path = loadSourceTerminal
			}
			result, err = model.applyLoad(location, action, *request.Content, path)
		}
	default:
		err = fmt.Errorf("command %q is not an editor display or load command; run help", tokens[0])
	}
	if err != nil {
		return answer, err
	}
	model.applyResult(result)
	answer.Path = model.ContextPath()
	answer.Navigated = result.clearContext
	if result.newContext != nil {
		answer.Navigated = true
	}
	answer.Output = model.ViewportContent()
	answer.Feedback = result.statusMessage
	answer.ShowHints = model.showHints
	return answer, nil
}
