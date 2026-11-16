// Design: docs/architecture/config/yang-config-design.md — command mode completion
// Related: completer.go — edit mode YANG-driven completion
// Related: completer_plugin.go — plugin SDK method completion
// Overview: model_mode.go — editor mode switching

package cli

import (
	"github.com/ze-software/ze/internal/component/command"
)

// commandNode is an alias for command.Node. Use command.Node directly in new code.
type commandNode = command.Node

// CommandCompleter delegates to command.TreeCompleter and converts
// command.Suggestion to the editor's Completion type at the boundary.
type CommandCompleter struct {
	inner *command.TreeCompleter
}

// NewCommandCompleter creates a completer from a command tree root.
func NewCommandCompleter(root *command.Node) *CommandCompleter {
	return &CommandCompleter{inner: command.NewTreeCompleter(root)}
}

// SetActiveBackends propagates per-component backend names to the tree completer.
func (c *CommandCompleter) SetActiveBackends(backends map[string]string) {
	c.inner.SetActiveBackends(backends)
}

// Complete returns completions for the given input.
func (c *CommandCompleter) Complete(input string) []Completion {
	suggestions := c.inner.Complete(input)
	return commandCompletions(suggestions)
}

// CompleteChecked keeps a live argument-query failure distinct from no values.
func (c *CommandCompleter) CompleteChecked(input string) ([]Completion, error) {
	suggestions, err := c.inner.CompleteChecked(input)
	if err != nil {
		return nil, err
	}
	return commandCompletions(suggestions), nil
}

func commandCompletions(suggestions []command.Suggestion) []Completion {
	completions := make([]Completion, len(suggestions))
	for i, s := range suggestions {
		completions[i] = Completion{Text: s.Text, ShortHelp: s.Description, Type: s.Type}
	}
	return completions
}

// CommandGhostText derives a suffix from this update's candidates, reusing the
// command producer's quoting and prefix rules without issuing another query.
func CommandGhostText(input string, completions []Completion) string {
	suggestions := make([]command.Suggestion, len(completions))
	for i, candidate := range completions {
		suggestions[i] = command.Suggestion{Text: candidate.Text, Type: candidate.Type}
	}
	return command.GhostTextFrom(input, suggestions)
}

// GhostText returns the best single completion for inline display.
func (c *CommandCompleter) GhostText(input string) string {
	return c.inner.GhostText(input)
}

// Explain returns the long explanation the named command declares, and false
// when the input names no command or the command declares none.
func (c *CommandCompleter) Explain(input string) (string, bool) {
	return c.inner.Explain(input)
}
