package server

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/core/ipc"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// TestRPCRegistrationTable verifies the builtin RPC registration table.
//
// VALIDATES: All handlers mapped to valid wire methods and unique CLI commands.
// PREVENTS: Lost handlers during migration from RegisterBuiltin pattern.
func TestRPCRegistrationTable(t *testing.T) {
	rpcs := AllBuiltinRPCs()

	// Only server-package init() RPCs are visible here (handler/editor register
	// from their own packages and can't be imported due to cycle).
	assert.Len(t, rpcs, 18, "expected 18 server RPCs (system 11 + session 3 + plugin-rpc 4)")

	// Track uniqueness
	wireMethodsSeen := make(map[string]bool)
	cliCommandsSeen := make(map[string]bool)

	for _, reg := range rpcs {
		t.Run(reg.WireMethod, func(t *testing.T) {
			// Valid wire method format
			_, _, err := ipc.ParseMethod(reg.WireMethod)
			require.NoError(t, err, "invalid wire method format")

			// Non-empty CLI command
			assert.NotEmpty(t, reg.CLICommand, "missing CLI command")

			// All server-package RPCs must have handlers (editor RPCs moved to editor package)
			assert.NotNil(t, reg.Handler, "nil handler for %s", reg.WireMethod)

			// Non-empty help
			assert.NotEmpty(t, reg.Help, "missing help text")

			// Unique wire method
			assert.False(t, wireMethodsSeen[reg.WireMethod], "duplicate wire method: %s", reg.WireMethod)
			wireMethodsSeen[reg.WireMethod] = true

			// Unique CLI command
			assert.False(t, cliCommandsSeen[reg.CLICommand], "duplicate CLI command: %s", reg.CLICommand)
			cliCommandsSeen[reg.CLICommand] = true
		})
	}
}

// TestRPCRegistrationPerModule verifies each module registers the expected RPCs.
//
// VALIDATES: Per-module registration via init() produces correct counts per namespace.
// PREVENTS: Commands accidentally placed in wrong module.
func TestRPCRegistrationPerModule(t *testing.T) {
	rpcs := AllBuiltinRPCs()

	// Count RPCs per module prefix
	counts := make(map[string]int)
	for _, reg := range rpcs {
		module, _, err := ipc.ParseMethod(reg.WireMethod)
		require.NoError(t, err, "invalid wire method: %s", reg.WireMethod)
		counts[module]++
	}

	assert.Equal(t, 0, counts["ze-bgp"], "ze-bgp RPCs registered from bgp/plugins/bgp-cmd-*, not here")
	assert.Equal(t, 11, counts["ze-system"], "ze-system RPCs")
	assert.Equal(t, 7, counts["ze-plugin"], "ze-plugin RPCs (session-peer-ready in bgp/plugins/cmd/peer)")
	assert.Equal(t, 0, counts["ze-editor"], "ze-editor RPCs registered from editor package, not here")
	assert.Equal(t, 0, counts["ze-rib"], "ze-rib RPCs live in bgp-rib plugin, not here")
}

// TestRPCRegistrationExpectedMethods verifies specific critical wire methods are present.
//
// VALIDATES: Essential commands are not accidentally removed.
// PREVENTS: Missing critical handlers after refactoring.
func TestRPCRegistrationExpectedMethods(t *testing.T) {
	rpcs := AllBuiltinRPCs()

	methodMap := make(map[string]*RPCRegistration, len(rpcs))
	for i := range rpcs {
		methodMap[rpcs[i].WireMethod] = &rpcs[i]
	}

	// Only server-package RPCs are visible here.
	// BGP handler RPCs (subscribe, rib, peer ops) are tested in handler_test.go.
	expectedMethods := []string{
		"ze-system:daemon-shutdown",
		"ze-system:daemon-status",
		"ze-system:daemon-reload",
		"ze-system:help",
		"ze-system:command-list",
		"ze-plugin:session-ready",
		"ze-plugin:session-bye",
	}

	for _, method := range expectedMethods {
		_, exists := methodMap[method]
		assert.True(t, exists, "missing expected wire method: %s", method)
	}
}

// TestRPCRegistrationLoadDispatcher verifies handlers load into RPCDispatcher.
//
// VALIDATES: AllBuiltinRPCs register with RPCDispatcher successfully.
// PREVENTS: Registration errors from invalid wire methods.
func TestRPCRegistrationLoadDispatcher(t *testing.T) {
	dispatcher := ipc.NewRPCDispatcher()

	for _, reg := range AllBuiltinRPCs() {
		err := dispatcher.Register(reg.WireMethod, func(_ string, _ json.RawMessage) (any, error) {
			return struct{}{}, nil
		})
		require.NoError(t, err, "failed to register %s", reg.WireMethod)
	}

	// Only server-package RPCs are registered in this test's scope
	assert.True(t, dispatcher.HasMethod("ze-system:help"))
	assert.True(t, dispatcher.HasMethod("ze-plugin:session-ready"))
	assert.False(t, dispatcher.HasMethod("ze-bgp:subscribe"), "handler RPCs not in server scope")
}

