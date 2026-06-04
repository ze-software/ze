// Design: docs/architecture/api/commands.md — command tree types
// Related: completer.go — command completion using tree
// Related: help.go — dynamic help generation

// Package command provides shared types and logic for operational command execution.
// Both the standalone CLI (cmd/ze/cli) and the unified CLI model's command mode
// (internal/component/cli) use this package for command trees,
// completion, and pipe operators.
package command

import "regexp"

// ArgKind identifies the type of a command argument from YANG leaf metadata.
type ArgKind uint8

const (
	ArgString ArgKind = iota
	ArgEnum
	ArgUint
	ArgUnion
)

// UintRange represents a contiguous range of unsigned integer values.
type UintRange struct {
	Min uint64
	Max uint64
}

// ArgDef declares a typed argument for an operational command, extracted from
// YANG leaves inside ze:command containers. Drives completion, validation,
// and documentation from a single source.
type ArgDef struct {
	Name       string         // YANG leaf name (kebab-case, used as keyword detector)
	Kind       ArgKind        // Argument type category
	EnumValues []string       // Valid enum values (for ArgEnum and ArgUnion)
	UintBits   int            // 8, 16, 32, or 64 for ArgUint
	Ranges     []UintRange    // Valid ranges for ArgUint (disjoint segments supported)
	Pattern    *regexp.Regexp // Compiled XSD pattern for ArgString (nil = accept any)
	UnionDefs  []ArgDef       // Member types for ArgUnion (tried in order)
	Mandatory  bool           // True if YANG leaf has mandatory true
}

// Node represents a node in the operational command tree.
// Used for completion and command validation across CLI and editor command mode.
type Node struct {
	Name         string
	Description  string
	WireMethod   string   // Handler dispatch key (from ze:command argument). Empty for grouping nodes.
	TaskSupport  string   // MCP task-support level (from ze:task-support). Empty = optional.
	Backend      []string // Allowed backends (from ze:backend). Nil = unrestricted.
	EnsureExists string   // Rollback WireMethod from ze:ensure-exists. Empty = not a checkpoint.
	Children     map[string]*Node
	ArgDefs      []ArgDef // Typed argument definitions from YANG leaves inside ze:command.

	// DynamicChildren returns additional completion suggestions at this node.
	// Called alongside static Children when completing. Used for runtime data
	// like peer names/IPs that aren't known at tree build time.
	DynamicChildren func() []Suggestion

	// ValueHints returns terminal argument value suggestions at this node.
	// Unlike DynamicChildren (navigation targets), value hints complete an
	// argument and do not lead to further subcommands. Examples: address
	// families ("ipv4/unicast"), log levels ("debug", "warn").
	ValueHints func() []Suggestion
}

// RPCInfo holds the fields needed to build a command tree from RPC registrations.
// Callers convert their domain-specific RPC types to this before calling BuildTree.
type RPCInfo struct {
	CLICommand string
	ReadOnly   bool
}

// BuildTree creates a command tree from RPC registrations.
// If readOnly is true, only RPCs marked ReadOnly are included.
func BuildTree(rpcs []RPCInfo, readOnly bool) *Node {
	root := &Node{Children: make(map[string]*Node)}

	for _, rpc := range rpcs {
		if readOnly && !rpc.ReadOnly {
			continue
		}

		parts := splitFields(rpc.CLICommand)
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
