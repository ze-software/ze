package plugin

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/bgp/nlri"
)

// =============================================================================
// Phase 2: Wire mode parsing tests (hex/b64)
// =============================================================================

// TestParseUpdateWire_HexAttrs verifies hex attribute decoding.
//
// VALIDATES: Hex-encoded attributes are decoded and stored in PathAttributes.Wire.
// PREVENTS: Attribute decoding failures.
func TestParseUpdateWire_HexAttrs(t *testing.T) {
	// 400101 = ORIGIN IGP (flags=0x40, type=1, len=1, value=0)
	result, err := ParseUpdateWire([]string{
		"attr", "set", "400101",
		"nhop", "set", "0a000001", // 10.0.0.1
		"nlri", "ipv4/unicast", "add", "180a0000", // 10.0.0.0/24
	}, WireEncodingHex)
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.NotNil(t, result.Groups[0].Wire)
	assert.Equal(t, []byte{0x40, 0x01, 0x01}, result.Groups[0].Wire.Packed())
}

// TestParseUpdateWire_HexNLRI verifies hex NLRI decoding and splitting.
//
// VALIDATES: Hex-encoded NLRIs are decoded and split correctly.
// PREVENTS: NLRI splitting failures.
func TestParseUpdateWire_HexNLRI(t *testing.T) {
	result, err := ParseUpdateWire([]string{
		"attr", "set", "400101",
		"nhop", "set", "0a000001",
		"nlri", "ipv4/unicast", "add", "180a0000", // 10.0.0.0/24
	}, WireEncodingHex)
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Announce, 1)

	// Verify NLRI is WireNLRI with correct bytes
	wn, ok := result.Groups[0].Announce[0].(*nlri.WireNLRI)
	require.True(t, ok, "expected *WireNLRI")
	assert.Equal(t, []byte{0x18, 0x0a, 0x00, 0x00}, wn.Bytes())
}

// TestParseUpdateWire_HexNhop verifies hex next-hop decoding.
//
// VALIDATES: Hex-encoded next-hop is decoded correctly.
// PREVENTS: Next-hop decoding failures.
func TestParseUpdateWire_HexNhop(t *testing.T) {
	result, err := ParseUpdateWire([]string{
		"attr", "set", "400101",
		"nhop", "set", "0a000001", // 10.0.0.1
		"nlri", "ipv4/unicast", "add", "180a0000",
	}, WireEncodingHex)
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.True(t, result.Groups[0].NextHop.IsExplicit())
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), result.Groups[0].NextHop.Addr)
}

// TestParseUpdateWire_NhopDel verifies nhop del unsets next-hop.
//
// VALIDATES: nhop del clears next-hop for subsequent sections.
// PREVENTS: Next-hop bleeding between families.
func TestParseUpdateWire_NhopDel(t *testing.T) {
	result, err := ParseUpdateWire([]string{
		"attr", "set", "400101",
		"nhop", "set", "0a000001",
		"nlri", "ipv4/unicast", "add", "180a0000",
		"nhop", "del",
		"nlri", "ipv4/unicast", "del", "180b0000", // Withdraw doesn't need nhop
	}, WireEncodingHex)
	require.NoError(t, err)
	require.Len(t, result.Groups, 2)

	// First group has next-hop
	assert.True(t, result.Groups[0].NextHop.IsExplicit())

	// Second group has no next-hop (cleared by nhop del)
	assert.False(t, result.Groups[1].NextHop.IsExplicit())
	assert.False(t, result.Groups[1].NextHop.IsSelf())
}

// TestParseUpdateWire_NhopSetSelf verifies nhop set self.
//
// VALIDATES: nhop set self sets next-hop policy to self.
// PREVENTS: Missing self support in wire mode.
func TestParseUpdateWire_NhopSetSelf(t *testing.T) {
	result, err := ParseUpdateWire([]string{
		"attr", "set", "400101",
		"nhop", "set", "self",
		"nlri", "ipv4/unicast", "add", "180a0000",
	}, WireEncodingHex)
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.True(t, result.Groups[0].NextHop.IsSelf())
}

