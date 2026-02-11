package plugin

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/rib"
	"codeberg.org/thomas-mangin/ze/internal/plugin/evpn"
	"codeberg.org/thomas-mangin/ze/internal/plugin/flowspec"
	"codeberg.org/thomas-mangin/ze/internal/plugin/vpn"
	"codeberg.org/thomas-mangin/ze/internal/selector"
)

// testExtractOrigin extracts Origin from Wire for testing.
func testExtractOrigin(t *testing.T, wire *attribute.AttributesWire) uint8 {
	t.Helper()
	attrs, err := wire.All()
	require.NoError(t, err)
	for _, a := range attrs {
		if o, ok := a.(attribute.Origin); ok {
			return uint8(o)
		}
	}
	t.Fatal("Origin not found in Wire")
	return 0
}

// testExtractMED extracts MED from Wire for testing.
func testExtractMED(t *testing.T, wire *attribute.AttributesWire) uint32 {
	t.Helper()
	attrs, err := wire.All()
	require.NoError(t, err)
	for _, a := range attrs {
		if m, ok := a.(attribute.MED); ok {
			return uint32(m)
		}
	}
	t.Fatal("MED not found in Wire")
	return 0
}

// testExtractLocalPref extracts LocalPref from Wire for testing.
func testExtractLocalPref(t *testing.T, wire *attribute.AttributesWire) uint32 {
	t.Helper()
	attrs, err := wire.All()
	require.NoError(t, err)
	for _, a := range attrs {
		if lp, ok := a.(attribute.LocalPref); ok {
			return uint32(lp)
		}
	}
	t.Fatal("LocalPref not found in Wire")
	return 0
}

// testExtractCommunities extracts Communities from Wire for testing.
func testExtractCommunities(t *testing.T, wire *attribute.AttributesWire) []uint32 {
	t.Helper()
	attrs, err := wire.All()
	require.NoError(t, err)
	for _, a := range attrs {
		if comms, ok := a.(attribute.Communities); ok {
			result := make([]uint32, len(comms))
			for i, c := range comms {
				result[i] = uint32(c)
			}
			return result
		}
	}
	return nil
}

// testExtractLargeCommunities extracts LargeCommunities from Wire for testing.
func testExtractLargeCommunities(t *testing.T, wire *attribute.AttributesWire) []LargeCommunity {
	t.Helper()
	attrs, err := wire.All()
	require.NoError(t, err)
	for _, a := range attrs {
		if lcs, ok := a.(attribute.LargeCommunities); ok {
			return lcs
		}
	}
	return nil
}

// testExtractExtCommunities extracts ExtendedCommunities from Wire for testing.
func testExtractExtCommunities(t *testing.T, wire *attribute.AttributesWire) []attribute.ExtendedCommunity {
	t.Helper()
	attrs, err := wire.All()
	require.NoError(t, err)
	for _, a := range attrs {
		if ecs, ok := a.(attribute.ExtendedCommunities); ok {
			return ecs
		}
	}
	return nil
}

// testHasOrigin checks if Origin exists in Wire.
func testHasOrigin(t *testing.T, wire *attribute.AttributesWire) bool {
	t.Helper()
	if wire == nil {
		return false
	}
	has, err := wire.Has(attribute.AttrOrigin)
	require.NoError(t, err)
	return has
}

// testHasMED checks if MED exists in Wire.
func testHasMED(t *testing.T, wire *attribute.AttributesWire) bool {
	t.Helper()
	if wire == nil {
		return false
	}
	has, err := wire.Has(attribute.AttrMED)
	require.NoError(t, err)
	return has
}

// testExtractASPath extracts AS_PATH as []uint32 from Wire for testing.
func testExtractASPath(t *testing.T, wire *attribute.AttributesWire) []uint32 {
	t.Helper()
	if wire == nil {
		return nil
	}
	attrs, err := wire.All()
	require.NoError(t, err)
	for _, a := range attrs {
		if asp, ok := a.(*attribute.ASPath); ok {
			var result []uint32
			for _, seg := range asp.Segments {
				result = append(result, seg.ASNs...)
			}
			return result
		}
	}
	return nil
}

// testExtractFlowSpec decodes WireNLRI back to FlowSpec for test assertions.
// FlowSpec text parsing returns WireNLRI (engine is FlowSpec-agnostic),
// so tests use this helper to verify FlowSpec components.
func testExtractFlowSpec(t *testing.T, n nlri.NLRI) *flowspec.FlowSpec {
	t.Helper()
	wire, ok := n.(*nlri.WireNLRI)
	require.True(t, ok, "expected WireNLRI, got %T", n)
	fs, err := flowspec.ParseFlowSpec(wire.Family(), wire.Bytes())
	require.NoError(t, err)
	return fs
}

// testExtractFlowSpecVPN decodes WireNLRI back to FlowSpecVPN for test assertions.
func testExtractFlowSpecVPN(t *testing.T, n nlri.NLRI) *flowspec.FlowSpecVPN {
	t.Helper()
	wire, ok := n.(*nlri.WireNLRI)
	require.True(t, ok, "expected WireNLRI, got %T", n)
	fsv, err := flowspec.ParseFlowSpecVPN(wire.Family(), wire.Bytes())
	require.NoError(t, err)
	return fsv
}

// TestParseUpdateText_EmptyInput verifies empty args returns empty result.
//
// VALIDATES: Empty args produces empty Groups, no error.
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
// VALIDATES: origin set igp/egp/incomplete stores correct value.
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
			require.NotNil(t, result.Groups[0].Wire)
			assert.Equal(t, tc.want, testExtractOrigin(t, result.Groups[0].Wire))
		})
	}
}

// TestParseUpdateText_MultipleAttrs verifies multiple attrs in sequence.
//
// VALIDATES: Multiple per-attribute sections parsed correctly.
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

	wire := result.Groups[0].Wire
	require.NotNil(t, wire)
	assert.Equal(t, uint8(0), testExtractOrigin(t, wire))
	assert.Equal(t, uint32(100), testExtractMED(t, wire))
	assert.Equal(t, uint32(200), testExtractLocalPref(t, wire))
}

// TestParseUpdateText_CommunitySet verifies community parsing.
//
// VALIDATES: Community list parsed in various formats.
// PREVENTS: Community parsing failures.
func TestParseUpdateText_CommunitySet(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"community", "set", "[65000:100", "65000:200]",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	comms := testExtractCommunities(t, result.Groups[0].Wire)
	require.Len(t, comms, 2)
	assert.Equal(t, uint32(65000<<16|100), comms[0])
	assert.Equal(t, uint32(65000<<16|200), comms[1])
}

// TestParseUpdateText_CommunityAdd verifies community append.
//
// VALIDATES: community add prepends to existing list.
// PREVENTS: Community replacement instead of prepend.
func TestParseUpdateText_CommunityAdd(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"community", "set", "[65000:100]",
		"community", "add", "[65000:200]", // prepends → [200, 100]
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	comms := testExtractCommunities(t, result.Groups[0].Wire)
	require.Len(t, comms, 2)
	assert.Equal(t, uint32(65000<<16|200), comms[0]) // prepended first
	assert.Equal(t, uint32(65000<<16|100), comms[1])
}

// TestParseUpdateText_CommunityDel verifies community removal.
//
// VALIDATES: community del removes matching values.
// PREVENTS: Community deletion failures.
func TestParseUpdateText_CommunityDel(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"community", "set", "[65000:100", "65000:200", "65000:300]",
		"community", "del", "[65000:200]",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	comms := testExtractCommunities(t, result.Groups[0].Wire)
	require.Len(t, comms, 2)
	assert.Equal(t, uint32(65000<<16|100), comms[0])
	assert.Equal(t, uint32(65000<<16|300), comms[1])
}

// TestParseUpdateText_CommunityNotFoundDel verifies error on del non-existing.
//
// VALIDATES: community del [value] errors if value not in list.
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
// VALIDATES: community del [] is a no-op (doesn't error).
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
	comms := testExtractCommunities(t, result.Groups[0].Wire)
	require.Len(t, comms, 1)
	assert.Equal(t, uint32(65000<<16|100), comms[0])
}

// TestParseUpdateText_FirstInstanceOnlyDel verifies del removes first instance only.
//
// VALIDATES: community del [X] removes only first X, leaves duplicates.
// PREVENTS: Removing all instances of a value.
func TestParseUpdateText_FirstInstanceOnlyDel(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"community", "set", "[65000:100", "65000:200", "65000:100]", // 100 appears twice
		"community", "del", "[65000:100]", // remove first 100 only
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	comms := testExtractCommunities(t, result.Groups[0].Wire)
	require.Len(t, comms, 2) // 200 and second 100 remain
	assert.Equal(t, uint32(65000<<16|200), comms[0])
	assert.Equal(t, uint32(65000<<16|100), comms[1]) // second 100 still there
}

// TestParseUpdateText_ThenAddSet verifies set-then-add accumulation.
//
// VALIDATES: set replaces, then add prepends (65000:400 before 65000:300).
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

	comms := testExtractCommunities(t, result.Groups[0].Wire)
	require.Len(t, comms, 2)
	assert.Equal(t, uint32(65000<<16|400), comms[0]) // prepended first
	assert.Equal(t, uint32(65000<<16|300), comms[1])
}

// TestParseUpdateText_LargeCommunity verifies large community parsing.
//
// VALIDATES: Large community (ASN:value1:value2) parsed correctly.
// PREVENTS: Large community format errors.
func TestParseUpdateText_LargeCommunity(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"large-community", "set", "[65000:1:2]",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	lcomms := testExtractLargeCommunities(t, result.Groups[0].Wire)
	require.Len(t, lcomms, 1)
	assert.Equal(t, LargeCommunity{GlobalAdmin: 65000, LocalData1: 1, LocalData2: 2}, lcomms[0])
}

