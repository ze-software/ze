package config

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
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
		RunEngine:   func(_ net.Conn) int { return 0 },
		CLIHandler:  func(_ []string) int { return 0 },
	}))

	v := AddressFamilyValidator()

	// Valid registered families.
	assert.NoError(t, v.ValidateFn("bgp/peer/family", "ipv4/unicast"))
	assert.NoError(t, v.ValidateFn("bgp/peer/family", "ipv6/unicast"))

	// Invalid: not registered.
	err := v.ValidateFn("bgp/peer/family", "invalid/family")
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
	assert.NoError(t, v.ValidateFn("bgp/peer/route.next-hop", "1.2.3.4"))
	assert.NoError(t, v.ValidateFn("bgp/peer/route.next-hop", "192.168.1.1"))
	assert.NoError(t, v.ValidateFn("bgp/peer/route.next-hop", "255.255.255.255"))

	// 0.0.0.0 rejected.
	err := v.ValidateFn("bgp/peer/route.next-hop", "0.0.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "0.0.0.0")

	// Non-IPv4 rejected (use "literal-self" validator for "self").
	assert.Error(t, v.ValidateFn("bgp/peer/route.next-hop", "self"))
	assert.Error(t, v.ValidateFn("bgp/peer/route.next-hop", "notanip"))

	// Non-string rejected.
	assert.Error(t, v.ValidateFn("bgp/peer/route.next-hop", 42))
}

// TestLiteralSelfValidator verifies the "self" literal validator.
//
// VALIDATES: "self" accepted, everything else rejected.
// PREVENTS: Arbitrary strings passing as "self".
func TestLiteralSelfValidator(t *testing.T) {
	v := LiteralSelfValidator()

	assert.NoError(t, v.ValidateFn("bgp/peer/route.next-hop", "self"))
	assert.Error(t, v.ValidateFn("bgp/peer/route.next-hop", "1.2.3.4"))
	assert.Error(t, v.ValidateFn("bgp/peer/route.next-hop", "other"))
	assert.Error(t, v.ValidateFn("bgp/peer/route.next-hop", 42))
}

// TestCommunityRangeValidator verifies community ASN:value range checking.
//
// VALIDATES: Valid communities accepted, out-of-range parts rejected (AC-21, AC-22).
// PREVENTS: Communities with values exceeding uint16 silently accepted.
func TestCommunityRangeValidator(t *testing.T) {
	v := CommunityRangeValidator()

	// Valid communities.
	assert.NoError(t, v.ValidateFn("bgp/peer/route.community", "0:0"))
	assert.NoError(t, v.ValidateFn("bgp/peer/route.community", "65535:65535"))
	assert.NoError(t, v.ValidateFn("bgp/peer/route.community", "100:200"))

	// ASN part out of range.
	err := v.ValidateFn("bgp/peer/route.community", "65536:0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "65536")

	// Value part out of range.
	err = v.ValidateFn("bgp/peer/route.community", "0:65536")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "65536")

	// Missing colon.
	err = v.ValidateFn("bgp/peer/route.community", "nocolon")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ASN:value")

	// Non-string rejected.
	assert.Error(t, v.ValidateFn("bgp/peer/route.community", 42))
}