// TestParseUpdateWire_B64Attrs verifies base64 attribute decoding.
//
// VALIDATES: Base64-encoded attributes are decoded correctly.
// PREVENTS: Base64 decoding failures.
func TestParseUpdateWire_B64Attrs(t *testing.T) {
	// QAEB = 400101 = ORIGIN IGP
	result, err := ParseUpdateWire([]string{
		"attr", "set", "QAEB",
		"nhop", "set", "CgAAAQ==", // 10.0.0.1
		"nlri", "ipv4/unicast", "add", "GAoAAA==", // 10.0.0.0/24
	}, WireEncodingB64)
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.NotNil(t, result.Groups[0].Wire)
	assert.Equal(t, []byte{0x40, 0x01, 0x01}, result.Groups[0].Wire.Packed())
}

// TestParseUpdateWire_B64NLRI verifies base64 NLRI decoding.
//
// VALIDATES: Base64-encoded NLRIs are decoded correctly.
// PREVENTS: Base64 NLRI decoding failures.
func TestParseUpdateWire_B64NLRI(t *testing.T) {
	result, err := ParseUpdateWire([]string{
		"attr", "set", "QAEB",
		"nhop", "set", "CgAAAQ==",
		"nlri", "ipv4/unicast", "add", "GAoAAA==", // 10.0.0.0/24
	}, WireEncodingB64)
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Announce, 1)

	wn, ok := result.Groups[0].Announce[0].(*nlri.WireNLRI)
	require.True(t, ok)
	assert.Equal(t, []byte{0x18, 0x0a, 0x00, 0x00}, wn.Bytes())
}

// TestParseUpdateWire_B64Nhop verifies base64 next-hop decoding.
//
// VALIDATES: Base64-encoded next-hop is decoded correctly.
// PREVENTS: Base64 next-hop decoding failures.
func TestParseUpdateWire_B64Nhop(t *testing.T) {
	result, err := ParseUpdateWire([]string{
		"attr", "set", "QAEB",
		"nhop", "set", "CgAAAQ==", // 10.0.0.1
		"nlri", "ipv4/unicast", "add", "GAoAAA==",
	}, WireEncodingB64)
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.True(t, result.Groups[0].NextHop.IsExplicit())
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), result.Groups[0].NextHop.Addr)
}

// TestParseUpdateWire_InvalidHex verifies invalid hex is rejected.
//
// VALIDATES: Invalid hex encoding returns error.
// PREVENTS: Silent corruption of data.
func TestParseUpdateWire_InvalidHex(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"invalid attr", []string{"attr", "set", "GGGG", "nhop", "set", "0a000001", "nlri", "ipv4/unicast", "add", "180a0000"}},
		{"invalid nhop", []string{"attr", "set", "400101", "nhop", "set", "ZZZZ", "nlri", "ipv4/unicast", "add", "180a0000"}},
		{"invalid nlri", []string{"attr", "set", "400101", "nhop", "set", "0a000001", "nlri", "ipv4/unicast", "add", "XXXX"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseUpdateWire(tt.args, WireEncodingHex)
			require.Error(t, err)
		})
	}
}

// TestParseUpdateWire_InvalidB64 verifies invalid base64 is rejected.
//
// VALIDATES: Invalid base64 encoding returns error.
// PREVENTS: Silent corruption of data.
func TestParseUpdateWire_InvalidB64(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"invalid attr", []string{"attr", "set", "!!!!", "nhop", "set", "CgAAAQ==", "nlri", "ipv4/unicast", "add", "GAoAAA=="}},
		{"invalid nhop", []string{"attr", "set", "QAEB", "nhop", "set", "!!!!", "nlri", "ipv4/unicast", "add", "GAoAAA=="}},
		{"invalid nlri", []string{"attr", "set", "QAEB", "nhop", "set", "CgAAAQ==", "nlri", "ipv4/unicast", "add", "!!!!"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseUpdateWire(tt.args, WireEncodingB64)
			require.Error(t, err)
		})
	}
}

