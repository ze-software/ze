package api

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/attribute"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
	"codeberg.org/thomas-mangin/zebgp/pkg/rib"
)

// TestParseUpdateText_EmptyInput verifies empty args returns empty result.
//
// VALIDATES: Empty args produces empty Groups, no error
// PREVENTS: Panic on nil/empty input.
func TestParseUpdateText_EmptyInput(t *testing.T) {
	result, err := ParseUpdateText([]string{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Groups)
	assert.Empty(t, result.WatchdogName)
}

// TestParseUpdateText_OriginSet verifies origin attribute parsing.
//
// VALIDATES: origin set igp/egp/incomplete stores correct value
// PREVENTS: Origin value misinterpretation.
func TestParseUpdateText_OriginSet(t *testing.T) {
	tests := []struct {
		name   string
		origin string
		want   uint8
	}{
		{"igp", "igp", 0},
		{"egp", "egp", 1},
		{"incomplete", "incomplete", 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseUpdateText([]string{
				"origin", "set", tc.origin,
				"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
			})
			require.NoError(t, err)
			require.Len(t, result.Groups, 1)
			require.NotNil(t, result.Groups[0].Attrs.Origin)
			assert.Equal(t, tc.want, *result.Groups[0].Attrs.Origin)
		})
	}
}

// TestParseUpdateText_MultipleAttrs verifies multiple attrs in sequence.
//
// VALIDATES: Multiple per-attribute sections parsed correctly
// PREVENTS: Only first attribute being parsed.
func TestParseUpdateText_MultipleAttrs(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"origin", "set", "igp",
		"med", "set", "100",
		"local-preference", "set", "200",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	attrs := result.Groups[0].Attrs
	require.NotNil(t, attrs.Origin)
	assert.Equal(t, uint8(0), *attrs.Origin)
	require.NotNil(t, attrs.MED)
	assert.Equal(t, uint32(100), *attrs.MED)
	require.NotNil(t, attrs.LocalPreference)
	assert.Equal(t, uint32(200), *attrs.LocalPreference)
}

// TestParseUpdateText_CommunitySet verifies community parsing.
//
// VALIDATES: Community list parsed in various formats
// PREVENTS: Community parsing failures.
func TestParseUpdateText_CommunitySet(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"community", "set", "[65000:100", "65000:200]",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	comms := result.Groups[0].Attrs.Communities
	require.Len(t, comms, 2)
	assert.Equal(t, uint32(65000<<16|100), comms[0])
	assert.Equal(t, uint32(65000<<16|200), comms[1])
}

// TestParseUpdateText_CommunityAdd verifies community append.
//
// VALIDATES: community add prepends to existing list
// PREVENTS: Community replacement instead of prepend.
func TestParseUpdateText_CommunityAdd(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"community", "set", "[65000:100]",
		"community", "add", "[65000:200]", // prepends → [200, 100]
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	comms := result.Groups[0].Attrs.Communities
	require.Len(t, comms, 2)
	assert.Equal(t, uint32(65000<<16|200), comms[0]) // prepended first
	assert.Equal(t, uint32(65000<<16|100), comms[1])
}

// TestParseUpdateText_CommunityDel verifies community removal.
//
// VALIDATES: community del removes matching values
// PREVENTS: Community deletion failures.
func TestParseUpdateText_CommunityDel(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"community", "set", "[65000:100", "65000:200", "65000:300]",
		"community", "del", "[65000:200]",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	comms := result.Groups[0].Attrs.Communities
	require.Len(t, comms, 2)
	assert.Equal(t, uint32(65000<<16|100), comms[0])
	assert.Equal(t, uint32(65000<<16|300), comms[1])
}

// TestParseUpdateText_CommunityNotFoundDel verifies error on del non-existing.
//
// VALIDATES: community del [value] errors if value not in list
// PREVENTS: Silent ignore of non-existing delete targets.
func TestParseUpdateText_CommunityNotFoundDel(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"community", "set", "[65000:100]",
		"community", "del", "[65000:999]", // 65000:999 not in list
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "65000:999")
	assert.Contains(t, err.Error(), "not found")
}

// TestParseUpdateText_EmptyListOKDel verifies del [] always succeeds.
//
// VALIDATES: community del [] is a no-op (doesn't error)
// PREVENTS: False errors on empty delete list.
func TestParseUpdateText_EmptyListOKDel(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"community", "set", "[65000:100]",
		"community", "del", "[]", // empty list = no-op
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	// Original community preserved
	require.Len(t, result.Groups[0].Attrs.Communities, 1)
	assert.Equal(t, uint32(65000<<16|100), result.Groups[0].Attrs.Communities[0])
}

// TestParseUpdateText_FirstInstanceOnlyDel verifies del removes first instance only.
//
// VALIDATES: community del [X] removes only first X, leaves duplicates
// PREVENTS: Removing all instances of a value.
func TestParseUpdateText_FirstInstanceOnlyDel(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"community", "set", "[65000:100", "65000:200", "65000:100]", // 100 appears twice
		"community", "del", "[65000:100]", // remove first 100 only
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	comms := result.Groups[0].Attrs.Communities
	require.Len(t, comms, 2) // 200 and second 100 remain
	assert.Equal(t, uint32(65000<<16|200), comms[0])
	assert.Equal(t, uint32(65000<<16|100), comms[1]) // second 100 still there
}

// TestParseUpdateText_ThenAddSet verifies set-then-add accumulation.
//
// VALIDATES: set replaces, then add prepends (65000:400 before 65000:300)
// PREVENTS: Wrong accumulation order (append instead of prepend).
func TestParseUpdateText_ThenAddSet(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"community", "set", "[65000:100]",
		"community", "add", "[65000:200]",
		"community", "set", "[65000:300]", // replaces all
		"community", "add", "[65000:400]", // prepends → [400, 300]
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	comms := result.Groups[0].Attrs.Communities
	require.Len(t, comms, 2)
	assert.Equal(t, uint32(65000<<16|400), comms[0]) // prepended first
	assert.Equal(t, uint32(65000<<16|300), comms[1])
}

// TestParseUpdateText_LargeCommunity verifies large community parsing.
//
// VALIDATES: Large community (ASN:value1:value2) parsed correctly
// PREVENTS: Large community format errors.
func TestParseUpdateText_LargeCommunity(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"large-community", "set", "[65000:1:2]",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	lcomms := result.Groups[0].Attrs.LargeCommunities
	require.Len(t, lcomms, 1)
	assert.Equal(t, LargeCommunity{GlobalAdmin: 65000, LocalData1: 1, LocalData2: 2}, lcomms[0])
}

// TestParseUpdateText_ExtendedCommunity verifies extended community parsing.
//
// VALIDATES: Extended community parsed correctly
// PREVENTS: Extended community format errors.
func TestParseUpdateText_ExtendedCommunity(t *testing.T) {
	// Parser supports: origin:ASN:IP, redirect:ASN:target, rate-limit:bps
	result, err := ParseUpdateText([]string{
		"extended-community", "set", "[origin:65000:1.2.3.4]",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	extcomms := result.Groups[0].Attrs.ExtendedCommunities
	require.Len(t, extcomms, 1)
	// Origin: Type 0x00, Subtype 0x03, 2-byte ASN + IPv4
	// 65000 = 0xFDE8 → bytes [0xFD, 0xE8]
	// 1.2.3.4 → bytes [1, 2, 3, 4]
	assert.Equal(t, attribute.ExtendedCommunity{0x00, 0x03, 0xfd, 0xe8, 1, 2, 3, 4}, extcomms[0])
}

// TestParseUpdateText_ScalarErrorAdd verifies add on scalar fails.
//
// VALIDATES: origin add/med/local-pref returns error
// PREVENTS: Silent scalar modification via add.
func TestParseUpdateText_ScalarErrorAdd(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"origin", []string{"origin", "add", "igp", "nlri", "ipv4/unicast", "add", "10.0.0.0/24"}},
		{"med", []string{"med", "add", "100", "nlri", "ipv4/unicast", "add", "10.0.0.0/24"}},
		{"local-preference", []string{"local-preference", "add", "100", "nlri", "ipv4/unicast", "add", "10.0.0.0/24"}},
		// Note: next-hop and next-hop-self tested separately (not valid inside attr)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseUpdateText(tc.args)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrAddOnScalar)
		})
	}
}

