// Design: docs/architecture/ospf/ospf-9-inter-area-abr.md -- Type 3/4 Summary-LSA origination.
// RFC: rfc/short/rfc2328.md -- Section 12.4.3 (flush a summary whose destination became unreachable).

package spf

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// unreachableABR is an ABR (1.1.1.1) in the backbone and area 0.0.0.1. Its
// backbone SPF result holds router 2.2.2.2 with stub 10.20.0.0/24; reachable
// says whether SPF reached 2.2.2.2. Area 0.0.0.1 has one Full neighbor
// (3.3.3.3 on eth1), so a summary-LSA originated or flushed into it is flooded
// there, and every LS Update sent is decoded into sent.
type unreachableABR struct {
	db   *lsdb.LSDB
	root types.RouterID
	area types.AreaID
	sent []packet.Packet
}

func newUnreachableABR(t *testing.T) *unreachableABR {
	t.Helper()
	a := &unreachableABR{root: testRID(t, "1.1.1.1"), area: areaID(t, "0.0.0.1")}
	clock := time.Unix(100, 0)
	a.db = lsdb.New(func() time.Time { return clock })
	a.db.SetTopology(func() []lsdb.InterfaceInfo {
		return []lsdb.InterfaceInfo{{
			Name: "eth1", AreaID: a.area, AreaType: types.AreaTypeNormal,
			NetworkType: types.NetworkBroadcast, State: lsdb.InterfaceStateDR,
			Address: [4]byte{10, 0, 1, 1}, NetworkMask: [4]byte{255, 255, 255, 0},
			RouterID: a.root, DR: a.root, TransmitDelay: 1,
			Neighbors: []lsdb.NeighborInfo{{RouterID: testRID(t, "3.3.3.3"), Address: netip.MustParseAddr("10.0.1.3"), State: lsdb.NeighborStateFull}},
		}}
	})
	a.db.SetTx(func(_ string, _ netip.Addr, payload []byte) error {
		p, err := packet.DecodePacket(payload)
		if err != nil {
			return err
		}
		a.sent = append(a.sent, p)
		return nil
	})
	return a
}

// pass runs one summary origination pass with 2.2.2.2 reachable or not.
func (a *unreachableABR) pass(t *testing.T, reachable bool) {
	t.Helper()
	backbone := types.BackboneArea
	far := testRID(t, "2.2.2.2")
	res := resultWithStub(backbone, a.root, "10.10.0.0", 7)
	res.Graph.Routers[far] = &RouterVertex{ID: far, Links: []packet.RouterLink{stubLinkFromPrefix("10.20.0.0", 3)}}
	if reachable {
		res.Nodes[routerVertex(far)] = &NodeResult{ID: routerVertex(far), Metric: 10}
	}
	OriginateSummaries(SummaryInput{
		Sink:    a.db,
		Root:    a.root,
		Areas:   []types.AreaID{backbone, a.area},
		Options: map[types.AreaID]types.Options{backbone: types.OptionE, a.area: types.OptionE},
		Results: map[types.AreaID]*Result{backbone: res},
	})
}

// floodedMaxAge reports whether an LS Update carried key at MaxAge.
func (a *unreachableABR) floodedMaxAge(key types.LSAKey) bool {
	for i := range a.sent {
		if a.sent[i].LSUpdate == nil {
			continue
		}
		for _, l := range a.sent[i].LSUpdate.LSAs {
			if l.Header.Key() == key && l.Header.Age.IsMaxAge() {
				return true
			}
		}
	}
	return false
}

// VALIDATES: the whole step from an unreachable destination to the flush. Pass
// one, with 2.2.2.2 reachable, originates the Type 3 summary for 10.20.0.0/24
// into area 0.0.0.1. Pass two runs from an SPF result in which 2.2.2.2 is no
// longer reached: the summary's age becomes MaxAge in the database and the
// MaxAge instance is flooded to 3.3.3.3, while the still-reachable 10.10.0.0
// summary is untouched.
//
// RFC requirement: RFC2328-12.4-1 positive -- after SPF stops reaching 2.2.2.2, the next OriginateSummaries pass sets the self-originated Type 3 summary for its 10.20.0.0/24 to MaxAge in area 0.0.0.1 and refloods that MaxAge instance out of eth1; the summary for the still-reachable 10.10.0.0/24 stays below MaxAge.
func TestRFC2328SummaryFlushedWhenDestinationUnreachable(t *testing.T) {
	a := newUnreachableABR(t)
	key := type3Key(a.root, testLSID(t, "10.20.0.0"))
	a.pass(t, true)
	if h, ok := a.db.Lookup(a.area, key); !ok || h.Age.IsMaxAge() {
		t.Fatalf("setup: summary for reachable 10.20.0.0/24 = %+v ok=%v, want originated below MaxAge", h, ok)
	}
	a.sent = nil
	a.pass(t, false)
	h, ok := a.db.Lookup(a.area, key)
	if !ok || !h.Age.IsMaxAge() {
		t.Fatalf("summary for unreachable 10.20.0.0/24 = %+v ok=%v, want MaxAge", h, ok)
	}
	if !a.floodedMaxAge(key) {
		t.Fatalf("the MaxAge summary for 10.20.0.0/24 was not reflooded (sent %d packets)", len(a.sent))
	}
	if kept, ok := a.db.Lookup(a.area, type3Key(a.root, testLSID(t, "10.10.0.0"))); !ok || kept.Age.IsMaxAge() {
		t.Fatalf("summary for still-reachable 10.10.0.0/24 = %+v ok=%v, want kept below MaxAge", kept, ok)
	}
}

// VALIDATES: a destination that stays reachable keeps its summary. Two passes
// with 2.2.2.2 reachable both times leave the 10.20.0.0/24 summary below
// MaxAge, and no MaxAge copy of it is flooded.
//
// RFC requirement: RFC2328-12.4-1 negative -- while SPF still reaches 2.2.2.2, a second OriginateSummaries pass neither ages the 10.20.0.0/24 summary to MaxAge nor floods a MaxAge instance of it.
func TestRFC2328SummaryKeptWhileDestinationReachable(t *testing.T) {
	a := newUnreachableABR(t)
	key := type3Key(a.root, testLSID(t, "10.20.0.0"))
	a.pass(t, true)
	a.sent = nil
	a.pass(t, true)
	if h, ok := a.db.Lookup(a.area, key); !ok || h.Age.IsMaxAge() {
		t.Fatalf("summary for reachable 10.20.0.0/24 = %+v ok=%v, want kept below MaxAge", h, ok)
	}
	if a.floodedMaxAge(key) {
		t.Fatal("a MaxAge copy of the reachable 10.20.0.0/24 summary was flooded")
	}
}
