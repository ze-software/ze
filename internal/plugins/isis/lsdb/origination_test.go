// Design: plan/spec-isis-6-lsdb.md -- own-LSP origination tests.
//
// VALIDATES: own LSP built with TLV 1/129/22/132/135/137, valid sequence +
// Fletcher checksum (TestISISOriginateOnAdjacencyUp); full regen + sequence
// bump on change (TestISISOriginateRegenOnChange); refresh increments sequence,
// resets lifetime (TestISISRefreshIncrementsSeq); sequence wraparound purges +
// suspends + re-originates from 1 (TestISISSequenceWraparound); fragmentation
// across LSP numbers with no entry split (TestISISOriginateFragmentation);
// overload bit only in fragment 0 (TestISISOriginateOverloadBit) -- AC-1, AC-3,
// AC-4, AC-5, AC-6.

package lsdb

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// mkMetric builds a wide IS-reachability metric, failing the test on overflow.
func mkMetric(t *testing.T, v uint32) types.Metric {
	t.Helper()
	m, err := types.NewMetric(v)
	if err != nil {
		t.Fatalf("NewMetric(%d): %v", v, err)
	}
	return m
}

// sampleNode returns a NodeInfo for system 0000.0000.0001 in area 49.0001 with a
// hostname and IPv4 advertised.
func sampleNode(t *testing.T) NodeInfo {
	t.Helper()
	net, err := types.ParseNET("49.0001.0000.0000.0001.00")
	if err != nil {
		t.Fatalf("ParseNET: %v", err)
	}
	return NodeInfo{
		SystemID:      net.SystemID(),
		Areas:         []types.AreaID{net.AreaID()},
		Hostname:      "router-a",
		AdvertiseIPv4: true,
		MaxLifetime:   1200,
	}
}

// decodeOwnFrag0 looks up the node's own L1L2 fragment 0 at level and decodes it.
func decodeOwnFrag0(t *testing.T, d *LSDB, level Level, sys types.SystemID) packet.LSP {
	t.Helper()
	id := types.NewLSPID(types.NewSourceID(sys, 0), 0)
	e := d.Lookup(level, id)
	if e == nil {
		t.Fatalf("own fragment 0 missing at %s", level)
	}
	lsp, err := e.Decode()
	if err != nil {
		t.Fatalf("decode own fragment 0: %v", err)
	}
	return lsp
}

// tlvTypes returns the set of TLV types present in an LSP body.
func tlvTypes(lsp packet.LSP) map[uint8]int {
	m := map[uint8]int{}
	for _, t := range lsp.TLVs {
		m[t.Type]++
	}
	return m
}