// TestParseUpdateWire_SpacesStripped verifies whitespace is stripped before decode.
//
// VALIDATES: Spaces in hex data are stripped.
// PREVENTS: Decode failures due to formatting.
func TestParseUpdateWire_SpacesStripped(t *testing.T) {
	// Spaces should be stripped: "40 01 01" -> "400101"
	result, err := ParseUpdateWire([]string{
		"attr", "set", "40 01 01",
		"nhop", "set", "0a 00 00 01",
		"nlri", "ipv4/unicast", "add", "18 0a 00 00",
	}, WireEncodingHex)
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, []byte{0x40, 0x01, 0x01}, result.Groups[0].Wire.Packed())
}

// TestParseUpdateWire_MultipleNLRI verifies concatenated NLRIs are split.
//
// VALIDATES: Multiple NLRIs in one hex string are split correctly.
// PREVENTS: NLRI boundary detection failures.
func TestParseUpdateWire_MultipleNLRI(t *testing.T) {
	// Two NLRIs concatenated: 10.0.0.0/24 + 11.0.0.0/24
	result, err := ParseUpdateWire([]string{
		"attr", "set", "400101",
		"nhop", "set", "0a000001",
		"nlri", "ipv4/unicast", "add", "180a0000180b0000",
	}, WireEncodingHex)
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Announce, 2)

	// First NLRI: 10.0.0.0/24
	wn1, ok := result.Groups[0].Announce[0].(*nlri.WireNLRI)
	require.True(t, ok)
	assert.Equal(t, []byte{0x18, 0x0a, 0x00, 0x00}, wn1.Bytes())

	// Second NLRI: 11.0.0.0/24
	wn2, ok := result.Groups[0].Announce[1].(*nlri.WireNLRI)
	require.True(t, ok)
	assert.Equal(t, []byte{0x18, 0x0b, 0x00, 0x00}, wn2.Bytes())
}

// TestParseUpdateWire_AddDel verifies mixed add/del in wire mode.
//
// VALIDATES: Both add and del NLRIs in same section.
// PREVENTS: Missing del support.
func TestParseUpdateWire_AddDel(t *testing.T) {
	result, err := ParseUpdateWire([]string{
		"attr", "set", "400101",
		"nhop", "set", "0a000001",
		"nlri", "ipv4/unicast", "add", "180a0000", "del", "180b0000",
	}, WireEncodingHex)
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.Len(t, result.Groups[0].Announce, 1)
	assert.Len(t, result.Groups[0].Withdraw, 1)
}

// TestParseUpdateWire_NhopPerFamily verifies nhop snapshot per nlri section.
//
// VALIDATES: Each nlri section gets its own nhop snapshot.
// PREVENTS: nhop applying retroactively.
func TestParseUpdateWire_NhopPerFamily(t *testing.T) {
	result, err := ParseUpdateWire([]string{
		"attr", "set", "400101",
		"nhop", "set", "0a000001", // 10.0.0.1
		"nlri", "ipv4/unicast", "add", "180a0000",
		"nhop", "set", "0a000002", // 10.0.0.2
		"nlri", "ipv4/unicast", "add", "180b0000",
	}, WireEncodingHex)
	require.NoError(t, err)
	require.Len(t, result.Groups, 2)

	// First group: 10.0.0.1
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), result.Groups[0].NextHop.Addr)

	// Second group: 10.0.0.2
	assert.Equal(t, netip.MustParseAddr("10.0.0.2"), result.Groups[1].NextHop.Addr)
}

// TestParseUpdateWire_AddPath verifies addpath flag enables path-id in split.
//
// VALIDATES: addpath keyword triggers ADD-PATH aware splitting.
// PREVENTS: Missing addpath support.
func TestParseUpdateWire_AddPath(t *testing.T) {
	// With addpath: [path-id:4][prefix-len][prefix]
	// path-id=1, 10.0.0.0/24 = 00000001 18 0a0000
	result, err := ParseUpdateWire([]string{
		"attr", "set", "400101",
		"nhop", "set", "0a000001",
		"nlri", "ipv4/unicast", "addpath", "add", "00000001180a0000",
	}, WireEncodingHex)
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Announce, 1)

	wn, ok := result.Groups[0].Announce[0].(*nlri.WireNLRI)
	require.True(t, ok)
	assert.True(t, wn.HasAddPath())
	assert.Equal(t, uint32(1), wn.PathID())
}

