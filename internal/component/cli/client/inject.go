// Design: docs/architecture/cli/command-completion.md -- plugin command completion injection

package client

import (
	"strings"

	cmd "github.com/ze-software/ze/internal/component/command"
)

// commandEntry matches the anonymous struct used in buildRuntimeTree and
// buildRuntimeTreeFromDispatch to parse the system command list response.
type commandEntry struct {
	Value  string `json:"value"`
	Help   string `json:"help"`
	Hidden bool   `json:"hidden"`
}

// injectPluginCommands adds plugin-registered commands to the completion tree.
// Commands that already exist in the tree (from YANG proxy RPCs) are skipped.
// Hidden commands are skipped. This ensures plugin commands that have no YANG
// backing still appear in tab-completion.
func injectPluginCommands(tree *cmd.Node, commands []commandEntry, hidden map[string]bool) {
	if tree == nil {
		return
	}
	for _, c := range commands {
		key := strings.ToLower(c.Value)
		if hidden[key] {
			continue
		}
		parts := strings.Fields(c.Value)
		if len(parts) == 0 {
			continue
		}

		// Walk the tree to check if this command path already exists.
		// If every token exists, the command is already in the tree from
		// YANG proxy RPCs and we skip it.
		current := tree
		exists := true
		for _, part := range parts {
			if current.Children == nil {
				exists = false
				break
			}
			child, ok := current.Children[part]
			if !ok {
				exists = false
				break
			}
			current = child
		}
		if exists {
			continue
		}

		// Add missing path nodes to the tree.
		current = tree
		for _, part := range parts {
			if current.Children == nil {
				current.Children = make(map[string]*cmd.Node)
			}
			child, ok := current.Children[part]
			if !ok {
				child = &cmd.Node{Name: part}
				current.Children[part] = child
			}
			current = child
		}
		// Set description on the leaf node if not already set.
		if current.Description == "" && c.Help != "" {
			current.Description = c.Help
		}
	}
}