func TestISISOriginateOnAdjacencyUp(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)
	node := sampleNode(t)

	// One Up adjacency (TLV 22), one connected prefix (TLV 135), one own
	// interface address (TLV 132).
	state := LevelState{
		Neighbors: []AdjacencyInfo{{
			Neighbor: types.NewSourceID(testSys(2), 0),
			Metric:   mkMetric(t, 10),
		}},
		Prefixes: []PrefixInfo{{
			Prefix: netip.MustParsePrefix("192.0.2.0/24"),
			Metric: types.NewPrefixMetric(10),
		}},
		InterfaceAddrs: []netip.Addr{netip.MustParseAddr("192.0.2.1")},
	}

	res := o.Originate(Level2, node, state)
	if len(res.Originated) == 0 {
		t.Fatal("origination produced no LSP")
	}

	lsp := decodeOwnFrag0(t, d, Level2, node.SystemID)

	// AC-1: a valid (non-zero) sequence number and a valid Fletcher checksum.
	if lsp.SequenceNumber == 0 {
		t.Error("originated LSP has the reserved sequence 0")
	}
	if lsp.SequenceNumber != types.FirstSequenceNumber {
		t.Errorf("first origination sequence = %d, want %d", lsp.SequenceNumber, types.FirstSequenceNumber)
	}
	if !lsp.VerifyChecksum() {
		t.Error("originated LSP checksum invalid")
	}

	// RFC requirement: RFC1195-5.2-2 positive -- the originated LSP fragment 0 carries the IP Interface Address TLV 132 (RFC 1195 sec 5.2: every IP-capable router MUST include TLV 132 in its LSPs).
	// AC-1: TLV 1, 129, 22, 132, 135, 137 all present.
	have := tlvTypes(lsp)
	for _, want := range []uint8{
		packet.TLVAreaAddresses, packet.TLVProtocolsSupported, packet.TLVExtendedISReach,
		packet.TLVIPInterfaceAddress, packet.TLVExtendedIPReach, packet.TLVDynamicHostname,
	} {
		if have[want] == 0 {
			t.Errorf("originated LSP missing TLV %d", want)
		}
	}

	// RFC requirement: RFC1195-5.2-1 positive -- the originated LSP fragment 0 carries the Protocols Supported TLV 129 advertising NLPID 0xCC for IPv4 (RFC 1195 sec 5.2: TLV 129 MUST be included in every IP-capable router's LSP number 0).
	// TLV 129 advertises NLPID IPv4 (0xCC).
	for _, tl := range lsp.TLVs {
		if tl.Type == packet.TLVProtocolsSupported {
			ps := packet.DecodeProtocolsSupportedTLV(tl.Value)
			if len(ps.NLPIDs) != 1 || ps.NLPIDs[0] != packet.NLPIDIPv4 {
				t.Errorf("TLV 129 NLPIDs = %v, want [0xCC]", ps.NLPIDs)
			}
		}
	}

	// TLV 22 carries the neighbor with metric 10.
	for _, tl := range lsp.TLVs {
		if tl.Type == packet.TLVExtendedISReach {
			ext, err := packet.DecodeExtendedISReachTLV(tl.Value)
			if err != nil {
				t.Fatalf("decode TLV 22: %v", err)
			}
			if len(ext.Entries) != 1 || ext.Entries[0].Metric.Value() != 10 {
				t.Errorf("TLV 22 = %+v, want one neighbor metric 10", ext.Entries)
			}
			if ext.Entries[0].Neighbor != types.NewSourceID(testSys(2), 0) {
				t.Errorf("TLV 22 neighbor = %v, want sys 2", ext.Entries[0].Neighbor)
			}
		}
	}
}

func TestISISOriginateRegenOnChange(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)
	node := sampleNode(t)
	state := LevelState{
		Neighbors:      []AdjacencyInfo{{Neighbor: types.NewSourceID(testSys(2), 0), Metric: mkMetric(t, 10)}},
		InterfaceAddrs: []netip.Addr{netip.MustParseAddr("10.0.0.1")},
	}
	o.Originate(Level1, node, state)
	seq1 := decodeOwnFrag0(t, d, Level1, node.SystemID).SequenceNumber

	// A topology change (a second neighbor) regenerates the full set and bumps
	// the sequence (clause 7.3.12).
	state.Neighbors = append(state.Neighbors, AdjacencyInfo{Neighbor: types.NewSourceID(testSys(3), 0), Metric: mkMetric(t, 20)})
	o.Originate(Level1, node, state)
	lsp2 := decodeOwnFrag0(t, d, Level1, node.SystemID)
	if lsp2.SequenceNumber != seq1+1 {
		t.Errorf("regen sequence = %d, want %d (one more than %d)", lsp2.SequenceNumber, seq1+1, seq1)
	}
	// Both neighbors are now advertised.
	var count int
	for _, tl := range lsp2.TLVs {
		if tl.Type == packet.TLVExtendedISReach {
			ext, _ := packet.DecodeExtendedISReachTLV(tl.Value)
			count += len(ext.Entries)
		}
	}
	if count != 2 {
		t.Errorf("after adding a neighbor, TLV 22 has %d entries, want 2", count)
	}
	if !lsp2.VerifyChecksum() {
		t.Error("regenerated LSP checksum invalid (checksum not recomputed)")
	}
}