// TestParseUpdateText_ScalarDelConditional verifies del with value is conditional for scalars.
//
// VALIDATES: origin del <value> succeeds if current matches, fails otherwise
// PREVENTS: Confusion about scalar conditional deletion semantics.
func TestParseUpdateText_ScalarDelConditional(t *testing.T) {
	// Conditional delete succeeds when value matches
	result, err := ParseUpdateText([]string{
		"origin", "set", "igp",
		"origin", "del", "igp", // Matches current value
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	assert.Nil(t, result.Groups[0].Attrs.Origin) // Should be cleared

	// Conditional delete fails when value doesn't match
	_, err = ParseUpdateText([]string{
		"origin", "set", "igp",
		"origin", "del", "egp", // Doesn't match current value
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "origin del: current value is igp, not egp")

	// Conditional delete fails when no current value
	_, err = ParseUpdateText([]string{
		"origin", "del", "igp", // No current value
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "origin del: current value is nil")
}

// TestParseUpdateText_ScalarDelClearsAttribute verifies del without value clears scalar.
//
// VALIDATES: origin del (no value) clears the attribute
// PREVENTS: Scalar del being a no-op.
func TestParseUpdateText_ScalarDelClearsAttribute(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"origin", "set", "igp",
		"med", "set", "100",
		"origin", "del", // del without value - clears origin
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	// Origin should be cleared (nil)
	assert.Nil(t, result.Groups[0].Attrs.Origin, "origin should be cleared by del")
	// MED should still be set
	require.NotNil(t, result.Groups[0].Attrs.MED)
	assert.Equal(t, uint32(100), *result.Groups[0].Attrs.MED)
}

// TestParseUpdateText_ASPathAdd verifies add on as-path prepends.
//
// VALIDATES: as-path add prepends ASNs to existing path
// PREVENTS: AS-PATH prepend not working.
func TestParseUpdateText_ASPathAdd(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"as-path", "set", "[65001", "65002]",
		"as-path", "add", "[65000]", // prepends → [65000, 65001, 65002]
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	asPath := result.Groups[0].Attrs.ASPath
	require.Len(t, asPath, 3)
	assert.Equal(t, uint32(65000), asPath[0]) // prepended
	assert.Equal(t, uint32(65001), asPath[1])
	assert.Equal(t, uint32(65002), asPath[2])
}

// TestParseUpdateText_ASPathDelValue verifies del on as-path removes specific ASN.
//
// VALIDATES: as-path del [ASN] removes first occurrence
// PREVENTS: AS-PATH del not removing ASN.
func TestParseUpdateText_ASPathDelValue(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"as-path", "set", "[65000", "65001", "65002]",
		"as-path", "del", "[65001]", // removes 65001 → [65000, 65002]
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	asPath := result.Groups[0].Attrs.ASPath
	require.Len(t, asPath, 2)
	assert.Equal(t, uint32(65000), asPath[0])
	assert.Equal(t, uint32(65002), asPath[1])
}

// TestParseUpdateText_ASPathDelClear verifies del without value clears as-path.
//
// VALIDATES: as-path del (no value) clears entire path
// PREVENTS: AS-PATH del not clearing.
func TestParseUpdateText_ASPathDelClear(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"as-path", "set", "[65000", "65001]",
		"as-path", "del", // clears entire path
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	assert.Nil(t, result.Groups[0].Attrs.ASPath)
}

// TestParseUpdateText_ASPathDelNotFound verifies error when ASN not in path.
//
// VALIDATES: as-path del [ASN] errors if ASN not present
// PREVENTS: Silent ignore of non-existent ASN deletion.
func TestParseUpdateText_ASPathDelNotFound(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"as-path", "set", "[65000", "65001]",
		"as-path", "del", "[65999]", // 65999 not in path
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "65999")
	assert.Contains(t, err.Error(), "not found")
}

// TestParseUpdateText_NLRISectionBasic verifies basic NLRI add.
//
// VALIDATES: nlri <family> add <prefix> creates group
// PREVENTS: Basic NLRI parsing failures.
func TestParseUpdateText_NLRISectionBasic(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	g := result.Groups[0]
	assert.Equal(t, nlri.IPv4Unicast, g.Family)
	require.Len(t, g.Announce, 1)
	assert.Equal(t, "10.0.0.0/24", g.Announce[0].String())
	assert.Empty(t, g.Withdraw)
}

// TestParseUpdateText_NLRIMultiplePrefixes verifies multiple prefixes.
//
// VALIDATES: Multiple prefixes in single nlri section
// PREVENTS: Only first prefix being parsed.
func TestParseUpdateText_NLRIMultiplePrefixes(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24", "10.0.1.0/24", "10.0.2.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Announce, 3)
}

// TestParseUpdateText_NLRIMixedAddDel verifies mixed add/del.
//
// VALIDATES: add X del Y in same section produces both lists
// PREVENTS: add/del mode confusion.
func TestParseUpdateText_NLRIMixedAddDel(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24", "del", "10.0.1.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	g := result.Groups[0]
	require.Len(t, g.Announce, 1)
	require.Len(t, g.Withdraw, 1)
	assert.Equal(t, "10.0.0.0/24", g.Announce[0].String())
	assert.Equal(t, "10.0.1.0/24", g.Withdraw[0].String())
}

// TestParseUpdateText_NLRIWithdrawOnly verifies del-only section.
//
// VALIDATES: nlri <family> del <prefix> works without add
// PREVENTS: Requiring add before del.
func TestParseUpdateText_NLRIWithdrawOnly(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv4/unicast", "del", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	g := result.Groups[0]
	assert.Empty(t, g.Announce)
	require.Len(t, g.Withdraw, 1)
	assert.Equal(t, "10.0.0.0/24", g.Withdraw[0].String())
}

// TestParseUpdateText_NLRIMultipleAddDel verifies multiple add/del switches.
//
// VALIDATES: add X Y del Z add W produces correct lists
// PREVENTS: Mode switching errors.
func TestParseUpdateText_NLRIMultipleAddDel(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24", "10.0.1.0/24",
		"del", "10.0.2.0/24",
		"add", "10.0.3.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	g := result.Groups[0]
	require.Len(t, g.Announce, 3) // 10.0.0.0, 10.0.1.0, 10.0.3.0
	require.Len(t, g.Withdraw, 1) // 10.0.2.0
}

// TestParseUpdateText_NLRIEmptyError verifies empty section fails.
//
// VALIDATES: nlri <family> with no prefixes returns error
// PREVENTS: Empty groups in result.
func TestParseUpdateText_NLRIEmptyError(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"nlri", "ipv4/unicast",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEmptyNLRISection)
}

// TestParseUpdateText_NLRIMissingAddDel verifies missing add/del fails.
//
// VALIDATES: Prefix without add/del mode returns error
// PREVENTS: Silent default behavior.
func TestParseUpdateText_NLRIMissingAddDel(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"nlri", "ipv4/unicast", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingAddDel)
}

// TestParseUpdateText_AttrAndNLRI verifies combined attr + nlri.
//
// VALIDATES: Attributes applied to NLRI group
// PREVENTS: Attribute/NLRI disconnection.
func TestParseUpdateText_AttrAndNLRI(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"origin", "set", "igp",
		"nhop", "set", "192.0.2.1",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	g := result.Groups[0]
	assert.True(t, g.NextHop.IsExplicit())
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), g.NextHop.Addr)
	require.NotNil(t, g.Attrs.Origin)
	assert.Equal(t, uint8(0), *g.Attrs.Origin)
	require.Len(t, g.Announce, 1)
}

// TestParseUpdateText_MultipleGroups verifies snapshot deep copy.
//
// VALIDATES: Each group has independent attribute snapshot
// PREVENTS: Shared slice mutation between groups.
func TestParseUpdateText_MultipleGroups(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"community", "set", "[65000:100]",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
		"community", "add", "[65000:200]", // prepends → [200, 100]
		"nlri", "ipv4/unicast", "add", "10.0.1.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 2)

	// First group: only 65000:100
	require.Len(t, result.Groups[0].Attrs.Communities, 1)
	assert.Equal(t, uint32(65000<<16|100), result.Groups[0].Attrs.Communities[0])

	// Second group: 65000:200 prepended + 65000:100
	require.Len(t, result.Groups[1].Attrs.Communities, 2)
	assert.Equal(t, uint32(65000<<16|200), result.Groups[1].Attrs.Communities[0]) // prepended
	assert.Equal(t, uint32(65000<<16|100), result.Groups[1].Attrs.Communities[1])
}