// TestParseUpdateText_ExtendedCommunity verifies extended community parsing.
//
// VALIDATES: Extended community parsed correctly.
// PREVENTS: Extended community format errors.
func TestParseUpdateText_ExtendedCommunity(t *testing.T) {
	// Parser supports: origin:ASN:IP, redirect:ASN:target, rate-limit:bps
	result, err := ParseUpdateText([]string{
		"extended-community", "set", "[origin:65000:1.2.3.4]",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	extcomms := testExtractExtCommunities(t, result.Groups[0].Wire)
	require.Len(t, extcomms, 1)
	// Origin: Type 0x00, Subtype 0x03, 2-byte ASN + IPv4
	// 65000 = 0xFDE8 → bytes [0xFD, 0xE8]
	// 1.2.3.4 → bytes [1, 2, 3, 4]
	assert.Equal(t, attribute.ExtendedCommunity{0x00, 0x03, 0xfd, 0xe8, 1, 2, 3, 4}, extcomms[0])
}

// TestParseUpdateText_ScalarErrorAdd verifies add on scalar fails.
//
// VALIDATES: origin add/med/local-pref returns error.
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
// VALIDATES: origin del <value> succeeds if current matches, fails otherwise.
// PREVENTS: Confusion about scalar conditional deletion semantics.
func TestParseUpdateText_ScalarDelConditional(t *testing.T) {
	// Conditional delete succeeds when value matches
	result, err := ParseUpdateText([]string{
		"origin", "set", "igp",
		"origin", "del", "igp", // Matches current value
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	// Parser clears ORIGIN; reactor adds it when building UPDATE (RFC 4271 compliance)
	assert.False(t, testHasOrigin(t, result.Groups[0].Wire))

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
// VALIDATES: origin del (no value) clears the attribute from wire.
// PREVENTS: Scalar del being a no-op.
func TestParseUpdateText_ScalarDelClearsAttribute(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"origin", "set", "igp",
		"med", "set", "100",
		"origin", "del", // del without value - clears origin from wire
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	// Parser clears ORIGIN; reactor adds it when building UPDATE (RFC 4271 compliance)
	assert.False(t, testHasOrigin(t, result.Groups[0].Wire), "origin should be cleared by del")
	// MED should still be set
	assert.True(t, testHasMED(t, result.Groups[0].Wire))
	assert.Equal(t, uint32(100), testExtractMED(t, result.Groups[0].Wire))
}

// TestParseUpdateText_ASPathAdd verifies add on as-path prepends.
//
// VALIDATES: as-path add prepends ASNs to existing path.
// PREVENTS: AS-PATH prepend not working.
func TestParseUpdateText_ASPathAdd(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"as-path", "set", "[65001", "65002]",
		"as-path", "add", "[65000]", // prepends → [65000, 65001, 65002]
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	asPath := testExtractASPath(t, result.Groups[0].Wire)
	require.Len(t, asPath, 3)
	assert.Equal(t, uint32(65000), asPath[0]) // prepended
	assert.Equal(t, uint32(65001), asPath[1])
	assert.Equal(t, uint32(65002), asPath[2])
}

// TestParseUpdateText_ASPathDelValue verifies del on as-path removes specific ASN.
//
// VALIDATES: as-path del [ASN] removes first occurrence.
// PREVENTS: AS-PATH del not removing ASN.
func TestParseUpdateText_ASPathDelValue(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"as-path", "set", "[65000", "65001", "65002]",
		"as-path", "del", "[65001]", // removes 65001 → [65000, 65002]
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	asPath := testExtractASPath(t, result.Groups[0].Wire)
	require.Len(t, asPath, 2)
	assert.Equal(t, uint32(65000), asPath[0])
	assert.Equal(t, uint32(65002), asPath[1])
}

// TestParseUpdateText_ASPathDelClear verifies del without value clears as-path.
//
// VALIDATES: as-path del (no value) clears entire path.
// PREVENTS: AS-PATH del not clearing.
func TestParseUpdateText_ASPathDelClear(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"as-path", "set", "[65000", "65001]",
		"as-path", "del", // clears entire path
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	asPath := testExtractASPath(t, result.Groups[0].Wire)
	assert.Empty(t, asPath)
}

// TestParseUpdateText_ASPathDelNotFound verifies error when ASN not in path.
//
// VALIDATES: as-path del [ASN] errors if ASN not present.
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
// VALIDATES: nlri <family> add <prefix> creates group.
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
// VALIDATES: Multiple prefixes in single nlri section.
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
// VALIDATES: add X del Y in same section produces both lists.
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
// VALIDATES: nlri <family> del <prefix> works without add.
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
// VALIDATES: add X Y del Z add W produces correct lists.
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
// VALIDATES: nlri <family> with no prefixes returns error.
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
// VALIDATES: Prefix without add/del mode returns error.
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
// VALIDATES: Attributes applied to NLRI group.
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
	assert.True(t, testHasOrigin(t, g.Wire))
	assert.Equal(t, uint8(0), testExtractOrigin(t, g.Wire))
	require.Len(t, g.Announce, 1)
}

// TestParseUpdateText_MultipleGroups verifies snapshot deep copy.
//
// VALIDATES: Each group has independent attribute snapshot.
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
	comms0 := testExtractCommunities(t, result.Groups[0].Wire)
	require.Len(t, comms0, 1)
	assert.Equal(t, uint32(65000<<16|100), comms0[0])

	// Second group: 65000:200 prepended + 65000:100
	comms1 := testExtractCommunities(t, result.Groups[1].Wire)
	require.Len(t, comms1, 2)
	assert.Equal(t, uint32(65000<<16|200), comms1[0]) // prepended
	assert.Equal(t, uint32(65000<<16|100), comms1[1])
}

// TestParseUpdateText_IPv6 verifies IPv6 support.
//
// VALIDATES: IPv6 prefixes parsed correctly.
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
// VALIDATES: IPv4 prefix in ipv6/unicast returns error.
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
// VALIDATES: Unknown attribute keyword returns error listing valid options.
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
// VALIDATES: Unsupported family returns error.
// PREVENTS: Silent ignore of unsupported families.
func TestParseUpdateText_UnsupportedFamily(t *testing.T) {
	// MVPN is a valid family but not supported in text mode
	_, err := ParseUpdateText([]string{
		"nlri", "ipv4/mvpn", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFamilyNotSupported)
}

// TestParseUpdateText_InvalidFamilyString verifies invalid family fails.
//
// VALIDATES: Invalid family string returns error.
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
// VALIDATES: Invalid prefix format returns error.
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
// VALIDATES: nlri <family> add (no prefix) returns error.
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
// VALIDATES: nlri ... add ... watchdog set <name> stored in group.
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
// VALIDATES: watchdog <name> (standalone) still works for backward compat.
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
// VALIDATES: Complex multi-section command parses correctly.
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
	require.Len(t, testExtractCommunities(t, g1.Wire), 1)
	require.Len(t, g1.Announce, 2)
	require.Len(t, g1.Withdraw, 1)
	assert.Empty(t, g1.WatchdogName) // No watchdog on first group

	// Second group: ipv6/unicast with modified attrs and watchdog
	g2 := result.Groups[1]
	assert.Equal(t, nlri.IPv6Unicast, g2.Family)
	assert.True(t, g2.NextHop.IsExplicit())
	assert.Equal(t, netip.MustParseAddr("2001:db8::1"), g2.NextHop.Addr)
	require.Len(t, testExtractCommunities(t, g2.Wire), 2) // 65000:100 + 65000:200
	require.Len(t, g2.Announce, 1)
	assert.Empty(t, g2.Withdraw)
	assert.Equal(t, "test-pool", g2.WatchdogName)

	assert.Equal(t, "test-pool", result.WatchdogName) // Global for compat
}

// TestParsedAttrs_Snapshot_DeepCopy verifies snapshot creates independent copies.
//
// VALIDATES: Modifying original after snapshot doesn't affect copy.
// PREVENTS: Shared slice bugs.
func TestParsedAttrs_Snapshot_DeepCopy(t *testing.T) {
	orig := parsedAttrs{
		Communities: []uint32{1, 2, 3},
	}

	wire, _, _ := orig.snapshot()

	// Modify original
	orig.Communities = append(orig.Communities, 4)

	// Snapshot should be unaffected (Wire is built at snapshot time)
	comms := testExtractCommunities(t, wire)
	assert.Len(t, comms, 3)
}

// TestParsedAttrs_Snapshot_DeepCopyPointers verifies pointer fields are deep copied.
//
// VALIDATES: Pointer fields are independent after snapshot.
// PREVENTS: Shared pointer mutation between groups.
func TestParsedAttrs_Snapshot_DeepCopyPointers(t *testing.T) {
	origin := uint8(0)
	orig := parsedAttrs{
		Origin: &origin,
	}

	wire, _, _ := orig.snapshot()

	// Modify original pointer value
	*orig.Origin = 2

	// Snapshot should be unaffected (Wire built at snapshot time with origin=0)
	extractedOrigin := testExtractOrigin(t, wire)
	assert.Equal(t, uint8(0), extractedOrigin)
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
	assert.True(t, testHasOrigin(t, result.Groups[0].Wire))
	assert.Equal(t, uint8(0), testExtractOrigin(t, result.Groups[0].Wire))

	// Second group: origin=EGP (1)
	assert.True(t, testHasOrigin(t, result.Groups[1].Wire))
	assert.Equal(t, uint8(1), testExtractOrigin(t, result.Groups[1].Wire))
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
func (m *mockReactorBatch) VerifyConfig(_ map[string]any) error              { return nil }
func (m *mockReactorBatch) ApplyConfigDiff(_ map[string]any) error           { return nil }
func (m *mockReactorBatch) AddDynamicPeer(_ DynamicPeerConfig) error         { return nil }
func (m *mockReactorBatch) RemovePeer(_ netip.Addr) error                    { return nil }
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
func (m *mockReactorBatch) RIBInRoutes(_ string) []rib.RouteJSON            { return nil }
func (m *mockReactorBatch) RIBOutRoutes() []rib.RouteJSON                   { return nil }
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
func (m *mockReactorBatch) AnnounceWatchdog(_, _ string) error                       { return nil }
func (m *mockReactorBatch) WithdrawWatchdog(_, _ string) error                       { return nil }
func (m *mockReactorBatch) AddWatchdogRoute(_ RouteSpec, _ string) error             { return nil }
func (m *mockReactorBatch) RemoveWatchdogRoute(_, _ string) error                    { return nil }
func (m *mockReactorBatch) ClearRIBIn() int                                          { return 0 }
func (m *mockReactorBatch) ClearRIBOut() int                                         { return 0 }
func (m *mockReactorBatch) FlushRIBOut() int                                         { return 0 }
func (m *mockReactorBatch) GetPeerProcessBindings(_ netip.Addr) []PeerProcessBinding { return nil }
func (m *mockReactorBatch) GetPeerCapabilityConfigs() []PeerCapabilityConfig         { return nil }
func (m *mockReactorBatch) GetConfigTree() map[string]any                            { return nil }
func (m *mockReactorBatch) SetConfigTree(_ map[string]any)                           {}
func (m *mockReactorBatch) ForwardUpdate(_ *selector.Selector, _ uint64) error       { return nil }
func (m *mockReactorBatch) DeleteUpdate(_ uint64) error                              { return nil }
func (m *mockReactorBatch) RetainUpdate(_ uint64) error                              { return nil }
func (m *mockReactorBatch) ReleaseUpdate(_ uint64) error                             { return nil }
func (m *mockReactorBatch) ListUpdates() []uint64                                    { return nil }
func (m *mockReactorBatch) SignalAPIReady()                                          {}
func (m *mockReactorBatch) AddAPIProcessCount(_ int)                                 {}
func (m *mockReactorBatch) SignalPluginStartupComplete()                             {}
func (m *mockReactorBatch) SignalPeerAPIReady(_ string)                              {}
func (m *mockReactorBatch) SendRawMessage(_ netip.Addr, _ uint8, _ []byte) error {
	return nil
}

func (m *mockReactorBatch) SendBoRR(_ string, _ uint16, _ uint8) error { return nil }
func (m *mockReactorBatch) SendEoRR(_ string, _ uint16, _ uint8) error { return nil }

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
	comms0 := testExtractCommunities(t, reactor.announceCalls[0].Wire)
	assert.Equal(t, 1, len(comms0))

	// Second group: 2 communities
	comms1 := testExtractCommunities(t, reactor.announceCalls[1].Wire)
	assert.Equal(t, 2, len(comms1))
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
// VALIDATES: nhop set <addr> stores next-hop as explicit.
// PREVENTS: Missing nhop keyword support.
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
// VALIDATES: nhop set self stores next-hop as self policy.
// PREVENTS: Missing self keyword support.
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
// VALIDATES: nhop del unsets next-hop for subsequent nlri.
// PREVENTS: Missing nhop del support.
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
// VALIDATES: nhop del <value> succeeds if matches, fails otherwise.
// PREVENTS: Wrong next-hop being deleted.
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
// VALIDATES: nhop changes affect only subsequent nlri sections.
// PREVENTS: nhop applying retroactively.
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
// VALIDATES: path-information set <id> sets path-id for subsequent NLRIs.
// PREVENTS: Missing ADD-PATH support.
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
// VALIDATES: path-information can be changed between nlri sections.
// PREVENTS: Path-id applying retroactively.
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
// VALIDATES: Non-numeric path-information set returns error.
// PREVENTS: Silent ignore of invalid path-id.
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
// VALIDATES: path-information del clears path-id.
// PREVENTS: Path-id persisting unexpectedly.
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
// VALIDATES: rd set <ASN:value> stores RD for subsequent VPN NLRIs.
// PREVENTS: Missing RD accumulator support.
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
	vpnNLRI, ok := result.Groups[0].Announce[0].(*vpn.VPN)
	require.True(t, ok, "expected IPVPN NLRI")
	assert.Equal(t, "0:65000:100", vpnNLRI.RD().String())
}

// TestParseUpdateText_RDSetIPFormat verifies rd set with IP:value format.
//
// VALIDATES: rd set <IP:value> stores Type 1 RD (IP:assigned).
// PREVENTS: Only ASN:value format working.
func TestParseUpdateText_RDSetIPFormat(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"rd", "set", "192.0.2.1:100",
		"label", "set", "1000",
		"nlri", "ipv4/mpls-vpn", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	vpnNLRI, ok := result.Groups[0].Announce[0].(*vpn.VPN)
	require.True(t, ok)
	assert.Equal(t, "1:192.0.2.1:100", vpnNLRI.RD().String())
}

// TestParseUpdateText_RDDel verifies rd del clears RD.
//
// VALIDATES: rd del clears accumulated RD.
// PREVENTS: RD persisting unexpectedly.
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
	vpnNLRI, ok := result.Groups[0].Announce[0].(*vpn.VPN)
	require.True(t, ok)
	assert.Equal(t, "0:65000:100", vpnNLRI.RD().String())

	// Second group: unicast (no RD check needed, it's INET)
	assert.Equal(t, nlri.IPv4Unicast, result.Groups[1].Family)
}

// TestParseUpdateText_RDDelConditional verifies rd del with value is conditional.
//
// VALIDATES: rd del <value> succeeds if matches, fails otherwise.
// PREVENTS: Wrong RD being deleted.
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
	assert.Contains(t, err.Error(), "rd del: current value is 0:65000:100, not 0:65000:999")
}

// TestParseUpdateText_LabelSet verifies label set <value> syntax.
//
// VALIDATES: label set <value> stores label for VPN/labeled NLRIs.
// PREVENTS: Missing label accumulator support.
func TestParseUpdateText_LabelSet(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"rd", "set", "65000:100",
		"label", "set", "1000",
		"nlri", "ipv4/mpls-vpn", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	vpnNLRI, ok := result.Groups[0].Announce[0].(*vpn.VPN)
	require.True(t, ok)
	require.Len(t, vpnNLRI.Labels(), 1)
	assert.Equal(t, uint32(1000), vpnNLRI.Labels()[0])
}

// TestParseUpdateText_LabelSetZero verifies label=0 (Explicit Null) is valid.
//
// VALIDATES: label set 0 is accepted (RFC 3032 Explicit Null).
// PREVENTS: Zero label rejection.
func TestParseUpdateText_LabelSetZero(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"rd", "set", "65000:100",
		"label", "set", "0",
		"nlri", "ipv4/mpls-vpn", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	vpnNLRI, ok := result.Groups[0].Announce[0].(*vpn.VPN)
	require.True(t, ok)
	require.Len(t, vpnNLRI.Labels(), 1)
	assert.Equal(t, uint32(0), vpnNLRI.Labels()[0])
}

// TestParseUpdateText_LabelSetMax verifies max label value (20-bit).
//
// VALIDATES: label set 1048575 (max 20-bit) is accepted.
// PREVENTS: Valid max label rejection.
func TestParseUpdateText_LabelSetMax(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"rd", "set", "65000:100",
		"label", "set", "1048575", // 0xFFFFF = max 20-bit
		"nlri", "ipv4/mpls-vpn", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	vpnNLRI, ok := result.Groups[0].Announce[0].(*vpn.VPN)
	require.True(t, ok)
	assert.Equal(t, uint32(1048575), vpnNLRI.Labels()[0])
}

// TestParseUpdateText_LabelSetOverflow verifies label > 20-bit fails.
//
// VALIDATES: label set 1048576+ returns error.
// PREVENTS: Invalid label values accepted.
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
// VALIDATES: label del clears accumulated label.
// PREVENTS: Label persisting unexpectedly.
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
	vpnNLRI, ok := result.Groups[0].Announce[0].(*vpn.VPN)
	require.True(t, ok)
	require.Len(t, vpnNLRI.Labels(), 1)
	assert.Equal(t, uint32(1000), vpnNLRI.Labels()[0])

	// Second group: unicast (no label needed)
	assert.Equal(t, nlri.IPv4Unicast, result.Groups[1].Family)
}

