// Design: docs/architecture/wire/nlri-bgpls.md -- native IS-IS LSDB export.
// Related: bgpls_export.go -- bgplsBuilder.lsp and capabilities, the producer.
//
// VALIDATES: RFC 9085 section 2.1 placement on the IS-IS exporter: the SR Capabilities
// (1034), SR Algorithm (1035) and SR Local Block (1036) TLVs translated from a Router
// Capability TLV 242 are added only to the Node NLRI of the IS-IS node whose LSP carries
// that TLV 242.
// PREVENTS: the capability TLVs attached to another node, or to a Link or Prefix NLRI.
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

// rfc9085PlacementSnapshot stores two Level 1 LSPs. 0000.0000.0002 carries an Extended
// IS Reachability (TLV 22) to 0000.0000.0003, an Extended IP Reachability (TLV 135) for
// 192.0.2.0/24, and a Router Capability (TLV 242) with an SR-Capabilities sub-TLV 2
// (I flag, SRGB 16000/100), an SR-Algorithm sub-TLV 19 (algorithm 0) and an SRLB sub-TLV
// 22 (16000/100). 0000.0000.0003 carries TLV 22 back and TLV 135 for 198.51.100.0/24, and
// no TLV 242. It returns the BGP-LS snapshot the builder produces.
func rfc9085PlacementSnapshot(t *testing.T) *linkstateevents.Snapshot {
	t.Helper()
	eng := newEngine(transport.New(&fakeBackend{}))
	t.Cleanup(eng.shutdown)
	block := []byte{0x80, 0, 0, 100, 1, 3, 0x00, 0x3e, 0x80}
	capabilities := []byte{192, 0, 2, 2, 0}
	capabilities = append(capabilities, 2, byte(len(block)))
	capabilities = append(capabilities, block...)
	capabilities = append(capabilities, 19, 1, 0, 22, byte(len(block)))
	capabilities = append(capabilities, block...)
	bgplsStoreLSP(t, eng, lsdb.Level1, types.LSPID{0, 0, 0, 0, 0, 2, 0, 0}, 1, 1200, []packet.TLV{
		{Type: 22, Value: []byte{0, 0, 0, 0, 0, 3, 0, 0, 0, 10, 0}},
		{Type: 135, Value: []byte{0, 0, 0, 10, 24, 192, 0, 2}},
		{Type: 242, Value: capabilities},
	})
	bgplsStoreLSP(t, eng, lsdb.Level1, types.LSPID{0, 0, 0, 0, 0, 3, 0, 0}, 1, 1200, []packet.TLV{
		{Type: 22, Value: []byte{0, 0, 0, 0, 0, 2, 0, 0, 0, 10, 0}},
		{Type: 135, Value: []byte{0, 0, 0, 10, 24, 198, 51, 100}},
	})
	var builder bgplsBuilder
	builder.build(eng.lsdb.RawSnapshot(lsdb.Level1))
	return &builder.snapshot
}

// rfc9085PlacementNode returns the Node NLRI whose IS-IS router ID is systemID.
func rfc9085PlacementNode(t *testing.T, snapshot *linkstateevents.Snapshot, systemID []byte) *linkstateevents.Node {
	t.Helper()
	for i := range snapshot.Nodes {
		if bytes.Equal(snapshot.Nodes[i].ID.RouterID, systemID) {
			return &snapshot.Nodes[i]
		}
	}
	t.Fatalf("no Node NLRI for %x in %+v", systemID, snapshot.Nodes)
	return nil
}

// RFC requirement: RFC9085-2.1-1 positive -- on the IS-IS exporter, the Node NLRI of
// 0000.0000.0002, whose LSP carries the Router Capability TLV 242, gets exactly one TLV
// 1034 (80 00 00 00 64 04 89 00 03 00 3e 80), one TLV 1035 (00) and one TLV 1036
// (00 00 00 00 64 04 89 00 03 00 3e 80).
func TestRFC9085ISISCapabilitiesOnOriginatorNode(t *testing.T) {
	// Goal: the capability TLVs land on the originating node. Method: two LSPs through the
	// LSDB, the BGP-LS builder run, the originator's Node NLRI read.
	snapshot := rfc9085PlacementSnapshot(t)
	node := rfc9085PlacementNode(t, snapshot, []byte{0, 0, 0, 0, 0, 2})
	want := map[uint16][]byte{
		1034: {0x80, 0, 0, 0, 100, 0x04, 0x89, 0, 3, 0x00, 0x3e, 0x80},
		1035: {0},
		1036: {0, 0, 0, 0, 100, 0x04, 0x89, 0, 3, 0x00, 0x3e, 0x80},
	}
	for typ, value := range want {
		got := rfc9514Attributes(node.Attributes, typ)
		if len(got) != 1 {
			t.Fatalf("node 0000.0000.0002 carries %d TLV %d, want 1: %x", len(got), typ, got)
		}
		if !bytes.Equal(got[0], value) {
			t.Fatalf("node 0000.0000.0002 TLV %d = %x, want %x", typ, got[0], value)
		}
	}
}

// RFC requirement: RFC9085-2.1-1 negative -- on the same IS-IS export, the Node NLRI of
// 0000.0000.0003 (no TLV 242 in its LSP), every Link NLRI (two asserted present) and every
// Prefix NLRI (two asserted present) carry no TLV 1034, 1035 or 1036.
func TestRFC9085ISISCapabilitiesOnNoOtherNLRI(t *testing.T) {
	// Goal: the capability TLVs are added nowhere but the originator's Node NLRI. Method:
	// the positive's LSDB, every other NLRI's attributes read.
	snapshot := rfc9085PlacementSnapshot(t)
	other := rfc9085PlacementNode(t, snapshot, []byte{0, 0, 0, 0, 0, 3})
	if len(snapshot.Links) < 2 {
		t.Fatalf("Link NLRIs = %d, want both directions: %+v", len(snapshot.Links), snapshot.Links)
	}
	if len(snapshot.Prefixes) < 2 {
		t.Fatalf("Prefix NLRIs = %d, want both prefixes: %+v", len(snapshot.Prefixes), snapshot.Prefixes)
	}
	for _, typ := range []uint16{1034, 1035, 1036} {
		if got := rfc9514Attributes(other.Attributes, typ); len(got) != 0 {
			t.Fatalf("node 0000.0000.0003 carries TLV %d: %x", typ, got)
		}
		for i := range snapshot.Links {
			if got := rfc9514Attributes(snapshot.Links[i].Attributes, typ); len(got) != 0 {
				t.Fatalf("a Link NLRI carries TLV %d: %x", typ, got)
			}
		}
		for i := range snapshot.Prefixes {
			if got := rfc9514Attributes(snapshot.Prefixes[i].Attributes, typ); len(got) != 0 {
				t.Fatalf("a Prefix NLRI carries TLV %d: %x", typ, got)
			}
		}
	}
}
