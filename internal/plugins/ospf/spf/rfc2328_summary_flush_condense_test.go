// Design: docs/architecture/ospf/ospf-9-inter-area-abr.md -- Type 3/4 Summary-LSA origination.
// RFC: rfc/short/rfc2328.md -- Section 16.7 (flush a summary no longer advertised) and
// Section 12.4.3 (condense networks inside a configured area range).
//
// VALIDATES: through OriginateSummaries into a real LSDB: a summary-LSA whose routing entry
// was deleted is flushed at MaxAge and reflooded; a summary-LSA no longer advertisable to
// one area is flushed there and kept in the other; several networks inside one configured
// range give one Type 3 summary-LSA, while a network outside every range keeps its own.
// PREVENTS: a deleted or newly suppressed destination staying advertised, a flush leaking
// into an area that still wants the summary, and a range emitting one LSA per component.
package spf

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// flushABR is an ABR (1.1.1.1) in the backbone, area 0.0.0.1 (eth1, Full neighbor 3.3.3.3)
// and area 0.0.0.2 (eth2, Full neighbor 4.4.4.4). Every LS Update it sends is decoded into
// sent, keyed by the interface it left on.
type flushABR struct {
	db    *lsdb.LSDB
	root  types.RouterID
	area1 types.AreaID
	area2 types.AreaID
	sent  map[string][]packet.Packet
}

func newFlushABR(t *testing.T) *flushABR {
	t.Helper()
	a := &flushABR{root: testRID(t, "1.1.1.1"), area1: areaID(t, "0.0.0.1"), area2: areaID(t, "0.0.0.2"), sent: map[string][]packet.Packet{}}
	clock := time.Unix(100, 0)
	a.db = lsdb.New(func() time.Time { return clock })
	iface := func(name string, area types.AreaID, addr [4]byte, nbr string, nbrAddr string) lsdb.InterfaceInfo {
		return lsdb.InterfaceInfo{
			Name: name, AreaID: area, AreaType: types.AreaTypeNormal,
			NetworkType: types.NetworkBroadcast, State: lsdb.InterfaceStateDR,
			Address: addr, NetworkMask: [4]byte{255, 255, 255, 0},
			RouterID: a.root, DR: a.root, TransmitDelay: 1,
			Neighbors: []lsdb.NeighborInfo{{RouterID: testRID(t, nbr), Address: netip.MustParseAddr(nbrAddr), State: lsdb.NeighborStateFull}},
		}
	}
	a.db.SetTopology(func() []lsdb.InterfaceInfo {
		return []lsdb.InterfaceInfo{
			iface("eth1", a.area1, [4]byte{10, 0, 1, 1}, "3.3.3.3", "10.0.1.3"),
			iface("eth2", a.area2, [4]byte{10, 0, 2, 1}, "4.4.4.4", "10.0.2.4"),
		}
	})
	a.db.SetTx(func(name string, _ netip.Addr, payload []byte) error {
		p, err := packet.DecodePacket(payload)
		if err != nil {
			return err
		}
		a.sent[name] = append(a.sent[name], p)
		return nil
	})
	return a
}

// pass runs one origination pass. stubs are the root's backbone /24 stub networks;
// policy2 is area 0.0.0.2's summary policy.
func (a *flushABR) pass(stubs []string, policy2 AreaSummaryPolicy) {
	backbone := types.BackboneArea
	res := resultWithStub(backbone, a.root, stubs[0], 7)
	for _, s := range stubs[1:] {
		res.Graph.Routers[a.root].Links = append(res.Graph.Routers[a.root].Links, stubLinkFromPrefix(s, 7))
	}
	OriginateSummaries(SummaryInput{
		Sink:     a.db,
		Root:     a.root,
		Areas:    []types.AreaID{backbone, a.area1, a.area2},
		Options:  map[types.AreaID]types.Options{backbone: types.OptionE, a.area1: types.OptionE, a.area2: types.OptionE},
		Results:  map[types.AreaID]*Result{backbone: res},
		Policies: map[types.AreaID]AreaSummaryPolicy{a.area2: policy2},
	})
}

