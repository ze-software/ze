// Design: docs/architecture/ospf/ospf-7-lsdb-flooding.md.
// VALIDATES: RFC 2328 Section 13: a Link State Update from a neighbor in a state lesser
// than Exchange is dropped by the engine without further processing, and the same packet
// from a neighbor in Exchange is processed.
// PREVENTS: an engine that ignores the neighbor-state gate letting a pre-Exchange
// neighbor's LSAs into the database, acknowledging them, or flooding them on.
package ospf

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// floodGateSends counts every LS Update and LS Ack the LSDB transmits.
type floodGateSends struct {
	mu      sync.Mutex
	packets int
}

func (s *floodGateSends) send(_ string, _ netip.Addr, payload []byte) error {
	p, err := packet.DecodePacket(payload)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.LSUpdate != nil {
		s.packets++
	}
	if p.LSAck != nil {
		s.packets++
	}
	return nil
}

func (s *floodGateSends) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.packets
}

// floodGateEngine opens point-to-point eth0 and returns a dispatcher for packets from
// 2.2.2.2. The neighbor's state is whatever the caller's packets make it.
func floodGateEngine(t *testing.T) (*engine, *floodGateSends, func(packet.Packet)) {
	t.Helper()
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"1.1.1.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"point-to-point"}}}}}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	backend := &fakeBackend{}
	eng := newEngine(transport.New(backend))
	eng.setConfig(cfg)
	t.Cleanup(eng.shutdown)
	if err := eng.openInterfaces(); err != nil {
		t.Fatal(err)
	}
	backend.mu.Lock()
	handle := backend.handles["eth0"]
	backend.mu.Unlock()
	if handle == nil {
		t.Fatal("eth0 transport handle missing")
	}
	sends := &floodGateSends{}
	eng.lsdb.SetTx(sends.send)
	return eng, sends, func(p packet.Packet) {
		eng.dispatch.dispatch(transport.RawPacket{IfIndex: handle.ifindex, Src: netip.MustParseAddr("10.0.0.2"), Payload: encodePacketPayloadForTest(t, p)})
	}
}

func floodGateHello(self types.RouterID, seen bool) *packet.Hello {
	h := &packet.Hello{HelloInterval: DefaultHelloInterval, DeadInterval: uint32(DefaultDeadInterval), Options: types.OptionE, Priority: 1}
	if seen {
		h.Neighbors = []types.RouterID{self}
	}
	return h
}

// RFC requirement: RFC2328-13-3 negative -- a Link State Update dispatched through the engine from a neighbor below Exchange (unknown, Init from a Hello that does not list us, ExStart from a two-way Hello on point-to-point) is dropped without further processing: the LSA is not in the database, and no LS Ack or LS Update goes out, even after the delayed acknowledgments are flushed (AcceptsFlooding gate in handleLSUpdate, instance.go).
func TestRFC2328LSUpdateBelowExchangeDroppedByEngine(t *testing.T) {
	cases := []struct {
		name  string
		hello bool
		seen  bool
	}{
		{name: "unknown"},
		{name: "init", hello: true},
		{name: "exstart", hello: true, seen: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eng, sends, dispatch := floodGateEngine(t)
			self := mustRouterID(t, "1.1.1.1")
			peer := mustRouterID(t, "2.2.2.2")
			header := packet.Header{RouterID: peer, AreaID: types.BackboneArea}
			if tc.hello {
				dispatch(packet.Packet{Header: header, Hello: floodGateHello(self, tc.seen)})
			}
			if reason := eng.neighbors.AcceptsFlooding("eth0", peer); reason == "" {
				t.Fatalf("setup: neighbor already accepts flooding in case %s", tc.name)
			}
			before := sends.count()
			lsa := routerLSAForTest(t, mustRouterID(t, "4.4.4.4"), types.InitialSequenceNumber, 0)
			dispatch(packet.Packet{Header: header, LSUpdate: &packet.LSUpdate{LSAs: []packet.LSA{lsa}}})
			eng.lsdb.FlushDelayedAcks("eth0")
			if _, ok := eng.lsdb.Lookup(types.BackboneArea, lsa.Header.Key()); ok {
				t.Fatalf("an LS Update from a neighbor below Exchange (%s) reached the database", tc.name)
			}
			if n := sends.count() - before; n != 0 {
				t.Fatalf("an LS Update from a neighbor below Exchange (%s) produced %d LS Update/LS Ack packets", tc.name, n)
			}
		})
	}
}

// RFC requirement: RFC2328-13-3 positive -- the same Link State Update from the same neighbor once it has reached Exchange (Hello listing us, then the master's initial Database Description) is processed: the LSA is installed and acknowledged (handleLSUpdate passes the AcceptsFlooding gate to ReceiveUpdate, instance.go).
func TestRFC2328LSUpdateFromExchangeNeighborProcessed(t *testing.T) {
	eng, sends, dispatch := floodGateEngine(t)
	self := mustRouterID(t, "1.1.1.1")
	peer := mustRouterID(t, "2.2.2.2")
	header := packet.Header{RouterID: peer, AreaID: types.BackboneArea}
	dispatch(packet.Packet{Header: header, Hello: floodGateHello(self, true)})
	dispatch(packet.Packet{Header: header, DBDesc: &packet.DBDesc{
		InterfaceMTU: 1500, Options: types.OptionE,
		Flags: packet.DDFlagInit | packet.DDFlagMore | packet.DDFlagMaster, DDSequence: 7,
	}})
	if reason := eng.neighbors.AcceptsFlooding("eth0", peer); reason != "" {
		t.Fatalf("setup: neighbor did not reach Exchange: %s", reason)
	}
	before := sends.count()
	lsa := routerLSAForTest(t, mustRouterID(t, "4.4.4.4"), types.InitialSequenceNumber, 0)
	dispatch(packet.Packet{Header: header, LSUpdate: &packet.LSUpdate{LSAs: []packet.LSA{lsa}}})
	eng.lsdb.FlushDelayedAcks("eth0")
	got, ok := eng.lsdb.Lookup(types.BackboneArea, lsa.Header.Key())
	if !ok || got.Sequence != lsa.Header.Sequence {
		t.Fatalf("the LS Update from an Exchange neighbor was not installed: %+v, %v", got, ok)
	}
	if sends.count() == before {
		t.Fatal("the LS Update from an Exchange neighbor was not acknowledged")
	}
}