// TestParseUpdateText_IPv6 verifies IPv6 support.
//
// VALIDATES: IPv6 prefixes parsed correctly
// PREVENTS: IPv6 parsing failures.
func TestParseUpdateText_IPv6(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv6/unicast", "add", "2001:db8::/32",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	g := result.Groups[0]
	assert.Equal(t, nlri.IPv6Unicast, g.Family)
	require.Len(t, g.Announce, 1)
	assert.Equal(t, "2001:db8::/32", g.Announce[0].String())
}

// TestParseUpdateText_FamilyMismatch verifies family/prefix validation.
//
// VALIDATES: IPv4 prefix in ipv6/unicast returns error
// PREVENTS: Family/prefix mismatches.
func TestParseUpdateText_FamilyMismatch(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"nlri", "ipv6/unicast", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFamilyMismatch)
}

// TestParseUpdateText_UnknownAttribute verifies unknown attr fails with valid list.
//
// VALIDATES: Unknown attribute keyword returns error listing valid options
// PREVENTS: Silent ignore of typos.
func TestParseUpdateText_UnknownAttribute(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"attr", "set", "unknown-attr", "value",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown-attr")
	assert.Contains(t, err.Error(), "valid:")
}

// TestParseUpdateText_UnsupportedFamily verifies unsupported family fails.
//
// VALIDATES: Unsupported family returns error
// PREVENTS: Silent ignore of unsupported families.
func TestParseUpdateText_UnsupportedFamily(t *testing.T) {
	// FlowSpec is a valid family but not supported in text mode
	_, err := ParseUpdateText([]string{
		"nlri", "ipv4/flowspec", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFamilyNotSupported)
}

// TestParseUpdateText_InvalidFamilyString verifies invalid family fails.
//
// VALIDATES: Invalid family string returns error
// PREVENTS: Panic on invalid family.
func TestParseUpdateText_InvalidFamilyString(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"nlri", "not-a-family", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidFamily)
}

// TestParseUpdateText_InvalidPrefix verifies invalid prefix fails.
//
// VALIDATES: Invalid prefix format returns error
// PREVENTS: Panic on invalid prefix.
func TestParseUpdateText_InvalidPrefix(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"nlri", "ipv4/unicast", "add", "not-a-prefix",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidPrefix)
}

// TestParseUpdateText_MissingPrefixAfterAdd verifies add without prefix fails.
//
// VALIDATES: nlri <family> add (no prefix) returns error
// PREVENTS: Empty announce list.
func TestParseUpdateText_MissingPrefixAfterAdd(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"nlri", "ipv4/unicast", "add",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEmptyNLRISection)
}

// TestParseUpdateText_Watchdog verifies watchdog inside nlri section.
//
// VALIDATES: nlri ... add ... watchdog set <name> stored in group
// PREVENTS: Watchdog name loss.
func TestParseUpdateText_Watchdog(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24", "watchdog", "set", "my-watchdog",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, "my-watchdog", result.Groups[0].WatchdogName)
	assert.Equal(t, "my-watchdog", result.WatchdogName) // Also set globally for compat
}

// TestParseUpdateText_WatchdogLegacy verifies legacy standalone watchdog still works.
//
// VALIDATES: watchdog <name> (standalone) still works for backward compat
// PREVENTS: Breaking existing scripts.
func TestParseUpdateText_WatchdogLegacy(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"watchdog", "my-watchdog",
	})
	require.NoError(t, err)
	assert.Equal(t, "my-watchdog", result.WatchdogName)
	assert.Empty(t, result.Groups)
}

// TestParseUpdateText_SpecExample verifies full chained example from spec.
//
// VALIDATES: Complex multi-section command parses correctly
// PREVENTS: Inter-section interaction bugs.
func TestParseUpdateText_SpecExample(t *testing.T) {
	// Example: set attrs, add ipv4 routes, modify attrs, add ipv6 routes with watchdog
	result, err := ParseUpdateText([]string{
		"origin", "set", "igp",
		"nhop", "set", "192.0.2.1",
		"community", "set", "[65000:100]",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24", "10.0.1.0/24", "del", "10.0.2.0/24",
		"community", "add", "[65000:200]",
		"nhop", "set", "2001:db8::1",
		"nlri", "ipv6/unicast", "add", "2001:db8:1::/48", "watchdog", "set", "test-pool",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 2)

	// First group: ipv4/unicast with original attrs
	g1 := result.Groups[0]
	assert.Equal(t, nlri.IPv4Unicast, g1.Family)
	assert.True(t, g1.NextHop.IsExplicit())
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), g1.NextHop.Addr)
	require.Len(t, g1.Attrs.Communities, 1)
	require.Len(t, g1.Announce, 2)
	require.Len(t, g1.Withdraw, 1)
	assert.Empty(t, g1.WatchdogName) // No watchdog on first group

	// Second group: ipv6/unicast with modified attrs and watchdog
	g2 := result.Groups[1]
	assert.Equal(t, nlri.IPv6Unicast, g2.Family)
	assert.True(t, g2.NextHop.IsExplicit())
	assert.Equal(t, netip.MustParseAddr("2001:db8::1"), g2.NextHop.Addr)
	require.Len(t, g2.Attrs.Communities, 2) // 65000:100 + 65000:200
	require.Len(t, g2.Announce, 1)
	assert.Empty(t, g2.Withdraw)
	assert.Equal(t, "test-pool", g2.WatchdogName)

	assert.Equal(t, "test-pool", result.WatchdogName) // Global for compat
}

// TestParsedAttrs_Snapshot_DeepCopy verifies snapshot creates independent copies.
//
// VALIDATES: Modifying original after snapshot doesn't affect copy
// PREVENTS: Shared slice bugs.
func TestParsedAttrs_Snapshot_DeepCopy(t *testing.T) {
	orig := parsedAttrs{
		PathAttributes: PathAttributes{
			Communities: []uint32{1, 2, 3},
		},
	}

	pa, _, _ := orig.snapshot()

	// Modify original
	orig.Communities = append(orig.Communities, 4)

	// Snapshot should be unaffected
	assert.Len(t, pa.Communities, 3)
}

// TestParsedAttrs_Snapshot_DeepCopyPointers verifies pointer fields are deep copied.
//
// VALIDATES: Pointer fields are independent after snapshot.
// PREVENTS: Shared pointer mutation between groups.
func TestParsedAttrs_Snapshot_DeepCopyPointers(t *testing.T) {
	origin := uint8(0)
	orig := parsedAttrs{
		PathAttributes: PathAttributes{
			Origin: &origin,
		},
	}

	pa, _, _ := orig.snapshot()

	// Modify original pointer value
	*orig.Origin = 2

	// Snapshot should be unaffected
	require.NotNil(t, pa.Origin)
	assert.Equal(t, uint8(0), *pa.Origin)
}

// TestParseUpdateText_EmptyAttrSection verifies empty attr section is valid.
//
// VALIDATES: with set no attrs before nlri is accepted.
// PREVENTS: False error on valid syntax.
func TestParseUpdateText_EmptyAttrSection(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"attr", "set",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
}

// TestParseUpdateText_MultipleWatchdog verifies per-group watchdog.
//
// VALIDATES: Each nlri section can have its own watchdog.
// PREVENTS: Watchdog bleeding across sections.
func TestParseUpdateText_MultipleWatchdog(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24", "watchdog", "set", "first",
		"nlri", "ipv4/unicast", "add", "10.0.1.0/24", "watchdog", "set", "second",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 2)
	assert.Equal(t, "first", result.Groups[0].WatchdogName)
	assert.Equal(t, "second", result.Groups[1].WatchdogName)
	assert.Equal(t, "second", result.WatchdogName) // Global is last one for compat
}

// TestParseUpdateText_IPv6InIPv4Family verifies reverse family mismatch.
//
// VALIDATES: IPv6 prefix in ipv4/unicast returns error.
// PREVENTS: Wrong address family accepted.
func TestParseUpdateText_IPv6InIPv4Family(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"nlri", "ipv4/unicast", "add", "2001:db8::/32",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFamilyMismatch)
}

