// VALIDATES: spec-ospf-13 plugin-self-containment (owner half) -- the OSPF command schema
// owned by this component declares every `show ospf ...` and `clear ospf ...` command
// token. The central show/clear schemas assert the SAME tokens are ABSENT (the central-guard
// half, in internal/component/cmd/{show,clear}/yang/self_containment_test.go), so removing the
// OSPF plugin removes the whole subtree with no dangling, handler-less CLI node.
// PREVENTS: the owner schema silently losing a command token (leaving a registered CLI verb
// with no schema node) while the central guard still passes -- both halves must hold together.
package yang

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

func TestOSPFCmdSchemaOwnsShowOSPF(t *testing.T) {
	want := []string{
		`ze:command "ze-ospf:show-ospf";`,
		`ze:command "ze-ospf:show-neighbor";`,
		`ze:command "ze-ospf:show-interface";`,
		`ze:command "ze-ospf:show-database";`,
		`ze:command "ze-ospf:show-database-router";`,
		`ze:command "ze-ospf:show-database-network";`,
		`ze:command "ze-ospf:show-database-summary";`,
		`ze:command "ze-ospf:show-database-asbr-summary";`,
		`ze:command "ze-ospf:show-database-external";`,
		`ze:command "ze-ospf:show-database-nssa-external";`,
		`ze:command "ze-ospf:show-database-opaque-link";`,
		`ze:command "ze-ospf:show-database-opaque-area";`,
		`ze:command "ze-ospf:show-database-opaque-as";`,
		`ze:command "ze-ospf:show-route";`,
		`ze:command "ze-ospf:show-border-routers";`,
		`ze:command "ze-ospf:show-spf";`,
	}
	for _, tok := range want {
		if !strings.Contains(ZeOSPFCmdYANG, tok) {
			t.Errorf("OSPF cmd schema is missing show command %q (owner half of plugin-self-containment; see ai/rules/plugins.md)", tok)
		}
	}
}

// TestNewCommandsDiscoverable / TestV3NewCommandsDiscoverable: spec-ospf-ext-14 R-4 -- every
// new IPv4 and IPv6 command self-documents its dispatch key in the owner schema, so it
// appears in completion and the dispatch-key listing (no hidden RPC-name-only command).
func TestNewCommandsDiscoverable(t *testing.T) {
	want := []string{
		`ze:command "ze-ospf:show-database-opaque-area-detail";`,
		`ze:command "ze-ospf:show-database-opaque-as-detail";`,
		`ze:command "ze-ospf:show-database-opaque-link-detail";`,
		`ze:command "ze-ospf:show-spf-detail";`,
		`ze:command "ze-ospf:show-neighbor-detail";`,
		`ze:command "ze-ospf:show-interface-detail";`,
		`ze:command "ze-ospf:debug-inject";`,
		`ze:command "ze-ospf:debug-inject-enable";`,
		`ze:command "ze-ospf:debug-inject-disable";`,
	}
	for _, tok := range want {
		if !strings.Contains(ZeOSPFCmdYANG, tok) {
			t.Errorf("OSPF cmd schema is missing new IPv4 command %q (spec-ospf-ext-14 discoverability)", tok)
		}
	}
}

func TestV3NewCommandsDiscoverable(t *testing.T) {
	want := []string{
		`ze:command "ze-ospf:show-ospfv3-database";`,
		`ze:command "ze-ospf:show-ospfv3-database-detail";`,
		`ze:command "ze-ospf:show-ospfv3-database-router-detail";`,
		`ze:command "ze-ospf:show-ospfv3-database-scope-link";`,
		`ze:command "ze-ospf:show-ospfv3-database-scope-area";`,
		`ze:command "ze-ospf:show-ospfv3-database-scope-as";`,
		`ze:command "ze-ospf:show-ospfv3-database-router-information";`,
		`ze:command "ze-ospf:show-ospfv3-database-extended";`,
		`ze:command "ze-ospf:show-ospfv3-database-segment-routing";`,
		`ze:command "ze-ospf:show-ospfv3-instance";`,
		`ze:command "ze-ospf:show-ospfv3-neighbor";`,
		`ze:command "ze-ospf:show-ospfv3-neighbor-detail";`,
		`ze:command "ze-ospf:show-ospfv3-interface-detail";`,
		`ze:command "ze-ospf:show-ospfv3-spf";`,
		`ze:command "ze-ospf:show-ospfv3-spf-detail";`,
		`ze:command "ze-ospf:debug-ospfv3-inject";`,
	}
	for _, tok := range want {
		if !strings.Contains(ZeOSPFCmdYANG, tok) {
			t.Errorf("OSPF cmd schema is missing new IPv6 command %q (spec-ospf-ext-14 discoverability)", tok)
		}
	}
}

// ospfv3AddressFamilyNames are the four values the address-family keyword
// admits: the names the ospf address-family config spells (multiaf.go afName*).
var ospfv3AddressFamilyNames = []string{"ipv4-multicast", "ipv4-unicast", "ipv6-multicast", "ipv6-unicast"}

// ospfCmdTree builds the command tree of the embedded modules plus this
// package's command module, so the assertions read the module a reviewer can
// open rather than whatever else the binary registered.
func ospfCmdTree(t *testing.T) *command.Node {
	t.Helper()
	loader := configyang.NewLoader()
	require.NoError(t, loader.LoadEmbedded(), "load the embedded modules")
	require.NoError(t, loader.AddModuleFromText("ze-ospf-cmd", ZeOSPFCmdYANG), "load the OSPF command module")
	schema, err := loader.Resolve()
	require.NoError(t, err, "resolve the OSPF command module")
	return configyang.BuildCommandTree(schema)
}

