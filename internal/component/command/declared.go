// Design: docs/architecture/api/commands.md — what a command declares about its answer
// Related: answer_shape.go — the shape and address-field registries this reads first
// Related: column_order.go — the column-order registry this reads first
//
// declared.go joins the two channels a command declaration travels, for a
// reader that has to answer for a plugin's commands as well as for the ones
// its own process compiled in.

package command

import (
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
// Inside a DAEMON there is only one channel. registerPluginShapes
// (internal/component/plugin/server/startup.go) writes each Stage 1 declaration
// into the three registries below, at the plugin's own command path, and every
// later read resolves by the longest declared path that is a prefix of the
// command. This function answers what those registries would hold, for a reader
// that starts no engine and therefore never receives a Stage 1 message:
// Registration.Commands is the only place a plugin's declaration exists there.
// Both channels carry the same commandDecls() slice, so they cannot disagree,
// and `./le plugin declarations check` refuses a plugin that lets them.
//
// The two channels are weighed by PATH LENGTH together rather than one after
// the other. A reader that asked the registries first and the plugin second
// would give an ancestor's registry declaration to a command that declares its
// own: `show bgp rib` declares eleven route columns in the tree, and
// `show bgp rib help` answers subcommands, so the second published the first's
// columns and its address fields until this resolution replaced that one.
//
// A plugin declaration that names no column and no address field is a BARRIER
// and not an absence, which is the same rule RegisterPluginShapes writes into
// the registries: a path that declares nothing inherits its nearest declared
// ancestor, so a command whose answer has no columns has to say so.
//
// Resolution stays per registry, because the three do not declare at one
// granularity: an in-tree command package declares a shape for a path whose
// plugin declares the column order beside it.
func DeclaredForCommand(name string) Declared {
	shapes, shapePath, shapeFound := shapeRegistry.lookupPath(name)
	orders, columnPath, columnFound := columnRegistry.lookupPath(name)
	fields, addressPath, addressFound := addressFieldRegistry.lookupPath(name)

	declared := Declared{
		Shape:         ShapeDoc,
		ShapeDeclared: shapeFound && len(shapes) > 0,
		Aliases:       aliasesIncludingPlugins(name),
	}
	if declared.ShapeDeclared {
		declared.Shape = shapes[0]
	}
	if columnFound {
		declared.Columns = orders
	}
	if addressFound {
		declared.AddressFields = fields
	}

	decl, declPath, found := declaredByPlugin(name)
	if !found {
		return declared
	}

	// The shape is never empty on this channel: registerPluginShapes writes
	// nothing for a declaration that states none, and declaredByPlugin passes
	// over the same ones for the same reason.
	declaresShape := declarationSite{path: declPath, declared: true}
	if declaresShape.winsOver(declarationSite{path: shapePath, declared: shapeFound, empty: len(shapes) == 0}) {
		declared.Shape, declared.ShapeDeclared = ParseAnswerShape(decl.Shape)
	}

	order := normalizedNames(decl.Columns)
	declaresColumns := declarationSite{path: declPath, declared: true, empty: len(order) == 0}
	if declaresColumns.winsOver(declarationSite{path: columnPath, declared: columnFound, empty: len(orders) == 0}) {
		declared.Columns = nil
		if len(order) > 0 {
			declared.Columns = []ColumnOrder{order}
		}
	}

	addresses := normalizedNames(decl.AddressFields)
	declaresAddresses := declarationSite{path: declPath, declared: true, empty: len(addresses) == 0}
	if declaresAddresses.winsOver(declarationSite{path: addressPath, declared: addressFound, empty: len(fields) == 0}) {
		declared.AddressFields = addresses
	}
	return declared
}

// declarationSite is where one channel's answer for a command comes from: the
// declared path it resolved to, whether that path declares anything at all, and
// whether what it declares is a barrier rather than a value.
type declarationSite struct {
	path     string
	declared bool
	empty    bool
}

// winsOver reports whether this site's declaration is the one a daemon's
// registry would hold for the command, against the other channel's site.
//
// The longer path wins, because that is how the registries resolve a command
// once a plugin's declaration has been written into them. At the SAME path the
// registry's declaration wins unless it is a barrier, which is declareFor's
// rule (column_order.go): a declaration of nothing is a floor that stops
// inheritance and states nothing, so a value replaces one and never the
// reverse. A conflict between two values at one path is refused there, and the
// registry's is what the daemon keeps.
func (s declarationSite) winsOver(other declarationSite) bool {
	if !s.declared {
		return false
	}
	if !other.declared {
		return true
	}
	if len(s.path) != len(other.path) {
		return len(s.path) > len(other.path)
	}
	return other.empty && !s.empty
}

// aliasesIncludingPlugins answers the aliases a command answers to, resolved
// the way a RUNNING daemon resolves them: over the alias registry this process
// holds, and over the registrations of the plugins it compiled in.
//
// AliasesForCommand is left alone deliberately. It is what a running daemon
// reads, and a daemon has already written each started plugin's Stage 1
// aliases into the registry, so adding a registration's aliases there would
// offer an operator a name no running command answers to.
//
// The two channels are weighed by PATH LENGTH, which is how DeclaredForCommand
// weighs the other three. A daemon writes a plugin's aliases into the registry
// at the plugin's own path, and lookupAlias then reads the LONGEST registered
// path that is a prefix of the command and never falls back to a shorter one.
// A reader that added the plugin's set to whatever the registry answered would
// publish an ancestor's aliases beside a plugin's own, which no daemon does.
//
// At ONE path the two sets merge, which is what mergedAliases (alias.go) does
// when a plugin registers on a path the tree already holds. An alias the
// registry holds there WINS: Stage 1 refuses that collision
// (RegisterPluginAliases, through aliasOnPath), so keeping the registry's is
// the answer a daemon reaches.
//
// The global aliases sit under both, as they sit under every registered set.
func aliasesIncludingPlugins(name string) []Alias {
	name = normalizeCommand(name)

	registered, registryPath, registryFound := aliasRegistry.lookupPath(name)
	declared, pluginPath, pluginFound := aliasesByPlugin(name)

	bestLen := 0
	if registryFound {
		bestLen = len(registryPath)
	}
	if pluginFound && len(pluginPath) > bestLen {
		bestLen = len(pluginPath)
	}

	byName := globalAliasesByName()
	// The plugin's set goes first, so a name the registry holds replaces it.
	if pluginFound && len(pluginPath) == bestLen {
		for _, alias := range declared {
			byName[alias.Name] = alias
		}
	}
	if registryFound && len(registryPath) == bestLen {
		addAliases(byName, registered)
	}
	return sortedAliases(byName)
}

// aliasesByPlugin answers the aliases a registered plugin puts on this command,
// the path they are declared on, and whether any plugin declares for it at all.
//
// The longest declared alias path that is a prefix of the command wins, and
// several plugins declaring on ONE path merge, which is how the daemon's
// registry holds them.
//
// A command the SAME plugin declares BELOW its alias path answers no alias, and
// the path reported for it is then the command itself. That is the read side of
// aliasBarriers (alias.go): the daemon writes an EMPTY declaration on such a
// command, and no path is longer than the command itself, so nothing above it
// is answered -- not the plugin's alias, and not an in-tree ancestor's either.
//
// aliasBarriers writes no barrier on a path that ALREADY carries a
// declaration, and this reader needs no such condition. That declaration sits
// on the command itself, so it wins on length either way; the daemon's
// condition exists to stop the barrier ERASING it, which a reader cannot do.
//
// The walk is bounded by the registered plugin count and each plugin's
// declaration lists, both fixed once init() has run.
func aliasesByPlugin(name string) ([]Alias, string, bool) {
	name = normalizeCommand(name)

	var aliases []Alias
	bestPath := ""
	bestLen := -1
	for _, registration := range pluginregistry.All() {
		barrier := false
		for index := range registration.Pipes {
			decl := &registration.Pipes[index]
			path := normalizeCommand(decl.Command)
			if path == "" || decl.Name == "" || !commandMatchesPrefix(name, path) {
				continue
			}
			if path != name && declaresCommand(registration.Commands, name) {
				barrier = true
				continue
			}
			if len(path) < bestLen {
				continue
			}
			if len(path) > bestLen {
				aliases = aliases[:0]
				bestPath = path
				bestLen = len(path)
			}
			aliases = append(aliases, Alias{
				Name:        decl.Name,
				Description: decl.Description,
				Expansion:   decl.Expansion,
			})
		}
		if barrier && len(name) > bestLen {
			aliases = aliases[:0]
			bestPath = name
			bestLen = len(name)
		}
	}
	return aliases, bestPath, bestLen >= 0
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
// longest declared command path that is a prefix of name, and the path it was
// declared on.
//
// A declaration that states no answer shape is passed over, because a daemon
// writes nothing for one, and that is the ONE such declaration a daemon
// tolerates. validateShapeDecls (internal/component/plugin/server/startup.go)
// judges the list before any registry is written, and it returns an error both
// for a shape that is not doc, map or tab and for an empty shape carrying a
// column order or an address field. That error fails the WHOLE Stage 1
// registration, so a plugin declaring either serves none of its commands. What
// reaches registerPluginShapes, and what this walk passes over, is a
// declaration whose shape is empty and which names no column and no address
// field: it reaches no registry, so it inherits its nearest declared ancestor
// exactly as an undeclared command does.
//
// The pass-over is written as ParseAnswerShape rather than as a test for the
// empty string, so a spelling a daemon refuses publishes nothing here either.
// It is not a state a reader can meet in a tree this repository builds:
// TestEveryDeclaredShapeIsOneStage1Accepts
// (internal/component/plugin/all/all_test.go) holds every in-tree declaration
// to what Stage 1 accepts, because a catalog that passed over the one
// mis-spelled command and published the plugin's others would name commands the
// daemon refused to register at all.
//
// The walk is bounded by the registered plugin count and by each plugin's
// declaration list, both of which are fixed once init() has run. Nothing on a
// wire path reads this: the callers are the command catalog generators and
// `ze help command`.
func declaredByPlugin(name string) (rpc.CommandDecl, string, bool) {
	name = normalizeCommand(name)

	var best rpc.CommandDecl
	bestPath := ""
	bestLen := -1
	found := false
	for _, registration := range pluginregistry.All() {
		for index := range registration.Commands {
			decl := &registration.Commands[index]
			if _, known := ParseAnswerShape(decl.Shape); !known {
				continue
			}
			path := normalizeCommand(decl.Name)
			if !commandMatchesPrefix(name, path) {
				continue
			}
			if len(path) <= bestLen {
				continue
			}
			best = *decl
			bestPath = path
			bestLen = len(path)
			found = true
		}
	}
	return best, bestPath, found
}
