// Design: docs/architecture/wire/isis.md -- core TLV round-trip + boundary tests
package packet

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// mustArea builds an AreaID from bytes or fails the test.
func mustArea(t *testing.T, b []byte) types.AreaID {
	t.Helper()
	a, err := types.AreaIDFromBytes(b)
	if err != nil {
		t.Fatalf("AreaIDFromBytes(% x): %v", b, err)
	}
	return a
}

// mustSource builds a SourceID from a SystemID and pseudonode.
func mustSource(sys types.SystemID, pn uint8) types.SourceID {
	return types.NewSourceID(sys, pn)
}

// VALIDATES: AC-12 -- TLV 1 (Area Addresses) round-trips, preserving each
// variable-length area and the entry order.
func TestISISTLVAreaAddressesRoundTrip(t *testing.T) {
	in := AreaAddressesTLV{Areas: []types.AreaID{
		mustArea(t, []byte{0x49, 0x00, 0x01}),
		mustArea(t, []byte{0x49}),
		mustArea(t, []byte{0x49, 0x00, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c}), // 13 octets (max)
	}}
	buf := make([]byte, 256)
	n := writeAreaAddressesTLV(buf, 0, in)
	// The encoded region is type+len+value; decode the value through the
	// generic iterator to confirm framing, then the typed decoder.
	it := NewTLVIterator(buf[:n])
	typ, value, ok := it.Next()
	if !ok || typ != TLVAreaAddresses {
		t.Fatalf("framing: ok=%v typ=%d", ok, typ)
	}
	out, err := DecodeAreaAddressesTLV(value)
	if err != nil {
		t.Fatalf("DecodeAreaAddressesTLV: %v", err)
	}
	if len(out.Areas) != len(in.Areas) {
		t.Fatalf("got %d areas, want %d", len(out.Areas), len(in.Areas))
	}
	for i := range in.Areas {
		if !out.Areas[i].Equal(in.Areas[i]) {
			t.Errorf("area[%d] = %v, want %v", i, out.Areas[i], in.Areas[i])
		}
	}
}

// VALIDATES: TLV 1 rejects an out-of-range area length without panicking.
func TestISISTLVAreaAddressesBadLength(t *testing.T) {
	// Length octet 14 exceeds MaxAreaIDLen (13).
	bad := []byte{14, 1, 2, 3}
	if _, err := DecodeAreaAddressesTLV(bad); err == nil {
		t.Fatal("expected error for area length 14")
	}
	// Length octet 0 is below MinAreaIDLen.
	if _, err := DecodeAreaAddressesTLV([]byte{0}); err == nil {
		t.Fatal("expected error for area length 0")
	}
	// Length that overruns the value.
	if _, err := DecodeAreaAddressesTLV([]byte{3, 1, 2}); err == nil {
		t.Fatal("expected truncation error")
	}
}

// VALIDATES: AC-12 -- TLV 9 (LSP Entries) round-trips every 16-octet field.
// Story 4: CSNP carries TLV 9; entries must match after decode.
func TestISISTLVLSPEntriesRoundTrip(t *testing.T) {
	sys := types.SystemID{0, 1, 0, 2, 0, 3}
	in := LSPEntriesTLV{Entries: []LSPEntry{
		{RemainingLifetime: 1199, LSPID: types.NewLSPID(mustSource(sys, 0), 0), SequenceNumber: 1, Checksum: 0xabcd},
		{RemainingLifetime: 65535, LSPID: types.NewLSPID(mustSource(sys, 1), 5), SequenceNumber: 0xFFFFFFFF, Checksum: 0x0001},
	}}
	buf := make([]byte, 256)
	n := writeLSPEntriesTLV(buf, 0, in)
	it := NewTLVIterator(buf[:n])
	_, value, _ := it.Next()
	out, err := DecodeLSPEntriesTLV(value)
	if err != nil {
		t.Fatalf("DecodeLSPEntriesTLV: %v", err)
	}
	if len(out.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(out.Entries))
	}
	for i := range in.Entries {
		if out.Entries[i] != in.Entries[i] {
			t.Errorf("entry[%d] = %+v, want %+v", i, out.Entries[i], in.Entries[i])
		}
	}
}

// VALIDATES: TLV 9 rejects a value that is not a whole number of entries.
func TestISISTLVLSPEntriesBadLength(t *testing.T) {
	if _, err := DecodeLSPEntriesTLV(make([]byte, LSPEntryLen+1)); err == nil {
		t.Fatal("expected ErrLength for non-multiple value")
	}
}