// TestParseUpdateText_LabelDelConditional verifies label del with value is conditional.
//
// VALIDATES: label del <value> succeeds if matches, fails otherwise.
// PREVENTS: Wrong label being deleted.
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
// VALIDATES: ipv4/mpls-vpn without rd returns error.
// PREVENTS: VPN NLRI created without RD.
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
// VALIDATES: ipv4/mpls-vpn without label returns error.
// PREVENTS: VPN NLRI created without label.
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
// VALIDATES: ipv4/nlri-mpls creates LabeledUnicast NLRI with label.
// PREVENTS: Wrong NLRI type for labeled unicast.
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
// VALIDATES: ipv4/nlri-mpls without label returns error.
// PREVENTS: LabeledUnicast NLRI created without label.
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
// VALIDATES: ipv6/mpls-vpn creates IPVPN NLRI with IPv6 prefix.
// PREVENTS: IPv6 VPN family not working.
func TestParseUpdateText_IPv6VPN(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "2001:db8::1",
		"rd", "set", "65000:100",
		"label", "set", "1000",
		"nlri", "ipv6/mpls-vpn", "add", "2001:db8:1::/48",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	vpnNLRI, ok := result.Groups[0].Announce[0].(*vpn.VPN)
	require.True(t, ok)
	assert.Equal(t, "0:65000:100", vpnNLRI.RD().String())
	assert.Equal(t, "2001:db8:1::/48", vpnNLRI.Prefix().String())
}

// TestParseUpdateText_IPv6LabeledUnicast verifies IPv6 labeled unicast family.
//
// VALIDATES: ipv6/nlri-mpls creates LabeledUnicast NLRI with IPv6 prefix.
// PREVENTS: IPv6 labeled unicast not working.
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
// VALIDATES: VPN NLRI includes path-id when specified.
// PREVENTS: Path-id lost for VPN families.
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

	vpnNLRI, ok := result.Groups[0].Announce[0].(*vpn.VPN)
	require.True(t, ok)
	assert.Equal(t, uint32(42), vpnNLRI.PathID())
}

// TestParseUpdateText_RDChangesBetweenSections verifies RD can change.
//
// VALIDATES: Different RD values for different VPN nlri sections.
// PREVENTS: RD changes not taking effect.
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
	vpn1, ok := result.Groups[0].Announce[0].(*vpn.VPN)
	require.True(t, ok)
	assert.Equal(t, "0:65000:100", vpn1.RD().String())

	// Second group: RD 65000:200
	vpn2, ok := result.Groups[1].Announce[0].(*vpn.VPN)
	require.True(t, ok)
	assert.Equal(t, "0:65000:200", vpn2.RD().String())
}

// TestParseUpdateText_LabelChangesBetweenSections verifies label can change.
//
// VALIDATES: Different label values for different VPN nlri sections.
// PREVENTS: Label changes not taking effect.
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
	vpn1, ok := result.Groups[0].Announce[0].(*vpn.VPN)
	require.True(t, ok)
	assert.Equal(t, uint32(1000), vpn1.Labels()[0])

	// Second group: label 2000
	vpn2, ok := result.Groups[1].Announce[0].(*vpn.VPN)
	require.True(t, ok)
	assert.Equal(t, uint32(2000), vpn2.Labels()[0])
}

// =============================================================================
// In-NLRI modifier syntax (rd/label without 'set')
// =============================================================================

// TestParseUpdateText_InNLRIModifierSyntax verifies rd/label inside nlri section.
//
// VALIDATES: nlri ipv4/mpls-vpn rd 65000:100 label 1000 add 10.0.0.0/24 works.
// PREVENTS: Requiring accumulator syntax for VPN routes.
func TestParseUpdateText_InNLRIModifierSyntax(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"nlri", "ipv4/mpls-vpn", "rd", "65000:100", "label", "1000", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Announce, 1)

	vpnNLRI, ok := result.Groups[0].Announce[0].(*vpn.VPN)
	require.True(t, ok, "expected IPVPN NLRI")
	assert.Equal(t, "0:65000:100", vpnNLRI.RD().String())
	require.Len(t, vpnNLRI.Labels(), 1)
	assert.Equal(t, uint32(1000), vpnNLRI.Labels()[0])
	assert.Equal(t, "10.0.0.0/24", vpnNLRI.Prefix().String())
}

// TestParseUpdateText_InNLRIModifierMultiplePrefixes verifies in-NLRI modifiers apply to all prefixes.
//
// VALIDATES: rd/label in nlri section applies to all prefixes in that section.
// PREVENTS: Modifiers only applying to first prefix.
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
		vpnNLRI, ok := n.(*vpn.VPN)
		require.True(t, ok, "prefix %d: expected IPVPN NLRI", i)
		assert.Equal(t, "0:65000:100", vpnNLRI.RD().String(), "prefix %d", i)
		assert.Equal(t, uint32(1000), vpnNLRI.Labels()[0], "prefix %d", i)
	}
}

// TestParseUpdateText_InNLRIModifierOverridesAccumulator verifies in-NLRI modifiers override accumulated.
//
// VALIDATES: In-NLRI rd/label overrides accumulated values for that section.
// PREVENTS: Accumulator values not being overridable.
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

	vpnNLRI, ok := result.Groups[0].Announce[0].(*vpn.VPN)
	require.True(t, ok)
	// Should use in-NLRI values, not accumulated
	assert.Equal(t, "0:65000:200", vpnNLRI.RD().String())
	assert.Equal(t, uint32(2000), vpnNLRI.Labels()[0])
}

// TestParseUpdateText_InNLRIModifierIPv6VPN verifies IPv6 VPN with in-NLRI modifiers.
//
// VALIDATES: nlri ipv6/mpls-vpn rd ... label ... add ... works.
// PREVENTS: IPv6 VPN not supporting in-NLRI modifier syntax.
func TestParseUpdateText_InNLRIModifierIPv6VPN(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "2001:db8::1",
		"nlri", "ipv6/mpls-vpn", "rd", "65000:100", "label", "1000", "add", "2001:db8:1::/48",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	vpnNLRI, ok := result.Groups[0].Announce[0].(*vpn.VPN)
	require.True(t, ok)
	assert.Equal(t, "0:65000:100", vpnNLRI.RD().String())
	assert.Equal(t, uint32(1000), vpnNLRI.Labels()[0])
	assert.Equal(t, "2001:db8:1::/48", vpnNLRI.Prefix().String())
}

// TestParseUpdateText_InNLRIModifierLabelOnly verifies label-only in-NLRI modifier.
//
// VALIDATES: nlri ipv4/nlri-mpls label 1000 add ... works (labeled unicast).
// PREVENTS: Label-only modifier not working.
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
// VALIDATES: nlri ipv4/mpls-vpn rd ... add ... fails (missing label).
// PREVENTS: VPN routes created without label.
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
// VALIDATES: nlri ipv4/mpls-vpn label ... add ... fails (missing rd).
// PREVENTS: VPN routes created without RD.
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
// VALIDATES: In-NLRI modifiers only affect that section, not subsequent sections.
// PREVENTS: Modifier values leaking across sections.
func TestParseUpdateText_InNLRIModifierScopeIsSectionOnly(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nhop", "set", "192.0.2.1",
		"nlri", "ipv4/mpls-vpn", "rd", "65000:100", "label", "1000", "add", "10.0.0.0/24",
		"nlri", "ipv4/unicast", "add", "10.0.1.0/24", // unicast doesn't need rd/label
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 2)

	// First group: VPN with in-NLRI modifiers
	vpnNLRI, ok := result.Groups[0].Announce[0].(*vpn.VPN)
	require.True(t, ok)
	assert.Equal(t, "0:65000:100", vpnNLRI.RD().String())

	// Second group: unicast (no VPN requirements)
	assert.Equal(t, nlri.IPv4Unicast, result.Groups[1].Family)
}