// addressFamilyDef answers the address-family definition node declares, or
// fails the test naming the path that lacks it.
func addressFamilyDef(t *testing.T, node *command.Node, path string) command.ArgDef {
	t.Helper()
	for _, def := range node.ArgDefs {
		if def.Name() == "address-family" {
			return def
		}
	}
	t.Fatalf("%q declares no address-family leaf", path)
	return command.ArgDef{}
}

// TestOSPFCommandSchemaCarriesAddressFamily: spec-ospfv3-views-every-address-family
// AC-3 and AC-6.
// VALIDATES: every OSPFv3 view, the new show ospf ipv6 route among them, and the
// shared show ospf database router-information declare an optional
// address-family leaf, enumerated with exactly the four config spellings, so the
// registry refuses any other word before the OSPF plugin is asked. debug ipv6 ospf
// inject lsa takes the same values in an address-family modifier group. The two
// AF listings take no keyword.
// METHOD: build the command tree from the module, read each command node's
// definitions, and validate a bogus value against one of them.
func TestOSPFCommandSchemaCarriesAddressFamily(t *testing.T) {
	root := ospfCmdTree(t)
	views := []string{
		"show ospf ipv6 interface",
		"show ospf ipv6 interface detail",
		"show ospf ipv6 graceful-restart",
		"show ospf ipv6 segment-routing",
		"show ospf ipv6 database",
		"show ospf ipv6 database detail",
		"show ospf ipv6 database router detail",
		"show ospf ipv6 database scope link",
		"show ospf ipv6 database scope area",
		"show ospf ipv6 database scope as",
		"show ospf ipv6 database router-information",
		"show ospf ipv6 database extended",
		"show ospf ipv6 database segment-routing",
		"show ospf ipv6 neighbor",
		"show ospf ipv6 neighbor detail",
		"show ospf ipv6 spf",
		"show ospf ipv6 spf detail",
		"show ospf ipv6 route",
		"show ospf database router-information",
	}
	for _, path := range views {
		t.Run(path, func(t *testing.T) {
			node := command.FindNode(root, strings.Fields(path))
			require.NotNil(t, node, "the module declares no %q command", path)
			require.NotEmpty(t, node.WireMethod, "%q is not a command", path)
			def := addressFamilyDef(t, node, path)
			assert.Equal(t, command.ArgEnum, def.Kind(), "%q address-family is not an enumeration", path)
			assert.False(t, def.Mandatory(), "%q address-family must be optional: without it the view answers every family", path)
			assert.Equal(t, ospfv3AddressFamilyNames, slices.Sorted(def.EnumValues()), "%q address-family values", path)
		})
	}

	t.Run("show ospf ipv6 route wire method", func(t *testing.T) {
		node := command.FindNode(root, strings.Fields("show ospf ipv6 route"))
		require.NotNil(t, node)
		assert.Equal(t, "ze-ospf:show-ospfv3-route", node.WireMethod)
	})

	t.Run("debug ipv6 ospf inject lsa", func(t *testing.T) {
		group := command.FindNode(root, strings.Fields("debug ipv6 ospf inject lsa address-family"))
		require.NotNil(t, group, "debug ipv6 ospf inject lsa has no address-family modifier group")
		assert.Equal(t, command.ModifierOnce, group.Modifier, "the address-family group is typed at most once")
		def := addressFamilyDef(t, group, "debug ipv6 ospf inject lsa address-family")
		assert.Equal(t, command.ArgEnum, def.Kind())
		assert.Equal(t, ospfv3AddressFamilyNames, slices.Sorted(def.EnumValues()))
	})

	for _, path := range []string{"show ospf ipv6", "show ospf ipv6 instance"} {
		t.Run(path+" takes no keyword", func(t *testing.T) {
			node := command.FindNode(root, strings.Fields(path))
			require.NotNil(t, node)
			for _, def := range node.ArgDefs {
				assert.NotEqual(t, "address-family", def.Name(), "%q lists every family and takes no address-family", path)
			}
		})
	}

	t.Run("a bogus value is refused naming the four", func(t *testing.T) {
		path := "show ospf ipv6 neighbor"
		node := command.FindNode(root, strings.Fields(path))
		require.NotNil(t, node)
		_, err := command.ValidateArgs([]string{"address-family", "bogus"}, node.ArgDefs, nil)
		require.Error(t, err, "%s address-family bogus was accepted", path)
		for _, name := range ospfv3AddressFamilyNames {
			assert.Contains(t, err.Error(), name, "the refusal does not name %s", name)
		}
		validated, err := command.ValidateArgs([]string{"address-family", "ipv4-unicast"}, node.ArgDefs, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"address-family", "ipv4-unicast"}, validated.Tokens())
	})
}

func TestOSPFCmdSchemaOwnsClearOSPF(t *testing.T) {
	want := []string{
		`ze:command "ze-ospf:clear-process";`,
		`ze:command "ze-ospf:clear-neighbor";`,
		`ze:command "ze-ospf:clear-counters";`,
	}
	for _, tok := range want {
		if !strings.Contains(ZeOSPFCmdYANG, tok) {
			t.Errorf("OSPF cmd schema is missing clear command %q (owner half of plugin-self-containment; see ai/rules/plugins.md)", tok)
		}
	}
}
