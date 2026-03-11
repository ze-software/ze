// Design: docs/architecture/config/yang-config-design.md — command mode completion
// Related: completer.go — edit mode YANG-driven completion
// Overview: model_mode.go — editor mode switching

package cli

import (
	"codeberg.org/thomas-mangin/ze/internal/component/command"
)

// CommandNode is an alias for command.Node. Use command.Node directly in new code.
type CommandNode = command.Node

// CommandCompleter delegates to command.TreeCompleter and converts
// command.Suggestion to the editor's Completion type at the boundary.
type CommandCompleter struct {
	inner *command.TreeCompleter
}

// NewCommandCompleter creates a completer from a command tree root.
func NewCommandCompleter(root *command.Node) *CommandCompleter {
	return &CommandCompleter{inner: command.NewTreeCompleter(root)}
}

// Complete returns completions for the given input.
func (c *CommandCompleter) Complete(input string) []Completion {
	suggestions := c.inner.Complete(input)
	completions := make([]Completion, len(suggestions))
	for i, s := range suggestions {
		completions[i] = Completion{Text: s.Text, Description: s.Description, Type: s.Type}
	}
	return completions
}

// GhostText returns the best single completion for inline display.
func (c *CommandCompleter) GhostText(input string) string {
	return c.inner.GhostText(input)
}