// TestParseUpdateText_MulticastFamily verifies multicast family support.
//
// VALIDATES: ipv4/multicast and ipv6/multicast are supported.
// PREVENTS: Multicast treated as unsupported.
func TestParseUpdateText_MulticastFamily(t *testing.T) {
	tests := []struct {
		name   string
		family string
		prefix string
		want   nlri.Family
	}{
		{"ipv4/multicast", "ipv4/multicast", "224.0.0.0/4", nlri.IPv4Multicast},
		{"ipv6/multicast", "ipv6/multicast", "ff00::/8", nlri.IPv6Multicast},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseUpdateText([]string{
				"nlri", tc.family, "add", tc.prefix,
			})
			require.NoError(t, err)
			require.Len(t, result.Groups, 1)
			assert.Equal(t, tc.want, result.Groups[0].Family)
		})
	}
}

// TestParseUpdateText_FamilyCaseSensitive verifies family is case-sensitive.
//
// VALIDATES: Uppercase family string fails.
// PREVENTS: Case-insensitive matching confusion.
func TestParseUpdateText_FamilyCaseSensitive(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"nlri", "IPV4/UNICAST", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidFamily)
}

// TestParseUpdateText_OnlySet verifies alone set returns empty result.
//
// VALIDATES: Standalone without set nlri returns empty groups.
// PREVENTS: Error on valid partial command.
func TestParseUpdateText_OnlySet(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"origin", "set", "igp",
	})
	require.NoError(t, err)
	assert.Empty(t, result.Groups)
	assert.Empty(t, result.WatchdogName)
}

// TestParseUpdateText_WatchdogBeforeNLRI verifies watchdog can appear before nlri.
//
// VALIDATES: Order of sections is flexible.
// PREVENTS: Requiring specific section order.
func TestParseUpdateText_WatchdogBeforeNLRI(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"watchdog", "my-pool",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	assert.Equal(t, "my-pool", result.WatchdogName)
	require.Len(t, result.Groups, 1)
}

// TestParseUpdateText_AttrBetweenNLRISections verifies attrs between nlri sections.
//
// VALIDATES: Interleaved attr/nlri produces correct snapshots.
// PREVENTS: Attribute leakage between groups.
func TestParseUpdateText_AttrBetweenNLRISections(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"origin", "set", "igp",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
		"origin", "set", "egp",
		"nlri", "ipv4/unicast", "add", "10.0.1.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 2)

	// First group: origin=IGP (0)
	require.NotNil(t, result.Groups[0].Attrs.Origin)
	assert.Equal(t, uint8(0), *result.Groups[0].Attrs.Origin)

	// Second group: origin=EGP (1)
	require.NotNil(t, result.Groups[1].Attrs.Origin)
	assert.Equal(t, uint8(1), *result.Groups[1].Attrs.Origin)
}

// =============================================================================
// Handler Integration Tests (TDD for handleUpdateText)
// =============================================================================

// mockReactorBatch implements ReactorInterface for batch handler testing.
// Used by handler integration tests.
type mockReactorBatch struct {
	announceError      error
	withdrawError      error
	announceCalls      []NLRIBatch
	withdrawCalls      []NLRIBatch
	peerSelector       string
	noPeersMatching    bool
	noPeersAccepted    bool // Simulates family not negotiated
	noPeersAcceptedFor nlri.Family
}

func (m *mockReactorBatch) AnnounceNLRIBatch(peerSelector string, batch NLRIBatch) error {
	if m.noPeersMatching {
		return ErrNoPeersMatch
	}
	if m.noPeersAccepted || (m.noPeersAcceptedFor != nlri.Family{} && m.noPeersAcceptedFor == batch.Family) {
		return ErrNoPeersAcceptedFamily
	}
	m.peerSelector = peerSelector
	m.announceCalls = append(m.announceCalls, batch)
	return m.announceError
}

func (m *mockReactorBatch) WithdrawNLRIBatch(peerSelector string, batch NLRIBatch) error {
	if m.noPeersMatching {
		return ErrNoPeersMatch
	}
	if m.noPeersAccepted || (m.noPeersAcceptedFor != nlri.Family{} && m.noPeersAcceptedFor == batch.Family) {
		return ErrNoPeersAcceptedFamily
	}
	m.peerSelector = peerSelector
	m.withdrawCalls = append(m.withdrawCalls, batch)
	return m.withdrawError
}

// Stub implementations for other ReactorInterface methods.
func (m *mockReactorBatch) Peers() []PeerInfo                                { return nil }
func (m *mockReactorBatch) Stats() ReactorStats                              { return ReactorStats{} }
func (m *mockReactorBatch) Stop()                                            {}
func (m *mockReactorBatch) Reload() error                                    { return nil }
func (m *mockReactorBatch) AnnounceRoute(_ string, _ RouteSpec) error        { return nil }
func (m *mockReactorBatch) WithdrawRoute(_ string, _ netip.Prefix) error     { return nil }
func (m *mockReactorBatch) AnnounceFlowSpec(_ string, _ FlowSpecRoute) error { return nil }
func (m *mockReactorBatch) WithdrawFlowSpec(_ string, _ FlowSpecRoute) error { return nil }
func (m *mockReactorBatch) AnnounceVPLS(_ string, _ VPLSRoute) error         { return nil }
func (m *mockReactorBatch) WithdrawVPLS(_ string, _ VPLSRoute) error         { return nil }
func (m *mockReactorBatch) AnnounceL2VPN(_ string, _ L2VPNRoute) error       { return nil }
func (m *mockReactorBatch) WithdrawL2VPN(_ string, _ L2VPNRoute) error       { return nil }
func (m *mockReactorBatch) AnnounceL3VPN(_ string, _ L3VPNRoute) error       { return nil }
func (m *mockReactorBatch) WithdrawL3VPN(_ string, _ L3VPNRoute) error       { return nil }
func (m *mockReactorBatch) AnnounceLabeledUnicast(_ string, _ LabeledUnicastRoute) error {
	return nil
}
func (m *mockReactorBatch) WithdrawLabeledUnicast(_ string, _ LabeledUnicastRoute) error {
	return nil
}
func (m *mockReactorBatch) AnnounceMUPRoute(_ string, _ MUPRouteSpec) error { return nil }
func (m *mockReactorBatch) WithdrawMUPRoute(_ string, _ MUPRouteSpec) error { return nil }
func (m *mockReactorBatch) TeardownPeer(_ netip.Addr, _ uint8) error        { return nil }
func (m *mockReactorBatch) AnnounceEOR(_ string, _ uint16, _ uint8) error   { return nil }
func (m *mockReactorBatch) RIBInRoutes(_ string) []RIBRoute                 { return nil }
func (m *mockReactorBatch) RIBOutRoutes() []RIBRoute                        { return nil }
func (m *mockReactorBatch) RIBStats() RIBStatsInfo                          { return RIBStatsInfo{} }
func (m *mockReactorBatch) BeginTransaction(_, _ string) error              { return nil }
func (m *mockReactorBatch) CommitTransaction(_ string) (TransactionResult, error) {
	return TransactionResult{}, nil
}
func (m *mockReactorBatch) CommitTransactionWithLabel(_, _ string) (TransactionResult, error) {
	return TransactionResult{}, nil
}
func (m *mockReactorBatch) RollbackTransaction(_ string) (TransactionResult, error) {
	return TransactionResult{}, nil
}
func (m *mockReactorBatch) InTransaction(_ string) bool   { return false }
func (m *mockReactorBatch) TransactionID(_ string) string { return "" }
func (m *mockReactorBatch) SendRoutes(_ string, _ []*rib.Route, _ []nlri.NLRI, _ bool) (TransactionResult, error) {
	return TransactionResult{}, nil
}
func (m *mockReactorBatch) AnnounceWatchdog(_, _ string) error               { return nil }
func (m *mockReactorBatch) WithdrawWatchdog(_, _ string) error               { return nil }
func (m *mockReactorBatch) AddWatchdogRoute(_ RouteSpec, _ string) error     { return nil }
func (m *mockReactorBatch) RemoveWatchdogRoute(_, _ string) error            { return nil }
func (m *mockReactorBatch) ClearRIBIn() int                                  { return 0 }
func (m *mockReactorBatch) ClearRIBOut() int                                 { return 0 }
func (m *mockReactorBatch) FlushRIBOut() int                                 { return 0 }
func (m *mockReactorBatch) GetPeerAPIBindings(_ netip.Addr) []PeerAPIBinding { return nil }
func (m *mockReactorBatch) ForwardUpdate(_ *Selector, _ uint64) error        { return nil }
func (m *mockReactorBatch) DeleteUpdate(_ uint64) error                      { return nil }
func (m *mockReactorBatch) SignalAPIReady()                                  {}
func (m *mockReactorBatch) SignalPeerAPIReady(_ string)                      {}
func (m *mockReactorBatch) SendRawMessage(_ netip.Addr, _ uint8, _ []byte) error {
	return nil
}

