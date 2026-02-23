package config

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugin/registry"
)

// TestAddressFamilyValidator_Validate verifies address family validation.
//
// VALIDATES: Registered family accepted, unregistered rejected (AC-16, AC-17).
// PREVENTS: Silent acceptance of invalid address families.
func TestAddressFamilyValidator_Validate(t *testing.T) {
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	registry.Reset()

	require.NoError(t, registry.Register(registry.Registration{
		Name:        "test-family-validator",
		Description: "test plugin for family validation",
		Families:    []string{"ipv4/unicast", "ipv6/unicast"},
		RunEngine:   func(_, _ net.Conn) int { return 0 },
		CLIHandler:  func(_ []string) int { return 0 },
	}))

	v := AddressFamilyValidator()

	// Valid registered families.
	assert.NoError(t, v.ValidateFn("bgp.peer.family", "ipv4/unicast"))
	assert.NoError(t, v.ValidateFn("bgp.peer.family", "ipv6/unicast"))

	// Invalid: not registered.
	err := v.ValidateFn("bgp.peer.family", "invalid/family")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid/family")
	assert.Contains(t, err.Error(), "not a registered")
}

// TestNonzeroIPv4Validator verifies IPv4 validation rejecting 0.0.0.0.
//
// VALIDATES: Valid IPv4 accepted, 0.0.0.0 and non-IPv4 rejected (AC-19, AC-20).
// PREVENTS: Zero or invalid next-hop silently accepted in config.
func TestNonzeroIPv4Validator(t *testing.T) {
	v := NonzeroIPv4Validator()

	// Valid IPv4 addresses pass.
	assert.NoError(t, v.ValidateFn("bgp.peer.route.next-hop", "1.2.3.4"))
	assert.NoError(t, v.ValidateFn("bgp.peer.route.next-hop", "192.168.1.1"))
	assert.NoError(t, v.ValidateFn("bgp.peer.route.next-hop", "255.255.255.255"))

	// 0.0.0.0 rejected.
	err := v.ValidateFn("bgp.peer.route.next-hop", "0.0.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "0.0.0.0")

	// Non-IPv4 rejected (use "literal-self" validator for "self").
	assert.Error(t, v.ValidateFn("bgp.peer.route.next-hop", "self"))
	assert.Error(t, v.ValidateFn("bgp.peer.route.next-hop", "notanip"))

	// Non-string rejected.
	assert.Error(t, v.ValidateFn("bgp.peer.route.next-hop", 42))
}

// TestLiteralSelfValidator verifies the "self" literal validator.
//
// VALIDATES: "self" accepted, everything else rejected.
// PREVENTS: Arbitrary strings passing as "self".
func TestLiteralSelfValidator(t *testing.T) {
	v := LiteralSelfValidator()

	assert.NoError(t, v.ValidateFn("bgp.peer.route.next-hop", "self"))
	assert.Error(t, v.ValidateFn("bgp.peer.route.next-hop", "1.2.3.4"))
	assert.Error(t, v.ValidateFn("bgp.peer.route.next-hop", "other"))
	assert.Error(t, v.ValidateFn("bgp.peer.route.next-hop", 42))
}

// TestCommunityRangeValidator verifies community ASN:value range checking.
//
// VALIDATES: Valid communities accepted, out-of-range parts rejected (AC-21, AC-22).
// PREVENTS: Communities with values exceeding uint16 silently accepted.
func TestCommunityRangeValidator(t *testing.T) {
	v := CommunityRangeValidator()

	// Valid communities.
	assert.NoError(t, v.ValidateFn("bgp.peer.route.community", "0:0"))
	assert.NoError(t, v.ValidateFn("bgp.peer.route.community", "65535:65535"))
	assert.NoError(t, v.ValidateFn("bgp.peer.route.community", "100:200"))

	// ASN part out of range.
	err := v.ValidateFn("bgp.peer.route.community", "65536:0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "65536")

	// Value part out of range.
	err = v.ValidateFn("bgp.peer.route.community", "0:65536")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "65536")

	// Missing colon.
	err = v.ValidateFn("bgp.peer.route.community", "nocolon")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ASN:value")

	// Non-string rejected.
	assert.Error(t, v.ValidateFn("bgp.peer.route.community", 42))
}

// TestAddressFamilyValidator_Complete verifies completion returns registered families.
//
// VALIDATES: Complete() returns all registered families for CLI completion (AC-18).
// PREVENTS: Missing completion values for address families.
func TestAddressFamilyValidator_Complete(t *testing.T) {
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	registry.Reset()

	require.NoError(t, registry.Register(registry.Registration{
		Name:        "test-family-complete",
		Description: "test plugin for family completion",
		Families:    []string{"ipv4/unicast", "ipv6/unicast"},
		RunEngine:   func(_, _ net.Conn) int { return 0 },
		CLIHandler:  func(_ []string) int { return 0 },
	}))

	v := AddressFamilyValidator()
	require.NotNil(t, v.CompleteFn)

	values := v.CompleteFn()
	assert.Contains(t, values, "ipv4/unicast")
	assert.Contains(t, values, "ipv6/unicast")
}
