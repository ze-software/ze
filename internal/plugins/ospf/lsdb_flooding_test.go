package ospf

import (
	"net/netip"
	"testing"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestOSPFFloodingFunctional(t *testing.T) {
	now := time.Unix(0, 0)
	db := ospflsdb.New(func() time.Time { return now })
	self := mustRouterID(t, "1.1.1.1")
	db.SetSelfRouterID(self)
	db.SetTopology(func() []ospflsdb.InterfaceInfo {
		return []ospflsdb.InterfaceInfo{
			{Name: "eth0", AreaID: mustBackboneArea(t), AreaType: ospflsdb.AreaTypeNormal, NetworkType: ospflsdb.NetworkBroadcast, State: ospflsdb.InterfaceStateBackup, Address: ip4ForTest("10.0.0.1"), DR: mustRouterID(t, "2.2.2.2"), BDR: self, TransmitDelay: 1, Neighbors: []ospflsdb.NeighborInfo{{RouterID: mustRouterID(t, "2.2.2.2"), Address: naddrForTest("10.0.0.2"), State: ospflsdb.NeighborStateFull}}},
			{Name: "eth1", AreaID: mustBackboneArea(t), AreaType: ospflsdb.AreaTypeNormal, NetworkType: ospflsdb.NetworkBroadcast, State: ospflsdb.InterfaceStateDR, Address: ip4ForTest("10.0.1.1"), DR: self, TransmitDelay: 1, Neighbors: []ospflsdb.NeighborInfo{{RouterID: mustRouterID(t, "3.3.3.3"), Address: naddrForTest("10.0.1.3"), State: ospflsdb.NeighborStateFull}}},
		}
	})
	var sends []struct {
		iface string
		dst   netip.Addr
		pkt   packet.Packet
	}
	db.SetTx(func(iface string, dst netip.Addr, payload []byte) error {
		p, err := packet.DecodePacket(payload)
		if err != nil {
			return err
		}
		sends = append(sends, struct {
			iface string
			dst   netip.Addr
			pkt   packet.Packet
		}{iface: iface, dst: dst, pkt: p})
		return nil
	})
	lsa := routerLSAForTest(t, mustRouterID(t, "4.4.4.4"), types.InitialSequenceNumber, 0)
	reason := db.ReceiveUpdate(ospflsdb.ReceiveInput{Interface: "eth0", AreaID: mustBackboneArea(t), RouterID: mustRouterID(t, "2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}})
	if reason != "" {
		t.Fatalf("ReceiveUpdate reason = %q", reason)
	}
	if len(sends) != 1 || sends[0].iface != "eth1" || sends[0].dst != transport.AllSPFRouters {
		t.Fatalf("flood sends = %+v", sends)
	}
	if len(db.Snapshot().Areas) != 1 {
		t.Fatalf("LSA not installed: %+v", db.Snapshot())
	}
	now = now.Add(time.Duration(types.MaxAge) * time.Second)
	if tick := db.Tick(now); tick.Purged != 1 {
		t.Fatalf("purged = %d", tick.Purged)
	}
	if len(db.Snapshot().Areas[0].LSAs) != 1 || db.Snapshot().Areas[0].LSAs[0].Age != types.MaxAge {
		t.Fatalf("purge not retained: %+v", db.Snapshot())
	}
}

func TestOSPFLSUpdateBelowExchangeDoesNotReachLSDB(t *testing.T) {
	fb := &fakeBackend{}
	tr := transport.New(fb)
	e := newEngine(tr)
	defer e.shutdown()
	a := mustBackboneArea(t)
	self := mustRouterID(t, "1.1.1.1")
	cfg := defaultOSPFConfig()
	cfg.present = true
	cfg.RouterID = self
	cfg.Areas = []areaConfig{{AreaID: a, AreaType: areaTypeNormal}}
	cfg.Interfaces = []interfaceConfig{{
		Name:               "eth0",
		AreaID:             a,
		Enabled:            true,
		NetworkType:        networkBroadcast,
		HelloInterval:      DefaultHelloInterval,
		DeadInterval:       DefaultDeadInterval,
		Priority:           DefaultPriority,
		RetransmitInterval: DefaultRetransmitInterval,
		TransmitDelay:      DefaultTransmitDelay,
	}}
	e.setConfig(cfg)
	if err := e.openConfiguredInterface(cfg.Interfaces[0]); err != nil {
		t.Fatalf("openConfiguredInterface: %v", err)
	}
	fb.mu.Lock()
	handle := fb.handles["eth0"]
	fb.mu.Unlock()
	if handle == nil {
		t.Fatalf("eth0 handle missing")
	}
	peer := mustRouterID(t, "2.2.2.2")
	lsa := routerLSAForTest(t, mustRouterID(t, "4.4.4.4"), types.InitialSequenceNumber, 0)
	payload := encodePacketPayloadForTest(t, packet.Packet{Header: packet.Header{RouterID: peer, AreaID: a}, LSUpdate: &packet.LSUpdate{LSAs: []packet.LSA{lsa}}})
	e.handleLSUpdate(transport.RawPacket{IfIndex: handle.ifindex, Src: netip.MustParseAddr("10.0.0.2"), Payload: payload}, Header{RouterID: peer, AreaID: a})
	if _, ok := e.lsdb.Lookup(a, lsa.Header.Key()); ok {
		t.Fatalf("LS Update from non-Exchange neighbor reached LSDB")
	}
}

func encodePacketPayloadForTest(t *testing.T, p packet.Packet) []byte {
	t.Helper()
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)
	return buf
}

func routerLSAForTest(t *testing.T, adv types.RouterID, seq types.LSSequenceNumber, age types.LSAge) packet.LSA {
	t.Helper()
	body := packet.RouterLSA{Links: []packet.RouterLink{{LinkID: mustLinkStateID(t, "10.0.0.0"), LinkData: ip4ForTest("255.255.255.0"), Type: packet.RouterLinkTypeStub, Metric: 10}}}
	lsa := packet.LSA{Header: packet.LSAHeader{Age: age, Options: types.OptionE, Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(adv), AdvertisingRouter: adv, Sequence: seq}, Router: &body}
	buf := make([]byte, lsa.EncodedLen())
	lsa.WriteTo(buf, 0)
	out, err := packet.DecodeLSA(buf)
	if err != nil {
		t.Fatalf("DecodeLSA: %v", err)
	}
	return out
}

func mustRouterID(t *testing.T, s string) types.RouterID {
	t.Helper()
	id, err := types.ParseRouterID(s)
	if err != nil {
		t.Fatalf("ParseRouterID(%q): %v", s, err)
	}
	return id
}

func mustBackboneArea(t *testing.T) types.AreaID {
	t.Helper()
	id, err := types.ParseAreaID("0.0.0.0")
	if err != nil {
		t.Fatalf("ParseAreaID(0.0.0.0): %v", err)
	}
	return id
}

func mustLinkStateID(t *testing.T, s string) types.LinkStateID {
	t.Helper()
	id, err := types.ParseLinkStateID(s)
	if err != nil {
		t.Fatalf("ParseLinkStateID(%q): %v", s, err)
	}
	return id
}

func ip4ForTest(s string) [4]byte { return netip.MustParseAddr(s).As4() }

// naddrForTest is the netip.Addr form for NeighborInfo.Address (reachable address).
func naddrForTest(s string) netip.Addr { return netip.MustParseAddr(s) }