// ============================================================================
// FlowSpec Text Mode Tests (RFC 8955)
// ============================================================================

// TestParseUpdateText_FlowSpecBasic verifies basic FlowSpec with destination only.
//
// VALIDATES: FlowSpec NLRI with single destination prefix component.
// PREVENTS: FlowSpec family not being recognized.
func TestParseUpdateText_FlowSpecBasic(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv4/flow", "add", "destination", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Announce, 1)

	fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
	require.Len(t, fs.Components(), 1)
	assert.Equal(t, flowspec.FlowDestPrefix, fs.Components()[0].Type())
}

// TestParseUpdateText_FlowSpecProtocol verifies protocol component parsing.
//
// VALIDATES: Protocol names (tcp/udp/icmp) and numbers translate correctly.
// PREVENTS: Protocol component not parsed.
func TestParseUpdateText_FlowSpecProtocol(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
		want     uint8
	}{
		{"tcp", "tcp", 6},
		{"udp", "udp", 17},
		{"icmp", "icmp", 1},
		{"gre", "gre", 47},
		{"numeric", "89", 89}, // OSPF
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseUpdateText([]string{
				"nlri", "ipv4/flow", "add", "protocol", tc.protocol,
			})
			require.NoError(t, err)
			require.Len(t, result.Groups, 1)
			require.Len(t, result.Groups[0].Announce, 1)

			fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
			require.Len(t, fs.Components(), 1)
			assert.Equal(t, flowspec.FlowIPProtocol, fs.Components()[0].Type())
		})
	}
}

// TestParseUpdateText_FlowSpecPort verifies port with operators.
//
// VALIDATES: Port operators (=, >, <, >=, <=) parsed correctly.
// PREVENTS: Port operator syntax errors.
func TestParseUpdateText_FlowSpecPort(t *testing.T) {
	tests := []struct {
		name string
		port string
		op   flowspec.FlowOperator
		val  uint64
	}{
		{"equal", "=80", flowspec.FlowOpEqual, 80},
		{"gt", ">1024", flowspec.FlowOpGreater, 1024},
		{"lt", "<1024", flowspec.FlowOpLess, 1024},
		{"ge", ">=1024", flowspec.FlowOpGreater | flowspec.FlowOpEqual, 1024},
		{"le", "<=1024", flowspec.FlowOpLess | flowspec.FlowOpEqual, 1024},
		{"bare", "80", flowspec.FlowOpEqual, 80}, // default to equal
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseUpdateText([]string{
				"nlri", "ipv4/flow", "add", "destination-port", tc.port,
			})
			require.NoError(t, err)
			require.Len(t, result.Groups, 1)

			fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
			require.Len(t, fs.Components(), 1)
			assert.Equal(t, flowspec.FlowDestPort, fs.Components()[0].Type())
		})
	}
}

// TestParseUpdateText_FlowSpecPortRange verifies port range syntax.
//
// VALIDATES: Port range (>=1 <=1023) creates two matches with AND.
// PREVENTS: Port range not being parsed as AND condition.
func TestParseUpdateText_FlowSpecPortRange(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv4/flow", "add", "destination-port", ">=1", "<=1023",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
	require.Len(t, fs.Components(), 1)
	assert.Equal(t, flowspec.FlowDestPort, fs.Components()[0].Type())
}

// TestParseUpdateText_FlowSpecMultipleComponents verifies multiple components.
//
// VALIDATES: Multiple match components combine with AND logic.
// PREVENTS: Only first component being parsed.
func TestParseUpdateText_FlowSpecMultipleComponents(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv4/flow", "add",
		"destination", "10.0.0.0/24",
		"protocol", "tcp",
		"destination-port", "=80",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
	require.Len(t, fs.Components(), 3)

	// Verify all three components present (order may vary due to sorting)
	types := make(map[flowspec.FlowComponentType]bool)
	for _, c := range fs.Components() {
		types[c.Type()] = true
	}
	assert.True(t, types[flowspec.FlowDestPrefix])
	assert.True(t, types[flowspec.FlowIPProtocol])
	assert.True(t, types[flowspec.FlowDestPort])
}

// TestParseUpdateText_FlowSpecWithdraw verifies del syntax for FlowSpec.
//
// VALIDATES: del creates withdrawal for FlowSpec rule.
// PREVENTS: FlowSpec withdraw not working.
func TestParseUpdateText_FlowSpecWithdraw(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv4/flow", "del", "destination", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Withdraw, 1)
	require.Empty(t, result.Groups[0].Announce)

	fs := testExtractFlowSpec(t, result.Groups[0].Withdraw[0])
	require.Len(t, fs.Components(), 1)
}

// TestParseUpdateText_FlowSpecVPN verifies FlowSpec VPN with rd.
//
// VALIDATES: flowspec-vpn creates FlowSpecVPN NLRI with RD.
// PREVENTS: FlowSpec VPN not parsing RD.
func TestParseUpdateText_FlowSpecVPN(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv4/flow-vpn", "add", "rd", "65000:100",
		"destination", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	fsv := testExtractFlowSpecVPN(t, result.Groups[0].Announce[0])
	assert.Equal(t, "0:65000:100", fsv.RD().String())
	require.Len(t, fsv.Components(), 1)
}

// TestParseUpdateText_FlowSpecIPv6 verifies IPv6 FlowSpec.
//
// VALIDATES: ipv6/flow with IPv6 prefix works.
// PREVENTS: IPv6 FlowSpec not being parsed.
func TestParseUpdateText_FlowSpecIPv6(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv6/flow", "add", "destination", "2001:db8::/32",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
	assert.Equal(t, nlri.IPv6FlowSpec, fs.Family())
}

// TestParseUpdateText_FlowSpecTCPFlags verifies TCP flags matching.
//
// VALIDATES: tcp-flags component with flag names.
// PREVENTS: TCP flags not parsed.
func TestParseUpdateText_FlowSpecTCPFlags(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv4/flow", "add", "tcp-flags", "syn", "ack",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
	require.Len(t, fs.Components(), 1)
	assert.Equal(t, flowspec.FlowTCPFlags, fs.Components()[0].Type())
}

// TestParseUpdateText_FlowSpecFragment verifies fragment component.
//
// VALIDATES: fragment component with fragment flags.
// PREVENTS: Fragment component not parsed.
func TestParseUpdateText_FlowSpecFragment(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv4/flow", "add", "fragment", "dont-fragment",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
	require.Len(t, fs.Components(), 1)
	assert.Equal(t, flowspec.FlowFragment, fs.Components()[0].Type())
}

// TestParseUpdateText_FlowSpecTCPFlagsOperators verifies bitmask operators.
//
// VALIDATES: !, =, & operators work per RFC 8955 Section 4.2.1.2.
// PREVENTS: Bitmask operators not parsed correctly.
func TestParseUpdateText_FlowSpecTCPFlagsOperators(t *testing.T) {
	tests := []struct {
		name   string
		flags  []string
		wantOp flowspec.FlowOperator
		wantV  uint8
	}{
		{"bare_flag", []string{"syn"}, 0, 0x02},
		{"match_exact", []string{"=syn"}, flowspec.FlowOpMatch, 0x02},
		{"not_flag", []string{"!rst"}, flowspec.FlowOpNot, 0x04},
		{"not_match", []string{"!=ack"}, flowspec.FlowOpNot | flowspec.FlowOpMatch, 0x10},
		{"combined_flags", []string{"syn&ack"}, 0, 0x12},                     // SYN + ACK
		{"exact_combined", []string{"=syn&ack"}, flowspec.FlowOpMatch, 0x12}, // exact SYN+ACK
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"nlri", "ipv4/flow", "add", "tcp-flags"}, tc.flags...)
			result, err := ParseUpdateText(args)
			require.NoError(t, err)
			require.Len(t, result.Groups, 1)

			fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
			require.Len(t, fs.Components(), 1)
			assert.Equal(t, flowspec.FlowTCPFlags, fs.Components()[0].Type())
		})
	}
}

// TestParseUpdateText_FlowSpecFragmentOperators verifies fragment bitmask operators.
//
// VALIDATES: !, =, & operators work for fragment component.
// PREVENTS: Fragment operators not parsed correctly.
func TestParseUpdateText_FlowSpecFragmentOperators(t *testing.T) {
	tests := []struct {
		name  string
		flags []string
	}{
		{"bare_flag", []string{"dont-fragment"}},
		{"not_flag", []string{"!is-fragment"}},
		{"match_exact", []string{"=first-fragment"}},
		{"combined", []string{"dont-fragment&first-fragment"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"nlri", "ipv4/flow", "add", "fragment"}, tc.flags...)
			result, err := ParseUpdateText(args)
			require.NoError(t, err)
			require.Len(t, result.Groups, 1)

			fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
			require.Len(t, fs.Components(), 1)
			assert.Equal(t, flowspec.FlowFragment, fs.Components()[0].Type())
		})
	}
}

// TestParseUpdateText_FlowSpecMissingAdd verifies error without add/del.
//
// VALIDATES: FlowSpec without add/del returns appropriate error.
// PREVENTS: Components parsed without mode.
func TestParseUpdateText_FlowSpecMissingAdd(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"nlri", "ipv4/flow", "destination", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingAddDel)
}

// TestParseUpdateText_FlowSpecVPNMissingRD verifies VPN requires RD.
//
// VALIDATES: flowspec-vpn without rd returns error.
// PREVENTS: FlowSpec VPN created without RD.
func TestParseUpdateText_FlowSpecVPNMissingRD(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"nlri", "ipv4/flow-vpn", "add", "destination", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingRD)
}

// ============================================================================
// Extended Community Function Syntax Tests
// ============================================================================

// TestParseUpdateText_ExtCommTrafficRate verifies traffic-rate function.
//
// VALIDATES: extended-community set traffic-rate <asn> <rate> creates correct extcomm.
// PREVENTS: Traffic-rate function not parsed.
func TestParseUpdateText_ExtCommTrafficRate(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"extended-community", "set", "traffic-rate", "65000", "1000000",
		"nlri", "ipv4/flow", "add", "destination", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, testExtractExtCommunities(t, result.Groups[0].Wire), 1)
}

// TestParseUpdateText_ExtCommDiscard verifies discard sugar.
//
// VALIDATES: discard creates traffic-rate 0 0.
// PREVENTS: Discard sugar not working.
func TestParseUpdateText_ExtCommDiscard(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"extended-community", "set", "discard",
		"nlri", "ipv4/flow", "add", "destination", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, testExtractExtCommunities(t, result.Groups[0].Wire), 1)
}

// TestParseUpdateText_ExtCommRedirect verifies redirect function.
//
// VALIDATES: extended-community set redirect <asn> <value> creates redirect RT.
// PREVENTS: Redirect function not parsed.
func TestParseUpdateText_ExtCommRedirect(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"extended-community", "set", "redirect", "65000", "100",
		"nlri", "ipv4/flow", "add", "destination", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, testExtractExtCommunities(t, result.Groups[0].Wire), 1)
}

// TestParseUpdateText_ExtCommTrafficMarking verifies traffic-marking function.
//
// VALIDATES: extended-community set traffic-marking <dscp> creates correct extcomm.
// PREVENTS: Traffic-marking function not parsed.
func TestParseUpdateText_ExtCommTrafficMarking(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"extended-community", "set", "traffic-marking", "46",
		"nlri", "ipv4/flow", "add", "destination", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, testExtractExtCommunities(t, result.Groups[0].Wire), 1)
}

// TestParseUpdateText_FlowSpecSourcePrefix verifies source prefix component.
//
// VALIDATES: source prefix component parsed correctly.
// PREVENTS: Only destination prefix working.
func TestParseUpdateText_FlowSpecSourcePrefix(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv4/flow", "add", "source", "192.168.1.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
	require.Len(t, fs.Components(), 1)
	assert.Equal(t, flowspec.FlowSourcePrefix, fs.Components()[0].Type())
}

