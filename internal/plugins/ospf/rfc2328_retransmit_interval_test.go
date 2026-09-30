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

// RFC requirement: RFC2328-13.3-1 positive -- an LSA flooded out two adjacencies whose
// interfaces are configured with retransmit-interval 3 and 11 is retransmitted on each
// adjacency every configured RxmtInterval, repeatedly while unacknowledged: eth1 at 3, 6, 9
// and 12 s, eth2 at 11 and 22 s (flood via ReceiveUpdate, RxmtInterval read from the parsed
// config through engine.lsdbTopology, resend by RetransmitTick).
// RFC requirement: RFC2328-13.3-1 negative -- no adjacency is retransmitted to before its own
// configured RxmtInterval has elapsed, and once eth1's neighbor acknowledges the LSA at 12 s
// eth1 sees no further retransmission while the unacknowledged eth2 still does.
//
// Goal: prove "retransmitted until they are acknowledged" and "a configurable per-interface
// value, RxmtInterval" on the real flood and retransmit path, with values that differ from
// the 5 s default and from each other, so neither a single retransmission nor a hard-coded
// interval passes.
// Method: parse an OSPFv2 config with three point-to-point interfaces, take the engine's
// LSDB topology (which carries each interface's configured RxmtInterval), mark one Full
// neighbor per interface, receive an LSA on eth0 so it floods out eth1 and eth2, then tick
// the LSDB once per second for 24 s on a fake clock and record which interface each LS
// Update left on.
func TestRFC2328RetransmitEveryConfiguredRxmtInterval(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"1.1.1.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{`+
		`"eth0":{"name":"eth0","area":"0","network-type":"point-to-point"},`+
		`"eth1":{"name":"eth1","area":"0","network-type":"point-to-point","retransmit-interval":"3"},`+
		`"eth2":{"name":"eth2","area":"0","network-type":"point-to-point","retransmit-interval":"11"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	eng := newEngine(transport.New(&fakeBackend{}))
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	defer eng.shutdown()

	peers := map[string]string{"eth0": "2.2.2.2", "eth1": "3.3.3.3", "eth2": "4.4.4.4"}
	topology := eng.lsdbTopology()
	if len(topology) != 3 {
		t.Fatalf("engine topology holds %d interfaces, want 3", len(topology))
	}
	for idx := range topology {
		info := &topology[idx]
		info.Neighbors = []ospflsdb.NeighborInfo{{RouterID: mustRouterID(t, peers[info.Name]), Address: naddrForTest("10.0.0.2"), State: ospflsdb.NeighborStateFull}}
	}

	now := time.Unix(0, 0)
	db := ospflsdb.New(func() time.Time { return now })
	db.SetSelfRouterID(mustRouterID(t, "1.1.1.1"))
	db.SetTopology(func() []ospflsdb.InterfaceInfo { return topology })
	var sent []string
	db.SetTx(func(iface string, _ netip.Addr, payload []byte) error {
		p, err := packet.DecodePacket(payload)
		if err != nil {
			return err
		}
		if p.LSUpdate != nil {
			sent = append(sent, iface)
		}
		return nil
	})

	lsa := routerLSAForTest(t, mustRouterID(t, "9.9.9.9"), types.InitialSequenceNumber, 0)
	reason := db.ReceiveUpdate(ospflsdb.ReceiveInput{Interface: "eth0", AreaID: mustBackboneArea(t), RouterID: mustRouterID(t, "2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}})
	if reason != "" {
		t.Fatalf("ReceiveUpdate reason = %q", reason)
	}
	if len(sent) != 2 {
		t.Fatalf("initial flood left on %v, want eth1 and eth2", sent)
	}

	// retransmits[s] lists the interfaces RetransmitTick sent on at second s.
	retransmits := make(map[int][]string)
	for second := 1; second <= 24; second++ {
		now = time.Unix(int64(second), 0)
		sent = sent[:0]
		db.RetransmitTick(now)
		retransmits[second] = append([]string(nil), sent...)
		if second == 12 {
			db.ReceiveAck(ospflsdb.AckInput{Interface: "eth1", AreaID: mustBackboneArea(t), RouterID: mustRouterID(t, "3.3.3.3"), Ack: packet.LSAck{Headers: []packet.LSAHeader{lsa.Header}}})
		}
	}

	want := map[int][]string{3: {"eth1"}, 6: {"eth1"}, 9: {"eth1"}, 11: {"eth2"}, 12: {"eth1"}, 22: {"eth2"}}
	for second := 1; second <= 24; second++ {
		got, expected := retransmits[second], want[second]
		if len(got) != len(expected) || (len(got) == 1 && got[0] != expected[0]) {
			t.Errorf("second %d: retransmitted on %v, want %v", second, got, expected)
		}
	}
}
