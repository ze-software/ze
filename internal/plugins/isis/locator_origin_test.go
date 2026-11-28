package isis

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/locatororigin"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func testSubTLV(subType uint8, val []byte) []byte {
	out := make([]byte, 0, 2+len(val))
	out = append(out, subType, byte(len(val)))
	return append(out, val...)
}

func testTLV27Entry(prefix netip.Prefix, subTLVs ...[]byte) []byte {
	return testTLV27RawBits(0, uint8(prefix.Bits()), prefix.Addr(), subTLVs...)
}

func testTLV27RawBits(algo, locSize uint8, addr netip.Addr, subTLVs ...[]byte) []byte {
	var subs []byte
	for _, s := range subTLVs {
		subs = append(subs, s...)
	}
	a16 := addr.As16()
	octets := min((int(locSize)+7)/8, len(a16))
	entry := make([]byte, 0, 8+octets+len(subs))
	entry = binary.BigEndian.AppendUint32(entry, 10) // Metric
	entry = append(entry, 0, algo, locSize)          // Flags, Algorithm, Loc-Size
	entry = append(entry, a16[:octets]...)           // Locator
	entry = append(entry, byte(len(subs)))           // Sub-TLV-len
	return append(entry, subs...)
}

func testTLV27(entries ...[]byte) packet.TLV {
	val := make([]byte, 2)
	binary.BigEndian.PutUint16(val, 0)
	for _, e := range entries {
		val = append(val, e...)
	}
	return packet.TLV{Type: tlvSRv6Locator, Value: val}
}

func testTLV140(addr netip.Addr) packet.TLV {
	a16 := addr.As16()
	return packet.TLV{Type: tlvIPv6TERouterID, Value: append([]byte(nil), a16[:]...)}
}

func testTLV232(addrs ...netip.Addr) packet.TLV {
	val := make([]byte, 0, len(addrs)*packet.IPv6AddrLen)
	for _, a := range addrs {
		a16 := a.As16()
		val = append(val, a16[:]...)
	}
	return packet.TLV{Type: packet.TLVIPv6InterfaceAddress, Value: val}
}

func testTLV236EntryFlags(prefix netip.Prefix, upDown, external bool, subTLVs ...[]byte) []byte {
	var subs []byte
	for _, s := range subTLVs {
		subs = append(subs, s...)
	}
	var flags byte
	if upDown {
		flags |= 0x80
	}
	if external {
		flags |= 0x40
	}
	if len(subs) > 0 {
		flags |= 0x20
	}
	bits := prefix.Bits()
	octets := (bits + 7) / 8
	a16 := prefix.Masked().Addr().As16()
	entry := make([]byte, 0, 6+octets+1+len(subs))
	entry = binary.BigEndian.AppendUint32(entry, 10)
	entry = append(entry, flags, byte(bits))
	entry = append(entry, a16[:octets]...)
	if len(subs) > 0 {
		entry = append(entry, byte(len(subs)))
		entry = append(entry, subs...)
	}
	return entry
}

func testTLV236Entry(prefix netip.Prefix, external bool, subTLVs ...[]byte) []byte {
	return testTLV236EntryFlags(prefix, false, external, subTLVs...)
}

func testTLV132(addrs ...netip.Addr) packet.TLV {
	val := make([]byte, 0, len(addrs)*packet.IPv4AddrLen)
	for _, a := range addrs {
		a4 := a.As4()
		val = append(val, a4[:]...)
	}
	return packet.TLV{Type: packet.TLVIPInterfaceAddress, Value: val}
}

func testTLV134(addr netip.Addr) packet.TLV {
	a4 := addr.As4()
	return packet.TLV{Type: tlvIPv4TERouterID, Value: append([]byte(nil), a4[:]...)}
}

func testTLV135Entry(prefix netip.Prefix, upDown bool, subTLVs ...[]byte) []byte {
	var subs []byte
	for _, s := range subTLVs {
		subs = append(subs, s...)
	}
	bits := prefix.Bits()
	ctrl := byte(bits) & 0x3f
	if upDown {
		ctrl |= 0x80
	}
	if len(subs) > 0 {
		ctrl |= 0x40
	}
	octets := (bits + 7) / 8
	a4 := prefix.Masked().Addr().As4()
	entry := make([]byte, 0, 5+octets+1+len(subs))
	entry = binary.BigEndian.AppendUint32(entry, 10)
	entry = append(entry, ctrl)
	entry = append(entry, a4[:octets]...)
	if len(subs) > 0 {
		entry = append(entry, byte(len(subs)))
		entry = append(entry, subs...)
	}
	return entry
}