// floodedMaxAge reports whether an LS Update sent on iface carried key at MaxAge.
func (a *flushABR) floodedMaxAge(iface string, key types.LSAKey) bool {
	for _, p := range a.sent[iface] {
		if p.LSUpdate == nil {
			continue
		}
		for _, l := range p.LSUpdate.LSAs {
			if l.Header.Key() == key && l.Header.Age.IsMaxAge() {
				return true
			}
		}
	}
	return false
}

// RFC requirement: RFC2328-16.7-1 positive -- a previously advertised entry that has been deleted, and one no longer advertisable to a particular area, are each flushed by MaxAge and reflooding: driven through OriginateSummaries, the summary for 10.20.0.0/24 whose backbone stub was removed is at MaxAge in area 0.0.0.1 and flooded at MaxAge on eth1 while 10.10.0.0/24 stays below MaxAge; and when area 0.0.0.2 becomes totally stubby, the 10.10.0.0/24 summary is at MaxAge in area 0.0.0.2 and flooded at MaxAge on eth2, while in area 0.0.0.1 it stays below MaxAge and is never flooded at MaxAge on eth1 (OriginateSummaries, FlushStaleSummaryLSAs).
func TestRFC2328SummaryFlushedWhenDeletedOrNoLongerAdvertisableToArea(t *testing.T) {
	normal := AreaSummaryPolicy{Type: types.AreaTypeNormal}
	t.Run("deleted-entry", func(t *testing.T) {
		a := newFlushABR(t)
		gone := type3Key(a.root, testLSID(t, "10.20.0.0"))
		kept := type3Key(a.root, testLSID(t, "10.10.0.0"))
		a.pass([]string{"10.10.0.0", "10.20.0.0"}, normal)
		if h, ok := a.db.Lookup(a.area1, gone); !ok || h.Age.IsMaxAge() {
			t.Fatalf("setup: summary for 10.20.0.0/24 = %+v ok=%v, want originated below MaxAge", h, ok)
		}
		a.sent = map[string][]packet.Packet{}
		a.pass([]string{"10.10.0.0"}, normal)
		if h, ok := a.db.Lookup(a.area1, gone); !ok || !h.Age.IsMaxAge() {
			t.Fatalf("summary for the deleted 10.20.0.0/24 = %+v ok=%v, want MaxAge", h, ok)
		}
		if !a.floodedMaxAge("eth1", gone) {
			t.Fatal("the MaxAge summary for the deleted 10.20.0.0/24 was not reflooded on eth1")
		}
		if h, ok := a.db.Lookup(a.area1, kept); !ok || h.Age.IsMaxAge() {
			t.Fatalf("summary for the still-present 10.10.0.0/24 = %+v ok=%v, want kept below MaxAge", h, ok)
		}
	})
	t.Run("no-longer-advertisable-to-one-area", func(t *testing.T) {
		a := newFlushABR(t)
		key := type3Key(a.root, testLSID(t, "10.10.0.0"))
		a.pass([]string{"10.10.0.0"}, normal)
		for _, area := range []types.AreaID{a.area1, a.area2} {
			if h, ok := a.db.Lookup(area, key); !ok || h.Age.IsMaxAge() {
				t.Fatalf("setup: summary for 10.10.0.0/24 in %v = %+v ok=%v, want originated below MaxAge", area, h, ok)
			}
		}
		a.sent = map[string][]packet.Packet{}
		a.pass([]string{"10.10.0.0"}, AreaSummaryPolicy{Type: types.AreaTypeStub, NoSummary: true, DefaultCost: 1})
		if h, ok := a.db.Lookup(a.area2, key); !ok || !h.Age.IsMaxAge() {
			t.Fatalf("summary for 10.10.0.0/24 in the now totally-stubby area 0.0.0.2 = %+v ok=%v, want MaxAge", h, ok)
		}
		if !a.floodedMaxAge("eth2", key) {
			t.Fatal("the MaxAge summary for 10.10.0.0/24 was not reflooded into area 0.0.0.2 on eth2")
		}
		if h, ok := a.db.Lookup(a.area1, key); !ok || h.Age.IsMaxAge() {
			t.Fatalf("summary for 10.10.0.0/24 in area 0.0.0.1 = %+v ok=%v, want kept below MaxAge", h, ok)
		}
		if a.floodedMaxAge("eth1", key) {
			t.Fatal("the flush of 10.10.0.0/24 leaked into area 0.0.0.1, which still advertises it")
		}
	})
}

