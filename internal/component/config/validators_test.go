package config

import (
	"net"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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

// TestPortSpecValidator verifies port spec validation for policy/firewall match fields.
//
// VALIDATES: Ports, ranges, lists, and named sets accepted; invalid ports rejected.
// PREVENTS: ze:validate "port-spec" registration accepting port 0 or malformed specs.
func TestPortSpecValidator(t *testing.T) {
	v := PortSpecValidator()

	for _, valid := range []string{"1", "65535", "80-90", "22,80,443", "5060-5061,16384-32767", "@web_ports"} {
		assert.NoError(t, v.ValidateFn("policy route term from source-port", valid), valid)
	}

	for _, invalid := range []any{"", "0", "65536", "90-80", "22,,443", "22, 443", "@", "@bad.name", 42} {
		assert.Error(t, v.ValidateFn("policy route term from source-port", invalid), invalid)
	}
}

// TestCommunityRangeValidator verifies community validation.
//
// VALIDATES: ASN:value, well-known names, hex, bare integers accepted; invalid rejected.
// PREVENTS: Communities with values exceeding uint16 silently accepted.
func TestCommunityRangeValidator(t *testing.T) {
	v := CommunityRangeValidator()

	// Valid ASN:value communities.
	assert.NoError(t, v.ValidateFn("bgp/peer/route.community", "0:0"))
	assert.NoError(t, v.ValidateFn("bgp/peer/route.community", "65535:65535"))
	assert.NoError(t, v.ValidateFn("bgp/peer/route.community", "100:200"))

	// Well-known community names.
	assert.NoError(t, v.ValidateFn("bgp/peer/route.community", "no-export"))
	assert.NoError(t, v.ValidateFn("bgp/peer/route.community", "blackhole"))
	assert.NoError(t, v.ValidateFn("bgp/peer/route.community", "graceful-shutdown"))

	// ASN part out of range.
	err := v.ValidateFn("bgp/peer/route.community", "65536:0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "65536")

	// Value part out of range.
	err = v.ValidateFn("bgp/peer/route.community", "0:65536")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "65536")

	// Invalid string.
	err = v.ValidateFn("bgp/peer/route.community", "nocolon")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid community")

	// Non-string rejected.
	assert.Error(t, v.ValidateFn("bgp/peer/route.community", 42))

	// CompleteFn returns well-known names.
	require.NotNil(t, v.CompleteFn)
	names := v.CompleteFn()
	assert.Contains(t, names, "no-export")
	assert.Contains(t, names, "graceful-shutdown")
	assert.Contains(t, names, "blackhole")
}

// TestBusMessageValidators verifies bus message and path custom validators.
//
// VALIDATES: BusMessageNameValidator, BusMessageOrNoneValidator,
// BusPathOrNoneValidator, and BusParticipantPathValidator accept registered
// non-cli, external-declared, and cli: messages and canonical paths, refuse
// malformed/unregistered/non-string values with errors naming both the
// offending token and valid registered messages, and offer sorted completions.
// PREVENTS: Malformed or unregistered bus message identifiers entering config
// trees, silent omission of valid tokens in error text, or unsorted completions.
func TestBusMessageValidators(t *testing.T) {
	restoreExt := ResetExternalBusDeclsForTest()
	t.Cleanup(restoreExt)

	require.NoError(t, events.RegisterMessage("valtest", "alpha"))
	require.NoError(t, events.RegisterMessage("valtest", "zeta"))
	RecordExternalBusDecl(ExternalBusDecl{
		Endpoint: "ext-probe",
		Sends:    []rpc.MessageSendDecl{{Message: "valext:ping"}, {Message: "valtest:alpha"}},
		Receives: []rpc.MessageReceiveDecl{{Message: "valext:pong"}},
	})

	msgVal := BusMessageNameValidator()
	msgOrNone := BusMessageOrNoneValidator()
	pathVal := BusParticipantPathValidator()
	pathOrNone := BusPathOrNoneValidator()

	// Valid registered non-cli messages, externally declared messages, and cli: prefixes.
	for _, valid := range []string{"valtest:alpha", "valtest:zeta", "valext:ping", "valext:pong", "cli:show version"} {
		assert.NoError(t, msgVal.ValidateFn("bus/endpoint/send/message.name", valid), "msgVal should accept %q", valid)
		assert.NoError(t, msgOrNone.ValidateFn("bus/endpoint/send.allow", valid), "msgOrNone should accept %q", valid)
	}
	assert.NoError(t, msgOrNone.ValidateFn("bus/endpoint/send.allow", "none"))

	// Malformed messages are refused naming the offending token and format example.
	err := msgVal.ValidateFn("bus/endpoint/send/message.name", "no-colon")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no-colon")
	assert.Contains(t, err.Error(), "expected namespace:name, e.g.")

	err = msgVal.ValidateFn("bus/endpoint/send/message.name", "none")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "none")

	// Unregistered messages are refused naming both the offending token and valid registered messages.
	for _, tc := range []struct {
		name string
		fn   func(string, any) error
		path string
	}{
		{"BusMessageNameValidator", msgVal.ValidateFn, "bus/endpoint/send/message.name"},
		{"BusMessageOrNoneValidator", msgOrNone.ValidateFn, "bus/endpoint/send.allow"},
	} {
		unregErr := tc.fn(tc.path, "unknownns:nosuch")
		require.Error(t, unregErr, tc.name)
		assert.Contains(t, unregErr.Error(), "unknownns:nosuch", tc.name)
		assert.Contains(t, unregErr.Error(), "(valid:", tc.name)
		assert.Contains(t, unregErr.Error(), "valtest:alpha", tc.name)
		assert.Contains(t, unregErr.Error(), "valext:ping", tc.name)
	}

	// Non-string values are rejected by both message validators.
	assert.Error(t, msgVal.ValidateFn("bus/endpoint/send/message.name", 42))
	assert.Error(t, msgOrNone.ValidateFn("bus/endpoint/send.allow", 42))

	// Valid and invalid bus participant paths.
	assert.NoError(t, pathVal.ValidateFn("bus/participant.path", "/bus/endpoint/collector"))
	assert.Error(t, pathVal.ValidateFn("bus/participant.path", "none"))
	assert.Error(t, pathVal.ValidateFn("bus/participant.path", "relative/path"))
	assert.Error(t, pathVal.ValidateFn("bus/participant.path", "/trailing/"))
	assert.Error(t, pathVal.ValidateFn("bus/participant.path", "/double//slash"))

	assert.NoError(t, pathOrNone.ValidateFn("bus/endpoint/send.to", "none"))
	assert.NoError(t, pathOrNone.ValidateFn("bus/endpoint/send.to", "/bus/endpoint/collector"))
	assert.Error(t, pathOrNone.ValidateFn("bus/endpoint/send.to", "relative/path"))

	// BusMessageNameValidator completions are non-empty, sorted, and include registered + external messages.
	require.NotNil(t, msgVal.CompleteFn)
	msgCompletions := msgVal.CompleteFn()
	require.NotEmpty(t, msgCompletions)
	assert.True(t, slices.IsSorted(msgCompletions), "BusMessageNameValidator completions must be sorted: %v", msgCompletions)
	assert.Contains(t, msgCompletions, "valtest:alpha")
	assert.Contains(t, msgCompletions, "valtest:zeta")
	assert.Contains(t, msgCompletions, "valext:ping")
	assert.Contains(t, msgCompletions, "valext:pong")
	for _, tok := range msgCompletions {
		assert.NoError(t, msgVal.ValidateFn("bus/endpoint/send/message.name", tok), "completion token %q must validate", tok)
	}

	// BusMessageOrNoneValidator completions begin with "none" followed by sorted registered + external messages.
	require.NotNil(t, msgOrNone.CompleteFn)
	orNoneCompletions := msgOrNone.CompleteFn()
	require.NotEmpty(t, orNoneCompletions)
	assert.Equal(t, "none", orNoneCompletions[0])
	assert.True(t, slices.IsSorted(orNoneCompletions[1:]), "BusMessageOrNoneValidator message completions must be sorted: %v", orNoneCompletions[1:])
	assert.Contains(t, orNoneCompletions, "valtest:alpha")
	assert.Contains(t, orNoneCompletions, "valext:ping")
	for _, tok := range orNoneCompletions {
		assert.NoError(t, msgOrNone.ValidateFn("bus/endpoint/send.allow", tok), "completion token %q must validate", tok)
	}

	assert.Contains(t, pathOrNone.CompleteFn(), "none")
	assert.Contains(t, pathVal.CompleteFn(), "/bus/endpoint")
}

