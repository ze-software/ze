package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