// TestHandleUpdateText_SimpleAnnounce verifies single route announcement.
//
// VALIDATES: Single NLRI announced via reactor batch method.
// PREVENTS: Handler not calling reactor.
func TestHandleUpdateText_SimpleAnnounce(t *testing.T) {
	reactor := &mockReactorBatch{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "192.0.2.1",
	}

	args := []string{
		"origin", "set", "igp",
		"nhop", "set", "10.0.0.1",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	}

	resp, err := handleUpdateText(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	// Verify reactor was called
	require.Len(t, reactor.announceCalls, 1)
	assert.Equal(t, "192.0.2.1", reactor.peerSelector)
	assert.Equal(t, nlri.IPv4Unicast, reactor.announceCalls[0].Family)
	assert.Len(t, reactor.announceCalls[0].NLRIs, 1)
}

// TestHandleUpdateText_MultipleRoutes verifies multiple NLRIs in one group.
//
// VALIDATES: Multiple NLRIs batched in single reactor call.
// PREVENTS: Separate calls per NLRI.
func TestHandleUpdateText_MultipleRoutes(t *testing.T) {
	reactor := &mockReactorBatch{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "*",
	}

	args := []string{
		"nhop", "set", "10.0.0.1",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24", "10.0.1.0/24", "10.0.2.0/24",
	}

	resp, err := handleUpdateText(ctx, args)
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	// All NLRIs in single batch
	require.Len(t, reactor.announceCalls, 1)
	assert.Len(t, reactor.announceCalls[0].NLRIs, 3)
}

// TestHandleUpdateText_MixedAnnounceWithdraw verifies add and del in same call.
//
// VALIDATES: Announce and withdraw in same group produce separate reactor calls.
// PREVENTS: Missing withdraw call.
func TestHandleUpdateText_MixedAnnounceWithdraw(t *testing.T) {
	reactor := &mockReactorBatch{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "*",
	}

	args := []string{
		"nhop", "set", "10.0.0.1",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24", "del", "10.0.1.0/24",
	}

	resp, err := handleUpdateText(ctx, args)
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	// One announce call, one withdraw call
	require.Len(t, reactor.announceCalls, 1)
	require.Len(t, reactor.withdrawCalls, 1)
	assert.Len(t, reactor.announceCalls[0].NLRIs, 1)
	assert.Len(t, reactor.withdrawCalls[0].NLRIs, 1)
}

// TestHandleUpdateText_MultipleGroups verifies different attrs per group.
//
// VALIDATES: Each NLRI section produces separate reactor call with correct attrs.
// PREVENTS: Attribute bleeding between groups.
func TestHandleUpdateText_MultipleGroups(t *testing.T) {
	reactor := &mockReactorBatch{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "*",
	}

	args := []string{
		"nhop", "set", "10.0.0.1",
		"community", "set", "[65000:100]",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
		"community", "add", "[65000:200]",
		"nlri", "ipv4/unicast", "add", "10.0.1.0/24",
	}

	resp, err := handleUpdateText(ctx, args)
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	// Two groups = two announce calls
	require.Len(t, reactor.announceCalls, 2)

	// First group: 1 community
	assert.Len(t, reactor.announceCalls[0].Attrs.Communities, 1)

	// Second group: 2 communities
	assert.Len(t, reactor.announceCalls[1].Attrs.Communities, 2)
}

// TestHandleUpdateText_WithdrawUnicast verifies unicast withdrawal batch.
//
// VALIDATES: Withdraw-only NLRI section calls withdraw method.
// PREVENTS: Withdraw interpreted as announce.
func TestHandleUpdateText_WithdrawUnicast(t *testing.T) {
	reactor := &mockReactorBatch{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "*",
	}

	args := []string{
		"nlri", "ipv4/unicast", "del", "10.0.0.0/24", "10.0.1.0/24",
	}

	resp, err := handleUpdateText(ctx, args)
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	// No announce, one withdraw
	assert.Empty(t, reactor.announceCalls)
	require.Len(t, reactor.withdrawCalls, 1)
	assert.Len(t, reactor.withdrawCalls[0].NLRIs, 2)
}

// TestHandleUpdateText_ParseError verifies invalid input returns error.
//
// VALIDATES: Parse errors propagate to response.
// PREVENTS: Silent failure on bad input.
func TestHandleUpdateText_ParseError(t *testing.T) {
	reactor := &mockReactorBatch{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "*",
	}

	args := []string{
		"nlri", "invalid-family", "add", "10.0.0.0/24",
	}

	resp, err := handleUpdateText(ctx, args)
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
}

// TestHandleUpdateText_PeerNotFound verifies reactor returns no peers error.
//
// VALIDATES: No-peers-match error propagates.
// PREVENTS: Silent success when no peers match.
func TestHandleUpdateText_PeerNotFound(t *testing.T) {
	reactor := &mockReactorBatch{noPeersMatching: true}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "192.0.2.99",
	}

	args := []string{
		"nhop", "set", "10.0.0.1",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	}

	resp, err := handleUpdateText(ctx, args)
	require.Error(t, err)
	assert.Equal(t, "error", resp.Status)
}

// TestHandleUpdateText_WatchdogDeferred verifies watchdog returns error (deferred).
//
// VALIDATES: Watchdog feature returns "not implemented" error.
// PREVENTS: Silent ignore of watchdog.
func TestHandleUpdateText_WatchdogDeferred(t *testing.T) {
	reactor := &mockReactorBatch{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "*",
	}

	args := []string{
		"nhop", "set", "10.0.0.1",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
		"watchdog", "my-pool",
	}

	resp, err := handleUpdateText(ctx, args)
	require.Error(t, err)
	assert.Equal(t, "error", resp.Status)
	dataStr, ok := resp.Data.(string)
	require.True(t, ok, "response Data should be a string")
	assert.Contains(t, dataStr, "watchdog")
}

// TestHandleUpdateText_EmptyResult verifies empty groups returns warning.
//
// VALIDATES: Empty result produces warning status.
// PREVENTS: Silent success with no routes.
func TestHandleUpdateText_EmptyResult(t *testing.T) {
	reactor := &mockReactorBatch{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "*",
	}

	// Just nhop set, no nlri section
	args := []string{
		"nhop", "set", "10.0.0.1",
	}

	resp, err := handleUpdateText(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "warning", resp.Status)
}

// TestHandleUpdateText_IPv6Announce verifies IPv6 unicast announcement.
//
// VALIDATES: IPv6 family handled correctly.
// PREVENTS: IPv6 parsing or dispatch failures.
func TestHandleUpdateText_IPv6Announce(t *testing.T) {
	reactor := &mockReactorBatch{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "*",
	}

	args := []string{
		"nhop", "set", "2001:db8::1",
		"nlri", "ipv6/unicast", "add", "2001:db8:1::/48",
	}

	resp, err := handleUpdateText(ctx, args)
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	require.Len(t, reactor.announceCalls, 1)
	assert.Equal(t, nlri.IPv6Unicast, reactor.announceCalls[0].Family)
}

// TestHandleUpdateText_NextHopSelf verifies nhop set self flag passed to batch.
//
// VALIDATES: NextHopSelf flag propagated to reactor.
// PREVENTS: Flag loss in handler.
func TestHandleUpdateText_NextHopSelf(t *testing.T) {
	reactor := &mockReactorBatch{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "*",
	}

	args := []string{
		"nhop", "set", "self",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	}

	resp, err := handleUpdateText(ctx, args)
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	require.Len(t, reactor.announceCalls, 1)
	assert.True(t, reactor.announceCalls[0].NextHop.IsSelf())
}

// TestHandleUpdateText_FamilyNotAccepted verifies warning when no peers accept family.
//
// VALIDATES: Warning response when all peers skip due to family.
// PREVENTS: Silent success when nothing was sent.
func TestHandleUpdateText_FamilyNotAccepted(t *testing.T) {
	reactor := &mockReactorBatch{noPeersAccepted: true}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "*",
	}

	args := []string{
		"nhop", "set", "10.0.0.1",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	}

	resp, err := handleUpdateText(ctx, args)
	require.NoError(t, err) // Warning is not an error at handler level
	assert.Equal(t, "warning", resp.Status)
	assert.Contains(t, resp.Data, "no peers have family negotiated")
}

