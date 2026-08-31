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

	// Anchor names the path keyword this value follows, and it is set when the
	// leaf is declared by a container ABOVE the command rather than by the
	// command itself: `request interface <name> down` declares `name` on
	// `interface`, so the anchor is `interface`.
	//
	// It is empty for a leaf the command declares. Such a leaf follows the
	// container whose name it repeats, and trails the last keyword when it
	// repeats none, which is the rule the renderer already applied.
	//
	// Nothing binds a value by anchor: a positional token still goes to the
	// definition whose type constrains it most (internal/component/plugin/server,
	// positionalDef). The anchor decides where a value is PRINTED and nothing
	// else.
	Anchor string
}

// ArgInherit says whether a command takes the values the containers ABOVE it
// declare. The zero value takes them, because that is what every command under
// a container that names an object does: `request interface <name> up` and
// `request interface <name> down` both act on the interface `interface` names.
type ArgInherit uint8

const (
	// ArgInheritAncestors is the zero value: every leaf a non-command container
	// on this command's path declares is part of this command's grammar.
	ArgInheritAncestors ArgInherit = iota
	// ArgInheritNone says this command acts on none of the objects its
	// containers name, so their leaves are not its arguments. `show bgp peer
	// list` reads the whole peer set and `request interface migrate` names two
	// interfaces of its own.
	ArgInheritNone
)

// argInheritNoneWord is the word a module writes to take none of the values
// its containers declare. It is named because it is the only one of the two a
// module ever writes, so it is the vocabulary this package publishes.
const argInheritNoneWord = "none"

// argInheritNames names each mode for a reader and for the YANG argument that
// selects it. ParseArgInherit reads this one table, so a module and this
// package cannot disagree about what a mode is called.
var argInheritNames = [...]string{
	ArgInheritAncestors: "ancestors",
	ArgInheritNone:      argInheritNoneWord,
}

// ParseArgInherit answers the mode a ze:inherit argument names, and false for a
// word the table does not hold. A word nobody declared must not fall back to a
// mode that silently drops a command's arguments (ai/rules/evidence.md).
func ParseArgInherit(argument string) (ArgInherit, bool) {
	for i, name := range argInheritNames {
		if name == argument {
			// i is a range index over argInheritNames, so the conversion cannot
			// reach a mode the table does not hold.
			return ArgInherit(i), true //nolint:gosec // i indexes argInheritNames itself
		}
	}
	return ArgInheritAncestors, false
}

// String names the mode. A value outside the declared set names itself as the
// inheriting one rather than as a number.
func (a ArgInherit) String() string {
	if int(a) < len(argInheritNames) {
		return argInheritNames[a]
	}
	return argInheritNames[ArgInheritAncestors]
}

// Node represents a node in the operational command tree.
// Used for completion and command validation across CLI and editor command mode.
type Node struct {
	Name string
	// Description is the one-line summary of this node, from the YANG
	// description statement. Every surface that shows a command on one line
	// reads it: a list row, a completion candidate, a table cell.
	Description string
	// Help is the long explanation of this node, from ze:help. Only the help
	// page for this one command reads it, and it holds the newlines its author
	// wrote. Empty means nobody has written an explanation for this command,
	// which is not a defect: the help page then prints the summary alone.
	Help         string
	WireMethod   string   // Handler dispatch key (from ze:command argument). Empty for grouping nodes.
	TaskSupport  string   // MCP task-support level (from ze:task-support). Empty = optional.
	Backend      []string // Allowed backends (from ze:backend). Nil = unrestricted.
	EnsureExists string   // Rollback WireMethod from ze:ensure-exists. Empty = not a checkpoint.
	Children     map[string]*Node
	ArgDefs      []ArgDef // Typed argument definitions from YANG leaves inside ze:command.

	// Modifier says this node is an optional trailing GROUP of its parent's
	// command rather than a command of its own: `announce ... [tag <key>
	// <value>]`. It is set from ze:modifier and is ModifierNone everywhere else.
	//
	// A group earns the extension only when it carries more than one value or
	// repeats. One optional value is one optional leaf, which already renders
	// `[name <name>]`.
	Modifier Modifier
	// ModifierOrder is the position the module declares this group in, counting
	// from 1 among its siblings. Children is a map, so the declaration order is
	// carried here or it is lost.
	ModifierOrder int

	// Inherit says whether the leaves the containers above this command declare
	// are part of its grammar. It is set from ze:inherit and is
	// ArgInheritAncestors everywhere else.
	Inherit ArgInherit

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

// CommandEntry is a minimal command descriptor for injecting plugin-registered
// commands into the operational command tree for tab-completion. Plugin commands
// dispatch through the plugin registry (not a YANG WireMethod); these entries
// exist only so the commands surface in completion and help.
type CommandEntry struct {
	Name        string // full command path, space-separated (e.g. "show bgp irr")
	Description string // the one-line summary shown alongside the completion
	Help        string // the long explanation the command's own help page prints
}

// MergeCommandPaths inserts each entry's command path into the tree as
// completion-only nodes, creating any missing intermediate and leaf nodes.
//
// It is NON-DESTRUCTIVE: an existing node (a YANG-backed command or grouping
// node) is never modified. Its WireMethod, ArgDefs, and children are left
// intact. Each of the entry's two help fields is applied ONLY to a leaf node
// this call creates, or to one that holds nothing in THAT field. So a plugin
// command can never overwrite a builtin's metadata, and dispatch precedence
// (builtins win over plugin commands) holds at the completion layer too.
//
// The two fields are decided separately because they are declared separately.
// A plugin that states a summary and no explanation fills the summary alone.
// A builtin that already states a summary still takes the plugin's
// explanation when it has none of its own.
//
// Entries whose Name is empty after whitespace splitting are skipped, and a nil
// root is a no-op. Called after the YANG-derived tree is built to surface
// plugin-registered commands (which are otherwise absent from tab-completion).
func MergeCommandPaths(root *Node, entries []CommandEntry) {
	if root == nil {
		return
	}
	for _, e := range entries {
		parts := splitFields(e.Name)
		if len(parts) == 0 {
			continue
		}
		current := root
		for i, part := range parts {
			if current.Children == nil {
				current.Children = make(map[string]*Node)
			}
			child, ok := current.Children[part]
			if !ok {
				child = &Node{Name: part}
				current.Children[part] = child
			}
			if i == len(parts)-1 {
				if child.Description == "" {
					child.Description = e.Description
				}
				if child.Help == "" {
					child.Help = e.Help
				}
			}
			current = child
		}
	}
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
