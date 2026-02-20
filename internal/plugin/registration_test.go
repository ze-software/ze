package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// TestConflictDetection verifies command/capability conflict detection.
//
// VALIDATES: Duplicate registrations are rejected at startup.
// PREVENTS: Silent command/capability overwrites.
func TestConflictDetection(t *testing.T) {
	reg := NewPluginRegistry()

	// First plugin registers a command
	plugin1 := &PluginRegistration{
		Name:     "plugin1",
		Commands: []string{"rib show"},
	}
	require.NoError(t, reg.Register(plugin1))

	// Second plugin tries same command - should fail
	plugin2 := &PluginRegistration{
		Name:     "plugin2",
		Commands: []string{"rib show"},
	}
	err := reg.Register(plugin2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "command conflict")
	assert.Contains(t, err.Error(), "rib show")
}

// TestCapabilityConflictDetection verifies capability type conflict detection.
//
// VALIDATES: Duplicate capability codes are rejected.
// PREVENTS: Two plugins sending conflicting OPEN capabilities.
func TestCapabilityConflictDetection(t *testing.T) {
	reg := NewPluginRegistry()

	// First plugin registers capability 73 (hostname)
	caps1 := &PluginCapabilities{
		PluginName: "plugin1",
		Capabilities: []PluginCapability{
			{Code: 73, Encoding: "b64", Payload: "dGVzdA=="},
		},
	}
	require.NoError(t, reg.RegisterCapabilities(caps1))

	// Second plugin tries same capability code - should fail
	caps2 := &PluginCapabilities{
		PluginName: "plugin2",
		Capabilities: []PluginCapability{
			{Code: 73, Encoding: "b64", Payload: "b3RoZXI="},
		},
	}
	err := reg.RegisterCapabilities(caps2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "capability conflict")
	assert.Contains(t, err.Error(), "73")
}

// TestPluginRegistrationStructConstruction verifies direct struct construction works.
//
// VALIDATES: PluginRegistration fields are correctly used by Register().
// PREVENTS: Regression when registration data comes from RPC instead of text parsing.
func TestPluginRegistrationStructConstruction(t *testing.T) {
	registry := NewPluginRegistry()

	reg := &PluginRegistration{
		Name:             "test-plugin",
		RFCs:             []uint16{4271, 9234},
		Encodings:        []string{"text", "b64"},
		Families:         []string{"ipv4/unicast", "ipv6/unicast"},
		DecodeFamilies:   []string{"ipv4/flow"},
		Commands:         []string{"rib show", "rib clear"},
		Receive:          []string{"update", "negotiated"},
		WantsConfigRoots: []string{"bgp"},
		Done:             true,
	}

	err := registry.Register(reg)
	require.NoError(t, err)

	// Verify registration was stored
	assert.Equal(t, "test-plugin", registry.LookupCommand("rib show"))
	assert.Equal(t, "test-plugin", registry.LookupFamily("ipv4/flow"))

	// Build command info
	info := registry.BuildCommandInfo()
	require.Contains(t, info, "test-plugin")
	assert.Len(t, info["test-plugin"], 2)
}

// TestPluginSchemaDecl verifies PluginSchemaDecl can be constructed and used.
//
// VALIDATES: Schema declaration struct holds YANG module info.
// PREVENTS: Schema data loss during RPC registration.
func TestPluginSchemaDecl(t *testing.T) {
	reg := &PluginRegistration{
		Name: "bgp-plugin",
		PluginSchema: &PluginSchemaDecl{
			Module:    "ze-bgp",
			Namespace: "urn:ze:bgp",
			Handlers:  []string{"bgp", "bgp.peer"},
			Yang:      "module ze-bgp { namespace \"urn:ze:bgp\"; }",
			Priority:  500,
		},
	}

	require.NotNil(t, reg.PluginSchema)
	assert.Equal(t, "ze-bgp", reg.PluginSchema.Module)
	assert.Equal(t, "urn:ze:bgp", reg.PluginSchema.Namespace)
	assert.Len(t, reg.PluginSchema.Handlers, 2)
	assert.Contains(t, reg.PluginSchema.Handlers, "bgp")
	assert.Contains(t, reg.PluginSchema.Handlers, "bgp.peer")
	assert.Equal(t, 500, reg.PluginSchema.Priority)
}

// TestRPCRegistrationToRegistry verifies the full RPC→conversion→registry integration path.
//
// VALIDATES: DeclareRegistrationInput fields flow through registrationFromRPC() into
// PluginRegistry, with correct command routing and family decode lookups.
// PREVENTS: Regression where RPC fields are converted but never reach the registry.
func TestRPCRegistrationToRegistry(t *testing.T) {
	t.Parallel()
	registry := NewPluginRegistry()

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
	injector := NewCapabilityInjector()

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
	registry := NewPluginRegistry()

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