// TestParseUpdateWire_AddPathSplit verifies correct NLRI splitting with addpath.
//
// VALIDATES: Multiple ADD-PATH NLRIs split correctly.
// PREVENTS: Wrong boundary detection with path-ids.
func TestParseUpdateWire_AddPathSplit(t *testing.T) {
	// Two ADD-PATH NLRIs: path-id=1 10.0.0.0/24 + path-id=2 11.0.0.0/24
	result, err := ParseUpdateWire([]string{
		"attr", "set", "400101",
		"nhop", "set", "0a000001",
		"nlri", "ipv4/unicast", "addpath", "add", "00000001180a000000000002180b0000",
	}, WireEncodingHex)
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Announce, 2)

	wn1, ok := result.Groups[0].Announce[0].(*nlri.WireNLRI)
	require.True(t, ok)
	assert.Equal(t, uint32(1), wn1.PathID())

	wn2, ok := result.Groups[0].Announce[1].(*nlri.WireNLRI)
	require.True(t, ok)
	assert.Equal(t, uint32(2), wn2.PathID())
}

// TestParseUpdateWire_NoAttrsAnnounce verifies error when announce without attr set.
//
// VALIDATES: Wire mode requires attr set for announce.
// PREVENTS: Silent missing attributes.
func TestParseUpdateWire_NoAttrsAnnounce(t *testing.T) {
	_, err := ParseUpdateWire([]string{
		"nhop", "set", "0a000001",
		"nlri", "ipv4/unicast", "add", "180a0000",
	}, WireEncodingHex)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wire mode requires attr set for announce")
}

// TestParseUpdateWire_NoAttrsWithdraw verifies OK when withdraw-only without attr set.
//
// VALIDATES: Withdraw-only commands don't need attributes.
// PREVENTS: False positive errors on valid withdrawals.
func TestParseUpdateWire_NoAttrsWithdraw(t *testing.T) {
	result, err := ParseUpdateWire([]string{
		"nlri", "ipv4/unicast", "del", "180a0000",
	}, WireEncodingHex)
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.Len(t, result.Groups[0].Withdraw, 1)
	assert.Empty(t, result.Groups[0].Announce)
}

// TestParseUpdateWire_IPv6 verifies IPv6 unicast parsing.
//
// VALIDATES: IPv6 family with hex data.
// PREVENTS: IPv6 parsing failures.
func TestParseUpdateWire_IPv6(t *testing.T) {
	// 2001:db8::/32 = 20 20 01 0d b8
	result, err := ParseUpdateWire([]string{
		"attr", "set", "400101",
		"nhop", "set", "20010db8000000000000000000000001", // 2001:db8::1
		"nlri", "ipv6/unicast", "add", "2020010db8",
	}, WireEncodingHex)
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, nlri.IPv6Unicast, result.Groups[0].Family)
	assert.Equal(t, netip.MustParseAddr("2001:db8::1"), result.Groups[0].NextHop.Addr)
}

// TestParseUpdateWire_PathInfoError verifies path-information rejected in wire mode.
//
// VALIDATES: path-information only valid in text mode.
// PREVENTS: Confusion between text and wire mode.
func TestParseUpdateWire_PathInfoError(t *testing.T) {
	_, err := ParseUpdateWire([]string{
		"attr", "set", "400101",
		"nhop", "set", "0a000001",
		"path-information", "1",
		"nlri", "ipv4/unicast", "add", "180a0000",
	}, WireEncodingHex)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path-information only valid in text mode")
}