// TestReceiveEventValidator_Validate verifies receive event type validation.
//
// VALIDATES: Valid BGP events accepted, invalid rejected.
// PREVENTS: Invalid event types silently accepted in config receive list.
func TestReceiveEventValidator_Validate(t *testing.T) {
	v := ReceiveEventValidator()

	// Valid base event types.
	assert.NoError(t, v.ValidateFn("bgp/peer/process.receive", "update"))
	assert.NoError(t, v.ValidateFn("bgp/peer/process.receive", "state"))
	assert.NoError(t, v.ValidateFn("bgp/peer/process.receive", "open"))

	// Invalid event type.
	err := v.ValidateFn("bgp/peer/process.receive", "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
	assert.Contains(t, err.Error(), "not a valid receive event type")

	// Non-string rejected.
	assert.Error(t, v.ValidateFn("bgp/peer/process.receive", 42))
}

// TestReceiveEventValidator_Complete verifies completion returns event type names.
//
// VALIDATES: CompleteFn returns sorted BGP event types.
// PREVENTS: Missing completion values for receive event types.
func TestReceiveEventValidator_Complete(t *testing.T) {
	v := ReceiveEventValidator()
	require.NotNil(t, v.CompleteFn)

	values := v.CompleteFn()
	require.NotEmpty(t, values, "should return event type names")
	assert.Contains(t, values, "update")
	assert.Contains(t, values, "state")
	assert.Contains(t, values, "open")

	// Should be sorted.
	for i := 1; i < len(values); i++ {
		assert.True(t, values[i-1] <= values[i],
			"values should be sorted: %q > %q", values[i-1], values[i])
	}
}

// TestSendMessageValidator_Validate verifies send message type validation.
//
// VALIDATES: Base send types accepted, invalid rejected.
// PREVENTS: Invalid send types silently accepted in config send list.
func TestSendMessageValidator_Validate(t *testing.T) {
	v := SendMessageValidator()

	// Valid base types.
	assert.NoError(t, v.ValidateFn("bgp/peer/process.send", "update"))
	assert.NoError(t, v.ValidateFn("bgp/peer/process.send", "refresh"))

	// Invalid type.
	err := v.ValidateFn("bgp/peer/process.send", "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
	assert.Contains(t, err.Error(), "not a valid send type")

	// Non-string rejected.
	assert.Error(t, v.ValidateFn("bgp/peer/process.send", 42))
}

// TestSendMessageValidator_Complete verifies completion returns send type names.
//
// VALIDATES: CompleteFn returns base send types sorted.
// PREVENTS: Missing completion values for send types.
func TestSendMessageValidator_Complete(t *testing.T) {
	v := SendMessageValidator()
	require.NotNil(t, v.CompleteFn)

	values := v.CompleteFn()
	require.NotEmpty(t, values, "should return send type names")
	assert.Contains(t, values, "update")
	assert.Contains(t, values, "refresh")

	// Should be sorted.
	for i := 1; i < len(values); i++ {
		assert.True(t, values[i-1] <= values[i],
			"values should be sorted: %q > %q", values[i-1], values[i])
	}
}

// TestAllBGPEventNames verifies the helper that extracts sorted event names.
//
// VALIDATES: Returns sorted, non-empty list from ValidBgpEvents.
// PREVENTS: Empty or unsorted completion lists.
func TestAllBGPEventNames(t *testing.T) {
	names := allBGPEventNames()
	require.NotEmpty(t, names, "should return event names from ValidBgpEvents")
	assert.Contains(t, names, "update")
	assert.Contains(t, names, "state")

	// Verify sorted.
	for i := 1; i < len(names); i++ {
		assert.True(t, names[i-1] <= names[i],
			"names should be sorted: %q > %q", names[i-1], names[i])
	}
}

// TestAllSendTypeNames verifies the helper that formats send type names.
//
// VALIDATES: Returns comma-separated base types.
// PREVENTS: Malformed error messages.
func TestAllSendTypeNames(t *testing.T) {
	result := allSendTypeNames()
	assert.Contains(t, result, "update")
	assert.Contains(t, result, "refresh")
}

// TestMACAddressValidator_Validate verifies MAC address format validation.
//
// VALIDATES: Valid MAC accepted, invalid formats rejected (AC-9).
// PREVENTS: Invalid MAC addresses silently accepted in interface config.
func TestMACAddressValidator_Validate(t *testing.T) {
	v := MACAddressValidator()

	// Valid MAC addresses.
	assert.NoError(t, v.ValidateFn("interface.ethernet.mac-address", "aa:bb:cc:dd:ee:ff"))
	assert.NoError(t, v.ValidateFn("interface.ethernet.mac-address", "AA:BB:CC:DD:EE:FF"))
	assert.NoError(t, v.ValidateFn("interface.ethernet.mac-address", "00:11:22:33:44:55"))
	assert.NoError(t, v.ValidateFn("interface.ethernet.mac-address", "01:23:45:67:89:aB"))

	// Invalid: wrong separator.
	err := v.ValidateFn("interface.ethernet.mac-address", "aa-bb-cc-dd-ee-ff")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid MAC address")

	// Invalid: too short.
	err = v.ValidateFn("interface.ethernet.mac-address", "aa:bb:cc:dd:ee")
	require.Error(t, err)

	// Invalid: too long.
	err = v.ValidateFn("interface.ethernet.mac-address", "aa:bb:cc:dd:ee:ff:00")
	require.Error(t, err)

	// Invalid: non-hex.
	err = v.ValidateFn("interface.ethernet.mac-address", "gg:hh:ii:jj:kk:ll")
	require.Error(t, err)

	// Invalid: empty.
	err = v.ValidateFn("interface.ethernet.mac-address", "")
	require.Error(t, err)

	// Non-string rejected.
	assert.Error(t, v.ValidateFn("interface.ethernet.mac-address", 42))
}

// TestMACAddressValidator_Complete verifies that MACAddressValidator has nil
// CompleteFn by default. The CompleteFn is registered separately by the iface
// package via yang.RegisterCompleteFn and merged at startup.
//
// VALIDATES: MACAddressValidator returns only ValidateFn (AC-10).
// PREVENTS: Accidental re-coupling of config to iface.
func TestMACAddressValidator_Complete(t *testing.T) {
	v := MACAddressValidator()
	assert.Nil(t, v.CompleteFn, "CompleteFn should be nil -- registered globally by iface")
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
		RunEngine:   func(_ net.Conn) int { return 0 },
		CLIHandler:  func(_ []string) int { return 0 },
	}))

	v := AddressFamilyValidator()
	require.NotNil(t, v.CompleteFn)

	values := v.CompleteFn()
	assert.Contains(t, values, "ipv4/unicast")
	assert.Contains(t, values, "ipv6/unicast")
}
