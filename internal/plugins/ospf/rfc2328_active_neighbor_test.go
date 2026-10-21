// Design: docs/architecture/ospf/ospf-7-lsdb-flooding.md -- packets accepted only on adjacencies.
// Related: instance.go -- handleLSUpdate, handleLSAck and handleNeighborPacket.
// Related: neighbor/table.go -- AcceptsFlooding, the neighbor-state gate.
//
// VALIDATES: RFC 2328 Section 8.2, "All other packet types are sent/received only on
// adjacencies. This means that the packet must have been sent by one of the router's active
// neighbors.", and RFC 2328 Section 13.7, "If this neighbor is in a lesser state than
// Exchange, the Link State Acknowledgment packet is discarded.", on the engine's dispatcher.
// PREVENTS: a Database Description, Link State Request, Link State Update or Link State
// Acknowledgment from a router that is not an active neighbor reaching the database or the
// neighbor state machine, and an acknowledgment from a neighbor below Exchange clearing a
// retransmission.
package ospf

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// dropReasonRegistry captures the labels of every ze_ospf_packets_dropped_total increment.
type dropReasonRegistry struct {
	metrics.NopRegistry
	mu    sync.Mutex
	drops [][]string
}

func (r *dropReasonRegistry) CounterVec(name, help string, labels []string) metrics.CounterVec {
	if name == "ze_ospf_packets_dropped_total" {
		return dropReasonVec{r: r}
	}
	return r.NopRegistry.CounterVec(name, help, labels)
}

// count returns how many drops were recorded with reason.
func (r *dropReasonRegistry) count(reason string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, labels := range r.drops {
		if len(labels) == 2 && labels[1] == reason {
			n++
		}
	}
	return n
}

type dropReasonVec struct{ r *dropReasonRegistry }

func (v dropReasonVec) With(labels ...string) metrics.Counter {
	v.r.mu.Lock()
	v.r.drops = append(v.r.drops, append([]string(nil), labels...))
	v.r.mu.Unlock()
	return metrics.NopRegistry{}.Counter("", "")
}
func (dropReasonVec) Delete(...string) bool { return false }

// lsaSends records the header of every LSA carried by an LS Update the database transmits,
// and counts every packet sent.
type lsaSends struct {
	mu      sync.Mutex
	headers []packet.LSAHeader
	packets int
}

func (s *lsaSends) send(_ string, _ netip.Addr, payload []byte) error {
	p, err := packet.DecodePacket(payload)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.packets++
	if p.LSUpdate != nil {
		for _, lsa := range p.LSUpdate.LSAs {
			s.headers = append(s.headers, lsa.Header)
		}
	}
	return nil
}

func (s *lsaSends) carried(key types.LSAKey) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, h := range s.headers {
		if h.Key() == key {
			n++
		}
	}
	return n
}

// carriedMaxAge counts the transmitted instances of key whose LS age is MaxAge.
func (s *lsaSends) carriedMaxAge(key types.LSAKey) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, h := range s.headers {
		if h.Key() == key && h.Age.IsMaxAge() {
			n++
		}
	}
	return n
}

func (s *lsaSends) total() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.packets
}

// activeNeighborEngine is floodGateEngine with the drop reasons and every LSA sent recorded.
func activeNeighborEngine(t *testing.T) (*engine, *dropReasonRegistry, *lsaSends, func(packet.Packet)) {
	t.Helper()
	eng, _, dispatch := floodGateEngine(t)
	reg := &dropReasonRegistry{}
	eng.transport.SetMetrics(reg)
	sends := &lsaSends{}
	eng.lsdb.SetTx(sends.send)
	return eng, reg, sends, dispatch
}

