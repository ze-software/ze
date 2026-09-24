// Design: docs/architecture/wire/nlri-bgpls.md -- native IS-IS SRv6 BGP-LS origination.
// RFC: rfc/short/rfc9514.md

package isis

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// rfc9514Inputs describes the native IS-IS SRv6 advertisements of one node.
type rfc9514Inputs struct {
	capabilities [][]byte // SRv6 Capabilities sub-TLV (25) values
	endFlags     byte     // native End SID flags
	locatorAlgo  byte     // native locator algorithm
	adjFlags     byte     // native End.X and LAN End.X flags
	adjWeight    byte     // native End.X and LAN End.X weight
}

// rfc9514Snapshot stores one L1 LSP carrying Router Capability, an SRv6
// locator with one End SID, and an extended IS reachability with an End.X
// (43) and a LAN End.X (44) SID, then builds the native BGP-LS snapshot.
func rfc9514Snapshot(t *testing.T, in rfc9514Inputs) *linkstateevents.Snapshot {
	t.Helper()
	eng := newEngine(transport.New(&fakeBackend{}))
	t.Cleanup(eng.shutdown)
	id := types.LSPID{0, 0, 0, 0, 0, 2, 0, 0}
	capabilities := []byte{192, 0, 2, 2, 0}
	for _, c := range in.capabilities {
		capabilities = append(capabilities, 25, byte(len(c)))
		capabilities = append(capabilities, c...)
	}
	end := []byte{in.endFlags, 0, 1}
	endAddress := netip.MustParseAddr("2001:db8:1::1").As16()
	end = append(end, endAddress[:]...)
	end = append(end, 0)
	locator := []byte{0, 2, 0, 0, 0, 10, 0, in.locatorAlgo, 64, 0x20, 1, 0x0d, 0xb8, 0, 1, 0, 0, byte(len(end) + 2), 5, byte(len(end))}
	locator = append(locator, end...)
	adjAddress := netip.MustParseAddr("2001:db8:1::2").As16()
	adjacency := []byte{in.adjFlags, in.locatorAlgo, in.adjWeight, 0, 5}
	adjacency = append(adjacency, adjAddress[:]...)
	adjacency = append(adjacency, 0)
	lanAddress := netip.MustParseAddr("2001:db8:1::3").As16()
	lan := []byte{0, 0, 0, 0, 0, 7, in.adjFlags, in.locatorAlgo, in.adjWeight, 0, 6}
	lan = append(lan, lanAddress[:]...)
	lan = append(lan, 0)
	link := []byte{0, 2, 0, 0, 0, 0, 0, 3, 0, 0, 0, 10, byte(len(adjacency) + len(lan) + 4), 43, byte(len(adjacency))}
	link = append(link, adjacency...)
	link = append(link, 44, byte(len(lan)))
	link = append(link, lan...)
	bgplsStoreLSP(t, eng, lsdb.Level1, id, 1, 1200, []packet.TLV{
		{Type: 242, Value: capabilities},
		{Type: 27, Value: locator},
		{Type: 222, Value: link},
	})
	var builder bgplsBuilder
	builder.build(eng.lsdb.RawSnapshot(lsdb.Level1))
	return &builder.snapshot
}

func rfc9514Attributes(attrs []linkstateevents.TLV, typ uint16) [][]byte {
	var values [][]byte
	for _, attr := range attrs {
		if attr.Type == typ {
			values = append(values, attr.Value)
		}
	}
	return values
}

// VALIDATES: an SRv6-capable IS-IS node's BGP-LS Node attributes carry exactly
// one SRv6 Capabilities TLV, even when the LSP repeats the native sub-TLV.
// PREVENTS: a node without the TLV, or with two instances of it.
func TestRFC9514ISISSRv6CapabilitiesSingleInstance(t *testing.T) {
	// RFC requirement: RFC9514-3.1-1 positive -- an IS-IS node advertising SRv6 Capabilities gets exactly one TLV 1038 carrying its flags and a zero Reserved field.
	snapshot := rfc9514Snapshot(t, rfc9514Inputs{capabilities: [][]byte{{0x40, 0}}})
	if len(snapshot.Nodes) != 1 {
		t.Fatalf("nodes = %+v", snapshot.Nodes)
	}
	got := rfc9514Attributes(snapshot.Nodes[0].Attributes, 1038)
	if len(got) != 1 || !bytes.Equal(got[0], []byte{0x40, 0, 0, 0}) {
		t.Fatalf("SRv6 Capabilities = %x, want one 40000000", got)
	}
	// RFC requirement: RFC9514-3.1-1 negative -- a repeated native SRv6 Capabilities sub-TLV does not produce a second TLV 1038; the first instance is kept.
	snapshot = rfc9514Snapshot(t, rfc9514Inputs{capabilities: [][]byte{{0x40, 0}, {0, 0}}})
	got = rfc9514Attributes(snapshot.Nodes[0].Attributes, 1038)
	if len(got) != 1 || !bytes.Equal(got[0], []byte{0x40, 0, 0, 0}) {
		t.Fatalf("repeated SRv6 Capabilities = %x, want one 40000000", got)
	}
}

