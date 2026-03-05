// Design: docs/architecture/config/yang-config-design.md — command mode completion
// Related: completer.go — edit mode YANG-driven completion
// Overview: model_mode.go — editor mode switching

package editor

import (
	"sort"
	"strings"
)

// CommandNode represents a node in the operational command tree.
// Callers build this from RPC registrations and pass it to the editor.
type CommandNode struct {
	Name        string
	Description string
	Children    map[string]*CommandNode
}

// CommandCompleter provides completions for operational commands.
type CommandCompleter struct {
	root *CommandNode
}

// NewCommandCompleter creates a completer from a command tree root.
func NewCommandCompleter(root *CommandNode) *CommandCompleter {
	if root == nil {
		return &CommandCompleter{root: &CommandNode{}}
	}
	return &CommandCompleter{root: root}
}

// Complete returns completions for the given input.
func (c *CommandCompleter) Complete(input string) []Completion {
	if c.root == nil || c.root.Children == nil {
		return nil
	}

	input = strings.TrimLeft(input, " ")
	words := strings.Fields(input)
	endsWithSpace := strings.HasSuffix(input, " ")

	// Navigate through completed words
	current := c.root
	var partial string

	for i, word := range words {
		if current.Children == nil {
			return nil
		}

		isLast := i == len(words)-1
		if isLast && !endsWithSpace {
			partial = word
			break
		}

		child, ok := current.Children[word]
		if !ok {
			return nil
		}
		current = child
	}

	if endsWithSpace || len(words) == 0 {
		partial = ""
	}

	return c.matchChildren(current, partial)
}

// GhostText returns the best single completion for inline display.
func (c *CommandCompleter) GhostText(input string) string {
	if input == "" || c.root == nil {
		return ""
	}

	if strings.HasSuffix(input, " ") {
		return ""
	}

	completions := c.Complete(input)
	if len(completions) == 0 {
		return ""
	}

	words := strings.Fields(input)
	if len(words) == 0 {
		return ""
	}
	lastWord := words[len(words)-1]

	var matches []Completion
	for _, comp := range completions {
		if strings.HasPrefix(comp.Text, lastWord) {
			matches = append(matches, comp)
		}
	}

	if len(matches) == 1 {
		return matches[0].Text[len(lastWord):]
	}

	if len(matches) > 1 {
		common := matches[0].Text
		for _, m := range matches[1:] {
			common = commonPrefix(common, m.Text)
		}
		if len(common) > len(lastWord) {
			return common[len(lastWord):]
		}
	}

	return ""
}

// matchChildren returns sorted completions for children matching prefix.
func (c *CommandCompleter) matchChildren(node *CommandNode, prefix string) []Completion {
	if node == nil || node.Children == nil {
		return nil
	}

	keys := make([]string, 0, len(node.Children))
	for k := range node.Children {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var completions []Completion
	for _, name := range keys {
		if prefix == "" || strings.HasPrefix(name, prefix) {
			child := node.Children[name]
			completions = append(completions, Completion{
				Text:        name,
				Description: child.Description,
				Type:        "command",
			})
		}
	}

	return completions
}
