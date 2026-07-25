// Design: plan/spec-isis-12-ipv6.md -- IPv6 origination tests.
//
// VALIDATES: own LSP carries TLV 236 entries for local non-link-local IPv6
// prefixes and a fe80::/10 link-local prefix is excluded (RFC 5308 sec 2,
// AC-1); TLV 232 in the LSP carries only non-link-local addresses (RFC 5308
// sec 3, AC-4); TLV 129 NLPID list includes 0x8E + 0xCC when dual-stack and
// only 0xCC when IPv4-only (RFC 5308 sec 4 / RFC 1195, AC-1). Boundary: TLV 236
// prefix length 0 and 128 round-trip.

package lsdb

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// dualStackNode returns sampleNode with IPv6 advertised (NLPID 0x8E added).
func dualStackNode(t *testing.T) NodeInfo {
	t.Helper()
	n := sampleNode(t)
	n.AdvertiseIPv6 = true
	return n
}

// findTLV236Entries decodes every TLV 236 in an LSP body into one entry slice.
func findTLV236Entries(t *testing.T, lsp packet.LSP) []packet.IPv6ReachEntry {
	t.Helper()
	var out []packet.IPv6ReachEntry
	for _, tl := range lsp.TLVs {
		if tl.Type != packet.TLVIPv6Reachability {
			continue
		}
		dec, err := packet.DecodeIPv6ReachabilityTLV(tl.Value)
		if err != nil {
			t.Fatalf("decode TLV 236: %v", err)
		}
		out = append(out, dec.Entries...)
	}
	return out
}

// findTLV232Addrs decodes every TLV 232 in an LSP body into one address slice.
func findTLV232Addrs(t *testing.T, lsp packet.LSP) []netip.Addr {
	t.Helper()
	var out []netip.Addr
	for _, tl := range lsp.TLVs {
		if tl.Type != packet.TLVIPv6InterfaceAddress {
			continue
		}
		dec, err := packet.DecodeIPv6InterfaceAddrTLV(tl.Value)
		if err != nil {
			t.Fatalf("decode TLV 232: %v", err)
		}
		out = append(out, dec.Addresses...)
	}
	return out
}

// TestISISOriginateTLV236 -- own LSP carries TLV 236 for non-link-local IPv6
// prefixes and excludes a fe80::/10 link-local prefix (RFC 5308 sec 2, AC-1).
//
// RFC requirement: RFC5308-2-1 positive -- the routable non-link-local prefixes 2001:db8:1::/64 and 2001:db8:2::/48 appear in TLV 236.
// RFC requirement: RFC5308-2-1 negative -- the fe80::/64 link-local prefix is excluded from TLV 236.
func TestISISOriginateTLV236(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)
	node := dualStackNode(t)

	// Two routable prefixes plus one link-local prefix; the engine filters
	// link-local before building state, so model that with NonLinkLocalV6Prefixes.
	raw := []PrefixInfoV6{
		{Prefix: netip.MustParsePrefix("2001:db8:1::/64"), Metric: types.NewPrefixMetric(10)},
		{Prefix: netip.MustParsePrefix("2001:db8:2::/48"), Metric: types.NewPrefixMetric(20), External: true},
		{Prefix: netip.MustParsePrefix("fe80::/64"), Metric: types.NewPrefixMetric(1)}, // link-local, must drop
	}
	state := LevelState{PrefixesV6: NonLinkLocalV6Prefixes(raw)}

	if len(o.Originate(Level2, node, state).Originated) == 0 {
		t.Fatal("origination produced no LSP")
	}
	lsp := decodeOwnFrag0(t, d, Level2, node.SystemID)

	entries := findTLV236Entries(t, lsp)
	got := map[string]packet.IPv6ReachEntry{}
	for _, e := range entries {
		got[e.Prefix.String()] = e
	}
	if _, ok := got["fe80::/64"]; ok {
		t.Error("RFC 5308 sec 2 violation: link-local prefix advertised in TLV 236")
	}
	if e, ok := got["2001:db8:1::/64"]; !ok {
		t.Error("missing TLV 236 entry 2001:db8:1::/64")
	} else if e.Metric.Value() != 10 || e.External {
		t.Errorf("2001:db8:1::/64 = metric %d external %v, want 10 false", e.Metric.Value(), e.External)
	}
	if e, ok := got["2001:db8:2::/48"]; !ok {
		t.Error("missing TLV 236 entry 2001:db8:2::/48")
	} else if e.Metric.Value() != 20 || !e.External {
		t.Errorf("2001:db8:2::/48 = metric %d external %v, want 20 true", e.Metric.Value(), e.External)
	}
}

