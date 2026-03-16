// Design: docs/architecture/api/commands.md — command completion
// Overview: node.go — command tree types

package command

import (
	"sort"
	"strings"
)

// Suggestion represents a single completion suggestion from the command completer.
// The editor maps these to its own Completion type for display.
type Suggestion struct {
	Text        string
	Description string
	Type        string // "command" or "pipe"
}

// TreeCompleter provides completions for operational commands from a Node tree.
// Used by both the CLI and the editor's command mode.
type TreeCompleter struct {
	root *Node
}

// NewTreeCompleter creates a completer from a command tree root.
func NewTreeCompleter(root *Node) *TreeCompleter {
	if root == nil {
		return &TreeCompleter{root: &Node{}}
	}
	return &TreeCompleter{root: root}
}

// PipeOperators lists the available pipe operators for completion.
var PipeOperators = []Suggestion{
	{Text: "match", Description: "Filter lines matching pattern", Type: "pipe"},
	{Text: "count", Description: "Count output items", Type: "pipe"},
	{Text: "table", Description: "Render as table", Type: "pipe"},
	{Text: "text", Description: "Space-aligned columns", Type: "pipe"},
	{Text: "json", Description: "JSON output", Type: "pipe"},
	{Text: "yaml", Description: "YAML output", Type: "pipe"},
	{Text: "no-more", Description: "Disable paging", Type: "pipe"},
}

// pipeSubArgs maps pipe operators to their sub-argument completions.
var pipeSubArgs = map[string][]Suggestion{
	"json": {
		{Text: "compact", Description: "Single-line JSON", Type: "pipe"},
		{Text: "pretty", Description: "Indented JSON (default)", Type: "pipe"},
	},
}

// CompletePipe returns pipe operator completions matching the partial input.
// When a pipe operator is fully matched (e.g., "json "), returns sub-argument
// completions instead of repeating the operator.
func CompletePipe(partial string) []Suggestion {
	trimmed := strings.TrimSpace(partial)

	// Check if the first word is a fully matched operator.
	// "json " → sub-args, "json c" → filtered sub-args, "json" → operator match.
	spaceIdx := strings.IndexByte(trimmed, ' ')
	if spaceIdx > 0 {
		opName := trimmed[:spaceIdx]
		subPartial := strings.TrimSpace(trimmed[spaceIdx+1:])
		if subs, ok := pipeSubArgs[opName]; ok {
			var completions []Suggestion
			for _, s := range subs {
				if subPartial == "" || strings.HasPrefix(s.Text, subPartial) {
					completions = append(completions, s)
				}
			}
			return completions
		}
		return nil // operator has no sub-args
	}

	// Exact match with trailing space (partial not trimmed has space).
	if trimmed != "" && strings.HasSuffix(partial, " ") {
		if subs, ok := pipeSubArgs[trimmed]; ok {
			return subs
		}
		return nil // fully typed operator with no sub-args
	}

	// Prefix matching against operators.
	var completions []Suggestion
	for _, op := range PipeOperators {
		if trimmed == "" || strings.HasPrefix(op.Text, trimmed) {
			completions = append(completions, op)
		}
	}
	return completions
}

// Complete returns completions for the given input.
func (c *TreeCompleter) Complete(input string) []Suggestion {
	// After a pipe character, complete pipe operators.
	if pipeIdx := strings.LastIndex(input, "|"); pipeIdx >= 0 {
		return CompletePipe(input[pipeIdx+1:])
	}

	if c.root == nil || c.root.Children == nil {
		return nil
	}

	input = strings.TrimLeft(input, " ")
	words := strings.Fields(input)
	endsWithSpace := strings.HasSuffix(input, " ")

	// Navigate through completed words.
	current := c.root
	var partial string

	for i, word := range words {
		isLast := i == len(words)-1
		if isLast && !endsWithSpace {
			partial = word
			break
		}

		if current.Children == nil {
			return nil
		}

		child, ok := current.Children[word]
		if !ok {
			// Word is not a static child. If parent has DynamicChildren,
			// the word might be a dynamic selector (e.g., peer name).
			// Skip it and continue showing the same node's children.
			if current.DynamicChildren != nil {
				continue
			}
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
func (c *TreeCompleter) GhostText(input string) string {
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

	// For pipe completions, extract the last word after the pipe.
	// When pipe has sub-args (e.g., "| json c"), lastWord should be "c".
	var lastWord string
	if pipeIdx := strings.LastIndex(input, "|"); pipeIdx >= 0 {
		fields := strings.Fields(strings.TrimSpace(input[pipeIdx+1:]))
		if len(fields) > 0 {
			lastWord = fields[len(fields)-1]
		}
	} else {
		words := strings.Fields(input)
		if len(words) == 0 {
			return ""
		}
		lastWord = words[len(words)-1]
	}

	if lastWord == "" {
		return ""
	}

	var matches []Suggestion
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
// Includes both static children and dynamic suggestions from DynamicChildren callback.
func (c *TreeCompleter) matchChildren(node *Node, prefix string) []Suggestion {
	if node == nil {
		return nil
	}

	var completions []Suggestion

	// Static children from tree.
	if node.Children != nil {
		keys := make([]string, 0, len(node.Children))
		for k := range node.Children {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, name := range keys {
			if prefix == "" || strings.HasPrefix(name, prefix) {
				child := node.Children[name]
				completions = append(completions, Suggestion{
					Text:        name,
					Description: child.Description,
					Type:        "command",
				})
			}
		}
	}

	// Dynamic children (e.g., peer names/IPs).
	if node.DynamicChildren != nil {
		for _, s := range node.DynamicChildren() {
			if prefix == "" || strings.HasPrefix(s.Text, prefix) {
				completions = append(completions, s)
			}
		}
	}

	return completions
}

// commonPrefix returns the longest common prefix of two strings.
func commonPrefix(a, b string) string {
	minLen := min(len(b), len(a))
	for i := range minLen {
		if a[i] != b[i] {
			return a[:i]
		}
	}
	return a[:minLen]
}
