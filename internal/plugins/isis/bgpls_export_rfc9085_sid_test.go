// Design: docs/architecture/wire/nlri-bgpls.md -- native IS-IS BGP-LS origination.
// RFC: rfc/short/rfc9085.md
// Related: bgpls_export.go -- sidValue inserts the Reserved field, binding builds TLV 1159
//
// VALIDATES: the Adjacency SID TLV 1099, LAN Adjacency SID TLV 1100, Prefix-SID
// TLV 1158 and Range TLV 1159 Ze originates from IS-IS carry a Reserved field
// of 0, as RFC 9085 Sections 2.2.1, 2.2.2, 2.3.1 and 2.3.5 require, whatever
// octets the IS-IS source carried beside or in the native reserved position.
// PREVENTS: a native SID, weight, flags or reserved octet reaching the
// Reserved field a collector reads.
package isis

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// rfc9085SIDSources holds the native IS-IS values one test originates.
type rfc9085SIDSources struct {
	adjacency    []byte // sub-TLV 31 value: flags, weight, 3-octet label
	lanAdjacency []byte // sub-TLV 32 value: flags, weight, system ID, 3-octet label
	prefixSID    []byte // sub-TLV 3 value: flags, algorithm, 4-octet index
	binding      []byte // TLV 149 value: flags, reserved, range, length, prefix, sub-TLVs
}

// rfc9085SIDAttributes stores one LSP carrying the sources and returns the
// originated TLV 1099, 1100, 1158 and 1159 values.
func rfc9085SIDAttributes(t *testing.T, sources *rfc9085SIDSources) (adjacency, lanAdjacency, prefixSID, span []byte) {
	t.Helper()
	eng := newEngine(transport.New(&fakeBackend{}))
	t.Cleanup(eng.shutdown)
	id := types.LSPID{0, 0, 0, 0, 0, 2, 0, 0}
	subs := []byte{31, byte(len(sources.adjacency))}
	subs = append(subs, sources.adjacency...)
	subs = append(subs, 32, byte(len(sources.lanAdjacency)))
	subs = append(subs, sources.lanAdjacency...)
	link := []byte{0, 0, 0, 0, 0, 3, 0, 0, 0, 9, byte(len(subs))}
	link = append(link, subs...)
	// TLV 135: metric 10, control 0x58 (sub-TLVs present, /24), 192.0.2.0.
	reach := []byte{0, 0, 0, 10, 0x58, 192, 0, 2, byte(2 + len(sources.prefixSID)), 3, byte(len(sources.prefixSID))}
	reach = append(reach, sources.prefixSID...)
	bgplsStoreLSP(t, eng, lsdb.Level1, id, 1, 1200, []packet.TLV{
		{Type: 22, Value: link},
		{Type: 135, Value: reach},
		{Type: 149, Value: sources.binding},
	})
	var builder bgplsBuilder
	builder.build(eng.lsdb.RawSnapshot(lsdb.Level1))
	snapshot := &builder.snapshot
	if len(snapshot.Links) != 1 {
		t.Fatalf("links = %+v", snapshot.Links)
	}
	adjacency = rfc9085OneAttribute(t, snapshot.Links[0].Attributes, 1099)
	lanAdjacency = rfc9085OneAttribute(t, snapshot.Links[0].Attributes, 1100)
	for i := range snapshot.Prefixes {
		if found := rfc9514Attributes(snapshot.Prefixes[i].Attributes, 1158); len(found) == 1 {
			prefixSID = found[0]
		}
		if found := rfc9514Attributes(snapshot.Prefixes[i].Attributes, 1159); len(found) == 1 {
			span = found[0]
		}
	}
	if prefixSID == nil || span == nil {
		t.Fatalf("prefixes = %+v, want one TLV 1158 and one TLV 1159", snapshot.Prefixes)
	}
	return adjacency, lanAdjacency, prefixSID, span
}

// rfc9085OneAttribute returns the only attribute of the given type.
func rfc9085OneAttribute(t *testing.T, attributes []linkstateevents.TLV, typ uint16) []byte {
	t.Helper()
	found := rfc9514Attributes(attributes, typ)
	if len(found) != 1 {
		t.Fatalf("TLV %d = %x, want one", typ, found)
	}
	return found[0]
}

// rfc9085WantValue fails unless got equals want for the named TLV.
func rfc9085WantValue(t *testing.T, typ uint16, got, want []byte) {
	t.Helper()
	if !bytes.Equal(got, want) {
		t.Fatalf("TLV %d = %x, want %x", typ, got, want)
	}
}