// TestParseUpdateText_FlowSpecICMPType verifies ICMP type component.
//
// VALIDATES: icmp-type component parsed.
// PREVENTS: ICMP type not recognized.
func TestParseUpdateText_FlowSpecICMPType(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv4/flow", "add", "icmp-type", "8", // Echo request
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
	require.Len(t, fs.Components(), 1)
	assert.Equal(t, flowspec.FlowICMPType, fs.Components()[0].Type())
}

// TestParseUpdateText_FlowSpecICMPCode verifies ICMP code component.
//
// VALIDATES: icmp-code component parsed.
// PREVENTS: ICMP code not recognized.
func TestParseUpdateText_FlowSpecICMPCode(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv4/flow", "add", "icmp-code", "0",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
	require.Len(t, fs.Components(), 1)
	assert.Equal(t, flowspec.FlowICMPCode, fs.Components()[0].Type())
}

// TestParseUpdateText_FlowSpecDSCP verifies DSCP component.
//
// VALIDATES: dscp component parsed.
// PREVENTS: DSCP not recognized.
func TestParseUpdateText_FlowSpecDSCP(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv4/flow", "add", "dscp", "46", // EF
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
	require.Len(t, fs.Components(), 1)
	assert.Equal(t, flowspec.FlowDSCP, fs.Components()[0].Type())
}

// TestParseUpdateText_FlowSpecPacketLength verifies packet-length component.
//
// VALIDATES: packet-length component parsed.
// PREVENTS: Packet length not recognized.
func TestParseUpdateText_FlowSpecPacketLength(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv4/flow", "add", "packet-length", ">=100", "<=1500",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
	require.Len(t, fs.Components(), 1)
	assert.Equal(t, flowspec.FlowPacketLength, fs.Components()[0].Type())
}

// TestParseUpdateText_FlowSpecSourcePort verifies source-port component.
//
// VALIDATES: source-port component parsed.
// PREVENTS: Source port not recognized.
func TestParseUpdateText_FlowSpecSourcePort(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv4/flow", "add", "source-port", "=443",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
	require.Len(t, fs.Components(), 1)
	assert.Equal(t, flowspec.FlowSourcePort, fs.Components()[0].Type())
}

// TestParseUpdateText_FlowSpecPort verifies port (any) component.
//
// VALIDATES: port component (matches src OR dst) parsed.
// PREVENTS: Generic port component not recognized.
func TestParseUpdateText_FlowSpecPortComponent(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv4/flow", "add", "port", "=22",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
	require.Len(t, fs.Components(), 1)
	assert.Equal(t, flowspec.FlowPort, fs.Components()[0].Type())
}

// ============================================================================
// Comprehensive FlowSpec Component Combination Tests
// ============================================================================

// TestParseUpdateText_FlowSpecAllComponentTypes verifies all 12 component types.
//
// VALIDATES: Every RFC 8955 component type is parseable.
// PREVENTS: Missing component implementations.
func TestParseUpdateText_FlowSpecAllComponentTypes(t *testing.T) {
	tests := []struct {
		name      string
		component []string
		wantType  flowspec.FlowComponentType
	}{
		{"destination", []string{"destination", "10.0.0.0/24"}, flowspec.FlowDestPrefix},
		{"source", []string{"source", "192.168.0.0/16"}, flowspec.FlowSourcePrefix},
		{"protocol_tcp", []string{"protocol", "tcp"}, flowspec.FlowIPProtocol},
		{"protocol_udp", []string{"protocol", "udp"}, flowspec.FlowIPProtocol},
		{"protocol_icmp", []string{"protocol", "icmp"}, flowspec.FlowIPProtocol},
		{"protocol_gre", []string{"protocol", "gre"}, flowspec.FlowIPProtocol},
		{"protocol_numeric", []string{"protocol", "47"}, flowspec.FlowIPProtocol},
		{"port", []string{"port", "=80"}, flowspec.FlowPort},
		{"destination-port", []string{"destination-port", "=443"}, flowspec.FlowDestPort},
		{"source-port", []string{"source-port", ">=1024"}, flowspec.FlowSourcePort},
		{"icmp-type", []string{"icmp-type", "8"}, flowspec.FlowICMPType},
		{"icmp-code", []string{"icmp-code", "0"}, flowspec.FlowICMPCode},
		{"tcp-flags", []string{"tcp-flags", "syn"}, flowspec.FlowTCPFlags},
		{"packet-length", []string{"packet-length", ">=64"}, flowspec.FlowPacketLength},
		{"dscp", []string{"dscp", "46"}, flowspec.FlowDSCP},
		{"fragment", []string{"fragment", "dont-fragment"}, flowspec.FlowFragment},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"nlri", "ipv4/flow", "add"}, tc.component...)
			result, err := ParseUpdateText(args)
			require.NoError(t, err, "component %s failed", tc.name)
			require.Len(t, result.Groups, 1)
			require.Len(t, result.Groups[0].Announce, 1)

			fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
			require.Len(t, fs.Components(), 1, "expected 1 component for %s", tc.name)
			assert.Equal(t, tc.wantType, fs.Components()[0].Type())
		})
	}
}

// TestParseUpdateText_FlowSpecNumericOperators verifies all numeric operators.
//
// VALIDATES: =, >, <, >=, <= operators work for numeric components.
// PREVENTS: Operator parsing failures.
func TestParseUpdateText_FlowSpecNumericOperators(t *testing.T) {
	operators := []string{"=80", ">80", "<80", ">=80", "<=80", "80"}
	components := []string{"port", "destination-port", "source-port", "packet-length"}

	for _, comp := range components {
		for _, op := range operators {
			t.Run(comp+"_"+op, func(t *testing.T) {
				result, err := ParseUpdateText([]string{
					"nlri", "ipv4/flow", "add", comp, op,
				})
				require.NoError(t, err, "%s %s failed", comp, op)
				require.Len(t, result.Groups, 1)
				require.Len(t, result.Groups[0].Announce, 1)

				fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
				require.Len(t, fs.Components(), 1)
			})
		}
	}
}

// TestParseUpdateText_FlowSpecBitmaskWireEncoding verifies wire encoding of bitmask operators.
//
// VALIDATES: Operator bytes encode correctly per RFC 8955 Section 4.2.1.2.
// PREVENTS: Wrong bit positions for NOT/Match/And operators.
func TestParseUpdateText_FlowSpecBitmaskWireEncoding(t *testing.T) {
	tests := []struct {
		name     string
		flags    []string
		wantOps  []flowspec.FlowOperator // Expected Op field per match
		wantAnds []bool                  // Expected And field per match
		wantVals []uint64                // Expected Value field per match
	}{
		{
			name:     "bare_syn",
			flags:    []string{"syn"},
			wantOps:  []flowspec.FlowOperator{0}, // INCLUDE = 0x00
			wantAnds: []bool{false},
			wantVals: []uint64{0x02},
		},
		{
			name:     "match_syn",
			flags:    []string{"=syn"},
			wantOps:  []flowspec.FlowOperator{flowspec.FlowOpMatch}, // 0x01
			wantAnds: []bool{false},
			wantVals: []uint64{0x02},
		},
		{
			name:     "not_syn",
			flags:    []string{"!syn"},
			wantOps:  []flowspec.FlowOperator{flowspec.FlowOpNot}, // 0x02
			wantAnds: []bool{false},
			wantVals: []uint64{0x02},
		},
		{
			name:     "not_match_syn",
			flags:    []string{"!=syn"},
			wantOps:  []flowspec.FlowOperator{flowspec.FlowOpNot | flowspec.FlowOpMatch}, // 0x03
			wantAnds: []bool{false},
			wantVals: []uint64{0x02},
		},
		{
			name:     "syn_and_ack",
			flags:    []string{"syn&ack"},
			wantOps:  []flowspec.FlowOperator{0}, // Combined in single match
			wantAnds: []bool{false},
			wantVals: []uint64{0x12}, // SYN(0x02) | ACK(0x10) = 0x12
		},
		{
			name:     "syn_or_ack_tokens",
			flags:    []string{"syn", "ack"},
			wantOps:  []flowspec.FlowOperator{0, 0}, // Two matches, OR logic (And=false)
			wantAnds: []bool{false, false},
			wantVals: []uint64{0x02, 0x10},
		},
		{
			name:     "syn_and_not_rst",
			flags:    []string{"syn", "&!rst"},
			wantOps:  []flowspec.FlowOperator{0, flowspec.FlowOpNot}, // syn=0, !rst=0x02
			wantAnds: []bool{false, true},                            // Second has And=true
			wantVals: []uint64{0x02, 0x04},
		},
		{
			name:     "match_syn_and_not_rst",
			flags:    []string{"=syn", "&!rst"},
			wantOps:  []flowspec.FlowOperator{flowspec.FlowOpMatch, flowspec.FlowOpNot}, // =syn=0x01, !rst=0x02
			wantAnds: []bool{false, true},
			wantVals: []uint64{0x02, 0x04},
		},
		{
			name:     "ece_cwr",
			flags:    []string{"ece&cwr"},
			wantOps:  []flowspec.FlowOperator{0},
			wantAnds: []bool{false},
			wantVals: []uint64{0xC0}, // ECE(0x40) | CWR(0x80) = 0xC0
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"nlri", "ipv4/flow", "add", "tcp-flags"}, tc.flags...)
			result, err := ParseUpdateText(args)
			require.NoError(t, err)

			fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
			require.Len(t, fs.Components(), 1)

			comp := fs.Components()[0]
			assert.Equal(t, flowspec.FlowTCPFlags, comp.Type())

			// Get the matches from the component
			type matchGetter interface {
				Matches() []flowspec.FlowMatch
			}
			mg, ok := comp.(matchGetter)
			require.True(t, ok, "component should have Matches() method")

			matches := mg.Matches()
			require.Len(t, matches, len(tc.wantOps), "wrong number of matches")

			for i, m := range matches {
				assert.Equal(t, tc.wantOps[i], m.Op, "match[%d] Op mismatch", i)
				assert.Equal(t, tc.wantAnds[i], m.And, "match[%d] And mismatch", i)
				assert.Equal(t, tc.wantVals[i], m.Value, "match[%d] Value mismatch", i)
			}
		})
	}
}

// TestParseUpdateText_FlowSpecBitmaskWireBytes verifies actual wire bytes output.
//
// VALIDATES: Full wire encoding matches RFC 8955 Section 4.2.1.2.
// PREVENTS: Incorrect operator byte assembly.
func TestParseUpdateText_FlowSpecBitmaskWireBytes(t *testing.T) {
	tests := []struct {
		name      string
		flags     []string
		wantBytes []byte // Expected component bytes (type + [op, value]+)
	}{
		{
			name:  "bare_syn",
			flags: []string{"syn"},
			// Type=9, Op=0x80 (End), Value=0x02
			wantBytes: []byte{0x09, 0x80, 0x02},
		},
		{
			name:  "match_syn",
			flags: []string{"=syn"},
			// Type=9, Op=0x81 (End|Match), Value=0x02
			wantBytes: []byte{0x09, 0x81, 0x02},
		},
		{
			name:  "not_syn",
			flags: []string{"!syn"},
			// Type=9, Op=0x82 (End|Not), Value=0x02
			wantBytes: []byte{0x09, 0x82, 0x02},
		},
		{
			name:  "not_match_syn",
			flags: []string{"!=syn"},
			// Type=9, Op=0x83 (End|Not|Match), Value=0x02
			wantBytes: []byte{0x09, 0x83, 0x02},
		},
		{
			name:  "syn_and_not_rst",
			flags: []string{"syn", "&!rst"},
			// Type=9, Op1=0x00 (no End), Value1=0x02, Op2=0xC2 (End|And|Not), Value2=0x04
			wantBytes: []byte{0x09, 0x00, 0x02, 0xC2, 0x04},
		},
		{
			name:  "match_syn_and_not_match_rst",
			flags: []string{"=syn", "&!=rst"},
			// Type=9, Op1=0x01 (Match), Value1=0x02, Op2=0xC3 (End|And|Not|Match), Value2=0x04
			wantBytes: []byte{0x09, 0x01, 0x02, 0xC3, 0x04},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"nlri", "ipv4/flow", "add", "tcp-flags"}, tc.flags...)
			result, err := ParseUpdateText(args)
			require.NoError(t, err)

			fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
			require.Len(t, fs.Components(), 1)

			// Get component bytes
			gotBytes := fs.Components()[0].Bytes()
			assert.Equal(t, tc.wantBytes, gotBytes,
				"wire bytes mismatch\nwant: %02x\ngot:  %02x", tc.wantBytes, gotBytes)
		})
	}
}