func testTLV135(entries ...[]byte) packet.TLV {
	var val []byte
	for _, e := range entries {
		val = append(val, e...)
	}
	return packet.TLV{Type: packet.TLVExtendedIPReach, Value: val}
}

func testTLV236(entries ...[]byte) packet.TLV {
	var val []byte
	for _, e := range entries {
		val = append(val, e...)
	}
	return packet.TLV{Type: packet.TLVIPv6Reachability, Value: val}
}

func testPrefixFlagsSubTLV(flags byte) []byte {
	return testSubTLV(subTLVPrefixAttrFlags, []byte{flags})
}

func testSourceRouterIDSubTLV(addr netip.Addr) []byte {
	a16 := addr.As16()
	return testSubTLV(subTLVIPv6SourceRouterID, a16[:])
}

func indexFromEngine(eng *engine) *locatororigin.Index {
	entries := buildLocatorOriginEntries(
		eng.lsdb.RawSnapshot(lsdb.Level1),
		eng.lsdb.RawSnapshot(lsdb.Level2),
	)
	idx := locatororigin.NewIndex()
	idx.ApplySnapshot(&locatororigin.Snapshot{
		Source:     componentName,
		Generation: 1,
		Active:     true,
		Entries:    entries,
	})
	return idx
}

func newConfiguredLocatorEngine(t *testing.T) *engine {
	t.Helper()
	eng := newEngine(transport.New(&fakeBackend{}))
	cfg, err := parseISISConfig(sec(`{"isis":{"net":"49.0001.0000.0000.0001.00"}}`))
	require.NoError(t, err)
	eng.setConfig(cfg)
	t.Cleanup(eng.shutdown)
	return eng
}