// VALIDATES: AC-6 -- TLV 22 (Extended IS Reachability) decodes the 7-octet
// neighbor, the 24-bit metric, and sub-TLVs 4/6/8 from wire bytes; outer and
// sub-TLV lengths stay consistent. Also covers the 24-bit metric boundary
// (16777215).
//
// The fixture is written by hand from the RFC rather than produced by Ze's own
// encoder: a round trip through one codec proves self-consistency, and a shared
// misreading of the wire format would pass it. These bytes assert the format.
//
// rfc-test-change-approved: 2026-08-17 -- Thomas approved the dead-codec sweep after being
// shown this specific change: the fixture rewrite was described to him and he directed it.
// He did not review the wording of this marker. writeExtendedISReachTLV had no production
// caller and was deleted, so the fixture it built is replaced by hand-written RFC 5305
// bytes. Every assertion about opaque sub-TLV retention is kept, and an entry-1 neighbor
// check is added; only the encoder re-encode drift check goes, because it has no subject
// once the encoder is gone.
func TestISISTLV22RoundTrip(t *testing.T) {
	sys := types.SystemID{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}

	// TLV 22 wire layout, hand-built from RFC 5305 sec 3 (entry: 7 octets of
	// system ID and pseudonode number, 3 octets of default metric, 1 octet of
	// sub-TLV block length, then the block) and RFC 5305 sec 2 (each sub-TLV:
	// 1 octet type, 1 octet length, then the value).
	//
	//	offset  field
	//	     0  TLV type   = 22
	//	     1  TLV length = 44 (two entries: 33 + 11)
	//	  ---- entry 0, offsets 2..34 ----
	//	  2- 8  neighbor: system ID aa bb cc dd ee ff, pseudonode 00
	//	  9-11  default metric = 00 00 0a (10)
	//	    12  sub-TLV block length = 22 (three sub-TLVs: 10 + 6 + 6)
	//	 13-14  sub-TLV type 4 (link local/remote ID), length 8
	//	 15-22    value 00 00 00 01 00 00 00 02
	//	 23-24  sub-TLV type 6 (IPv4 interface address), length 4
	//	 25-28    value c0 00 02 01 (192.0.2.1)
	//	 29-30  sub-TLV type 8 (IPv4 neighbor address), length 4
	//	 31-34    value c0 00 02 02 (192.0.2.2)
	//	  ---- entry 1, offsets 35..45 ----
	//	 35-41  neighbor: system ID aa bb cc dd ee ff, pseudonode 07
	//	 42-44  default metric = ff ff ff (16777215, the 24-bit boundary)
	//	    45  sub-TLV block length = 0 (an empty block, not an omitted octet)
	wire := []byte{
		TLVExtendedISReach, 44,
		0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00,
		0x00, 0x00, 0x0a,
		22,
		SubTLVLinkLocalRemoteID, 8, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02,
		SubTLVIPv4InterfaceAddr, 4, 0xc0, 0x00, 0x02, 0x01,
		SubTLVIPv4NeighborAddr, 4, 0xc0, 0x00, 0x02, 0x02,
		0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x07,
		0xff, 0xff, 0xff,
		0,
	}
	wantSubs := []SubTLV{
		{Type: SubTLVLinkLocalRemoteID, Value: []byte{0, 0, 0, 1, 0, 0, 0, 2}}, // 8-octet local/remote IDs
		{Type: SubTLVIPv4InterfaceAddr, Value: []byte{192, 0, 2, 1}},
		{Type: SubTLVIPv4NeighborAddr, Value: []byte{192, 0, 2, 2}},
	}

	it := NewTLVIterator(wire)
	typ, value, ok := it.Next()
	if !ok || typ != TLVExtendedISReach {
		t.Fatalf("framing: ok=%v typ=%d", ok, typ)
	}
	out, err := DecodeExtendedISReachTLV(value)
	if err != nil {
		t.Fatalf("DecodeExtendedISReachTLV: %v", err)
	}
	if len(out.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(out.Entries))
	}
	if out.Entries[0].Neighbor != mustSource(sys, 0) {
		t.Errorf("entry0 neighbor = %v, want %v", out.Entries[0].Neighbor, mustSource(sys, 0))
	}
	if out.Entries[1].Neighbor != mustSource(sys, 7) {
		t.Errorf("entry1 neighbor = %v, want %v (pseudonode octet)", out.Entries[1].Neighbor, mustSource(sys, 7))
	}
	if out.Entries[0].Metric.Value() != 10 {
		t.Errorf("entry0 metric = %d, want 10", out.Entries[0].Metric.Value())
	}
	if out.Entries[1].Metric.Value() != types.MaxMetric {
		t.Errorf("entry1 metric = %d, want %d (24-bit boundary)", out.Entries[1].Metric.Value(), types.MaxMetric)
	}
	// RFC requirement: RFC5305-2-1 positive -- sub-TLVs the codec does not interpret are retained opaquely on the decoded entry, never dropped or rejected (RFC 5305 sec 2: unknown sub-TLVs are ignored and skipped, not fatal).
	if len(out.Entries[0].SubTLVs) != 3 {
		t.Fatalf("entry0 got %d sub-TLVs, want 3", len(out.Entries[0].SubTLVs))
	}
	for i, want := range wantSubs {
		got := out.Entries[0].SubTLVs[i]
		if got.Type != want.Type || !bytes.Equal(got.Value, want.Value) {
			t.Errorf("sub-TLV[%d] = {%d,% x}, want {%d,% x}", i, got.Type, got.Value, want.Type, want.Value)
		}
	}
	if len(out.Entries[1].SubTLVs) != 0 {
		t.Errorf("entry1 should have no sub-TLVs, got %d", len(out.Entries[1].SubTLVs))
	}
}