func TestISISRefreshIncrementsSeq(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)
	node := sampleNode(t)
	state := LevelState{InterfaceAddrs: []netip.Addr{netip.MustParseAddr("10.0.0.1")}}

	o.Originate(Level2, node, state)
	first := decodeOwnFrag0(t, d, Level2, node.SystemID)

	// A refresh is just a re-origination with the same state: the sequence is
	// incremented, the checksum recomputed, and the lifetime reset to MaxAge
	// (clause 7.3.16.1, spec AC-3).
	o.Originate(Level2, node, state)
	second := decodeOwnFrag0(t, d, Level2, node.SystemID)

	if second.SequenceNumber != first.SequenceNumber+1 {
		t.Errorf("refresh sequence = %d, want %d", second.SequenceNumber, first.SequenceNumber+1)
	}
	if second.RemainingLifetime != types.RemainingLifetime(node.MaxLifetime) {
		t.Errorf("refresh lifetime = %d, want %d (reset to MaxAge)", second.RemainingLifetime, node.MaxLifetime)
	}
	if !second.VerifyChecksum() {
		t.Error("refreshed LSP checksum invalid")
	}
}

func TestISISSequenceWraparound(t *testing.T) {
	clk := newFakeClock()
	d := New(clk.now)
	o := NewOriginator(d, clk.now)
	node := sampleNode(t)
	state := LevelState{InterfaceAddrs: []netip.Addr{netip.MustParseAddr("10.0.0.1")}}

	frag0 := types.NewLSPID(types.NewSourceID(node.SystemID, 0), 0)

	// Force the next sequence to wrap: pretend the last assigned was the maximum.
	o.mu.Lock()
	o.lastSeq[frag0] = types.MaxSequenceNumber
	o.mu.Unlock()

	res := o.Originate(Level2, node, state)
	if !res.Wrapped {
		t.Fatal("origination at MaxSequenceNumber did not report a wraparound")
	}
	// AC-4: the LSP is purged (Remaining Lifetime 0) and origination is suspended.
	e := d.Lookup(Level2, frag0)
	if e == nil {
		t.Fatal("wrapped LSP not stored as a purge")
	}
	if !e.IsPurged() || e.Lifetime() != 0 {
		t.Errorf("wrapped LSP not a purge: purged=%v lifetime=%d", e.IsPurged(), e.Lifetime())
	}
	if e.Sequence() != types.MaxSequenceNumber {
		t.Errorf("wrap purge sequence = %d, want MaxSequenceNumber", e.Sequence())
	}

	// While suspended (before MaxAge + ZeroAgeLifetime) a re-origination does NOT
	// rewrite the LSP (clause 7.3.16.1).
	clk.advance(DefaultMaxAge) // still inside MaxAge + ZeroAgeLifetime
	res = o.Originate(Level2, node, state)
	if len(res.Originated) != 0 {
		t.Errorf("suspended LSP re-originated too early: %+v", res.Originated)
	}
	if d.Lookup(Level2, frag0).Sequence() != types.MaxSequenceNumber {
		t.Error("suspended LSP was rewritten during the suspension window")
	}

	// After the full window it re-originates from sequence 1.
	clk.advance(DefaultMaxAge + ZeroAgeLifetime)
	res = o.Originate(Level2, node, state)
	if len(res.Originated) == 0 {
		t.Fatal("LSP not re-originated after the suspension window")
	}
	if got := d.Lookup(Level2, frag0).Sequence(); got != types.FirstSequenceNumber {
		t.Errorf("post-suspension sequence = %d, want %d", got, types.FirstSequenceNumber)
	}
}