// TestHandleUpdateText_PartialFamilyAccepted verifies mixed success/warning.
//
// VALIDATES: Success with warnings when some families not accepted.
// PREVENTS: All-or-nothing behavior.
func TestHandleUpdateText_PartialFamilyAccepted(t *testing.T) {
	// Only IPv6 is not accepted
	reactor := &mockReactorBatch{noPeersAcceptedFor: nlri.IPv6Unicast}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "*",
	}

	// Use separate nhop sections with correct next-hops per family
	args := []string{
		"nhop", "set", "10.0.0.1",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
		"nhop", "set", "2001:db8::1",
		"nlri", "ipv6/unicast", "add", "2001:db8:1::/48",
	}

	resp, err := handleUpdateText(ctx, args)
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	// Should have IPv4 announced, IPv6 warning
	respData, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 1, respData["announced"])
	assert.NotNil(t, respData["warnings"])
}

// TestHandleUpdate_TextSubcommand verifies update text routing.
//
// VALIDATES: "update text" dispatches to handleUpdateText.
// PREVENTS: Wrong subcommand handler.
func TestHandleUpdate_TextSubcommand(t *testing.T) {
	reactor := &mockReactorBatch{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "*",
	}

	args := []string{
		"text",
		"nhop", "set", "10.0.0.1",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	}

	resp, err := handleUpdate(ctx, args)
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	require.Len(t, reactor.announceCalls, 1)
}

// TestHandleUpdate_UnknownEncoding verifies unknown encoding returns error.
//
// VALIDATES: Unsupported encodings fail with clear error.
// PREVENTS: Silent failure or panic.
func TestHandleUpdate_UnknownEncoding(t *testing.T) {
	reactor := &mockReactorBatch{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "*",
	}

	args := []string{"unknown", "some", "args"}

	_, err := handleUpdate(ctx, args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown encoding")
}

// =============================================================================
// Phase 1: nhop and path-information tests
// =============================================================================

// TestParseUpdateText_NhopSet verifies nhop set <addr> syntax.
//
// VALIDATES: nhop set <addr> stores next-hop as explicit
// PREVENTS: Missing nhop keyword support
func TestParseUpdateText_NhopSet(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.True(t, result.Groups[0].NextHop.IsExplicit())
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), result.Groups[0].NextHop.Addr)
}

// TestParseUpdateText_NhopSetSelf verifies nhop set self syntax.
//
// VALIDATES: nhop set self stores next-hop as self policy
// PREVENTS: Missing self keyword support
func TestParseUpdateText_NhopSetSelf(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "self",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.True(t, result.Groups[0].NextHop.IsSelf())
}

// TestParseUpdateText_NhopDel verifies nhop del syntax.
//
// VALIDATES: nhop del unsets next-hop for subsequent nlri
// PREVENTS: Missing nhop del support
func TestParseUpdateText_NhopDel(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
		"nhop", "del",
		"nlri", "ipv4/unicast", "add", "10.0.1.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 2)

	// First group: has next-hop
	assert.True(t, result.Groups[0].NextHop.IsExplicit())

	// Second group: next-hop cleared
	assert.False(t, result.Groups[1].NextHop.IsExplicit())
	assert.False(t, result.Groups[1].NextHop.IsSelf())
}

// TestParseUpdateText_NhopDelConditional verifies nhop del with value is conditional.
//
// VALIDATES: nhop del <value> succeeds if matches, fails otherwise
// PREVENTS: Wrong next-hop being deleted
func TestParseUpdateText_NhopDelConditional(t *testing.T) {
	// Conditional delete succeeds when value matches
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"nhop", "del", "192.0.2.1", // Matches
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	assert.False(t, result.Groups[0].NextHop.IsExplicit()) // Cleared

	// Conditional delete fails when value doesn't match
	_, err = ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"nhop", "del", "192.0.2.99", // Doesn't match
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nhop del: current value is 192.0.2.1, not 192.0.2.99")
}

// TestParseUpdateText_NhopPerFamily verifies nhop accumulates correctly.
//
// VALIDATES: nhop changes affect only subsequent nlri sections
// PREVENTS: nhop applying retroactively
func TestParseUpdateText_NhopPerFamily(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
		"nhop", "set", "2001:db8::1",
		"nlri", "ipv6/unicast", "add", "2001:db8::/32",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 2)

	// IPv4 group: uses first nhop
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), result.Groups[0].NextHop.Addr)

	// IPv6 group: uses second nhop
	assert.Equal(t, netip.MustParseAddr("2001:db8::1"), result.Groups[1].NextHop.Addr)
}

// TestParseUpdateText_PathInfo verifies path-information as accumulator.
//
// VALIDATES: path-information set <id> sets path-id for subsequent NLRIs
// PREVENTS: Missing ADD-PATH support
func TestParseUpdateText_PathInfo(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"path-information", "set", "1",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Announce, 1)
	// Path-id should be set on NLRI
	assert.Equal(t, uint32(1), result.Groups[0].Announce[0].PathID())
}

// TestParseUpdateText_PathInfoChange verifies path-information changes mid-command.
//
// VALIDATES: path-information can be changed between nlri sections
// PREVENTS: Path-id applying retroactively
func TestParseUpdateText_PathInfoChange(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"path-information", "set", "1",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
		"path-information", "set", "2",
		"nlri", "ipv4/unicast", "add", "10.0.1.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 2)

	// First group: path-id=1
	assert.Equal(t, uint32(1), result.Groups[0].Announce[0].PathID())

	// Second group: path-id=2
	assert.Equal(t, uint32(2), result.Groups[1].Announce[0].PathID())
}

// TestParseUpdateText_PathInfoInvalid verifies invalid path-information fails.
//
// VALIDATES: Non-numeric path-information set returns error
// PREVENTS: Silent ignore of invalid path-id
func TestParseUpdateText_PathInfoInvalid(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"path-information", "set", "not-a-number",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid path-information")
}

// TestParseUpdateText_PathInfoDel verifies path-information del.
//
// VALIDATES: path-information del clears path-id
// PREVENTS: Path-id persisting unexpectedly
func TestParseUpdateText_PathInfoDel(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"path-information", "set", "1",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
		"path-information", "del",
		"nlri", "ipv4/unicast", "add", "10.0.1.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 2)

	// First group: path-id=1
	assert.Equal(t, uint32(1), result.Groups[0].Announce[0].PathID())

	// Second group: path-id=0 (cleared)
	assert.Equal(t, uint32(0), result.Groups[1].Announce[0].PathID())
}

// =============================================================================
// Phase 2: rd and label tests (VPN/Labeled families)
// =============================================================================

// TestParseUpdateText_RDSet verifies rd set <value> syntax.
//
// VALIDATES: rd set <ASN:value> stores RD for subsequent VPN NLRIs
// PREVENTS: Missing RD accumulator support
func TestParseUpdateText_RDSet(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"rd", "set", "65000:100",
		"label", "set", "1000",
		"nlri", "ipv4/mpls-vpn", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Announce, 1)

	// Get IPVPN NLRI and check RD
	vpnNLRI, ok := result.Groups[0].Announce[0].(*nlri.IPVPN)
	require.True(t, ok, "expected IPVPN NLRI")
	assert.Equal(t, "65000:100", vpnNLRI.RD().String())
}

// TestParseUpdateText_RDSetIPFormat verifies rd set with IP:value format.
//
// VALIDATES: rd set <IP:value> stores Type 1 RD (IP:assigned)
// PREVENTS: Only ASN:value format working
func TestParseUpdateText_RDSetIPFormat(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"rd", "set", "192.0.2.1:100",
		"label", "set", "1000",
		"nlri", "ipv4/mpls-vpn", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	vpnNLRI, ok := result.Groups[0].Announce[0].(*nlri.IPVPN)
	require.True(t, ok)
	assert.Equal(t, "192.0.2.1:100", vpnNLRI.RD().String())
}

// TestParseUpdateText_RDDel verifies rd del clears RD.
//
// VALIDATES: rd del clears accumulated RD
// PREVENTS: RD persisting unexpectedly
func TestParseUpdateText_RDDel(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"rd", "set", "65000:100",
		"label", "set", "1000",
		"nlri", "ipv4/mpls-vpn", "add", "10.0.0.0/24",
		"rd", "del",
		"nlri", "ipv4/unicast", "add", "10.0.1.0/24", // unicast doesn't need RD
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 2)

	// First group: VPN with RD
	vpnNLRI, ok := result.Groups[0].Announce[0].(*nlri.IPVPN)
	require.True(t, ok)
	assert.Equal(t, "65000:100", vpnNLRI.RD().String())

	// Second group: unicast (no RD check needed, it's INET)
	assert.Equal(t, nlri.IPv4Unicast, result.Groups[1].Family)
}

