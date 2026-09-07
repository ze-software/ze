// VALIDATES: DeclaredForCommand resolves the registry channel and the plugin
// channel by path length TOGETHER, and answers what the same declarations would
// leave in the registries inside a running daemon.
// PREVENTS: a command that declares its own answer publishing an ancestor's,
// which is what put eleven route columns and two address fields on
// `show bgp rib help`.

package command

import (
	"net"
	"slices"
	"testing"

	pluginregistry "github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// declarationOwner is the plugin name the daemon half of these tests writes
// under, and the name RegisterPluginShapes takes a declaration back under.
const declarationOwner = "declaring-plugin"

// TestDeclaredPrefersTheExactPathOverAnAncestor pins the ORDER the two channels
// resolve in, over the three commands of BLOCKER 1 and not only over them.
//
// The registry holds a declaration on an ANCESTOR and the plugin declares the
// command itself. A daemon answers the plugin's, because registerPluginShapes
// writes it into the registry at the exact path and the resolution is longest
// path wins. A reader that consults the plugin only when the registry answered
// nothing answers the ancestor's, which is a different answer for one command.
func TestDeclaredPrefersTheExactPathOverAnAncestor(t *testing.T) {
	resetDeclarations(t)

	RegisterShape([]string{"show demo"}, ShapeTab)
	RegisterColumns([]string{"show demo"},
		ColumnOrder{"peer", "prefix", "next-hop"})
	RegisterAddressFields([]string{"show demo"}, "peer", "next-hop")

	registerDeclaringPlugin(t, rpc.CommandDecl{Name: "show demo help", Shape: "doc"})

	declared := DeclaredForCommand("show demo help")
	if !declared.ShapeDeclared || declared.Shape != ShapeDoc {
		t.Errorf("shape = %v (declared %v), and the plugin declares doc on the command itself",
			declared.Shape, declared.ShapeDeclared)
	}
	if len(declared.Columns) != 0 {
		t.Errorf("column orders = %v, and the command declares none: an ancestor's order is not its own",
			declared.Columns)
	}
	if len(declared.AddressFields) != 0 {
		t.Errorf("address fields = %v, and the command declares none: `| resolve` would decorate a field it does not write",
			declared.AddressFields)
	}
}

// TestDeclaredMatchesTheRegistriesADaemonWrites is the same tree of
// declarations, read the way a RUNNING daemon reads it: the plugin's Stage 1
// message goes into the registries through the write path the plugin server
// uses, and the three registry readers are asked directly.
//
// The two tests together are the claim. A reader that answers something the
// daemon does not is a catalog promising an operator an answer no running
// command gives.
func TestDeclaredMatchesTheRegistriesADaemonWrites(t *testing.T) {
	resetDeclarations(t)

	RegisterShape([]string{"show demo"}, ShapeTab)
	RegisterColumns([]string{"show demo"},
		ColumnOrder{"peer", "prefix", "next-hop"})
	RegisterAddressFields([]string{"show demo"}, "peer", "next-hop")

	if err := RegisterPluginShapes(declarationOwner, []PluginShape{
		{Command: "show demo help", Shape: ShapeDoc},
	}); err != nil {
		t.Fatalf("the daemon's own write path refused the declaration: %v", err)
	}

	if shape, declared := ShapeForCommand("show demo help"); !declared || shape != ShapeDoc {
		t.Errorf("the daemon holds shape %v (declared %v), want doc", shape, declared)
	}
	if orders := ColumnsForCommand("show demo help"); len(orders) != 0 {
		t.Errorf("the daemon holds column orders %v, want none", orders)
	}
	if fields := AddressFieldsForCommand("show demo help"); len(fields) != 0 {
		t.Errorf("the daemon holds address fields %v, want none", fields)
	}
}

// TestDeclaredKeepsAnAncestorPluginDeclaration is the other half of the same
// rule. A command the plugin does NOT declare still inherits the nearest
// declared ancestor, on the plugin channel as on the registry one, because that
// is what the registries do once the declaration is written into them.
func TestDeclaredKeepsAnAncestorPluginDeclaration(t *testing.T) {
	resetDeclarations(t)

	registerDeclaringPlugin(t, rpc.CommandDecl{
		Name:          "show demo",
		Shape:         "tab",
		Columns:       []string{"prefix", "asn"},
		AddressFields: []string{"prefix"},
	})

	declared := DeclaredForCommand("show demo rows")
	if !declared.ShapeDeclared || declared.Shape != ShapeTab {
		t.Errorf("shape = %v (declared %v), want the ancestor's tab", declared.Shape, declared.ShapeDeclared)
	}
	if len(declared.Columns) != 1 || !slices.Equal(declared.Columns[0], []string{"prefix", "asn"}) {
		t.Errorf("column orders = %v, want the ancestor's [prefix asn]", declared.Columns)
	}
	if !slices.Equal(declared.AddressFields, []string{"prefix"}) {
		t.Errorf("address fields = %v, want the ancestor's [prefix]", declared.AddressFields)
	}
}

// TestDeclaredPassesOverADeclarationWithNoShape pins the one CommandDecl a
// daemon writes nothing for, and it is the only one a daemon accepts: a shape
// that is empty, with no column and no address field beside it. Anything else
// with no shape, and any shape ParseAnswerShape does not know, is refused by
// validateShapeDecls (internal/component/plugin/server/startup.go), and that
// refusal fails the plugin's WHOLE Stage 1 registration.
//
// registerPluginShapes writes nothing for this declaration, so the command
// reaches no registry and inherits its nearest declared ancestor. A reader that
// took the declaration as a barrier would publish NO column order for a command
// a running daemon renders with its ancestor's.
func TestDeclaredPassesOverADeclarationWithNoShape(t *testing.T) {
	resetDeclarations(t)

	RegisterShape([]string{"show demo"}, ShapeTab)
	RegisterColumns([]string{"show demo"}, ColumnOrder{"peer", "prefix"})

	registerDeclaringPlugin(t, rpc.CommandDecl{Name: "show demo help"})

	declared := DeclaredForCommand("show demo help")
	if !declared.ShapeDeclared || declared.Shape != ShapeTab {
		t.Errorf("shape = %v (declared %v), want the ancestor's tab: a declaration with no shape reaches no registry",
			declared.Shape, declared.ShapeDeclared)
	}
	if len(declared.Columns) != 1 || !slices.Equal(declared.Columns[0], []string{"peer", "prefix"}) {
		t.Errorf("column orders = %v, want the ancestor's: a declaration with no shape reaches no registry",
			declared.Columns)
	}
}

// TestDeclaredAliasesTakeTheLongestDeclaredPath pins the alias channel to the
// resolution the other three already use: the longest declared path answers,
// and a shorter one is SHADOWED rather than added to it.
//
// A daemon writes the plugin's alias into the registry at the plugin's own
// path, and lookupAlias reads one path only. A reader that added the plugin's
// aliases to whatever the registry answered for an ancestor would publish
// `show demo rows | summary` over a payload that carries no such rows.
func TestDeclaredAliasesTakeTheLongestDeclaredPath(t *testing.T) {
	resetDeclarations(t)

	RegisterAliases([]string{"show demo"},
		Alias{Name: "summary", Description: "The aggregates alone", Expansion: "display local-as"})

	registerDeclaringPluginAliases(t,
		[]rpc.CommandDecl{{Name: "show demo rows", Shape: "tab"}},
		[]rpc.PipeDecl{{
			Command:     "show demo rows",
			Name:        "peers",
			Description: "The peer rows alone",
			Expansion:   "display peers",
		}})

	names := aliasNames(DeclaredForCommand("show demo rows"))
	if !slices.Equal(names, []string{"peers"}) {
		t.Errorf("aliases = %v, want the plugin's own: the ancestor's sits on a shorter path", names)
	}
}

// TestDeclaredAliasesStopAtAPluginBarrier pins the read side of aliasBarriers
// (alias.go). A command the same plugin declares BELOW its alias path answers
// no alias at all, because the daemon writes an empty declaration there and
// nothing above it is read.
//
// The in-tree alias on the ancestor is what makes this discriminating: a reader
// that only skipped the PLUGIN's alias would still publish `summary` here, and
// a running daemon publishes neither.
func TestDeclaredAliasesStopAtAPluginBarrier(t *testing.T) {
	resetDeclarations(t)

	RegisterAliases([]string{"show demo"},
		Alias{Name: "summary", Description: "The aggregates alone", Expansion: "display local-as"})

	// The alias sits on `show demo`, and the plugin declares both commands:
	// validatePipeDecls (internal/component/plugin/server/startup.go) refuses an
	// alias on a command its own message does not declare.
	registerDeclaringPluginAliases(t,
		[]rpc.CommandDecl{
			{Name: "show demo", Shape: "tab"},
			{Name: "show demo rows", Shape: "tab"},
		},
		[]rpc.PipeDecl{{
			Command:     "show demo",
			Name:        "peers",
			Description: "The peer rows alone",
			Expansion:   "display peers",
		}})

	if names := aliasNames(DeclaredForCommand("show demo rows")); len(names) != 0 {
		t.Errorf("aliases = %v, want none: the plugin's declaration is a barrier on this command", names)
	}
	if names := aliasNames(DeclaredForCommand("show demo")); !slices.Equal(names, []string{"peers", "summary"}) {
		t.Errorf("aliases = %v, want both: the barrier sits below this path, not on it", names)
	}
}

// aliasNames answers the alias names of a declaration, which is what these
// tests assert on: the description and the expansion travel with the name.
func aliasNames(declared Declared) []string {
	names := make([]string, 0, len(declared.Aliases))
	for _, alias := range declared.Aliases {
		names = append(names, alias.Name)
	}
	return names
}

// registerDeclaringPluginAliases registers one plugin carrying both declaration
// lists. registerDeclaringPlugin covers the command list alone, which is what
// most of these tests need.
func registerDeclaringPluginAliases(t *testing.T, decls []rpc.CommandDecl, pipes []rpc.PipeDecl) {
	t.Helper()

	err := pluginregistry.Register(pluginregistry.Registration{
		Name:       declarationOwner,
		RunEngine:  func(net.Conn) int { return 0 },
		CLIHandler: func([]string) int { return 0 },
		Commands:   decls,
		Pipes:      pipes,
	})
	if err != nil {
		t.Fatalf("register the declaring plugin: %v", err)
	}
}

// resetDeclarations empties every registry these tests write, before and after,
// so a declaration never crosses into another test in this binary.
func resetDeclarations(t *testing.T) {
	t.Helper()

	clear := func() {
		ResetShapesForTest()
		ResetColumnsForTest()
		ResetAddressFieldsForTest()
		ResetAliasesForTest()
		pluginregistry.Reset()
	}
	clear()
	t.Cleanup(clear)
}

// registerDeclaringPlugin registers one plugin carrying decls and nothing else.
// Register refuses a registration with no RunEngine and no CLIHandler, and
// neither is ever called: this gate reads the declaration, it does not run the
// plugin.
func registerDeclaringPlugin(t *testing.T, decls ...rpc.CommandDecl) {
	t.Helper()

	err := pluginregistry.Register(pluginregistry.Registration{
		Name:       declarationOwner,
		RunEngine:  func(net.Conn) int { return 0 },
		CLIHandler: func([]string) int { return 0 },
		Commands:   decls,
	})
	if err != nil {
		t.Fatalf("register the declaring plugin: %v", err)
	}
}