func TestISISOriginateFragmentation(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)
	node := sampleNode(t)
	// Force a tiny max LSP size so even a handful of prefixes overflow one
	// fragment (spec A-5 with a forced small max). The minLSPSize floor keeps
	// fragment 0 valid.
	node.MaxLSPSize = minLSPSize

	// Enough prefixes to require several fragments.
	var prefixes []PrefixInfo
	for i := range 40 {
		p := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i), 0, 0}), 16)
		prefixes = append(prefixes, PrefixInfo{Prefix: p, Metric: types.NewPrefixMetric(uint32(i + 1))})
	}
	state := LevelState{Prefixes: prefixes, InterfaceAddrs: []netip.Addr{netip.MustParseAddr("10.0.0.1")}}

	res := o.Originate(Level2, node, state)

	// AC-5: state split across multiple LSP numbers (more than fragment 0).
	if len(res.Originated) < 2 {
		t.Fatalf("expected multiple fragments, got %d", len(res.Originated))
	}

	// Every fragment is a distinct LSP with its own sequence + valid checksum,
	// no fragment exceeds the max size, and no TLV entry was split (each TLV 135
	// decodes cleanly). Collect all prefixes across fragments and confirm none
	// was lost or duplicated.
	seen := map[netip.Prefix]int{}
	src := types.NewSourceID(node.SystemID, 0)
	for num := range maxFragments {
		id := types.NewLSPID(src, uint8(num))
		e := d.Lookup(Level2, id)
		if e == nil {
			break
		}
		if len(e.Raw()) > node.MaxLSPSize {
			t.Errorf("fragment %d size %d exceeds max %d", num, len(e.Raw()), node.MaxLSPSize)
		}
		lsp, err := e.Decode()
		if err != nil {
			t.Fatalf("fragment %d decode: %v", num, err)
		}
		if !lsp.VerifyChecksum() {
			t.Errorf("fragment %d checksum invalid", num)
		}
		if lsp.SequenceNumber != types.FirstSequenceNumber {
			t.Errorf("fragment %d sequence = %d, want 1 on first origination", num, lsp.SequenceNumber)
		}
		for _, tl := range lsp.TLVs {
			if tl.Type == packet.TLVExtendedIPReach {
				ext, err := packet.DecodeExtendedIPReachTLV(tl.Value)
				if err != nil {
					t.Errorf("fragment %d TLV 135 decode (split entry?): %v", num, err)
				}
				for _, ent := range ext.Entries {
					seen[ent.Prefix]++
				}
			}
		}
	}
	if len(seen) != len(prefixes) {
		t.Errorf("fragmentation lost/duplicated prefixes: %d distinct across fragments, want %d", len(seen), len(prefixes))
	}
	for p, n := range seen {
		if n != 1 {
			t.Errorf("prefix %s appeared %d times across fragments, want 1", p, n)
		}
	}
}

func TestISISOriginateOverloadBit(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)
	node := sampleNode(t)
	node.Overload = true
	node.MaxLSPSize = minLSPSize // force >1 fragment so we can check non-zero fragments

	var prefixes []PrefixInfo
	for i := range 40 {
		p := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i), 0, 0}), 16)
		prefixes = append(prefixes, PrefixInfo{Prefix: p, Metric: types.NewPrefixMetric(uint32(i + 1))})
	}
	o.Originate(Level2, node, LevelState{Prefixes: prefixes})

	src := types.NewSourceID(node.SystemID, 0)
	// AC-6: the OL bit is set in fragment 0 (the non-pseudonode LSP number zero),
	// RFC 3787 sec 4.
	frag0 := d.Lookup(Level2, types.NewLSPID(src, 0))
	if frag0 == nil || !frag0.IsOverloaded() {
		t.Error("overload bit not set in fragment 0")
	}
	// And NOT in fragment 1 (the OL bit lives only in LSP number zero).
	frag1 := d.Lookup(Level2, types.NewLSPID(src, 1))
	if frag1 == nil {
		t.Skip("only one fragment produced; cannot check fragment 1")
	}
	if frag1.IsOverloaded() {
		t.Error("overload bit wrongly set in fragment 1 (must be fragment 0 only, RFC 3787)")
	}
}