// bringToExchange drives 2.2.2.2 to Exchange: a Hello listing us, then the master's initial
// Database Description.
func bringToExchange(t *testing.T, eng *engine, dispatch func(packet.Packet), header packet.Header) {
	t.Helper()
	dispatch(packet.Packet{Header: header, Hello: floodGateHello(mustRouterID(t, "1.1.1.1"), true)})
	dispatch(packet.Packet{Header: header, DBDesc: &packet.DBDesc{
		InterfaceMTU: 1500, Options: types.OptionE,
		Flags: packet.DDFlagInit | packet.DDFlagMore | packet.DDFlagMaster, DDSequence: 7,
	}})
	if reason := eng.neighbors.AcceptsFlooding("eth0", header.RouterID); reason != "" {
		t.Fatalf("setup: neighbor did not reach Exchange: %s", reason)
	}
}

// RFC requirement: RFC2328-8.2-5 negative -- a Database Description, a Link State Request, a Link State Update and a Link State Acknowledgment from 9.9.9.9, a router that sent no Hello and so is not an active neighbor on eth0, are each dropped with reason "neighbor": four such drops are counted, no neighbor row exists for 9.9.9.9, the update's LSA is not in the database, and nothing is transmitted even after the delayed acknowledgments are flushed (handleNeighborPacket and AcceptsFlooding, instance.go, neighbor/table.go).
func TestRFC2328NonHelloFromUnknownRouterDropped(t *testing.T) {
	eng, reg, sends, dispatch := activeNeighborEngine(t)
	stranger := mustRouterID(t, "9.9.9.9")
	header := packet.Header{RouterID: stranger, AreaID: types.BackboneArea}
	lsa := routerLSAForTest(t, mustRouterID(t, "4.4.4.4"), types.InitialSequenceNumber, 0)

	dispatch(packet.Packet{Header: header, DBDesc: &packet.DBDesc{
		InterfaceMTU: 1500, Options: types.OptionE,
		Flags: packet.DDFlagInit | packet.DDFlagMore | packet.DDFlagMaster, DDSequence: 7,
	}})
	dispatch(packet.Packet{Header: header, LSReq: &packet.LSReq{Requests: []types.LSRequestEntry{{Type: lsa.Header.Type, LinkStateID: lsa.Header.LinkStateID, AdvertisingRouter: lsa.Header.AdvertisingRouter}}}})
	dispatch(packet.Packet{Header: header, LSUpdate: &packet.LSUpdate{LSAs: []packet.LSA{lsa}}})
	dispatch(packet.Packet{Header: header, LSAck: &packet.LSAck{Headers: []packet.LSAHeader{lsa.Header}}})
	eng.lsdb.FlushDelayedAcks("eth0")

	if n := reg.count("neighbor"); n != 4 {
		t.Fatalf("drops with reason neighbor = %d, want 4 (one per non-Hello packet from a non-neighbor)", n)
	}
	if _, ok := eng.neighbors.Lookup("eth0", stranger); ok {
		t.Fatalf("a non-Hello packet created a neighbor entry for %s", stranger)
	}
	if _, ok := eng.lsdb.Lookup(types.BackboneArea, lsa.Header.Key()); ok {
		t.Fatal("an LS Update from a router that is not an active neighbor reached the database")
	}
	if n := sends.total(); n != 0 {
		t.Fatalf("packets from a router that is not an active neighbor produced %d transmissions", n)
	}
}