// TestRPCRegistrationToRegistry verifies the full RPC→conversion→registry integration path.
//
// VALIDATES: DeclareRegistrationInput fields flow through registrationFromRPC() into
// PluginRegistry, with correct command routing and family decode lookups.
// PREVENTS: Regression where RPC fields are converted but never reach the registry.
func TestRPCRegistrationToRegistry(t *testing.T) {
	t.Parallel()
	registry := plugin.NewPluginRegistry()

	input := &rpc.DeclareRegistrationInput{
		Families: []rpc.FamilyDecl{
			{Name: "ipv4/unicast", Mode: "both"},
			{Name: "ipv4/flow", Mode: "decode"},
			{Name: "l2vpn/evpn", Mode: "decode"},
			{Name: "ipv6/unicast", Mode: "encode"},
		},
		Commands: []rpc.CommandDecl{
			{Name: "rib adjacent in show", Description: "Show adjacent RIB"},
			{Name: "peer * refresh", Description: "Refresh peer"},
		},
		WantsConfig: []string{"bgp"},
		Schema: &rpc.SchemaDecl{
			Module:    "ze-rib-conf",
			Namespace: "urn:ze:rib:conf",
			YANGText:  "module ze-rib-conf { }",
			Handlers:  []string{"rib"},
		},
	}

	reg := registrationFromRPC(input)
	reg.Name = "rib"
	require.NoError(t, registry.Register(reg))

	// Verify multi-word commands are routable
	assert.Equal(t, "rib", registry.LookupCommand("rib adjacent in show"))
	assert.Equal(t, "rib", registry.LookupCommand("peer * refresh"))
	assert.Empty(t, registry.LookupCommand("unknown cmd"))

	// Verify decode families registered
	assert.Equal(t, "rib", registry.LookupFamily("ipv4/flow"))
	assert.Equal(t, "rib", registry.LookupFamily("l2vpn/evpn"))
	assert.Equal(t, "rib", registry.LookupFamily("ipv4/unicast")) // "both" → also in decode

	// Verify encode-only family is NOT in decode lookup
	assert.Empty(t, registry.LookupFamily("ipv6/unicast"))

	// Verify GetDecodeFamilies returns all decode-registered families
	decodeFamilies := registry.GetDecodeFamilies()
	assert.Contains(t, decodeFamilies, "ipv4/flow")
	assert.Contains(t, decodeFamilies, "l2vpn/evpn")
	assert.Contains(t, decodeFamilies, "ipv4/unicast")
	assert.NotContains(t, decodeFamilies, "ipv6/unicast")

	// Verify command info includes encoding default
	info := registry.BuildCommandInfo()
	require.Contains(t, info, "rib")
	assert.Len(t, info["rib"], 2)
}

// TestRPCCapabilityToInjector verifies the full RPC→conversion→injector integration path.
//
// VALIDATES: DeclareCapabilitiesInput fields flow through capabilitiesFromRPC() into
// CapabilityInjector with correct payload decoding and per-peer scoping.
// PREVENTS: Capability bytes lost or corrupted between RPC and OPEN injection.
func TestRPCCapabilityToInjector(t *testing.T) {
	t.Parallel()
	injector := plugin.NewCapabilityInjector()

	input := &rpc.DeclareCapabilitiesInput{
		Capabilities: []rpc.CapabilityDecl{
			{Code: 73, Encoding: "b64", Payload: "dGVzdA=="},                          // global
			{Code: 64, Encoding: "hex", Payload: "0078", Peers: []string{"10.0.0.1"}}, // per-peer
		},
	}

	caps := capabilitiesFromRPC(input)
	caps.PluginName = "hostname"
	require.NoError(t, injector.AddPluginCapabilities(caps))

	// Global capability available for any peer
	allCaps := injector.GetCapabilitiesForPeer("10.0.0.2")
	require.Len(t, allCaps, 1)
	assert.Equal(t, uint8(73), allCaps[0].Code)
	assert.Equal(t, []byte("test"), allCaps[0].Value) // b64 "dGVzdA==" → "test"

	// Specific peer gets both global and per-peer
	peerCaps := injector.GetCapabilitiesForPeer("10.0.0.1")
	require.Len(t, peerCaps, 2)

	// Per-peer comes first (takes precedence)
	assert.Equal(t, uint8(64), peerCaps[0].Code)
	assert.Equal(t, []byte{0x00, 0x78}, peerCaps[0].Value) // hex "0078"
	// Global second
	assert.Equal(t, uint8(73), peerCaps[1].Code)
}

// TestRPCRegistrationConflictThroughConversion verifies conflicts are detected
// when two plugins register via the RPC path.
//
// VALIDATES: Command/family conflicts detected even when registration comes from RPC input.
// PREVENTS: Silent command overwrites when multiple plugins register via YANG RPC.
func TestRPCRegistrationConflictThroughConversion(t *testing.T) {
	t.Parallel()
	registry := plugin.NewPluginRegistry()

	// First plugin registers via RPC
	input1 := &rpc.DeclareRegistrationInput{
		Commands: []rpc.CommandDecl{{Name: "rib show"}},
		Families: []rpc.FamilyDecl{{Name: "ipv4/flow", Mode: "decode"}},
	}
	reg1 := registrationFromRPC(input1)
	reg1.Name = "rib"
	require.NoError(t, registry.Register(reg1))

	// Second plugin tries same command
	input2 := &rpc.DeclareRegistrationInput{
		Commands: []rpc.CommandDecl{{Name: "rib show"}},
	}
	reg2 := registrationFromRPC(input2)
	reg2.Name = "rib2"
	err := registry.Register(reg2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "command conflict")

	// Third plugin tries same decode family
	input3 := &rpc.DeclareRegistrationInput{
		Families: []rpc.FamilyDecl{{Name: "ipv4/flow", Mode: "decode"}},
	}
	reg3 := registrationFromRPC(input3)
	reg3.Name = "flowspec"
	err = registry.Register(reg3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "family conflict")
}