// TestParseUpdateWire_MultipleAttrSetError verifies attr set can only appear once.
//
// VALIDATES: attr set can only appear once.
// PREVENTS: Ambiguous attribute handling.
func TestParseUpdateWire_MultipleAttrSetError(t *testing.T) {
	_, err := ParseUpdateWire([]string{
		"attr", "set", "400101",
		"attr", "set", "400102",
		"nhop", "set", "0a000001",
		"nlri", "ipv4/unicast", "add", "180a0000",
	}, WireEncodingHex)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "attr set can only appear once")
}

// =============================================================================
// Phase 3: Handler integration tests
// =============================================================================

// TestHandleUpdateHex_Integration verifies full handleUpdateHex flow.
//
// VALIDATES: Hex-encoded update command dispatches to reactor.
// PREVENTS: Handler integration failures.
func TestHandleUpdateHex_Integration(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "10.0.0.1",
	}

	// peer 10.0.0.1 update hex attr set 400101 nhop set 0a000001 nlri ipv4/unicast add 180a0000
	args := []string{
		"attr", "set", "400101", // ORIGIN IGP
		"nhop", "set", "0a000001", // 10.0.0.1
		"nlri", "ipv4/unicast", "add", "180a0000", // 10.0.0.0/24
	}

	resp, err := handleUpdateHex(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	// Verify AnnounceNLRIBatch was called
	require.Len(t, reactor.announcedBatches, 1)
	batch := reactor.announcedBatches[0].batch
	assert.Equal(t, nlri.IPv4Unicast, batch.Family)
	require.NotNil(t, batch.Wire, "Wire attrs should be set")
	assert.Equal(t, []byte{0x40, 0x01, 0x01}, batch.Wire.Packed())
	assert.True(t, batch.NextHop.IsExplicit())
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), batch.NextHop.Addr)
	require.Len(t, batch.NLRIs, 1)
}

// TestHandleUpdateB64_Integration verifies full handleUpdateB64 flow.
//
// VALIDATES: Base64-encoded update command dispatches to reactor.
// PREVENTS: Handler integration failures.
func TestHandleUpdateB64_Integration(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "10.0.0.1",
	}

	// peer 10.0.0.1 update b64 attr set QAEB nhop set CgAAAQ== nlri ipv4/unicast add GAoAAA==
	args := []string{
		"attr", "set", "QAEB", // ORIGIN IGP
		"nhop", "set", "CgAAAQ==", // 10.0.0.1
		"nlri", "ipv4/unicast", "add", "GAoAAA==", // 10.0.0.0/24
	}

	resp, err := handleUpdateB64(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	// Verify AnnounceNLRIBatch was called
	require.Len(t, reactor.announcedBatches, 1)
	batch := reactor.announcedBatches[0].batch
	assert.Equal(t, nlri.IPv4Unicast, batch.Family)
	require.NotNil(t, batch.Wire, "Wire attrs should be set")
	assert.Equal(t, []byte{0x40, 0x01, 0x01}, batch.Wire.Packed())
}

// TestHandleUpdateHex_WithdrawOnly verifies withdrawal-only hex command.
//
// VALIDATES: Withdrawal without attributes works.
// PREVENTS: Withdrawal failures.
func TestHandleUpdateHex_WithdrawOnly(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "10.0.0.1",
	}

	args := []string{
		"nlri", "ipv4/unicast", "del", "180a0000",
	}

	resp, err := handleUpdateHex(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	// Verify WithdrawNLRIBatch was called
	require.Len(t, reactor.withdrawnBatches, 1)
	assert.Empty(t, reactor.announcedBatches)
}

// TestHandleUpdateHex_MixedAddDel verifies mixed announce/withdraw in hex.
//
// VALIDATES: Both announce and withdraw in same command.
// PREVENTS: Missing operations.
func TestHandleUpdateHex_MixedAddDel(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "10.0.0.1",
	}

	args := []string{
		"attr", "set", "400101",
		"nhop", "set", "0a000001",
		"nlri", "ipv4/unicast", "add", "180a0000", "del", "180b0000",
	}

	resp, err := handleUpdateHex(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	// Both announce and withdraw should be called
	require.Len(t, reactor.announcedBatches, 1)
	require.Len(t, reactor.withdrawnBatches, 1)
}