// TestParseUpdateText_RDDelConditional verifies rd del with value is conditional.
//
// VALIDATES: rd del <value> succeeds if matches, fails otherwise
// PREVENTS: Wrong RD being deleted
func TestParseUpdateText_RDDelConditional(t *testing.T) {
	// Conditional delete succeeds when value matches
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"rd", "set", "65000:100",
		"rd", "del", "65000:100", // Matches
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	// Conditional delete fails when value doesn't match
	_, err = ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"rd", "set", "65000:100",
		"rd", "del", "65000:999", // Doesn't match
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rd del: current value is 65000:100, not 65000:999")
}

// TestParseUpdateText_LabelSet verifies label set <value> syntax.
//
// VALIDATES: label set <value> stores label for VPN/labeled NLRIs
// PREVENTS: Missing label accumulator support
func TestParseUpdateText_LabelSet(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"rd", "set", "65000:100",
		"label", "set", "1000",
		"nlri", "ipv4/mpls-vpn", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	vpnNLRI, ok := result.Groups[0].Announce[0].(*nlri.IPVPN)
	require.True(t, ok)
	require.Len(t, vpnNLRI.Labels(), 1)
	assert.Equal(t, uint32(1000), vpnNLRI.Labels()[0])
}

// TestParseUpdateText_LabelSetZero verifies label=0 (Explicit Null) is valid.
//
// VALIDATES: label set 0 is accepted (RFC 3032 Explicit Null)
// PREVENTS: Zero label rejection
func TestParseUpdateText_LabelSetZero(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"rd", "set", "65000:100",
		"label", "set", "0",
		"nlri", "ipv4/mpls-vpn", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	vpnNLRI, ok := result.Groups[0].Announce[0].(*nlri.IPVPN)
	require.True(t, ok)
	require.Len(t, vpnNLRI.Labels(), 1)
	assert.Equal(t, uint32(0), vpnNLRI.Labels()[0])
}

// TestParseUpdateText_LabelSetMax verifies max label value (20-bit).
//
// VALIDATES: label set 1048575 (max 20-bit) is accepted
// PREVENTS: Valid max label rejection
func TestParseUpdateText_LabelSetMax(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"rd", "set", "65000:100",
		"label", "set", "1048575", // 0xFFFFF = max 20-bit
		"nlri", "ipv4/mpls-vpn", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	vpnNLRI, ok := result.Groups[0].Announce[0].(*nlri.IPVPN)
	require.True(t, ok)
	assert.Equal(t, uint32(1048575), vpnNLRI.Labels()[0])
}

// TestParseUpdateText_LabelSetOverflow verifies label > 20-bit fails.
//
// VALIDATES: label set 1048576+ returns error
// PREVENTS: Invalid label values accepted
func TestParseUpdateText_LabelSetOverflow(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"rd", "set", "65000:100",
		"label", "set", "1048576", // > 20-bit max
		"nlri", "ipv4/mpls-vpn", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "label out of range")
}

// TestParseUpdateText_LabelDel verifies label del clears label.
//
// VALIDATES: label del clears accumulated label
// PREVENTS: Label persisting unexpectedly
func TestParseUpdateText_LabelDel(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"rd", "set", "65000:100",
		"label", "set", "1000",
		"nlri", "ipv4/mpls-vpn", "add", "10.0.0.0/24",
		"rd", "del",
		"label", "del",
		"nlri", "ipv4/unicast", "add", "10.0.1.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 2)

	// First group: VPN with label
	vpnNLRI, ok := result.Groups[0].Announce[0].(*nlri.IPVPN)
	require.True(t, ok)
	require.Len(t, vpnNLRI.Labels(), 1)
	assert.Equal(t, uint32(1000), vpnNLRI.Labels()[0])

	// Second group: unicast (no label needed)
	assert.Equal(t, nlri.IPv4Unicast, result.Groups[1].Family)
}

// TestParseUpdateText_LabelDelConditional verifies label del with value is conditional.
//
// VALIDATES: label del <value> succeeds if matches, fails otherwise
// PREVENTS: Wrong label being deleted
func TestParseUpdateText_LabelDelConditional(t *testing.T) {
	// Conditional delete succeeds when value matches
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"label", "set", "1000",
		"label", "del", "1000", // Matches
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	// Conditional delete fails when value doesn't match
	_, err = ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"label", "set", "1000",
		"label", "del", "2000", // Doesn't match
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "label del: current value is [1000], not [2000]")
}

// TestParseUpdateText_VPNMissingRD verifies VPN family requires RD.
//
// VALIDATES: ipv4/mpls-vpn without rd returns error
// PREVENTS: VPN NLRI created without RD
func TestParseUpdateText_VPNMissingRD(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"label", "set", "1000", // label but no rd
		"nlri", "ipv4/mpls-vpn", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingRD)
}

// TestParseUpdateText_VPNMissingLabel verifies VPN family requires label.
//
// VALIDATES: ipv4/mpls-vpn without label returns error
// PREVENTS: VPN NLRI created without label
func TestParseUpdateText_VPNMissingLabel(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"rd", "set", "65000:100", // rd but no label
		"nlri", "ipv4/mpls-vpn", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingLabel)
}

// TestParseUpdateText_LabeledUnicast verifies labeled unicast family.
//
// VALIDATES: ipv4/nlri-mpls creates LabeledUnicast NLRI with label
// PREVENTS: Wrong NLRI type for labeled unicast
func TestParseUpdateText_LabeledUnicast(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"label", "set", "1000",
		"nlri", "ipv4/nlri-mpls", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Announce, 1)

	labeledNLRI, ok := result.Groups[0].Announce[0].(*nlri.LabeledUnicast)
	require.True(t, ok, "expected LabeledUnicast NLRI")
	require.Len(t, labeledNLRI.Labels(), 1)
	assert.Equal(t, uint32(1000), labeledNLRI.Labels()[0])
}

// TestParseUpdateText_LabeledUnicastMissingLabel verifies labeled unicast requires label.
//
// VALIDATES: ipv4/nlri-mpls without label returns error
// PREVENTS: LabeledUnicast NLRI created without label
func TestParseUpdateText_LabeledUnicastMissingLabel(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		// no label
		"nlri", "ipv4/nlri-mpls", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingLabel)
}

// TestParseUpdateText_IPv6VPN verifies IPv6 VPN family.
//
// VALIDATES: ipv6/mpls-vpn creates IPVPN NLRI with IPv6 prefix
// PREVENTS: IPv6 VPN family not working
func TestParseUpdateText_IPv6VPN(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "2001:db8::1",
		"rd", "set", "65000:100",
		"label", "set", "1000",
		"nlri", "ipv6/mpls-vpn", "add", "2001:db8:1::/48",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	vpnNLRI, ok := result.Groups[0].Announce[0].(*nlri.IPVPN)
	require.True(t, ok)
	assert.Equal(t, "65000:100", vpnNLRI.RD().String())
	assert.Equal(t, "2001:db8:1::/48", vpnNLRI.Prefix().String())
}

// TestParseUpdateText_IPv6LabeledUnicast verifies IPv6 labeled unicast family.
//
// VALIDATES: ipv6/nlri-mpls creates LabeledUnicast NLRI with IPv6 prefix
// PREVENTS: IPv6 labeled unicast not working
func TestParseUpdateText_IPv6LabeledUnicast(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "2001:db8::1",
		"label", "set", "1000",
		"nlri", "ipv6/nlri-mpls", "add", "2001:db8:1::/48",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	labeledNLRI, ok := result.Groups[0].Announce[0].(*nlri.LabeledUnicast)
	require.True(t, ok)
	assert.Equal(t, "2001:db8:1::/48", labeledNLRI.Prefix().String())
}

// TestParseUpdateText_VPNWithPathInfo verifies VPN with ADD-PATH.
//
// VALIDATES: VPN NLRI includes path-id when specified
// PREVENTS: Path-id lost for VPN families
func TestParseUpdateText_VPNWithPathInfo(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"rd", "set", "65000:100",
		"label", "set", "1000",
		"path-information", "set", "42",
		"nlri", "ipv4/mpls-vpn", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	vpnNLRI, ok := result.Groups[0].Announce[0].(*nlri.IPVPN)
	require.True(t, ok)
	assert.Equal(t, uint32(42), vpnNLRI.PathID())
}