// VALIDATES: TLV 22 rejects a sub-TLV-length that overruns the value.
func TestISISTLV22Truncated(t *testing.T) {
	// 7 neighbor + 3 metric + subLen=200 but no sub-TLV bytes.
	v := make([]byte, types.SourceIDLen+types.MetricLen+1)
	v[len(v)-1] = 200
	// RFC requirement: RFC5305-2-1 negative -- a sub-TLV length that overruns the entry value is rejected, not silently skipped as if the (unknown) sub-TLV were absent.
	if _, err := DecodeExtendedISReachTLV(v); err == nil {
		t.Fatal("expected truncation error for over-long sub-TLV length")
	}
}

// rfc-test-change-approved: 2026-08-17 -- Thomas approved the dead-codec sweep, and this
// deletion was described to him before he directed it. He did not review the wording of
// this marker. TestISISTLVProtocolsSupported was
// deleted here together with writeProtocolsSupportedTLV, which had no production caller and
// was the test's only subject beyond the decoder. The RFC1195-5.2-1 positive proof survives
// at internal/plugins/isis/lsdb/origination_test.go:122, which reads TLV 129 off an LSP the
// daemon actually originated. DecodeProtocolsSupportedTLV keeps hand-built-bytes coverage in
// TestISISIgnoreObsoleteTLVs131And133 and ordered multi-NLPID coverage in
// internal/plugins/isis/lsdb/origination_ipv6_test.go.

// VALIDATES: TLV 137 (Dynamic Hostname) round-trips an ASCII name.
func TestISISTLVHostname(t *testing.T) {
	name := []byte("router-1.example.net")
	buf := make([]byte, 256)
	n := writeHostnameTLV(buf, 0, name)
	it := NewTLVIterator(buf[:n])
	typ, value, _ := it.Next()
	if typ != TLVDynamicHostname {
		t.Fatalf("type = %d", typ)
	}
	if !bytes.Equal(value, name) {
		t.Errorf("hostname = %q, want %q", value, name)
	}
}

// VALIDATES: AC-9 -- TLV 240 (P2P Three-Way) round-trips at lengths 1, 5, 15.
func TestISISTLV240ThreeWay(t *testing.T) {
	sys := types.SystemID{1, 2, 3, 4, 5, 6}
	cases := []struct {
		name string
		in   P2PThreeWayTLV
		vlen int
	}{
		{"state-only", P2PThreeWayTLV{State: AdjThreeWayDown}, 1},
		{"with-local", P2PThreeWayTLV{State: AdjThreeWayInitializing, HasCircuitID: true, LocalCircuitID: 0x01020304}, 5},
		{"full", P2PThreeWayTLV{
			State: AdjThreeWayUp, HasCircuitID: true, LocalCircuitID: 0xdeadbeef,
			HasNeighbor: true, NeighborID: sys, NeighborCircuit: 0xcafef00d,
		}, 15},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := make([]byte, 32)
			n := writeP2PThreeWayTLV(buf, 0, tc.in)
			if n != TLVHeaderLen+tc.vlen {
				t.Fatalf("encoded %d bytes, want %d", n, TLVHeaderLen+tc.vlen)
			}
			it := NewTLVIterator(buf[:n])
			typ, value, _ := it.Next()
			if typ != TLVP2PThreeWay {
				t.Fatalf("type = %d", typ)
			}
			if len(value) != tc.vlen {
				t.Fatalf("value len = %d, want %d", len(value), tc.vlen)
			}
			out, err := DecodeP2PThreeWayTLV(value)
			if err != nil {
				t.Fatalf("DecodeP2PThreeWayTLV: %v", err)
			}
			if out != tc.in {
				t.Errorf("round-trip = %+v, want %+v", out, tc.in)
			}
		})
	}
}

// VALIDATES: TLV 240 rejects an invalid length (not 1/5/15).
func TestISISTLV240BadLength(t *testing.T) {
	for _, badLen := range []int{0, 2, 6, 14, 16} {
		if _, err := DecodeP2PThreeWayTLV(make([]byte, badLen)); err == nil {
			t.Errorf("expected error for TLV 240 length %d", badLen)
		}
	}
}