func TestISISOriginatePurgeStaleFragments(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)
	node := sampleNode(t)
	node.MaxLSPSize = minLSPSize

	// Originate with many prefixes -> several fragments.
	var many []PrefixInfo
	for i := range 40 {
		p := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i), 0, 0}), 16)
		many = append(many, PrefixInfo{Prefix: p, Metric: types.NewPrefixMetric(uint32(i + 1))})
	}
	res := o.Originate(Level2, node, LevelState{Prefixes: many})
	produced := len(res.Originated)
	if produced < 2 {
		t.Fatalf("setup expected multiple fragments, got %d", produced)
	}

	// Shrink to no prefixes: the higher fragments must be purged (not left stale).
	res2 := o.Originate(Level2, node, LevelState{})
	if len(res2.Purged) == 0 {
		t.Error("shrinking state did not purge stale fragments")
	}
	// The purged higher fragments are now marked purged in the LSDB.
	src := types.NewSourceID(node.SystemID, 0)
	frag1 := d.Lookup(Level2, types.NewLSPID(src, 1))
	if frag1 == nil || !frag1.IsPurged() {
		t.Error("stale fragment 1 not purged after state shrank")
	}
}

// VALIDATES: RFC 5304 sec 2 -- the production purge path (purgeFragmentLocked)
// routes every purge through packet.StripPurgeBody, so a signed purge carries
// ONLY the authentication TLV (10) and no body. This is the regression for the B6
// finding that StripPurgeBody (the canonical body-stripping helper) was exported
// but had no production caller; the purge path now uses it as the single
// canonicalization point.
// PREVENTS: a purge that leaks a stray body past the RFC 5304 sec 2 rule, and a
// silent unwiring of StripPurgeBody back to test-only.
func TestISISOriginatePurgeStripsBodyAndAuthenticates(t *testing.T) {
	d := New(nil)
	o := NewOriginator(d, nil)
	// Install a real HMAC-SHA-256 signer so the purge takes the authenticated
	// path exactly as production does (auth_wiring installs packet.SignPDU).
	key := packet.Key{Algorithm: packet.AuthAlgoHMACSHA256, Secret: []byte("purge-wiring"), KeyID: 7}
	o.SetSigner(func(pdu []byte) []byte {
		signed, err := packet.SignPDU(pdu, key)
		if err != nil {
			t.Fatalf("SignPDU in signer: %v", err)
		}
		return signed
	})

	node := sampleNode(t)
	node.MaxLSPSize = minLSPSize

	// Originate several fragments, then shrink to force a purge of the tail.
	var many []PrefixInfo
	for i := range 40 {
		p := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i), 0, 0}), 16)
		many = append(many, PrefixInfo{Prefix: p, Metric: types.NewPrefixMetric(uint32(i + 1))})
	}
	if res := o.Originate(Level2, node, LevelState{Prefixes: many}); len(res.Originated) < 2 {
		t.Fatalf("setup expected multiple fragments, got %d", len(res.Originated))
	}
	res2 := o.Originate(Level2, node, LevelState{})
	if len(res2.Purged) == 0 {
		t.Fatal("shrinking state did not purge stale fragments")
	}

	src := types.NewSourceID(node.SystemID, 0)
	frag1 := d.Lookup(Level2, types.NewLSPID(src, 1))
	if frag1 == nil || !frag1.IsPurged() {
		t.Fatalf("stale fragment 1 not purged after state shrank")
	}

	// The stored raw purge must decode to ONLY TLV 10 (body stripped, RFC 5304
	// sec 2) and verify under the signing key.
	raw := frag1.Raw()
	if err := packet.VerifyPDU(raw, []packet.Key{key}); err != nil {
		t.Fatalf("VerifyPDU on stored purge: %v", err)
	}
	dec, err := packet.DecodePDU(raw)
	if err != nil {
		t.Fatalf("DecodePDU on stored purge: %v", err)
	}
	if dec.LSP == nil {
		t.Fatal("stored purge did not decode as an LSP")
	}
	if !dec.LSP.RemainingLifetime.IsPurge() {
		t.Errorf("stored purge Remaining Lifetime = %d, want 0", dec.LSP.RemainingLifetime)
	}
	if len(dec.LSP.TLVs) != 1 || dec.LSP.TLVs[0].Type != packet.TLVAuthentication {
		t.Fatalf("signed purge TLVs = %+v, want only TLV 10 (StripPurgeBody must run on the production path)", dec.LSP.TLVs)
	}
}

// test-relax: removed an unused `var _ = time.Second` placeholder and its
// `time` import; this file uses package duration constants, not the time
// package directly. No test coverage changed.