// TestRFC9085ISISOriginatedSIDReservedZero originates ordinary SIDs and
// compares the exact values, whose Reserved field sits after the first two
// octets (TLV 1099, 1100, 1158) or is the second octet (TLV 1159).
//
// RFC requirement: RFC9085-2.2.1-1 positive -- the Adjacency SID TLV 1099 Ze originates from IS-IS sub-TLV 31 has a 2-octet Reserved field of 0 (§2.2.1).
// RFC requirement: RFC9085-2.2.2-1 positive -- the LAN Adjacency SID TLV 1100 Ze originates from IS-IS sub-TLV 32 has a 2-octet Reserved field of 0 (§2.2.2).
// RFC requirement: RFC9085-2.3.1-1 positive -- the Prefix-SID TLV 1158 Ze originates from IS-IS sub-TLV 3 has a 2-octet Reserved field of 0 (§2.3.1).
// RFC requirement: RFC9085-2.3.5-1 positive -- the Range TLV 1159 Ze originates from an IS-IS SID/Label Binding TLV 149 has a Reserved octet of 0 (§2.3.5).
func TestRFC9085ISISOriginatedSIDReservedZero(t *testing.T) {
	adjacency, lanAdjacency, prefixSID, span := rfc9085SIDAttributes(t, &rfc9085SIDSources{
		adjacency:    []byte{0x30, 1, 0x00, 0x3e, 0x81},
		lanAdjacency: []byte{0x30, 1, 0, 0, 0, 0, 0, 4, 0x00, 0x3e, 0x82},
		prefixSID:    []byte{0x40, 0, 0, 0, 0, 7},
		binding:      []byte{0, 0, 0, 8, 24, 198, 51, 100, 3, 6, 0, 0, 0, 0, 0, 9},
	})
	rfc9085WantValue(t, 1099, adjacency, []byte{0x30, 1, 0, 0, 0x00, 0x3e, 0x81})
	rfc9085WantValue(t, 1100, lanAdjacency, []byte{0x30, 1, 0, 0, 0, 0, 0, 0, 0, 4, 0x00, 0x3e, 0x82})
	rfc9085WantValue(t, 1158, prefixSID, []byte{0x40, 0, 0, 0, 0, 0, 0, 7})
	rfc9085WantValue(t, 1159, span, []byte{0, 0, 0, 8, 0x04, 0x86, 0, 8, 0, 0, 0, 0, 0, 0, 0, 9})
}

// TestRFC9085ISISAllOnesSourceNeverReachesSIDReserved hands the producer
// all-ones flags, weights, algorithms and SIDs, the octets that land in the
// Reserved field if the producer copies the native value without inserting
// it, and a Binding TLV whose own native reserved octet is ff.
//
// RFC requirement: RFC9085-2.2.1-1 negative -- an IS-IS Adjacency SID of all-ones octets is originated as TLV 1099 with a Reserved field of 0 (§2.2.1).
// RFC requirement: RFC9085-2.2.2-1 negative -- an IS-IS LAN Adjacency SID of all-ones octets is originated as TLV 1100 with a Reserved field of 0 (§2.2.2).
// RFC requirement: RFC9085-2.3.1-1 negative -- an IS-IS Prefix-SID of all-ones octets is originated as TLV 1158 with a Reserved field of 0 (§2.3.1).
// RFC requirement: RFC9085-2.3.5-1 negative -- an IS-IS Binding TLV whose native reserved octet is ff is originated as TLV 1159 with a Reserved octet of 0 (§2.3.5).
func TestRFC9085ISISAllOnesSourceNeverReachesSIDReserved(t *testing.T) {
	adjacency, lanAdjacency, prefixSID, span := rfc9085SIDAttributes(t, &rfc9085SIDSources{
		adjacency:    []byte{0xff, 0xff, 0xff, 0xff, 0xff},
		lanAdjacency: []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		prefixSID:    []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		binding:      []byte{0, 0xff, 0, 8, 24, 198, 51, 100, 3, 6, 0, 0, 0xff, 0xff, 0xff, 0xff},
	})
	rfc9085WantValue(t, 1099, adjacency, []byte{0xfc, 0xff, 0, 0, 0x0f, 0xff, 0xff})
	rfc9085WantValue(t, 1100, lanAdjacency, []byte{0xfc, 0xff, 0, 0, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x0f, 0xff, 0xff})
	rfc9085WantValue(t, 1158, prefixSID, []byte{0xfc, 0xff, 0, 0, 0xff, 0xff, 0xff, 0xff})
	rfc9085WantValue(t, 1159, span, []byte{0, 0, 0, 8, 0x04, 0x86, 0, 8, 0, 0, 0, 0, 0xff, 0xff, 0xff, 0xff})
}