// condenseABR runs one OriginateSummaries pass from a backbone holding the root's /24 stub
// networks stubs into area 0.0.0.1 under ranges, and returns that area's self-originated
// Type 3 summary-LSAs, keyed by Link State ID, with their masks.
func condenseABR(t *testing.T, stubs []string, ranges []AreaRange) map[types.LinkStateID][4]byte {
	t.Helper()
	root := testRID(t, "1.1.1.1")
	other := areaID(t, "0.0.0.1")
	db := lsdb.New(nil)
	in := abrInput(db, root, other, AreaSummaryPolicy{Type: types.AreaTypeNormal}, ranges)
	res := resultWithStub(types.BackboneArea, root, stubs[0], 7)
	for _, s := range stubs[1:] {
		res.Graph.Routers[root].Links = append(res.Graph.Routers[root].Links, stubLinkFromPrefix(s, 3))
	}
	in.Results = map[types.AreaID]*Result{types.BackboneArea: res}
	OriginateSummaries(in)
	out := map[types.LinkStateID][4]byte{}
	for _, h := range db.Summary(other) {
		if h.Type != types.LSTypeSummaryNetwork || h.AdvertisingRouter != root {
			continue
		}
		lsa, ok := db.LookupLSA(other, h.Key())
		if !ok {
			t.Fatalf("summary %+v listed but not found", h)
		}
		body, err := lsa.DecodeSummary()
		if err != nil {
			t.Fatalf("DecodeSummary: %v", err)
		}
		out[h.LinkStateID] = body.NetworkMask
	}
	return out
}

// RFC requirement: RFC2328-12.4.3-2 positive -- the networks inside one configured range are condensed: three backbone networks 10.10.0.0/24, 10.10.1.0/24 and 10.10.2.0/24 under the range 10.10.0.0/16 give exactly one Type 3 summary-LSA in area 0.0.0.1, Link State ID 10.10.0.0 with mask 255.255.0.0, and none for 10.10.1.0 or 10.10.2.0 (OriginateSummaries, applyAreaRanges).
func TestRFC2328RangeCondensesSeveralNetworksIntoOneSummary(t *testing.T) {
	got := condenseABR(t, []string{"10.10.0.0", "10.10.1.0", "10.10.2.0"}, []AreaRange{{Prefix: netip.MustParsePrefix("10.10.0.0/16"), Advertise: true}})
	if len(got) != 1 {
		t.Fatalf("Type 3 summary-LSAs = %v, want exactly one for the range", got)
	}
	mask, ok := got[testLSID(t, "10.10.0.0")]
	if !ok {
		t.Fatalf("Type 3 summary-LSAs = %v, want the range 10.10.0.0", got)
	}
	if mask != ([4]byte{255, 255, 0, 0}) {
		t.Fatalf("range summary mask = %v, want 255.255.0.0", mask)
	}
}

// RFC requirement: RFC2328-12.4.3-2 negative -- condensing stops at the range boundary: with the range 10.10.0.0/16 configured, the backbone networks 10.10.0.0/24 and 10.10.1.0/24 give the one range summary-LSA (mask 255.255.0.0), and 10.30.0.0/24, outside every range, gets a Type 3 summary-LSA of its own with mask 255.255.255.0, two in all (OriginateSummaries, applyAreaRanges).
func TestRFC2328NetworkOutsideRangeKeepsOwnSummary(t *testing.T) {
	got := condenseABR(t, []string{"10.10.0.0", "10.10.1.0", "10.30.0.0"}, []AreaRange{{Prefix: netip.MustParsePrefix("10.10.0.0/16"), Advertise: true}})
	if len(got) != 2 {
		t.Fatalf("Type 3 summary-LSAs = %v, want two: the range and 10.30.0.0/24", got)
	}
	if mask := got[testLSID(t, "10.10.0.0")]; mask != ([4]byte{255, 255, 0, 0}) {
		t.Fatalf("range summary mask = %v, want 255.255.0.0", mask)
	}
	mask, ok := got[testLSID(t, "10.30.0.0")]
	if !ok {
		t.Fatalf("Type 3 summary-LSAs = %v, want one for 10.30.0.0/24 outside the range", got)
	}
	if mask != ([4]byte{255, 255, 255, 0}) {
		t.Fatalf("10.30.0.0 summary mask = %v, want 255.255.255.0", mask)
	}
}
