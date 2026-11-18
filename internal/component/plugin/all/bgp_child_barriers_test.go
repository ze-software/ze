//go:build ze_bgp

// Design: docs/architecture/api/commands.md -- registered BGP child declaration barriers

package all

import (
	"net"
	"slices"
	"strings"
	"testing"

	_ "github.com/ze-software/ze/internal/component/bgp/cli"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRegisteredShowBGPChildrenDoNotInheritParentAliases derives the population
// from all command registration channels and reads the aliases operators see.
// A child may declare its own summary; it must not inherit the parent's chain.
func TestRegisteredShowBGPChildrenDoNotInheritParentAliases(t *testing.T) {
	parent := command.AliasesForCommand("show bgp")
	if len(parent) < 2 {
		t.Fatalf("show bgp lost its own aliases: %v", parent)
	}
	paths := registeredShowBGPPaths(t)
	for _, control := range []string{"show bgp reject-asn", "show bgp decode", "show bgp encode", "show bgp rpki"} {
		if !slices.Contains(paths, control) {
			t.Fatalf("registration population omits control %q", control)
		}
	}
	for _, path := range paths {
		for _, alias := range command.DeclaredForCommand(path).Aliases {
			for _, inherited := range parent {
				if alias.Name == inherited.Name && alias.Expansion == inherited.Expansion {
					t.Errorf("%s inherits show bgp alias %s: %s", path, alias.Name, alias.Expansion)
				}
			}
		}
	}
}

// TestRegisteredShowBGPChildShapesBlockNewBranches registers a new branch with
// no answer declarations and checks both its canonical and selector spellings.
// Parent declarations must survive while the new branch inherits none of them.
func TestRegisteredShowBGPChildShapesBlockNewBranches(t *testing.T) {
	const path = "show bgp ac26-child detail"
	registerBGPBarrierProbe(t, path)
	for _, spelling := range []string{path, "show bgp ac26-child", "show bgp ac26-child 192.0.2.1 detail"} {
		if shape, declared := command.ShapeForCommand(spelling); declared {
			t.Errorf("%s inherits parent answer shape %s", spelling, shape)
		}
		if columns := command.ColumnsForCommand(spelling); len(columns) != 0 {
			t.Errorf("%s inherits parent columns %v", spelling, columns)
		}
		if fields := command.AddressFieldsForCommand(spelling); len(fields) != 0 {
			t.Errorf("%s inherits parent address fields %v", spelling, fields)
		}
	}
	if shape, declared := command.ShapeForCommand("show bgp"); !declared || shape != command.ShapeTab {
		t.Errorf("show bgp lost its own tab shape: %s/%v", shape, declared)
	}
	if fields := command.AddressFieldsForCommand("show bgp"); !slices.Equal(fields, []string{"address"}) {
		t.Errorf("show bgp lost its own address field: %v", fields)
	}
}

// TestRegisteredShowBGPChildDeclarationsSurvivePluginWithdrawal installs a
// runtime declaration on a registered branch, withdraws it, then registers it
// again. The owner's values win while present; withdrawal exposes the branch
// floor rather than the parent declarations.
func TestRegisteredShowBGPChildDeclarationsSurvivePluginWithdrawal(t *testing.T) {
	const path = "show bgp ac26-lifecycle"
	const owner = "ac26-runtime-child"
	registerBGPBarrierProbe(t, path)
	t.Cleanup(func() { command.UnregisterPluginShapes(owner) })
	for range 2 {
		if err := command.RegisterPluginShapes(owner, []command.PluginShape{{
			Command: path, Shape: command.ShapeMap,
			Columns: []string{"probe-address", "probe-state"}, AddressFields: []string{"probe-address"},
		}}); err != nil {
			t.Fatal(err)
		}
		if shape, declared := command.ShapeForCommand(path); !declared || shape != command.ShapeMap {
			t.Errorf("registered child shape = %s/%v, want map/true", shape, declared)
		}
		if columns := command.ColumnsForCommand(path); len(columns) != 1 || !slices.Equal(columns[0], command.ColumnOrder{"probe-address", "probe-state"}) {
			t.Errorf("registered child columns = %v", columns)
		}
		if fields := command.AddressFieldsForCommand(path); !slices.Equal(fields, []string{"probe-address"}) {
			t.Errorf("registered child address fields = %v", fields)
		}
		command.UnregisterPluginShapes(owner)
		if _, declared := command.ShapeForCommand(path); declared {
			t.Error("withdrawn child shape inherited show bgp")
		}
		if columns := command.ColumnsForCommand(path); len(columns) != 0 {
			t.Errorf("withdrawn child inherited columns %v", columns)
		}
		if fields := command.AddressFieldsForCommand(path); len(fields) != 0 {
			t.Errorf("withdrawn child inherited address fields %v", fields)
		}
	}
}

// registeredShowBGPPaths joins handlers with all their model spellings, local
// handlers, streaming registrations and compiled plugin declarations. No one
// channel is a complete command inventory.
func registeredShowBGPPaths(t *testing.T) []string {
	t.Helper()
	schema, err := yang.DefaultLoader()
	if err != nil {
		t.Fatal(err)
	}
	wireToPaths := yang.WireMethodToPaths(schema)
	paths := make(map[string]bool)
	add := func(path string) {
		if strings.HasPrefix(path, "show bgp ") {
			paths[path] = true
		}
	}
	for _, registration := range pluginserver.AllBuiltinRPCs() {
		for _, path := range wireToPaths[registration.WireMethod] {
			add(path)
		}
	}
	for _, path := range pluginserver.StreamingPrefixes() {
		add(path)
	}
	for _, registration := range command.ListLocal() {
		add(registration.Path)
	}
	registrations := registry.All()
	for i := range registrations {
		for _, declaration := range registrations[i].Commands {
			add(declaration.Name)
		}
	}
	result := make([]string, 0, len(paths))
	for path := range paths {
		result = append(result, path)
	}
	slices.Sort(result)
	return result
}

// registerBGPBarrierProbe adds a command without changing the shipping plugin
// population after this test. These tests cannot run in parallel with another
// test that changes the process-wide plugin registry.
func registerBGPBarrierProbe(t *testing.T, path string) {
	t.Helper()
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	if err := registry.Register(registry.Registration{
		Name: "ac26-child-probe", Description: "BGP child barrier probe",
		RunEngine: func(net.Conn) int { return 0 }, CLIHandler: func([]string) int { return 0 },
		Commands: []rpc.CommandDecl{{Name: path}},
	}); err != nil {
		t.Fatal(err)
	}
}
