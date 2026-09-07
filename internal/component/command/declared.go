// Design: docs/architecture/api/commands.md — what a command declares about its answer
// Related: answer_shape.go — the shape and address-field registries this reads first
// Related: column_order.go — the column-order registry this reads first
//
// declared.go joins the two channels a command declaration travels, for a
// reader that has to answer for a plugin's commands as well as for the ones
// its own process compiled in.

package command

import (
	"slices"

	pluginregistry "github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Declared is what one command states about its answer: whether the answer has
// rows, the order a person reads its columns in, and which of its fields hold
// an IP address.
//
// ShapeDeclared sits beside Shape because ShapeDoc is the zero AnswerShape and
// is also a legitimate declaration. Without the flag a reader cannot tell "this
// command declares one document" from "this command declares nothing", and the
// two publish different operator lists (ai/rules/principles.md).
type Declared struct {
	// Shape is what the answer holds. It is meaningful only when
	// ShapeDeclared is true.
	Shape AnswerShape
	// ShapeDeclared reports whether any package or plugin declared a shape.
	ShapeDeclared bool
	// Columns are the declared column orders, one per record shape the command
	// renders. Empty declares no order, and every column then renders
	// alphabetically.
	Columns []ColumnOrder
	// AddressFields are the answer's JSON keys whose value holds an IP address
	// or a prefix. Empty declares none, which refuses the address operators.
	AddressFields []string
	// Aliases are the pipe chains the command answers to by name, sorted by
	// name: the global ones, the ones an in-tree package registered on this
	// path, and the ones a registered plugin puts on it.
	Aliases []Alias
}

// DeclaredForCommand answers what a command states about its answer, reading
// both channels a declaration travels.
//
// The declaration registries answer first. Every in-tree package writes them
// from init(), and a RUNNING daemon has also written each plugin's Stage 1
// message into them (RegisterPluginShapes), so inside a daemon they hold the
// whole answer and this function adds nothing.
//
// A field no package in THIS process declared is answered by the registration
// of the plugin that provides the command. A catalog generator links the
// composition root and starts no engine, so a plugin's Stage 1 message never
// happens there and Registration.Commands is the only place its declaration
// exists. Both channels carry the same commandDecls() slice, so they cannot
// disagree, and `./le plugin declarations check` refuses a plugin that lets
// them.
//
// The fallback is per field rather than all-or-nothing, because the two
// channels declare at different granularities: an in-tree command package
// declares a shape for a path whose plugin declares the column order beside it.
//
// Resolution is by the longest declared command path that is a prefix of the
// command, in both channels. That is what the registries do for the first one,
// so a plugin declaration answers here exactly as it would inside a daemon.
func DeclaredForCommand(name string) Declared {
	shape, shapeDeclared := ShapeForCommand(name)
	declared := Declared{
		Shape:         shape,
		ShapeDeclared: shapeDeclared,
		Columns:       ColumnsForCommand(name),
		AddressFields: AddressFieldsForCommand(name),
		Aliases:       aliasesIncludingPlugins(name),
	}

	decl, found := declaredByPlugin(name)
	if !found {
		return declared
	}

	if !declared.ShapeDeclared {
		if parsed, ok := ParseAnswerShape(decl.Shape); ok {
			declared.Shape = parsed
			declared.ShapeDeclared = true
		}
	}
	if len(declared.Columns) == 0 {
		if order := normalizedNames(decl.Columns); len(order) > 0 {
			declared.Columns = []ColumnOrder{order}
		}
	}
	if len(declared.AddressFields) == 0 {
		if fields := normalizedNames(decl.AddressFields); len(fields) > 0 {
			declared.AddressFields = fields
		}
	}
	return declared
}

// aliasesIncludingPlugins answers the aliases a command answers to, from the
// alias registry and then from the registration of the plugin that puts one on
// the command.
//
// AliasesForCommand is left alone deliberately. It is what a RUNNING daemon
// reads, and a daemon has already written each started plugin's Stage 1
// aliases into the registry, so adding a registration's aliases there would
// offer an operator a name no running command answers to.
//
// An alias the registry already holds on the resolved path WINS. A plugin whose
// name collides with one is refused at Stage 1 (RegisterPluginAliases, through
// aliasOnPath), so keeping the registry's is the answer a daemon reaches.
func aliasesIncludingPlugins(name string) []Alias {
	registered := AliasesForCommand(name)
	declared := aliasesByPlugin(name)
	if len(declared) == 0 {
		return registered
	}

	byName := make(map[string]Alias, len(registered)+len(declared))
	for _, alias := range declared {
		byName[alias.Name] = alias
	}
	// Second, so a name the registry holds replaces the plugin's.
	for _, alias := range registered {
		byName[alias.Name] = alias
	}

	names := make([]string, 0, len(byName))
	for aliasName := range byName {
		names = append(names, aliasName)
	}
	slices.Sort(names)

	aliases := make([]Alias, 0, len(names))
	for _, aliasName := range names {
		aliases = append(aliases, byName[aliasName])
	}
	return aliases
}

// aliasesByPlugin answers the aliases a registered plugin puts on this command,
// resolved the way the alias registry resolves one.
//
// The longest declared alias path that is a prefix of the command wins, and a
// command the SAME plugin declares below that path answers none. That second
// rule is the read-side twin of aliasBarriers (alias.go): the daemon writes an
// empty declaration on such a path to stop the inheritance, and a reader that
// skipped the rule would report `show bgp rpki roa` answering to a name that
// sits on `show bgp rpki`.
//
// The walk is bounded by the registered plugin count and each plugin's
// declaration lists, both fixed once init() has run.
func aliasesByPlugin(name string) []Alias {
	name = normalizeCommand(name)

	var aliases []Alias
	bestLen := -1
	for _, registration := range pluginregistry.All() {
		for index := range registration.Pipes {
			decl := &registration.Pipes[index]
			path := normalizeCommand(decl.Command)
			if path == "" || decl.Name == "" || !commandMatchesPrefix(name, path) {
				continue
			}
			if path != name && declaresCommand(registration.Commands, name) {
				continue
			}
			if len(path) < bestLen {
				continue
			}
			if len(path) > bestLen {
				aliases = aliases[:0]
				bestLen = len(path)
			}
			aliases = append(aliases, Alias{
				Name:        decl.Name,
				Description: decl.Description,
				Expansion:   decl.Expansion,
			})
		}
	}
	return aliases
}

// declaresCommand reports whether the plugin names this exact command path.
func declaresCommand(commands []rpc.CommandDecl, name string) bool {
	for index := range commands {
		if normalizeCommand(commands[index].Name) == name {
			return true
		}
	}
	return false
}

// ColumnNames answers column orders as plain string lists, which is the form
// every JSON answer publishes them in.
//
// The answer stays a list of lists rather than one flat list, because a command
// that renders two record shapes declares two orders and a flat list would
// state neither (ColumnOrder). It answers nil for no order, so a `,omitempty`
// key leaves the answer rather than publishing an empty list.
func ColumnNames(orders []ColumnOrder) [][]string {
	if len(orders) == 0 {
		return nil
	}
	names := make([][]string, 0, len(orders))
	for _, order := range orders {
		names = append(names, []string(order))
	}
	return names
}

// declaredByPlugin answers the declaration a registered plugin makes for the
// longest declared command path that is a prefix of name.
//
// The walk is bounded by the registered plugin count and by each plugin's
// declaration list, both of which are fixed once init() has run. Nothing on a
// wire path reads this: the callers are the command catalog generators and
// `ze help command`.
func declaredByPlugin(name string) (rpc.CommandDecl, bool) {
	name = normalizeCommand(name)

	var best rpc.CommandDecl
	bestLen := -1
	found := false
	for _, registration := range pluginregistry.All() {
		for index := range registration.Commands {
			decl := &registration.Commands[index]
			path := normalizeCommand(decl.Name)
			if !commandMatchesPrefix(name, path) {
				continue
			}
			if len(path) <= bestLen {
				continue
			}
			best = *decl
			bestLen = len(path)
			found = true
		}
	}
	return best, found
}
