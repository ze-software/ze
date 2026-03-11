// Design: docs/architecture/api/commands.md — command tree types
// Related: completer.go — command completion using tree

// Package command provides shared types and logic for operational command execution.
// Both the standalone CLI (cmd/ze/cli) and the unified CLI model's command mode
// (internal/component/cli) use this package for command trees,
// completion, and pipe operators.
package command

// Node represents a node in the operational command tree.
// Used for completion and command validation across CLI and editor command mode.
type Node struct {
	Name        string
	Description string
	Children    map[string]*Node
}

// RPCInfo holds the fields needed to build a command tree from RPC registrations.
// Callers convert their domain-specific RPC types to this before calling BuildTree.
type RPCInfo struct {
	CLICommand string
	Help       string
	ReadOnly   bool
}

// BuildTree creates a command tree from RPC registrations.
// Strips the "bgp " prefix for BGP commands so the user types "peer list" not "bgp peer list".
// If readOnly is true, only RPCs marked ReadOnly are included.
func BuildTree(rpcs []RPCInfo, readOnly bool) *Node {
	root := &Node{Children: make(map[string]*Node)}

	for _, rpc := range rpcs {
		if readOnly && !rpc.ReadOnly {
			continue
		}

		cmd := rpc.CLICommand
		// Strip "bgp " prefix for BGP commands (user types "peer list", not "bgp peer list")
		if len(cmd) > 4 && cmd[:4] == "bgp " {
			cmd = cmd[4:]
		}

		parts := splitFields(cmd)
		if len(parts) == 0 {
			continue
		}

		current := root
		for _, part := range parts {
			if current.Children == nil {
				current.Children = make(map[string]*Node)
			}
			child, ok := current.Children[part]
			if !ok {
				child = &Node{Name: part}
				current.Children[part] = child
			}
			current = child
		}
		current.Description = rpc.Help
	}

	return root
}

// splitFields splits a string by whitespace, like strings.Fields but avoids the import.
func splitFields(s string) []string {
	var fields []string
	start := -1
	for i, c := range s {
		if c == ' ' || c == '\t' {
			if start >= 0 {
				fields = append(fields, s[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		fields = append(fields, s[start:])
	}
	return fields
}