// TestParseUpdateText_RDChangesBetweenSections verifies RD can change.
//
// VALIDATES: Different RD values for different VPN nlri sections
// PREVENTS: RD changes not taking effect
func TestParseUpdateText_RDChangesBetweenSections(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"rd", "set", "65000:100",
		"label", "set", "1000",
		"nlri", "ipv4/mpls-vpn", "add", "10.0.0.0/24",
		"rd", "set", "65000:200", // Change RD
		"nlri", "ipv4/mpls-vpn", "add", "10.0.1.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 2)

	// First group: RD 65000:100
	vpn1, ok := result.Groups[0].Announce[0].(*nlri.IPVPN)
	require.True(t, ok)
	assert.Equal(t, "65000:100", vpn1.RD().String())

	// Second group: RD 65000:200
	vpn2, ok := result.Groups[1].Announce[0].(*nlri.IPVPN)
	require.True(t, ok)
	assert.Equal(t, "65000:200", vpn2.RD().String())
}

// TestParseUpdateText_LabelChangesBetweenSections verifies label can change.
//
// VALIDATES: Different label values for different VPN nlri sections
// PREVENTS: Label changes not taking effect
func TestParseUpdateText_LabelChangesBetweenSections(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"rd", "set", "65000:100",
		"label", "set", "1000",
		"nlri", "ipv4/mpls-vpn", "add", "10.0.0.0/24",
		"label", "set", "2000", // Change label
		"nlri", "ipv4/mpls-vpn", "add", "10.0.1.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 2)

	// First group: label 1000
	vpn1, ok := result.Groups[0].Announce[0].(*nlri.IPVPN)
	require.True(t, ok)
	assert.Equal(t, uint32(1000), vpn1.Labels()[0])

	// Second group: label 2000
	vpn2, ok := result.Groups[1].Announce[0].(*nlri.IPVPN)
	require.True(t, ok)
	assert.Equal(t, uint32(2000), vpn2.Labels()[0])
}

// =============================================================================
// In-NLRI modifier syntax (rd/label without 'set')
// =============================================================================

// TestParseUpdateText_InNLRIModifierSyntax verifies rd/label inside nlri section.
//
// VALIDATES: nlri ipv4/mpls-vpn rd 65000:100 label 1000 add 10.0.0.0/24 works
// PREVENTS: Requiring accumulator syntax for VPN routes
func TestParseUpdateText_InNLRIModifierSyntax(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"nlri", "ipv4/mpls-vpn", "rd", "65000:100", "label", "1000", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Announce, 1)

	vpnNLRI, ok := result.Groups[0].Announce[0].(*nlri.IPVPN)
	require.True(t, ok, "expected IPVPN NLRI")
	assert.Equal(t, "65000:100", vpnNLRI.RD().String())
	require.Len(t, vpnNLRI.Labels(), 1)
	assert.Equal(t, uint32(1000), vpnNLRI.Labels()[0])
	assert.Equal(t, "10.0.0.0/24", vpnNLRI.Prefix().String())
}

// TestParseUpdateText_InNLRIModifierMultiplePrefixes verifies in-NLRI modifiers apply to all prefixes.
//
// VALIDATES: rd/label in nlri section applies to all prefixes in that section
// PREVENTS: Modifiers only applying to first prefix
func TestParseUpdateText_InNLRIModifierMultiplePrefixes(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"nlri", "ipv4/mpls-vpn", "rd", "65000:100", "label", "1000",
		"add", "10.0.0.0/24", "10.0.1.0/24", "10.0.2.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Announce, 3)

	// All three prefixes should have same RD and label
	for i, n := range result.Groups[0].Announce {
		vpnNLRI, ok := n.(*nlri.IPVPN)
		require.True(t, ok, "prefix %d: expected IPVPN NLRI", i)
		assert.Equal(t, "65000:100", vpnNLRI.RD().String(), "prefix %d", i)
		assert.Equal(t, uint32(1000), vpnNLRI.Labels()[0], "prefix %d", i)
	}
}

// TestParseUpdateText_InNLRIModifierOverridesAccumulator verifies in-NLRI modifiers override accumulated.
//
// VALIDATES: In-NLRI rd/label overrides accumulated values for that section
// PREVENTS: Accumulator values not being overridable
func TestParseUpdateText_InNLRIModifierOverridesAccumulator(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"rd", "set", "65000:100", // Accumulated RD
		"label", "set", "1000", // Accumulated label
		"nlri", "ipv4/mpls-vpn", "rd", "65000:200", "label", "2000", // Override in-section
		"add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	vpnNLRI, ok := result.Groups[0].Announce[0].(*nlri.IPVPN)
	require.True(t, ok)
	// Should use in-NLRI values, not accumulated
	assert.Equal(t, "65000:200", vpnNLRI.RD().String())
	assert.Equal(t, uint32(2000), vpnNLRI.Labels()[0])
}

// TestParseUpdateText_InNLRIModifierIPv6VPN verifies IPv6 VPN with in-NLRI modifiers.
//
// VALIDATES: nlri ipv6/mpls-vpn rd ... label ... add ... works
// PREVENTS: IPv6 VPN not supporting in-NLRI modifier syntax
func TestParseUpdateText_InNLRIModifierIPv6VPN(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "2001:db8::1",
		"nlri", "ipv6/mpls-vpn", "rd", "65000:100", "label", "1000", "add", "2001:db8:1::/48",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	vpnNLRI, ok := result.Groups[0].Announce[0].(*nlri.IPVPN)
	require.True(t, ok)
	assert.Equal(t, "65000:100", vpnNLRI.RD().String())
	assert.Equal(t, uint32(1000), vpnNLRI.Labels()[0])
	assert.Equal(t, "2001:db8:1::/48", vpnNLRI.Prefix().String())
}

// TestParseUpdateText_InNLRIModifierLabelOnly verifies label-only in-NLRI modifier.
//
// VALIDATES: nlri ipv4/nlri-mpls label 1000 add ... works (labeled unicast)
// PREVENTS: Label-only modifier not working
func TestParseUpdateText_InNLRIModifierLabelOnly(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"nlri", "ipv4/nlri-mpls", "label", "1000", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	labeledNLRI, ok := result.Groups[0].Announce[0].(*nlri.LabeledUnicast)
	require.True(t, ok, "expected LabeledUnicast NLRI")
	require.Len(t, labeledNLRI.Labels(), 1)
	assert.Equal(t, uint32(1000), labeledNLRI.Labels()[0])
}

// TestParseUpdateText_InNLRIModifierRDOnlyStillNeedsLabel verifies rd-only still requires label.
//
// VALIDATES: nlri ipv4/mpls-vpn rd ... add ... fails (missing label)
// PREVENTS: VPN routes created without label
func TestParseUpdateText_InNLRIModifierRDOnlyStillNeedsLabel(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"nlri", "ipv4/mpls-vpn", "rd", "65000:100", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingLabel)
}

// TestParseUpdateText_InNLRIModifierLabelOnlyStillNeedsRDForVPN verifies label-only still requires rd for VPN.
//
// VALIDATES: nlri ipv4/mpls-vpn label ... add ... fails (missing rd)
// PREVENTS: VPN routes created without RD
func TestParseUpdateText_InNLRIModifierLabelOnlyStillNeedsRDForVPN(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"nlri", "ipv4/mpls-vpn", "label", "1000", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingRD)
}

// TestParseUpdateText_InNLRIModifierScopeIsSectionOnly verifies modifiers don't leak to next section.
//
// VALIDATES: In-NLRI modifiers only affect that section, not subsequent sections
// PREVENTS: Modifier values leaking across sections
func TestParseUpdateText_InNLRIModifierScopeIsSectionOnly(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"nlri", "ipv4/mpls-vpn", "rd", "65000:100", "label", "1000", "add", "10.0.0.0/24",
		"nlri", "ipv4/unicast", "add", "10.0.1.0/24", // unicast doesn't need rd/label
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 2)

	// First group: VPN with in-NLRI modifiers
	vpnNLRI, ok := result.Groups[0].Announce[0].(*nlri.IPVPN)
	require.True(t, ok)
	assert.Equal(t, "65000:100", vpnNLRI.RD().String())

	// Second group: unicast (no VPN requirements)
	assert.Equal(t, nlri.IPv4Unicast, result.Groups[1].Family)
}