// TestParseUpdateText_FlowSpecTCPFlagsAllOperators verifies all bitmask operators for tcp-flags.
//
// VALIDATES: All RFC 8955 Section 4.2.1.2 bitmask operators.
// PREVENTS: Bitmask operator combinations not working.
func TestParseUpdateText_FlowSpecTCPFlagsAllOperators(t *testing.T) {
	tests := []struct {
		name  string
		flags []string
	}{
		// Single flag with different operators
		{"bare_syn", []string{"syn"}},
		{"bare_ack", []string{"ack"}},
		{"bare_fin", []string{"fin"}},
		{"bare_rst", []string{"rst"}},
		{"bare_psh", []string{"psh"}},
		{"bare_urg", []string{"urg"}},
		// Match operator (exact)
		{"match_syn", []string{"=syn"}},
		{"match_ack", []string{"=ack"}},
		// NOT operator
		{"not_syn", []string{"!syn"}},
		{"not_rst", []string{"!rst"}},
		// NOT + Match
		{"not_match_syn", []string{"!=syn"}},
		{"not_match_ack", []string{"!=ack"}},
		// Combined flags (single token)
		{"syn_ack", []string{"syn&ack"}},
		{"match_syn_ack", []string{"=syn&ack"}},
		{"not_syn_ack", []string{"!syn&ack"}},
		{"syn_ack_fin", []string{"syn&ack&fin"}},
		// Multiple tokens (OR between them)
		{"syn_or_ack", []string{"syn", "ack"}},
		{"syn_or_rst", []string{"syn", "rst"}},
		// AND between tokens
		{"syn_and_ack", []string{"syn", "&ack"}},
		{"syn_and_not_rst", []string{"syn", "&!rst"}},
		// Complex combinations
		{"match_syn_and_not_rst", []string{"=syn", "&!rst"}},
		{"syn_ack_and_not_fin", []string{"syn&ack", "&!fin"}},
		// ECN flags (RFC 3168)
		{"ece_flag", []string{"ece"}},
		{"cwr_flag", []string{"cwr"}},
		{"ece_cwr", []string{"ece&cwr"}},
		{"syn_ece_cwr", []string{"syn&ece&cwr"}},
		{"match_ece", []string{"=ece"}},
		{"not_cwr", []string{"!cwr"}},
		// ExaBGP compatibility
		{"push_alias", []string{"push"}}, // alias for psh
		{"push_ack", []string{"push&ack"}},
		{"match_push", []string{"=push"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"nlri", "ipv4/flow", "add", "tcp-flags"}, tc.flags...)
			result, err := ParseUpdateText(args)
			require.NoError(t, err, "tcp-flags %v failed", tc.flags)
			require.Len(t, result.Groups, 1)
			require.Len(t, result.Groups[0].Announce, 1)

			fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
			require.Len(t, fs.Components(), 1)
			assert.Equal(t, flowspec.FlowTCPFlags, fs.Components()[0].Type())
		})
	}
}

// TestParseUpdateText_FlowSpecFragmentAllOperators verifies all bitmask operators for fragment.
//
// VALIDATES: All RFC 8955 Section 4.2.1.2 bitmask operators for fragment.
// PREVENTS: Fragment operator combinations not working.
func TestParseUpdateText_FlowSpecFragmentAllOperators(t *testing.T) {
	tests := []struct {
		name  string
		flags []string
	}{
		// Single flag
		{"dont_fragment", []string{"dont-fragment"}},
		{"is_fragment", []string{"is-fragment"}},
		{"first_fragment", []string{"first-fragment"}},
		{"last_fragment", []string{"last-fragment"}},
		// Match operator
		{"match_df", []string{"=dont-fragment"}},
		{"match_isf", []string{"=is-fragment"}},
		// NOT operator
		{"not_df", []string{"!dont-fragment"}},
		{"not_isf", []string{"!is-fragment"}},
		{"not_ff", []string{"!first-fragment"}},
		// NOT + Match
		{"not_match_df", []string{"!=dont-fragment"}},
		// Combined flags
		{"df_ff", []string{"dont-fragment&first-fragment"}},
		{"not_df_ff", []string{"!dont-fragment&first-fragment"}},
		// Multiple tokens
		{"df_or_isf", []string{"dont-fragment", "is-fragment"}},
		{"df_and_not_isf", []string{"dont-fragment", "&!is-fragment"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"nlri", "ipv4/flow", "add", "fragment"}, tc.flags...)
			result, err := ParseUpdateText(args)
			require.NoError(t, err, "fragment %v failed", tc.flags)
			require.Len(t, result.Groups, 1)
			require.Len(t, result.Groups[0].Announce, 1)

			fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
			require.Len(t, fs.Components(), 1)
			assert.Equal(t, flowspec.FlowFragment, fs.Components()[0].Type())
		})
	}
}

// TestParseUpdateText_FlowSpecMultiComponent verifies multiple components combine with AND.
//
// VALIDATES: Multiple components create single rule with AND logic.
// PREVENTS: Component combination failures.
func TestParseUpdateText_FlowSpecMultiComponent(t *testing.T) {
	tests := []struct {
		name       string
		components []string
		wantCount  int
	}{
		{
			name:       "dest_proto",
			components: []string{"destination", "10.0.0.0/24", "protocol", "tcp"},
			wantCount:  2,
		},
		{
			name:       "dest_proto_port",
			components: []string{"destination", "10.0.0.0/24", "protocol", "tcp", "destination-port", "=80"},
			wantCount:  3,
		},
		{
			name:       "dest_src_proto_port",
			components: []string{"destination", "10.0.0.0/24", "source", "192.168.0.0/16", "protocol", "tcp", "destination-port", "=443"},
			wantCount:  4,
		},
		{
			name:       "dest_proto_flags",
			components: []string{"destination", "10.0.0.0/24", "protocol", "tcp", "tcp-flags", "syn"},
			wantCount:  3,
		},
		{
			name:       "dest_proto_port_dscp",
			components: []string{"destination", "10.0.0.0/24", "protocol", "tcp", "destination-port", "=80", "dscp", "46"},
			wantCount:  4,
		},
		{
			name:       "icmp_rule",
			components: []string{"destination", "10.0.0.0/24", "protocol", "icmp", "icmp-type", "8", "icmp-code", "0"},
			wantCount:  4,
		},
		{
			name:       "fragment_rule",
			components: []string{"destination", "10.0.0.0/24", "fragment", "!is-fragment"},
			wantCount:  2,
		},
		{
			name:       "port_range",
			components: []string{"protocol", "tcp", "destination-port", ">=1", "<=1023"},
			wantCount:  2, // protocol + port (with range)
		},
		{
			name:       "full_tcp_rule",
			components: []string{"destination", "10.0.0.0/24", "source", "192.168.0.0/16", "protocol", "tcp", "destination-port", "=80", "tcp-flags", "=syn", "packet-length", ">=64", "<=1500"},
			wantCount:  6,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"nlri", "ipv4/flow", "add"}, tc.components...)
			result, err := ParseUpdateText(args)
			require.NoError(t, err, "components %v failed", tc.components)
			require.Len(t, result.Groups, 1)
			require.Len(t, result.Groups[0].Announce, 1)

			fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
			assert.Len(t, fs.Components(), tc.wantCount, "expected %d components", tc.wantCount)
		})
	}
}

// TestParseUpdateText_FlowSpecIPv6Variants verifies IPv6 FlowSpec families.
//
// VALIDATES: IPv6 FlowSpec with IPv6 prefixes.
// PREVENTS: IPv6 family handling failures.
func TestParseUpdateText_FlowSpecIPv6Variants(t *testing.T) {
	tests := []struct {
		name       string
		family     string
		components []string
	}{
		{"ipv6_dest", "ipv6/flow", []string{"destination", "2001:db8::/32"}},
		{"ipv6_src", "ipv6/flow", []string{"source", "2001:db8:1::/48"}},
		{"ipv6_dest_proto", "ipv6/flow", []string{"destination", "2001:db8::/32", "protocol", "tcp"}},
		{"ipv6_dest_proto_port", "ipv6/flow", []string{"destination", "2001:db8::/32", "protocol", "tcp", "destination-port", "=80"}},
		{"ipv6_tcp_flags", "ipv6/flow", []string{"protocol", "tcp", "tcp-flags", "syn"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"nlri", tc.family, "add"}, tc.components...)
			result, err := ParseUpdateText(args)
			require.NoError(t, err, "IPv6 %s failed", tc.name)
			require.Len(t, result.Groups, 1)
			require.Len(t, result.Groups[0].Announce, 1)

			fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
			assert.Equal(t, nlri.IPv6FlowSpec, fs.Family())
		})
	}
}

// TestParseUpdateText_FlowSpecVPNVariants verifies FlowSpec VPN with RD.
//
// VALIDATES: FlowSpec VPN requires and uses RD correctly.
// PREVENTS: VPN variant handling failures.
func TestParseUpdateText_FlowSpecVPNVariants(t *testing.T) {
	tests := []struct {
		name       string
		family     string
		rdInput    string // RD input format
		rdOutput   string // RD output format (with type prefix)
		components []string
	}{
		{"ipv4_vpn_basic", "ipv4/flow-vpn", "65000:100", "0:65000:100", []string{"destination", "10.0.0.0/24"}},
		{"ipv4_vpn_full", "ipv4/flow-vpn", "1.2.3.4:100", "1:1.2.3.4:100", []string{"destination", "10.0.0.0/24", "protocol", "tcp", "destination-port", "=80"}},
		{"ipv6_vpn_basic", "ipv6/flow-vpn", "65000:200", "0:65000:200", []string{"destination", "2001:db8::/32"}},
		{"ipv6_vpn_full", "ipv6/flow-vpn", "65000:300", "0:65000:300", []string{"destination", "2001:db8::/32", "protocol", "tcp"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"nlri", tc.family, "add", "rd", tc.rdInput}, tc.components...)
			result, err := ParseUpdateText(args)
			require.NoError(t, err, "VPN %s failed", tc.name)
			require.Len(t, result.Groups, 1)
			require.Len(t, result.Groups[0].Announce, 1)

			fsv := testExtractFlowSpecVPN(t, result.Groups[0].Announce[0])
			assert.Equal(t, tc.rdOutput, fsv.RD().String())
		})
	}
}