// VALIDATES: the End.X (1106) and LAN End.X (1107) SID TLVs the IS-IS source
// originates carry a zero Reserved octet, whatever the native flags and weight.
// PREVENTS: a native bit landing in the Reserved octet.
func TestRFC9514ISISEndXReservedZero(t *testing.T) {
	for _, tc := range []struct {
		name          string
		flags, weight byte
	}{
		// RFC requirement: RFC9514-4.1-1 positive -- an End.X SID TLV originated from ordinary native values has Reserved octet 0.
		// RFC requirement: RFC9514-4.2-1 positive -- a LAN End.X SID TLV originated from ordinary native values has Reserved octet 0.
		{"clean", 0x20, 1},
		// RFC requirement: RFC9514-4.1-1 negative -- native End.X flags and weight of 0xff do not reach the Reserved octet.
		// RFC requirement: RFC9514-4.2-1 negative -- native LAN End.X flags and weight of 0xff do not reach the Reserved octet.
		{"all-ones", 0xff, 0xff},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := rfc9514Snapshot(t, rfc9514Inputs{capabilities: [][]byte{{0x40, 0}}, adjFlags: tc.flags, adjWeight: tc.weight})
			if len(snapshot.Links) != 1 {
				t.Fatalf("links = %+v", snapshot.Links)
			}
			for _, typ := range []uint16{1106, 1107} {
				got := rfc9514Attributes(snapshot.Links[0].Attributes, typ)
				if len(got) != 1 {
					t.Fatalf("TLV %d = %x, want one", typ, got)
				}
				if got[0][4] != tc.weight || got[0][5] != 0 {
					t.Fatalf("TLV %d weight/reserved = %x/%x, want %x/00", typ, got[0][4], got[0][5], tc.weight)
				}
			}
		})
	}
}

// VALIDATES: the SRv6 Endpoint Behavior TLV (1250) the IS-IS source originates
// has a zero Flags octet, whatever the native End SID flags carry.
// PREVENTS: undefined native flag bits reaching the collector.
func TestRFC9514ISISEndpointBehaviorFlagsZero(t *testing.T) {
	// RFC requirement: RFC9514-7.1-2 positive -- an End SID with native flags 0 originates Endpoint Behavior 0x0001 with Flags 0.
	// RFC requirement: RFC9514-7.1-2 negative -- an End SID with native flags 0xff still originates Flags 0.
	for _, flags := range []byte{0, 0xff} {
		snapshot := rfc9514Snapshot(t, rfc9514Inputs{capabilities: [][]byte{{0x40, 0}}, endFlags: flags})
		if len(snapshot.SIDs) != 1 {
			t.Fatalf("SIDs = %+v", snapshot.SIDs)
		}
		got := rfc9514Attributes(snapshot.SIDs[0].Attributes, 1250)
		if len(got) != 1 || !bytes.Equal(got[0], []byte{0, 1, 0, 0}) {
			t.Fatalf("native flags %x: Endpoint Behavior = %x, want 00010000", flags, got)
		}
	}
}

// VALIDATES: the Endpoint Behavior Algorithm is the algorithm of the locator
// the SID is allocated from: 0 when the locator has none, 1 when it has one.
// PREVENTS: an algorithm the locator does not carry.
func TestRFC9514ISISEndpointBehaviorAlgorithm(t *testing.T) {
	// RFC requirement: RFC9514-7.1-3 positive -- a SID from a locator with algorithm 0 originates Algorithm 0, and a SID from a locator with algorithm 1 originates Algorithm 1.
	for _, algo := range []byte{0, 1} {
		snapshot := rfc9514Snapshot(t, rfc9514Inputs{capabilities: [][]byte{{0x40, 0}}, locatorAlgo: algo})
		if len(snapshot.SIDs) != 1 {
			t.Fatalf("locator algorithm %d: SIDs = %+v", algo, snapshot.SIDs)
		}
		got := rfc9514Attributes(snapshot.SIDs[0].Attributes, 1250)
		if len(got) != 1 || got[0][3] != algo {
			t.Fatalf("locator algorithm %d: Endpoint Behavior = %x", algo, got)
		}
	}
}