// TestISISLocatorOriginatorSnapshot verifies every RFC 9352, RFC 7794, and
// RFC 5308 rule governing two-level IS-IS SRv6 locator-originator resolution,
// plus compact JSON EventBus publication, replay requests, and graceful stop.
//
// VALIDATES: longest-match TLV 27 lookup, Loc-Size 1..128 bounds, originator
// address/prefix collection across TLV 140/232/236 and multiple levels and
// fragments, missing flags (RFC 9352 Section 7.1), A-flag anycast and multi-
// originator anycast, R/X re-advertisement and IPv6 Source Router ID (sub-TLV
// 12), TLV 236 flag precedence and external bit (RFC 9352 Section 6, RFC 7794
// Section 2.1, RFC 5308 Section 2), undecoded TLV 237 guard, EmitEvent JSON
// transport, snapshot-request replay, and Active=false shutdown.
// PREVENTS: false BGP MUP withdrawals on leaked, anycast, multi-level, or
// unflagged SRv6 locators, or missed withdrawals when an originator is proven.
func TestISISLocatorOriginatorSnapshot(t *testing.T) {
	pe1 := types.LSPID{0x01, 0x00, 0x00, 0x00, 0x00, 0x01, 0, 0}
	pe1Frag1 := types.LSPID{0x01, 0x00, 0x00, 0x00, 0x00, 0x01, 0, 1}
	pe2 := types.LSPID{0x01, 0x00, 0x00, 0x00, 0x00, 0x02, 0, 0}
	pe3 := types.LSPID{0x01, 0x00, 0x00, 0x00, 0x00, 0x03, 0, 0}

	t.Run("longest match and Loc-Size bounds", func(t *testing.T) {
		eng := newConfiguredLocatorEngine(t)
		loc48 := netip.MustParsePrefix("fc00:1::/48")
		loc64 := netip.MustParsePrefix("fc00:1:0:1::/64")
		addrPE1 := netip.MustParseAddr("2001:db8::1")
		addrPE2 := netip.MustParseAddr("2001:db8::2")

		bgplsStoreLSP(t, eng, lsdb.Level1, pe1, 1, 1200, []packet.TLV{
			testTLV140(addrPE1),
			testTLV27(testTLV27Entry(loc48, testPrefixFlagsSubTLV(0))),
		})
		bgplsStoreLSP(t, eng, lsdb.Level1, pe2, 1, 1200, []packet.TLV{
			testTLV140(addrPE2),
			testTLV27(testTLV27Entry(loc64, testPrefixFlagsSubTLV(0))),
		})
		// Invalid Loc-Size 0 and 129 MUST cause the entire TLV 27 to be ignored.
		bgplsStoreLSP(t, eng, lsdb.Level1, pe3, 1, 1200, []packet.TLV{
			testTLV140(netip.MustParseAddr("2001:db8::3")),
			testTLV27(testTLV27RawBits(0, 0, netip.MustParseAddr("fc00:9::"), testPrefixFlagsSubTLV(0))),
			testTLV27(testTLV27RawBits(0, 129, netip.MustParseAddr("fc00:a::"), testPrefixFlagsSubTLV(0))),
		})

		idx := indexFromEngine(eng)
		res64 := idx.Resolve(netip.MustParseAddr("fc00:1:0:1::42"))
		require.True(t, res64.Known)
		assert.Equal(t, loc64, res64.Prefix)
		assert.Equal(t, pe2.SystemID().String(), res64.Originator)

		res48 := idx.Resolve(netip.MustParseAddr("fc00:1:0:2::42"))
		require.True(t, res48.Known)
		assert.Equal(t, loc48, res48.Prefix)
		assert.Equal(t, pe1.SystemID().String(), res48.Originator)

		resUncovered := idx.Resolve(netip.MustParseAddr("fc00:9::1"))
		assert.False(t, resUncovered.Known)
		assert.Equal(t, locatororigin.ReasonNoCoveringLocator, resUncovered.ReasonClass)

		resOver := idx.Resolve(netip.MustParseAddr("fc00:a::1"))
		assert.False(t, resOver.Known)
		assert.Equal(t, locatororigin.ReasonNoCoveringLocator, resOver.ReasonClass)
	})

	t.Run("address set union across TLV 140 TLV 232 host TLV 236 and non-host prefixes", func(t *testing.T) {
		eng := newConfiguredLocatorEngine(t)
		loc := netip.MustParsePrefix("fc00:2::/48")
		teID := netip.MustParseAddr("2001:db8:140::1")
		ifAddr := netip.MustParseAddr("2001:db8:232::1")
		hostAddr := netip.MustParseAddr("2001:db8:236::1")
		hostPfx := netip.PrefixFrom(hostAddr, 128)
		netPfx := netip.MustParsePrefix("2001:db8:236:10::/64")

		bgplsStoreLSP(t, eng, lsdb.Level2, pe1, 1, 1200, []packet.TLV{
			testTLV140(teID),
			testTLV232(ifAddr),
			testTLV236(
				testTLV236Entry(hostPfx, false),
				testTLV236Entry(netPfx, false),
			),
			testTLV27(testTLV27Entry(loc, testPrefixFlagsSubTLV(0))),
		})

		idx := indexFromEngine(eng)
		res := idx.Resolve(netip.MustParseAddr("fc00:2::1"))
		require.True(t, res.Known)
		assert.ElementsMatch(t, []netip.Addr{teID, ifAddr, hostAddr}, res.Addresses)
		assert.ElementsMatch(t, []netip.Prefix{hostPfx, netPfx}, res.Prefixes)
		// Non-/128 prefix address is NOT added to Addresses.
		outcome, _, _ := res.MatchAddress(netPfx.Addr())
		assert.Equal(t, locatororigin.MatchMismatch, outcome)
		pfxOutcome, _, _ := res.MatchPrefix(netPfx)
		assert.Equal(t, locatororigin.MatchKnown, pfxOutcome)
	})

	t.Run("two R/X-clear systems are anycast and TLV 237 guards unmatched addresses", func(t *testing.T) {
		eng := newConfiguredLocatorEngine(t)
		anycastLoc := netip.MustParsePrefix("fc00:3::/48")
		tlv237Loc := netip.MustParsePrefix("fc00:4::/48")
		addrPE1 := netip.MustParseAddr("2001:db8::1")
		addrPE2 := netip.MustParseAddr("2001:db8::2")
		addrPE3 := netip.MustParseAddr("2001:db8::3")

		bgplsStoreLSP(t, eng, lsdb.Level1, pe1, 1, 1200, []packet.TLV{
			testTLV140(addrPE1),
			testTLV27(testTLV27Entry(anycastLoc, testPrefixFlagsSubTLV(0))),
		})
		bgplsStoreLSP(t, eng, lsdb.Level1, pe2, 1, 1200, []packet.TLV{
			testTLV140(addrPE2),
			testTLV27(testTLV27Entry(anycastLoc, testPrefixFlagsSubTLV(0))),
		})
		bgplsStoreLSP(t, eng, lsdb.Level1, pe3, 1, 1200, []packet.TLV{
			testTLV140(addrPE3),
			packet.TLV{Type: tlvMTIPv6Reachability, Value: []byte{0, 2, 0, 0, 0, 10, 0, 64, 0x20, 0x01}},
			testTLV27(testTLV27Entry(tlv237Loc, testPrefixFlagsSubTLV(0))),
		})

		idx := indexFromEngine(eng)
		resAny := idx.Resolve(netip.MustParseAddr("fc00:3::1"))
		assert.False(t, resAny.Known)
		assert.Equal(t, locatororigin.ReasonAnycast, resAny.ReasonClass)
		assert.Contains(t, resAny.Reason, "anycast")

		res237 := idx.Resolve(netip.MustParseAddr("fc00:4::1"))
		require.True(t, res237.Known)
		assert.True(t, res237.HasUndecodedTLV237)
		okMatch, _, _ := res237.MatchAddress(addrPE3)
		assert.Equal(t, locatororigin.MatchKnown, okMatch)
		unknownMatch, cls, reason := res237.MatchAddress(netip.MustParseAddr("2001:db8::99"))
		assert.Equal(t, locatororigin.MatchUnknown, unknownMatch)
		assert.Equal(t, locatororigin.ReasonUndecodedTLV237, cls)
		assert.Contains(t, reason, "237")
	})

	t.Run("multi-level L1L2 same system across levels and fragments is one originator", func(t *testing.T) {
		eng := newConfiguredLocatorEngine(t)
		loc := netip.MustParsePrefix("fc00:5::/48")
		addrL1Frag0 := netip.MustParseAddr("2001:db8:5::1")
		addrL1Frag1 := netip.MustParseAddr("2001:db8:5::2")
		addrL2Frag0 := netip.MustParseAddr("2001:db8:5::3")

		bgplsStoreLSP(t, eng, lsdb.Level1, pe1, 1, 1200, []packet.TLV{
			testTLV140(addrL1Frag0),
			testTLV27(testTLV27Entry(loc, testPrefixFlagsSubTLV(0))),
		})
		bgplsStoreLSP(t, eng, lsdb.Level1, pe1Frag1, 1, 1200, []packet.TLV{
			testTLV232(addrL1Frag1),
		})
		bgplsStoreLSP(t, eng, lsdb.Level2, pe1, 1, 1200, []packet.TLV{
			testTLV236(testTLV236Entry(netip.PrefixFrom(addrL2Frag0, 128), false)),
			testTLV27(testTLV27Entry(loc, testPrefixFlagsSubTLV(0))),
		})

		idx := indexFromEngine(eng)
		res := idx.Resolve(netip.MustParseAddr("fc00:5::1"))
		require.True(t, res.Known)
		assert.Equal(t, pe1.SystemID().String(), res.Originator)
		assert.ElementsMatch(t, []netip.Addr{addrL1Frag0, addrL1Frag1, addrL2Frag0}, res.Addresses)
	})

	t.Run("L2 copy with R set beside L1 original resolves to L1 system", func(t *testing.T) {
		eng := newConfiguredLocatorEngine(t)
		loc := netip.MustParsePrefix("fc00:6::/48")
		addrPE1 := netip.MustParseAddr("2001:db8:6::1")
		addrPE2 := netip.MustParseAddr("2001:db8:6::2")

		bgplsStoreLSP(t, eng, lsdb.Level1, pe1, 1, 1200, []packet.TLV{
			testTLV140(addrPE1),
			testTLV27(testTLV27Entry(loc, testPrefixFlagsSubTLV(0))),
		})
		bgplsStoreLSP(t, eng, lsdb.Level2, pe2, 1, 1200, []packet.TLV{
			testTLV140(addrPE2),
			testTLV27(testTLV27Entry(loc, testPrefixFlagsSubTLV(prefixAttrFlagR))),
		})

		idx := indexFromEngine(eng)
		res := idx.Resolve(netip.MustParseAddr("fc00:6::1"))
		require.True(t, res.Known)
		assert.Equal(t, pe1.SystemID().String(), res.Originator)
		assert.Equal(t, []netip.Addr{addrPE1}, res.Addresses)
	})

	t.Run("leaked locator alone with R or X set answers re-advertised and respects Source Router ID", func(t *testing.T) {
		eng := newConfiguredLocatorEngine(t)
		locR := netip.MustParsePrefix("fc00:7::/48")
		locX := netip.MustParsePrefix("fc00:8::/48")
		locSrcID := netip.MustParsePrefix("fc00:9::/48")
		srcRouterID := netip.MustParseAddr("2001:db8:99::1")

		bgplsStoreLSP(t, eng, lsdb.Level2, pe2, 1, 1200, []packet.TLV{
			testTLV27(
				testTLV27Entry(locR, testPrefixFlagsSubTLV(prefixAttrFlagR)),
				testTLV27Entry(locX, testPrefixFlagsSubTLV(prefixAttrFlagX)),
				testTLV27Entry(locSrcID,
					testPrefixFlagsSubTLV(prefixAttrFlagR),
					testSourceRouterIDSubTLV(srcRouterID),
				),
			),
		})

		idx := indexFromEngine(eng)
		resR := idx.Resolve(netip.MustParseAddr("fc00:7::1"))
		assert.False(t, resR.Known)
		assert.Equal(t, locatororigin.ReasonReadvertised, resR.ReasonClass)
		assert.Contains(t, resR.Reason, "re-advertised")
		assert.Contains(t, resR.Reason, pe2.SystemID().String())

		resX := idx.Resolve(netip.MustParseAddr("fc00:8::1"))
		assert.False(t, resX.Known)
		assert.Equal(t, locatororigin.ReasonReadvertised, resX.ReasonClass)
		assert.Contains(t, resX.Reason, "re-advertised")
		assert.Contains(t, resX.Reason, pe2.SystemID().String())

		resSrc := idx.Resolve(netip.MustParseAddr("fc00:9::1"))
		assert.False(t, resSrc.Known)
		assert.Equal(t, locatororigin.ReasonReadvertised, resSrc.ReasonClass)
		matched, _, _ := resSrc.MatchAddress(srcRouterID)
		assert.Equal(t, locatororigin.MatchKnown, matched)
		unmatched, cls, _ := resSrc.MatchAddress(netip.MustParseAddr("2001:db8:99::2"))
		assert.Equal(t, locatororigin.MatchUnknown, unmatched)
		assert.Equal(t, locatororigin.ReasonReadvertised, cls)
	})

	t.Run("missing Prefix Attribute Flags sub-TLV answers not-known", func(t *testing.T) {
		eng := newConfiguredLocatorEngine(t)
		loc := netip.MustParsePrefix("fc00:10::/48")

		bgplsStoreLSP(t, eng, lsdb.Level1, pe1, 1, 1200, []packet.TLV{
			testTLV140(netip.MustParseAddr("2001:db8:10::1")),
			testTLV27(testTLV27Entry(loc)),
		})

		idx := indexFromEngine(eng)
		res := idx.Resolve(netip.MustParseAddr("fc00:10::1"))
		assert.False(t, res.Known)
		assert.Equal(t, locatororigin.ReasonMissingFlags, res.ReasonClass)
		assert.Contains(t, res.Reason, pe1.SystemID().String())
	})

	t.Run("A-flag on any covering advertisement including R-set leaked marks anycast", func(t *testing.T) {
		eng := newConfiguredLocatorEngine(t)
		loc := netip.MustParsePrefix("fc00:11::/48")

		bgplsStoreLSP(t, eng, lsdb.Level1, pe1, 1, 1200, []packet.TLV{
			testTLV140(netip.MustParseAddr("2001:db8:11::1")),
			testTLV27(testTLV27Entry(loc, testPrefixFlagsSubTLV(0))),
		})
		bgplsStoreLSP(t, eng, lsdb.Level2, pe2, 1, 1200, []packet.TLV{
			testTLV140(netip.MustParseAddr("2001:db8:11::2")),
			testTLV27(testTLV27Entry(loc, testPrefixFlagsSubTLV(prefixAttrFlagR|prefixAttrFlagA))),
		})

		idx := indexFromEngine(eng)
		res := idx.Resolve(netip.MustParseAddr("fc00:11::1"))
		assert.False(t, res.Known)
		assert.Equal(t, locatororigin.ReasonAnycast, res.ReasonClass)
	})

	t.Run("RFC 9352 Section 6 and RFC 7794 Section 2.1 TLV 236 flag precedence", func(t *testing.T) {
		eng := newConfiguredLocatorEngine(t)
		locROverride := netip.MustParsePrefix("fc00:12::/48")
		locAOverride := netip.MustParsePrefix("fc00:13::/48")
		locXSubIgnored := netip.MustParsePrefix("fc00:14::/48")
		locExtHeader := netip.MustParsePrefix("fc00:15::/48")
		addrPE1 := netip.MustParseAddr("2001:db8:12::1")

		bgplsStoreLSP(t, eng, lsdb.Level1, pe1, 1, 1200, []packet.TLV{
			testTLV140(addrPE1),
			testTLV27(
				testTLV27Entry(locROverride, testPrefixFlagsSubTLV(0)),
				testTLV27Entry(locAOverride, testPrefixFlagsSubTLV(0)),
				testTLV27Entry(locXSubIgnored, testPrefixFlagsSubTLV(0)),
				testTLV27Entry(locExtHeader, testPrefixFlagsSubTLV(0)),
			),
			testTLV236(
				// TLV 236 sub-TLV R overrides TLV 27 R=0.
				testTLV236Entry(locROverride, false, testPrefixFlagsSubTLV(prefixAttrFlagR)),
				// TLV 236 sub-TLV A overrides TLV 27 A=0.
				testTLV236Entry(locAOverride, false, testPrefixFlagsSubTLV(prefixAttrFlagA)),
				// TLV 236 sub-TLV X-Flag is ignored when fixed-header External is 0.
				testTLV236Entry(locXSubIgnored, false, testPrefixFlagsSubTLV(prefixAttrFlagX)),
				// TLV 236 fixed-header External=1 decides X even with no sub-TLV.
				testTLV236Entry(locExtHeader, true),
			),
		})

		idx := indexFromEngine(eng)
		resR := idx.Resolve(netip.MustParseAddr("fc00:12::1"))
		assert.False(t, resR.Known)
		assert.Equal(t, locatororigin.ReasonReadvertised, resR.ReasonClass)

		resA := idx.Resolve(netip.MustParseAddr("fc00:13::1"))
		assert.False(t, resA.Known)
		assert.Equal(t, locatororigin.ReasonAnycast, resA.ReasonClass)

		resXIgnored := idx.Resolve(netip.MustParseAddr("fc00:14::1"))
		require.True(t, resXIgnored.Known)
		assert.Equal(t, pe1.SystemID().String(), resXIgnored.Originator)

		resExt := idx.Resolve(netip.MustParseAddr("fc00:15::1"))
		assert.False(t, resExt.Known)
		assert.Equal(t, locatororigin.ReasonReadvertised, resExt.ReasonClass)
	})

	t.Run("EmitEvent compact JSON publication replay and Active false shutdown", func(t *testing.T) {
		eng := newConfiguredLocatorEngine(t)
		var mu sync.Mutex
		var snapshots []*locatororigin.Snapshot
		var namespaces []string
		var eventTypes []string

		eng.startLocatorOriginWithEmit(func(_ context.Context, namespace, eventType, _, _, payload string) (int, error) {
			var snap locatororigin.Snapshot
			if err := json.Unmarshal([]byte(payload), &snap); err != nil {
				return 0, err
			}
			mu.Lock()
			namespaces = append(namespaces, namespace)
			eventTypes = append(eventTypes, eventType)
			snapshots = append(snapshots, &snap)
			mu.Unlock()
			return 1, nil
		})

		loc := netip.MustParsePrefix("fc00:20::/48")
		addrPE1 := netip.MustParseAddr("2001:db8:20::1")
		bgplsStoreLSP(t, eng, lsdb.Level1, pe1, 1, 1200, []packet.TLV{
			testTLV140(addrPE1),
			testTLV27(testTLV27Entry(loc, testPrefixFlagsSubTLV(0))),
		})
		eng.publishLSPChange("l1", pe1.String(), 1, "add")

		// Replay via enveloped locator-origin:snapshot-request.
		reqEnv, err := json.Marshal(rpc.EventEnvelope{
			Namespace: locatororigin.Namespace,
			Event:     locatororigin.EventSnapshotRequest,
			Payload:   json.RawMessage("null"),
		})
		require.NoError(t, err)
		assert.True(t, eng.handleLocatorOriginEvent(string(reqEnv)))
		assert.False(t, eng.handleLocatorOriginEvent(`{"namespace":"other","event":"snapshot-request"}`))

		// Graceful shutdown emits Active=false.
		eng.stopLocatorOrigin()
		// Repeated stop is a no-op.
		eng.stopLocatorOrigin()

		mu.Lock()
		defer mu.Unlock()
		require.Len(t, snapshots, 3)
		for i := range snapshots {
			assert.Equal(t, locatororigin.Namespace, namespaces[i])
			assert.Equal(t, locatororigin.EventSnapshot, eventTypes[i])
			assert.Equal(t, componentName, snapshots[i].Source)
		}
		assert.Equal(t, uint64(1), snapshots[0].Generation)
		assert.True(t, snapshots[0].Active)
		require.Len(t, snapshots[0].Entries, 1)
		assert.Equal(t, loc, snapshots[0].Entries[0].Prefix)
		assert.True(t, snapshots[0].Entries[0].Known)

		assert.Equal(t, uint64(2), snapshots[1].Generation)
		assert.True(t, snapshots[1].Active)
		assert.Equal(t, snapshots[0].Entries, snapshots[1].Entries)

		assert.Equal(t, uint64(3), snapshots[2].Generation)
		assert.False(t, snapshots[2].Active)
		assert.Empty(t, snapshots[2].Entries)
	})

	t.Run("external leaked and R-flagged TLV 236 prefixes excluded from originator addresses and prefixes", func(t *testing.T) {
		eng := newConfiguredLocatorEngine(t)
		loc := netip.MustParsePrefix("fc00:21::/48")
		locExt := netip.MustParsePrefix("fc00:22::/48")
		ownHost := netip.MustParseAddr("2001:db8:21::1")
		ownHostPfx := netip.PrefixFrom(ownHost, 128)
		ownNet := netip.MustParsePrefix("2001:db8:21:10::/64")

		extHost := netip.MustParseAddr("2001:db8:21::2")
		extNet := netip.MustParsePrefix("2001:db8:21:20::/64")
		upDownHost := netip.MustParseAddr("2001:db8:21::3")
		upDownNet := netip.MustParsePrefix("2001:db8:21:30::/64")
		rFlagHost := netip.MustParseAddr("2001:db8:21::4")
		rFlagNet := netip.MustParsePrefix("2001:db8:21:40::/64")

		bgplsStoreLSP(t, eng, lsdb.Level1, pe1, 1, 1200, []packet.TLV{
			testTLV27(
				testTLV27Entry(loc, testPrefixFlagsSubTLV(0)),
				testTLV27Entry(locExt, testPrefixFlagsSubTLV(0)),
			),
			testTLV236(
				testTLV236EntryFlags(ownHostPfx, false, false),
				testTLV236EntryFlags(ownNet, false, false),
				testTLV236EntryFlags(netip.PrefixFrom(extHost, 128), false, true),
				testTLV236EntryFlags(extNet, false, true),
				testTLV236EntryFlags(netip.PrefixFrom(upDownHost, 128), true, false),
				testTLV236EntryFlags(upDownNet, true, false),
				testTLV236EntryFlags(netip.PrefixFrom(rFlagHost, 128), false, false, testPrefixFlagsSubTLV(prefixAttrFlagR)),
				testTLV236EntryFlags(rFlagNet, false, false, testPrefixFlagsSubTLV(prefixAttrFlagR)),
				// External TLV 236 for locExt must still apply X precedence on TLV 27.
				testTLV236EntryFlags(locExt, false, true),
			),
		})

		idx := indexFromEngine(eng)
		res := idx.Resolve(netip.MustParseAddr("fc00:21::1"))
		require.True(t, res.Known)
		assert.Equal(t, []netip.Addr{ownHost}, res.Addresses)
		assert.ElementsMatch(t, []netip.Prefix{ownHostPfx, ownNet}, res.Prefixes)

		resExt := idx.Resolve(netip.MustParseAddr("fc00:22::1"))
		assert.False(t, resExt.Known)
		assert.Equal(t, locatororigin.ReasonReadvertised, resExt.ReasonClass)
	})

	t.Run("distinct system advertising same locator without flags returns ReasonMissingFlags", func(t *testing.T) {
		eng := newConfiguredLocatorEngine(t)
		locConflict := netip.MustParsePrefix("fc00:23::/48")
		locSameSys := netip.MustParsePrefix("fc00:24::/48")
		addrPE1 := netip.MustParseAddr("2001:db8:23::1")
		srcID := netip.MustParseAddr("2001:db8:23::99")

		bgplsStoreLSP(t, eng, lsdb.Level1, pe1, 1, 1200, []packet.TLV{
			testTLV140(addrPE1),
			testTLV27(
				testTLV27Entry(locConflict, testPrefixFlagsSubTLV(0)),
				testTLV27Entry(locSameSys, testPrefixFlagsSubTLV(0)),
			),
		})
		// System B advertises locConflict without sub-TLV 4 (carrying a Source Router ID).
		bgplsStoreLSP(t, eng, lsdb.Level1, pe2, 1, 1200, []packet.TLV{
			testTLV27(testTLV27Entry(locConflict, testSourceRouterIDSubTLV(srcID))),
		})
		// Same system (pe1) advertising locSameSys in L2 without sub-TLV 4 remains unambiguous.
		bgplsStoreLSP(t, eng, lsdb.Level2, pe1, 1, 1200, []packet.TLV{
			testTLV27(testTLV27Entry(locSameSys)),
		})

		idx := indexFromEngine(eng)
		resConflict := idx.Resolve(netip.MustParseAddr("fc00:23::1"))
		assert.False(t, resConflict.Known)
		assert.Equal(t, locatororigin.ReasonMissingFlags, resConflict.ReasonClass)
		assert.Equal(t, []netip.Addr{srcID}, resConflict.SourceRouterIDs)
		assert.Contains(t, resConflict.Reason, pe2.SystemID().String())

		resSame := idx.Resolve(netip.MustParseAddr("fc00:24::1"))
		require.True(t, resSame.Known)
		assert.Equal(t, pe1.SystemID().String(), resSame.Originator)
	})

	t.Run("narrowing IS-IS level from l1-l2 to l1 excludes retained L2 LSPs", func(t *testing.T) {
		eng := newConfiguredLocatorEngine(t)
		locL1 := netip.MustParsePrefix("fc00:25::/48")
		locL2 := netip.MustParsePrefix("fc00:26::/48")

		bgplsStoreLSP(t, eng, lsdb.Level1, pe1, 1, 1200, []packet.TLV{
			testTLV140(netip.MustParseAddr("2001:db8:25::1")),
			testTLV27(testTLV27Entry(locL1, testPrefixFlagsSubTLV(0))),
		})
		bgplsStoreLSP(t, eng, lsdb.Level2, pe2, 1, 1200, []packet.TLV{
			testTLV140(netip.MustParseAddr("2001:db8:26::1")),
			testTLV27(testTLV27Entry(locL2, testPrefixFlagsSubTLV(0))),
		})

		var mu sync.Mutex
		var snaps []*locatororigin.Snapshot
		eng.startLocatorOriginWithEmit(func(_ context.Context, _, _, _, _, payload string) (int, error) {
			var snap locatororigin.Snapshot
			if err := json.Unmarshal([]byte(payload), &snap); err != nil {
				return 0, err
			}
			mu.Lock()
			snaps = append(snaps, &snap)
			mu.Unlock()
			return 1, nil
		})

		eng.publishLocatorOrigin()
		cfgL1, err := parseISISConfig(sec(`{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1"}}`))
		require.NoError(t, err)
		eng.setConfig(cfgL1)
		eng.publishLocatorOrigin()

		mu.Lock()
		defer mu.Unlock()
		require.Len(t, snaps, 2)
		assert.Len(t, snaps[0].Entries, 2)
		require.Len(t, snaps[1].Entries, 1)
		assert.Equal(t, locL1, snaps[1].Entries[0].Prefix)
	})

	t.Run("IPv4 TLV 132 134 and local TLV 135 collected while leaked or external TLV 135 excluded", func(t *testing.T) {
		eng := newConfiguredLocatorEngine(t)
		loc := netip.MustParsePrefix("fc00:27::/48")
		ifV4 := netip.MustParseAddr("192.0.2.10")
		teV4 := netip.MustParseAddr("198.51.100.1")
		localV4Host := netip.MustParseAddr("203.0.113.1")
		localV4Net := netip.MustParsePrefix("10.10.0.0/24")
		upDownV4 := netip.MustParsePrefix("10.20.0.0/24")
		rFlagV4 := netip.MustParsePrefix("10.30.0.0/24")
		xFlagV4 := netip.MustParsePrefix("10.40.0.0/24")

		bgplsStoreLSP(t, eng, lsdb.Level1, pe1, 1, 1200, []packet.TLV{
			testTLV132(ifV4),
			testTLV134(teV4),
			testTLV135(
				testTLV135Entry(netip.PrefixFrom(localV4Host, 32), false),
				testTLV135Entry(localV4Net, false),
				testTLV135Entry(upDownV4, true),
				testTLV135Entry(rFlagV4, false, testPrefixFlagsSubTLV(prefixAttrFlagR)),
				testTLV135Entry(xFlagV4, false, testPrefixFlagsSubTLV(prefixAttrFlagX)),
			),
			testTLV27(testTLV27Entry(loc, testPrefixFlagsSubTLV(0))),
		})

		idx := indexFromEngine(eng)
		res := idx.Resolve(netip.MustParseAddr("fc00:27::1"))
		require.True(t, res.Known)

		for _, wantPfx := range []netip.Prefix{
			netip.PrefixFrom(ifV4, 32),
			netip.PrefixFrom(teV4, 32),
			netip.PrefixFrom(localV4Host, 32),
			localV4Net,
		} {
			out, _, _ := res.MatchPrefix(wantPfx)
			assert.Equal(t, locatororigin.MatchKnown, out, "expected %s to match originator", wantPfx)
		}
		for _, badPfx := range []netip.Prefix{upDownV4, rFlagV4, xFlagV4, netip.MustParsePrefix("10.99.0.0/24")} {
			out, _, _ := res.MatchPrefix(badPfx)
			assert.Equal(t, locatororigin.MatchMismatch, out, "expected %s to mismatch originator", badPfx)
		}
		// DSD MatchAddress still rejects 4-octet IPv4 next hop against a known IPv6 locator originator.
		dsdOut, _, _ := res.MatchAddress(ifV4)
		assert.Equal(t, locatororigin.MatchMismatch, dsdOut)
	})
}