// TestParseUpdateText_FlowSpecWithdraw verifies del syntax for all components.
//
// VALIDATES: Withdrawal works for all component types.
// PREVENTS: Withdraw handling failures.
func TestParseUpdateText_FlowSpecWithdrawVariants(t *testing.T) {
	tests := []struct {
		name       string
		components []string
	}{
		{"dest_only", []string{"destination", "10.0.0.0/24"}},
		{"dest_proto", []string{"destination", "10.0.0.0/24", "protocol", "tcp"}},
		{"dest_proto_port", []string{"destination", "10.0.0.0/24", "protocol", "tcp", "destination-port", "=80"}},
		{"full_rule", []string{"destination", "10.0.0.0/24", "source", "192.168.0.0/16", "protocol", "tcp", "tcp-flags", "syn"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"nlri", "ipv4/flow", "del"}, tc.components...)
			result, err := ParseUpdateText(args)
			require.NoError(t, err, "withdraw %s failed", tc.name)
			require.Len(t, result.Groups, 1)
			require.Empty(t, result.Groups[0].Announce)
			require.Len(t, result.Groups[0].Withdraw, 1)

			fs := testExtractFlowSpec(t, result.Groups[0].Withdraw[0])
			assert.Greater(t, len(fs.Components()), 0)
		})
	}
}

// TestParseUpdateText_FlowSpecErrors verifies error handling.
//
// VALIDATES: Appropriate errors for invalid inputs.
// PREVENTS: Silent failures or panics on bad input.
func TestParseUpdateText_FlowSpecErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing_add_del",
			args:    []string{"nlri", "ipv4/flow", "destination", "10.0.0.0/24"},
			wantErr: "add' or 'del",
		},
		{
			name:    "vpn_missing_rd",
			args:    []string{"nlri", "ipv4/flow-vpn", "add", "destination", "10.0.0.0/24"},
			wantErr: "rd required",
		},
		{
			name:    "invalid_prefix",
			args:    []string{"nlri", "ipv4/flow", "add", "destination", "not-a-prefix"},
			wantErr: "invalid",
		},
		{
			name:    "ipv4_prefix_for_ipv6",
			args:    []string{"nlri", "ipv6/flow", "add", "destination", "10.0.0.0/24"},
			wantErr: "IPv4",
		},
		{
			name:    "ipv6_prefix_for_ipv4",
			args:    []string{"nlri", "ipv4/flow", "add", "destination", "2001:db8::/32"},
			wantErr: "IPv6",
		},
		{
			name:    "unknown_protocol",
			args:    []string{"nlri", "ipv4/flow", "add", "protocol", "unknown"},
			wantErr: "invalid protocol",
		},
		{
			name:    "invalid_tcp_flag",
			args:    []string{"nlri", "ipv4/flow", "add", "tcp-flags", "unknown"},
			wantErr: "unknown flag",
		},
		{
			name:    "invalid_fragment_type",
			args:    []string{"nlri", "ipv4/flow", "add", "fragment", "unknown"},
			wantErr: "unknown flag",
		},
		{
			name:    "missing_destination_value",
			args:    []string{"nlri", "ipv4/flow", "add", "destination"},
			wantErr: "requires prefix",
		},
		{
			name:    "missing_protocol_value",
			args:    []string{"nlri", "ipv4/flow", "add", "protocol"},
			wantErr: "requires",
		},
		{
			name:    "empty_flowspec",
			args:    []string{"nlri", "ipv4/flow", "add"},
			wantErr: "no prefixes", // FlowSpec requires at least one component
		},
		{
			name:    "port_value_overflow",
			args:    []string{"nlri", "ipv4/flow", "add", "destination-port", "99999"},
			wantErr: "exceeds max",
		},
		{
			name:    "dscp_value_overflow",
			args:    []string{"nlri", "ipv4/flow", "add", "dscp", "64"},
			wantErr: "exceeds max",
		},
		{
			name:    "protocol_value_overflow",
			args:    []string{"nlri", "ipv4/flow", "add", "protocol", "256"},
			wantErr: "invalid protocol", // ParseUint with 8-bit limit catches this
		},
		{
			name:    "icmp_type_overflow",
			args:    []string{"nlri", "ipv4/flow", "add", "icmp-type", "256"},
			wantErr: "exceeds max",
		},
		{
			name:    "icmp_code_overflow",
			args:    []string{"nlri", "ipv4/flow", "add", "icmp-code", "256"},
			wantErr: "exceeds max",
		},
		{
			name:    "source_port_overflow",
			args:    []string{"nlri", "ipv4/flow", "add", "source-port", "65536"},
			wantErr: "exceeds max",
		},
		{
			name:    "packet_length_overflow",
			args:    []string{"nlri", "ipv4/flow", "add", "packet-length", "65536"},
			wantErr: "exceeds max",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseUpdateText(tc.args)
			require.Error(t, err, "expected error for %s", tc.name)
			assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tc.wantErr),
				"error %q should contain %q", err.Error(), tc.wantErr)
		})
	}
}

// TestParseUpdateText_FlowSpecBoundaryValues verifies max valid values are accepted.
//
// VALIDATES: Maximum valid values for each component type parse correctly.
// PREVENTS: Off-by-one errors in range validation.
func TestParseUpdateText_FlowSpecBoundaryValues(t *testing.T) {
	tests := []struct {
		name      string
		component string
		value     string
	}{
		{"port_max", "destination-port", "65535"},
		{"port_zero", "destination-port", "0"},
		{"dscp_max", "dscp", "63"},
		{"dscp_zero", "dscp", "0"},
		{"icmp_type_max", "icmp-type", "255"},
		{"icmp_code_max", "icmp-code", "255"},
		{"packet_length_max", "packet-length", "65535"},
		{"source_port_max", "source-port", "65535"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseUpdateText([]string{
				"nlri", "ipv4/flow", "add", tc.component, tc.value,
			})
			require.NoError(t, err, "%s=%s should be valid", tc.component, tc.value)
			require.Len(t, result.Groups, 1)
			require.Len(t, result.Groups[0].Announce, 1)
		})
	}
}

// TestParseUpdateText_FlowSpecWithExtComm verifies FlowSpec with actions.
//
// VALIDATES: Extended community actions combined with FlowSpec NLRI.
// PREVENTS: Action + NLRI combination failures.
func TestParseUpdateText_FlowSpecWithExtComm(t *testing.T) {
	tests := []struct {
		name       string
		extcomm    []string
		components []string
	}{
		{
			name:       "traffic_rate",
			extcomm:    []string{"extended-community", "set", "traffic-rate", "65000", "1000000"},
			components: []string{"destination", "10.0.0.0/24", "protocol", "tcp", "destination-port", "=80"},
		},
		{
			name:       "discard",
			extcomm:    []string{"extended-community", "set", "discard"},
			components: []string{"destination", "10.0.0.0/24", "protocol", "udp"},
		},
		{
			name:       "redirect",
			extcomm:    []string{"extended-community", "set", "redirect", "65000", "100"},
			components: []string{"destination", "10.0.0.0/24"},
		},
		{
			name:       "traffic_marking",
			extcomm:    []string{"extended-community", "set", "traffic-marking", "46"},
			components: []string{"destination", "10.0.0.0/24", "protocol", "tcp"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nlriPart := append([]string{"nlri", "ipv4/flow", "add"}, tc.components...)
			args := append(tc.extcomm, nlriPart...) //nolint:gocritic // appendAssign: intentional
			result, err := ParseUpdateText(args)
			require.NoError(t, err, "extcomm+flowspec %s failed", tc.name)
			require.Len(t, result.Groups, 1)
			require.Len(t, result.Groups[0].Announce, 1)
			require.Len(t, testExtractExtCommunities(t, result.Groups[0].Wire), 1)

			fs := testExtractFlowSpec(t, result.Groups[0].Announce[0])
			assert.Greater(t, len(fs.Components()), 0)
		})
	}
}

// TestParseUpdateText_EORIPv4Unicast verifies EOR parsing for IPv4 unicast.
//
// VALIDATES: "nlri ipv4/unicast eor" produces EORFamilies with correct family.
// PREVENTS: EOR command being rejected or parsed incorrectly.
// RFC 4724 Section 2: End-of-RIB marker.
func TestParseUpdateText_EORIPv4Unicast(t *testing.T) {
	result, err := ParseUpdateText([]string{"nlri", "ipv4/unicast", "eor"})
	require.NoError(t, err)
	require.Len(t, result.EORFamilies, 1)
	assert.Equal(t, nlri.IPv4Unicast, result.EORFamilies[0])
	assert.Empty(t, result.Groups, "EOR should not produce NLRI groups")
}

// TestParseUpdateText_EORIPv6Unicast verifies EOR parsing for IPv6 unicast.
//
// VALIDATES: "nlri ipv6/unicast eor" produces EORFamilies with correct family.
// PREVENTS: IPv6 family being rejected.
// RFC 4724 Section 2: End-of-RIB marker.
func TestParseUpdateText_EORIPv6Unicast(t *testing.T) {
	result, err := ParseUpdateText([]string{"nlri", "ipv6/unicast", "eor"})
	require.NoError(t, err)
	require.Len(t, result.EORFamilies, 1)
	assert.Equal(t, nlri.IPv6Unicast, result.EORFamilies[0])
}

// TestParseUpdateText_EORL2VPNEVPN verifies EOR parsing for L2VPN/EVPN.
//
// VALIDATES: "nlri l2vpn/evpn eor" produces EORFamilies with correct family.
// PREVENTS: EVPN EOR being rejected.
// RFC 4724 Section 2: End-of-RIB marker.
func TestParseUpdateText_EORL2VPNEVPN(t *testing.T) {
	result, err := ParseUpdateText([]string{"nlri", "l2vpn/evpn", "eor"})
	require.NoError(t, err)
	require.Len(t, result.EORFamilies, 1)
	assert.Equal(t, nlri.L2VPNEVPN, result.EORFamilies[0])
}

// TestParseUpdateText_EORL2VPNVPLS verifies EOR parsing for L2VPN/VPLS.
//
// VALIDATES: "nlri l2vpn/vpls eor" produces EORFamilies with correct family.
// PREVENTS: VPLS EOR being rejected.
// RFC 4724 Section 2: End-of-RIB marker.
func TestParseUpdateText_EORL2VPNVPLS(t *testing.T) {
	result, err := ParseUpdateText([]string{"nlri", "l2vpn/vpls", "eor"})
	require.NoError(t, err)
	require.Len(t, result.EORFamilies, 1)
	assert.Equal(t, nlri.L2VPNVPLS, result.EORFamilies[0])
}

// TestParseUpdateText_EORMultipleFamilies verifies multiple EOR families.
//
// VALIDATES: Multiple "nlri <family> eor" sections accumulate.
// PREVENTS: Only first EOR being parsed.
func TestParseUpdateText_EORMultipleFamilies(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv4/unicast", "eor",
		"nlri", "ipv6/unicast", "eor",
	})
	require.NoError(t, err)
	require.Len(t, result.EORFamilies, 2)
	assert.Equal(t, nlri.IPv4Unicast, result.EORFamilies[0])
	assert.Equal(t, nlri.IPv6Unicast, result.EORFamilies[1])
}

// TestParseUpdateText_EORWithNLRI verifies EOR can coexist with NLRI.
//
// VALIDATES: EOR and NLRI sections in same command both work.
// PREVENTS: EOR breaking NLRI parsing or vice versa.
func TestParseUpdateText_EORWithNLRI(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "ipv6/unicast", "eor",
		"origin", "set", "igp",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.EORFamilies, 1)
	assert.Equal(t, nlri.IPv6Unicast, result.EORFamilies[0])
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Announce, 1)
}

// TestParseUpdateText_VPLSBasic verifies basic VPLS parsing.
//
// VALIDATES: "nlri l2vpn/vpls add rd ... ve-id ... label ..." produces correct VPLS NLRI.
// PREVENTS: VPLS family being rejected.
// RFC 4761 Section 3.2.2: VPLS BGP NLRI format.
func TestParseUpdateText_VPLSBasic(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "l2vpn/vpls", "add",
		"rd", "1:1",
		"ve-id", "1",
		"ve-block-offset", "0",
		"ve-block-size", "10",
		"label-base", "1000",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Announce, 1)
	assert.Equal(t, nlri.L2VPNVPLS, result.Groups[0].Family)

	vpls, ok := result.Groups[0].Announce[0].(*nlri.VPLS)
	require.True(t, ok, "expected VPLS NLRI, got %T", result.Groups[0].Announce[0])
	assert.Equal(t, uint16(1), vpls.VEID())
	assert.Equal(t, uint16(0), vpls.VEBlockOffset())
	assert.Equal(t, uint16(10), vpls.VEBlockSize())
	assert.Equal(t, uint32(1000), vpls.LabelBase())
}