// RFC requirement: RFC2328-8.2-5 positive -- once 2.2.2.2 is an active neighbor in Exchange, its Link State Update is installed in the database and its Link State Acknowledgment clears the retransmission owed to it, and none of its packets is dropped with reason "neighbor" or "state" (handleLSUpdate and handleLSAck pass AcceptsFlooding, instance.go).
// RFC requirement: RFC2328-13.7-1 positive -- a Link State Acknowledgment from a neighbor in state Exchange is processed: the summary-LSA this router originated was on 2.2.2.2's retransmission list (a retransmission tick resends it), and after 2.2.2.2 acknowledges it a later tick resends it no more (handleLSAck, ReceiveAck).
func TestRFC2328ActiveNeighborPacketsProcessed(t *testing.T) {
	eng, reg, sends, dispatch := activeNeighborEngine(t)
	peer := mustRouterID(t, "2.2.2.2")
	header := packet.Header{RouterID: peer, AreaID: types.BackboneArea}
	bringToExchange(t, eng, dispatch, header)

	lsa := routerLSAForTest(t, mustRouterID(t, "4.4.4.4"), types.InitialSequenceNumber, 0)
	dispatch(packet.Packet{Header: header, LSUpdate: &packet.LSUpdate{LSAs: []packet.LSA{lsa}}})
	if got, ok := eng.lsdb.Lookup(types.BackboneArea, lsa.Header.Key()); !ok || got.Sequence != lsa.Header.Sequence {
		t.Fatalf("the LS Update from an active neighbor was not installed: %+v, %v", got, ok)
	}

	h, ok := eng.lsdb.OriginateSummary(types.BackboneArea, mustRouterID(t, "1.1.1.1"), types.OptionE, types.LSTypeSummaryNetwork, types.LinkStateID{192, 0, 2, 0}, [4]byte{255, 255, 255, 0}, 10)
	if !ok {
		t.Fatal("setup: OriginateSummary refused")
	}
	now := time.Now()
	before := sends.carried(h.Key())
	eng.lsdb.RetransmitTick(now.Add(time.Hour))
	if sends.carried(h.Key()) == before {
		t.Fatal("setup: the originated summary-LSA was not on 2.2.2.2's retransmission list")
	}

	dispatch(packet.Packet{Header: header, LSAck: &packet.LSAck{Headers: []packet.LSAHeader{h}}})
	acked := sends.carried(h.Key())
	eng.lsdb.RetransmitTick(now.Add(2 * time.Hour))
	if n := sends.carried(h.Key()) - acked; n != 0 {
		t.Fatalf("the summary-LSA was retransmitted %d times after an Exchange neighbor acknowledged it", n)
	}
	if n := reg.count("neighbor") + reg.count("state"); n != 0 {
		t.Fatalf("an active neighbor's packets were dropped %d times for reason neighbor or state", n)
	}
}

// RFC requirement: RFC2328-13.7-1 negative -- a Link State Acknowledgment from 2.2.2.2 while it is in Init (a Hello that does not list us) and again in ExStart (a Hello that does) is discarded: each is dropped with reason "state", so two such drops are counted and nothing is transmitted (handleLSAck, AcceptsFlooding state gate, neighbor/table.go).
func TestRFC2328LSAckBelowExchangeDiscarded(t *testing.T) {
	eng, reg, sends, dispatch := activeNeighborEngine(t)
	self := mustRouterID(t, "1.1.1.1")
	peer := mustRouterID(t, "2.2.2.2")
	header := packet.Header{RouterID: peer, AreaID: types.BackboneArea}
	ack := packet.LSAck{Headers: []packet.LSAHeader{routerLSAForTest(t, self, types.InitialSequenceNumber, 0).Header}}

	dispatch(packet.Packet{Header: header, Hello: floodGateHello(self, false)})
	if reason := eng.neighbors.AcceptsFlooding("eth0", peer); reason != "state" {
		t.Fatalf("setup: Init neighbor AcceptsFlooding = %q, want state", reason)
	}
	dispatch(packet.Packet{Header: header, LSAck: &ack})

	dispatch(packet.Packet{Header: header, Hello: floodGateHello(self, true)})
	if reason := eng.neighbors.AcceptsFlooding("eth0", peer); reason != "state" {
		t.Fatalf("setup: ExStart neighbor AcceptsFlooding = %q, want state", reason)
	}
	dispatch(packet.Packet{Header: header, LSAck: &ack})

	if n := reg.count("state"); n != 2 {
		t.Fatalf("LS Acks dropped with reason state = %d, want 2 (one in Init, one in ExStart)", n)
	}
	if n := sends.total(); n != 0 {
		t.Fatalf("an LS Ack from a neighbor below Exchange produced %d transmissions", n)
	}
}