// TestAllBusMessageNames verifies allBusMessageNames collects, deduplicates,
// and sorts registered and external bus messages, and that every offered token
// passes both message validators.
//
// VALIDATES: Completion-validation agreement and deduplicated sorted output of
// allBusMessageNames across compiled-in and externally declared bus messages.
// PREVENTS: Duplicate or unsorted completions, or completion tokens rejected
// by BusMessageNameValidator / BusMessageOrNoneValidator.
func TestAllBusMessageNames(t *testing.T) {
	restoreExt := ResetExternalBusDeclsForTest()
	t.Cleanup(restoreExt)

	require.NoError(t, events.RegisterMessage("allmsgtest", "beta"))
	require.NoError(t, events.RegisterMessage("allmsgtest", "alpha"))
	RecordExternalBusDecl(ExternalBusDecl{
		Endpoint: "ext-dedup",
		Sends:    []rpc.MessageSendDecl{{Message: "allmsgtest:alpha"}, {Message: "allmsgext:one"}},
		Receives: []rpc.MessageReceiveDecl{{Message: "allmsgext:one"}, {Message: "allmsgext:two"}},
	})

	names := allBusMessageNames()
	require.NotEmpty(t, names)
	assert.True(t, slices.IsSorted(names), "allBusMessageNames must be sorted: %v", names)

	seen := make(map[string]bool, len(names))
	for _, name := range names {
		assert.False(t, seen[name], "duplicate message name %q in allBusMessageNames", name)
		seen[name] = true
	}
	for _, want := range []string{"allmsgtest:alpha", "allmsgtest:beta", "allmsgext:one", "allmsgext:two"} {
		assert.True(t, seen[want], "missing expected message %q in allBusMessageNames", want)
	}

	msgVal := BusMessageNameValidator()
	msgOrNone := BusMessageOrNoneValidator()
	for _, name := range names {
		assert.NoError(t, msgVal.ValidateFn("bus/endpoint/send/message.name", name), "allBusMessageNames token %q must pass BusMessageNameValidator", name)
		assert.NoError(t, msgOrNone.ValidateFn("bus/endpoint/send.allow", name), "allBusMessageNames token %q must pass BusMessageOrNoneValidator", name)
	}
}