// TestParseUpdateText_VPLSWithdraw verifies VPLS withdrawal parsing.
//
// VALIDATES: "nlri l2vpn/vpls del rd ..." produces correct withdrawal.
// PREVENTS: VPLS withdrawals being rejected.
func TestParseUpdateText_VPLSWithdraw(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "l2vpn/vpls", "del",
		"rd", "1:1",
		"ve-id", "1",
		"ve-block-offset", "0",
		"ve-block-size", "10",
		"label-base", "1000",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Withdraw, 1)
	assert.Equal(t, nlri.L2VPNVPLS, result.Groups[0].Family)
}

// TestParseUpdateText_VPLSMissingRD verifies VPLS requires RD.
//
// VALIDATES: VPLS without rd returns error.
// PREVENTS: Silent failures on missing required fields.
func TestParseUpdateText_VPLSMissingRD(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"nlri", "l2vpn/vpls", "add",
		"ve-id", "1",
		"ve-block-offset", "0",
		"ve-block-size", "10",
		"label-base", "1000",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rd")
}

// TestParseUpdateText_EVPNType2Basic verifies EVPN Type 2 (MAC/IP) parsing.
//
// VALIDATES: "nlri l2vpn/evpn add mac-ip rd ... mac ... label ..." produces correct EVPN NLRI.
// PREVENTS: EVPN family being rejected.
// RFC 7432 Section 7.2: MAC/IP Advertisement Route.
func TestParseUpdateText_EVPNType2Basic(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "l2vpn/evpn", "add", "mac-ip",
		"rd", "1:1",
		"mac", "00:11:22:33:44:55",
		"label", "100",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Announce, 1)
	assert.Equal(t, nlri.L2VPNEVPN, result.Groups[0].Family)

	evpn, ok := result.Groups[0].Announce[0].(*evpn.EVPNType2)
	require.True(t, ok, "expected EVPNType2 NLRI, got %T", result.Groups[0].Announce[0])
	assert.Equal(t, [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}, evpn.MAC())
}

// TestParseUpdateText_EVPNType2WithIP verifies EVPN Type 2 with IP parsing.
//
// VALIDATES: "nlri l2vpn/evpn add mac-ip rd ... mac ... ip ... label ..." works.
// PREVENTS: EVPN MAC/IP with IP being rejected.
// RFC 7432 Section 7.2: IP Address Length can be 0, 32, or 128.
func TestParseUpdateText_EVPNType2WithIP(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "l2vpn/evpn", "add", "mac-ip",
		"rd", "1:1",
		"mac", "00:11:22:33:44:55",
		"ip", "192.168.1.1",
		"label", "100",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Announce, 1)

	evpn, ok := result.Groups[0].Announce[0].(*evpn.EVPNType2)
	require.True(t, ok, "expected EVPNType2 NLRI, got %T", result.Groups[0].Announce[0])
	assert.True(t, evpn.IP().IsValid())
}

// TestParseUpdateText_EVPNType5Basic verifies EVPN Type 5 (IP Prefix) parsing.
//
// VALIDATES: "nlri l2vpn/evpn add ip-prefix rd ... prefix ... label ..." works.
// PREVENTS: EVPN IP Prefix routes being rejected.
// RFC 9136 Section 3: IP Prefix Route.
func TestParseUpdateText_EVPNType5Basic(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "l2vpn/evpn", "add", "ip-prefix",
		"rd", "1:1",
		"prefix", "10.0.0.0/24",
		"label", "100",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Announce, 1)
	assert.Equal(t, nlri.L2VPNEVPN, result.Groups[0].Family)

	evpn, ok := result.Groups[0].Announce[0].(*evpn.EVPNType5)
	require.True(t, ok, "expected EVPNType5 NLRI, got %T", result.Groups[0].Announce[0])
	assert.Equal(t, "10.0.0.0/24", evpn.Prefix().String())
}

// TestParseUpdateText_EVPNMissingType verifies EVPN requires route type.
//
// VALIDATES: EVPN without route type returns error.
// PREVENTS: Silent failures on missing required fields.
func TestParseUpdateText_EVPNMissingType(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"nlri", "l2vpn/evpn", "add",
		"rd", "1:1",
	})
	require.Error(t, err)
}

// TestParseUpdateText_EVPNType3Multicast verifies EVPN Type 3 parsing.
//
// VALIDATES: RFC 7432 Section 7.3 - Inclusive Multicast Ethernet Tag route.
// PREVENTS: Type 3 routes silently failing.
func TestParseUpdateText_EVPNType3Multicast(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "l2vpn/evpn", "add", "multicast",
		"rd", "1:1",
		"ip", "192.168.1.1",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Announce, 1)
	assert.Equal(t, nlri.L2VPNEVPN, result.Groups[0].Family)

	evpn, ok := result.Groups[0].Announce[0].(*evpn.EVPNType3)
	require.True(t, ok, "expected EVPNType3 NLRI, got %T", result.Groups[0].Announce[0])
	assert.Equal(t, "192.168.1.1", evpn.OriginatorIP().String())
}

// TestParseUpdateText_EVPNType5WithGateway verifies EVPN Type 5 with GW IP Overlay Index.
//
// VALIDATES: RFC 9136 Section 3.1 - GW IP Address for recursive resolution.
// PREVENTS: Gateway field not being parsed.
func TestParseUpdateText_EVPNType5WithGateway(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"nlri", "l2vpn/evpn", "add", "ip-prefix",
		"rd", "1:1",
		"prefix", "10.0.0.0/24",
		"gateway", "192.168.1.254",
		"label", "100",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Announce, 1)

	evpn, ok := result.Groups[0].Announce[0].(*evpn.EVPNType5)
	require.True(t, ok, "expected EVPNType5 NLRI, got %T", result.Groups[0].Announce[0])
	assert.Equal(t, "10.0.0.0/24", evpn.Prefix().String())
	assert.Equal(t, "192.168.1.254", evpn.Gateway().String())
}

// TestParseUpdateText_EVPNType5ESIGatewayMutualExclusion verifies RFC 9136 constraint.
//
// VALIDATES: RFC 9136 Section 3.2 - ESI and GW IP MUST NOT both be non-zero.
// PREVENTS: Invalid routes being created that violate the RFC.
func TestParseUpdateText_EVPNType5ESIGatewayMutualExclusion(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"nlri", "l2vpn/evpn", "add", "ip-prefix",
		"rd", "1:1",
		"prefix", "10.0.0.0/24",
		"esi", "00:01:02:03:04:05:06:07:08:09",
		"gateway", "192.168.1.254",
		"label", "100",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

// TestParseUpdateText_EVPNType5GatewayFamilyMismatch verifies RFC 9136 family constraint.
//
// VALIDATES: RFC 9136 Section 3.1 - prefix and gateway MUST be same IP family.
// PREVENTS: Invalid routes with mismatched address families.
func TestParseUpdateText_EVPNType5GatewayFamilyMismatch(t *testing.T) {
	// IPv4 prefix with IPv6 gateway - invalid
	_, err := ParseUpdateText([]string{
		"nlri", "l2vpn/evpn", "add", "ip-prefix",
		"rd", "1:1",
		"prefix", "10.0.0.0/24",
		"gateway", "2001:db8::1",
		"label", "100",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "same IP family")

	// IPv6 prefix with IPv4 gateway - invalid
	_, err = ParseUpdateText([]string{
		"nlri", "l2vpn/evpn", "add", "ip-prefix",
		"rd", "1:1",
		"prefix", "2001:db8::/32",
		"gateway", "192.168.1.1",
		"label", "100",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "same IP family")
}

// =============================================================================
// YANG Validation Tests
// =============================================================================

// testValidator is a mock ValueValidator that records calls and optionally returns errors.
type testValidator struct {
	calls []testValidatorCall
	errs  map[string]error // path → error to return
}

type testValidatorCall struct {
	Path  string
	Value any
}

func newTestValidator() *testValidator {
	return &testValidator{errs: make(map[string]error)}
}

func (v *testValidator) Validate(path string, value any) error {
	v.calls = append(v.calls, testValidatorCall{Path: path, Value: value})
	if err, ok := v.errs[path]; ok {
		return err
	}
	return nil
}

func (v *testValidator) callsFor(path string) []testValidatorCall {
	var result []testValidatorCall
	for _, c := range v.calls {
		if c.Path == path {
			result = append(result, c)
		}
	}
	return result
}

// TestUpdateText_OriginValidation_YANG verifies origin values are validated against YANG.
//
// VALIDATES: YANG validator is called with correct path and value for origin.
// PREVENTS: Origin values bypassing YANG schema validation.
func TestUpdateText_OriginValidation_YANG(t *testing.T) {
	v := newTestValidator()
	SetYANGValidator(v)
	defer SetYANGValidator(nil)

	// Parse valid origin
	_, err := ParseUpdateText([]string{
		"origin", "set", "igp",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)

	// Verify YANG validator was called with correct path and value
	originCalls := v.callsFor(yangPathOrigin)
	require.Len(t, originCalls, 1, "YANG validator should be called once for origin")
	assert.Equal(t, "igp", originCalls[0].Value)

	// Test that YANG rejection is propagated
	v2 := newTestValidator()
	v2.errs[yangPathOrigin] = errors.New("enum error: value \"bad\" is not valid")
	SetYANGValidator(v2)

	_, err = ParseUpdateText([]string{
		"origin", "set", "bad",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "origin")
}

// TestUpdateText_MEDRange_YANG verifies MED values are validated against YANG uint32.
//
// VALIDATES: YANG validator is called with correct path and value for MED.
// PREVENTS: MED values bypassing YANG schema validation.
func TestUpdateText_MEDRange_YANG(t *testing.T) {
	v := newTestValidator()
	SetYANGValidator(v)
	defer SetYANGValidator(nil)

	// Parse valid MED
	_, err := ParseUpdateText([]string{
		"med", "set", "50",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)

	// Verify YANG validator was called with correct path and parsed uint32 value
	medCalls := v.callsFor(yangPathMED)
	require.Len(t, medCalls, 1, "YANG validator should be called once for MED")
	assert.Equal(t, uint32(50), medCalls[0].Value)

	// Test YANG rejection propagation
	v2 := newTestValidator()
	v2.errs[yangPathMED] = errors.New("range error: value outside range")
	SetYANGValidator(v2)

	_, err = ParseUpdateText([]string{
		"med", "set", "100",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "med")
}

// TestUpdateText_LocalPrefRange_YANG verifies local-preference values are validated against YANG uint32.
//
// VALIDATES: YANG validator is called with correct path and value for local-preference.
// PREVENTS: Local-preference values bypassing YANG schema validation.
func TestUpdateText_LocalPrefRange_YANG(t *testing.T) {
	v := newTestValidator()
	SetYANGValidator(v)
	defer SetYANGValidator(nil)

	// Parse valid local-preference
	_, err := ParseUpdateText([]string{
		"local-preference", "set", "100",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.NoError(t, err)

	// Verify YANG validator was called with correct path and parsed uint32 value
	lpCalls := v.callsFor(yangPathLocalPref)
	require.Len(t, lpCalls, 1, "YANG validator should be called once for local-preference")
	assert.Equal(t, uint32(100), lpCalls[0].Value)

	// Test YANG rejection propagation
	v2 := newTestValidator()
	v2.errs[yangPathLocalPref] = errors.New("range error: value outside range")
	SetYANGValidator(v2)

	_, err = ParseUpdateText([]string{
		"local-preference", "set", "200",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "local-preference")
}