// TestISISOriginateTLV232Scope -- TLV 232 in the LSP carries ONLY non-link-local
// addresses (RFC 5308 sec 3, AC-4). The link-local addresses belong in the IIH
// (circuit layer), never in the LSP.
//
// RFC requirement: RFC5308-3-2 positive -- the non-link-local address 2001:db8::1 is carried in the LSP TLV 232.
// RFC requirement: RFC5308-3-2 negative -- the fe80::1 link-local address is excluded from the LSP TLV 232.
func TestISISOriginateTLV232Scope(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)
	node := dualStackNode(t)

	raw := []netip.Addr{
		netip.MustParseAddr("2001:db8::1"),
		netip.MustParseAddr("fe80::1"), // link-local, must NOT appear in an LSP
	}
	state := LevelState{InterfaceAddrsV6: NonLinkLocalV6Addrs(raw)}

	if len(o.Originate(Level2, node, state).Originated) == 0 {
		t.Fatal("origination produced no LSP")
	}
	lsp := decodeOwnFrag0(t, d, Level2, node.SystemID)

	addrs := findTLV232Addrs(t, lsp)
	for _, a := range addrs {
		if a.IsLinkLocalUnicast() {
			t.Errorf("RFC 5308 sec 3 violation: link-local %s in LSP TLV 232", a)
		}
	}
	found := false
	for _, a := range addrs {
		if a == netip.MustParseAddr("2001:db8::1") {
			found = true
		}
	}
	if !found {
		t.Error("LSP TLV 232 missing the non-link-local address 2001:db8::1")
	}
}

// TestISISProtocolsSupportedDualStack -- TLV 129 lists 0x8E + 0xCC when
// dual-stack and only 0xCC when IPv4-only (RFC 5308 sec 4, AC-1).
//
// RFC requirement: RFC5308-4-1 positive -- the dual-stack LSP lists NLPID 0x8E in the Protocols Supported TLV (129).
// RFC requirement: RFC5308-4-1 negative -- the IPv4-only LSP omits NLPID 0x8E from the Protocols Supported TLV (129).
func TestISISProtocolsSupportedDualStack(t *testing.T) {
	cases := []struct {
		name      string
		node      func(*testing.T) NodeInfo
		wantNLPID []uint8
	}{
		{"dual-stack", dualStackNode, []uint8{packet.NLPIDIPv4, packet.NLPIDIPv6}},
		{"ipv4-only", sampleNode, []uint8{packet.NLPIDIPv4}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := New(nil)
			o := NewOriginator(d, nil)
			node := tc.node(t)
			if len(o.Originate(Level2, node, LevelState{}).Originated) == 0 {
				t.Fatal("origination produced no LSP")
			}
			lsp := decodeOwnFrag0(t, d, Level2, node.SystemID)
			var nlpids []uint8
			for _, tl := range lsp.TLVs {
				if tl.Type == packet.TLVProtocolsSupported {
					nlpids = packet.DecodeProtocolsSupportedTLV(tl.Value).NLPIDs
				}
			}
			if len(nlpids) != len(tc.wantNLPID) {
				t.Fatalf("TLV 129 NLPIDs = %v, want %v", nlpids, tc.wantNLPID)
			}
			for i, want := range tc.wantNLPID {
				if nlpids[i] != want {
					t.Errorf("NLPID[%d] = %#x, want %#x", i, nlpids[i], want)
				}
			}
		})
	}
}

// TestISISOriginateTLV236PrefixLenBoundary -- TLV 236 prefix length 0 and 128
// (the boundary of the 0..128 range) round-trip through origination intact.
func TestISISOriginateTLV236PrefixLenBoundary(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)
	node := dualStackNode(t)

	state := LevelState{PrefixesV6: []PrefixInfoV6{
		{Prefix: netip.MustParsePrefix("::/0"), Metric: types.NewPrefixMetric(5)},            // 0 (last valid low)
		{Prefix: netip.MustParsePrefix("2001:db8::1/128"), Metric: types.NewPrefixMetric(7)}, // 128 (last valid high)
	}}
	if len(o.Originate(Level2, node, state).Originated) == 0 {
		t.Fatal("origination produced no LSP")
	}
	lsp := decodeOwnFrag0(t, d, Level2, node.SystemID)
	got := map[int]bool{}
	for _, e := range findTLV236Entries(t, lsp) {
		got[e.Prefix.Bits()] = true
	}
	if !got[0] {
		t.Error("TLV 236 ::/0 (prefix length 0) not originated")
	}
	if !got[128] {
		t.Error("TLV 236 /128 (prefix length 128) not originated")
	}
}