// TestMACAddressValidator_Validate verifies MAC address format validation.
//
// VALIDATES: Valid MAC accepted, invalid formats rejected (AC-9).
// PREVENTS: Invalid MAC addresses silently accepted in interface config.
func TestMACAddressValidator_Validate(t *testing.T) {
	v := MACAddressValidator()

	// Valid MAC addresses.
	assert.NoError(t, v.ValidateFn("interface.ethernet.mac.address", "aa:bb:cc:dd:ee:ff"))
	assert.NoError(t, v.ValidateFn("interface.ethernet.mac.address", "AA:BB:CC:DD:EE:FF"))
	assert.NoError(t, v.ValidateFn("interface.ethernet.mac.address", "00:11:22:33:44:55"))
	assert.NoError(t, v.ValidateFn("interface.ethernet.mac.address", "01:23:45:67:89:aB"))

	// Invalid: wrong separator.
	err := v.ValidateFn("interface.ethernet.mac.address", "aa-bb-cc-dd-ee-ff")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid MAC address")

	// Invalid: too short.
	err = v.ValidateFn("interface.ethernet.mac.address", "aa:bb:cc:dd:ee")
	require.Error(t, err)

	// Invalid: too long.
	err = v.ValidateFn("interface.ethernet.mac.address", "aa:bb:cc:dd:ee:ff:00")
	require.Error(t, err)

	// Invalid: non-hex.
	err = v.ValidateFn("interface.ethernet.mac.address", "gg:hh:ii:jj:kk:ll")
	require.Error(t, err)

	// Invalid: empty.
	err = v.ValidateFn("interface.ethernet.mac.address", "")
	require.Error(t, err)

	// Non-string rejected.
	assert.Error(t, v.ValidateFn("interface.ethernet.mac.address", 42))
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

// VALIDATES: ValidateFetchURL accepts plain HTTP only for the host itself, and
// compares that host after parsing rather than by prefix.
// PREVENTS: the shape this rule carried until 2026-09-03. It tested
// strings.HasPrefix(url, "http://127.0.0.1"), so "http://127.0.0.1.example.com"
// passed as loopback, and that rule also guards a self-update manifest's
// download URL (system.ValidateUpdateCheckURL).
func TestALoopbackPlainHTTPURLIsAcceptedAndALookalikeIsNot(t *testing.T) {
	accepted := []string{
		"https://ftp.ripe.net/pub/stats/ripencc/delegated-ripencc-extended-latest",
		"https://example.com:8443/delegated",
		"http://127.0.0.1/delegated",
		"http://127.0.0.1:8080/delegated",
		"http://localhost:9000/delegated",
		"http://[::1]:9000/delegated",
	}
	for _, rawURL := range accepted {
		assert.NoError(t, ValidateFetchURL(rawURL), "expected %q to be read", rawURL)
	}

	refused := []string{
		"",
		"http://127.0.0.1.example.com/delegated",
		"http://localhost.example.com/delegated",
		"http://127.0.0.1@example.com/delegated",
		"http://ftp.ripe.net/pub/stats/ripencc/delegated-ripencc-extended-latest",
		"ftp://ftp.ripe.net/pub/stats/ripencc/delegated-ripencc-extended-latest",
		"file:///etc/passwd",
		"https://",
	}
	for _, rawURL := range refused {
		assert.Error(t, ValidateFetchURL(rawURL), "expected %q to be refused", rawURL)
	}
}

// VALIDATES: the delegation-source validator refuses a URL the fetch rule
// refuses, and names the leaf it is refusing for.
// PREVENTS: a mirror an operator commits over plain HTTP off the box, which
// would let an unauthenticated answer decide which registry Ze reports as
// holding an AS number.
func TestDelegationSourceValidatorRefusesWhatTheFetchRuleRefuses(t *testing.T) {
	v := DelegationSourceValidator()
	require.NotNil(t, v.ValidateFn)

	require.NoError(t, v.ValidateFn("system/rir/delegation-source/url",
		"https://mirror.example.com/delegated-ripencc-extended-latest"))

	err := v.ValidateFn("system/rir/delegation-source/url",
		"http://mirror.example.com/delegated-ripencc-extended-latest")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delegation source")

	assert.Error(t, v.ValidateFn("system/rir/delegation-source/url", 42))
}
