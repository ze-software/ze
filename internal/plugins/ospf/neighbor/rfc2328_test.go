// VALIDATES: RFC 2328 Section 13 / Appendix A.3.3 obligations on the neighbor table that had
// no direct coverage: flooding packets are only accepted from a neighbor in Exchange or higher,
// and a Database Description packet carries the interface's MTU verbatim -- zero for the
// synthetic virtual-link interface, which is configured without an MTU.
// PREVENTS: an LS Update from a half-formed adjacency reaching the LSDB, and a virtual link
// failing the peer's MTU match because a real MTU was stamped into its DD.
package neighbor

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC2328-13-3 positive -- a neighbor that has reached Exchange (and above) is
// accepted as a source of LS Update / LS Ack packets; AcceptsFlooding returns the empty reason
// (AcceptsFlooding, table.go:405-418).
// RFC requirement: RFC2328-13-3 negative -- a neighbor in a state LESSER than Exchange (Init /
// 2-Way), and an unknown neighbor, are refused: AcceptsFlooding returns a non-empty reason, which
// the engine turns into a drop before the LSDB sees the packet (AcceptsFlooding state gate,
// table.go:415-417).
func TestRFC2328FloodingRequiresExchangeOrHigher(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	sender := &fakeSender{}
	tbl.SetSender(sender)
	tbl.SetLSDB(fakeLSDB{})
	peer := rid(t, "10.0.0.2")

	// One-way Hello: the neighbor reaches Init, which is below Exchange.
	_ = tbl.Hello(hello(cfg, peer, false, time.Unix(1, 0)))
	if reason := tbl.AcceptsFlooding(cfg.Name, peer); reason != reasonState {
		t.Fatalf("Init neighbor AcceptsFlooding = %q, want %q", reason, reasonState)
	}

	// Two-way Hello: still below Exchange on the way to the adjacency.
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(2, 0)))
	if snap, _ := tbl.Lookup(cfg.Name, peer); snap.State == "exchange" || snap.State == "full" {
		t.Fatalf("precondition: neighbor reached %s before any DD was exchanged", snap.State)
	}
	if reason := tbl.AcceptsFlooding(cfg.Name, peer); reason != reasonState {
		t.Fatalf("pre-Exchange neighbor AcceptsFlooding = %q, want %q", reason, reasonState)
	}

	// An unknown neighbor on a known interface is refused too.
	if reason := tbl.AcceptsFlooding(cfg.Name, rid(t, "10.0.0.9")); reason != reasonNeighbor {
		t.Fatalf("unknown neighbor AcceptsFlooding = %q, want %q", reason, reasonNeighbor)
	}

	// Drive the adjacency past Exchange: flooding is now accepted.
	driveNegotiation(t, tbl, cfg, peer)
	if reason := tbl.AcceptsFlooding(cfg.Name, peer); reason != "" {
		t.Fatalf("Exchange-or-higher neighbor AcceptsFlooding = %q, want accepted", reason)
	}
}

// RFC requirement: RFC2328-A.3.3-1 positive -- the synthetic virtual-link interface is configured
// with no Interface MTU (startVirtualInterface leaves ospfiface.Config.InterfaceMTU zero,
// virtual_link.go:229-244), and the DD encoder stamps the interface MTU verbatim, so every
// Database Description sent over a virtual link carries Interface MTU 0
// (sendDBDescLocked, dd.go:170).
// RFC requirement: RFC2328-A.3.3-1 negative -- the zero is not a blanket: a real interface's DD
// carries that interface's actual MTU (1500 here), so only the MTU-less virtual link emits 0
// (sendDBDescLocked, dd.go:170).
func TestRFC2328VirtualLinkDBDescCarriesZeroMTU(t *testing.T) {
	// A virtual link: no Interface MTU configured, and the MTU match is skipped.
	tbl, cfg := testTable(t, NetworkPointToPoint)
	cfg.Name = "*vl-0.0.0.1-10.0.0.2"
	cfg.InterfaceMTU = 0
	cfg.MTUIgnore = true
	tbl.ConfigureInterface(cfg)
	sender := &fakeSender{}
	tbl.SetSender(sender)
	peer := rid(t, "10.0.0.2")

	in := hello(cfg, peer, true, time.Unix(1, 0))
	in.InterfaceMTU = 0
	in.MTUIgnore = true
	_ = tbl.Hello(in)
	if reason := tbl.HandleDBDesc(cfg.Name, peer, peerExStartDD()); reason != "" {
		t.Fatalf("virtual-link ExStart DD: %s", reason)
	}
	if len(sender.sent) == 0 {
		t.Fatal("no Database Description was sent over the virtual link")
	}
	for i := range sender.sent {
		p := sentPacket(t, sender, i)
		if p.DBDesc == nil {
			continue
		}
		if p.DBDesc.InterfaceMTU != 0 {
			t.Fatalf("virtual-link DD %d Interface MTU = %d, want 0", i, p.DBDesc.InterfaceMTU)
		}
	}

	// A real interface keeps its MTU in the DD.
	realTbl, realCfg := testTable(t, NetworkPointToPoint)
	realSender := &fakeSender{}
	realTbl.SetSender(realSender)
	_ = realTbl.Hello(hello(realCfg, peer, true, time.Unix(1, 0)))
	if reason := realTbl.HandleDBDesc(realCfg.Name, peer, peerExStartDD()); reason != "" {
		t.Fatalf("real-interface ExStart DD: %s", reason)
	}
	sawReal := false
	for i := range realSender.sent {
		p := sentPacket(t, realSender, i)
		if p.DBDesc == nil {
			continue
		}
		sawReal = true
		if p.DBDesc.InterfaceMTU != 1500 {
			t.Fatalf("real-interface DD %d Interface MTU = %d, want 1500", i, p.DBDesc.InterfaceMTU)
		}
	}
	if !sawReal {
		t.Fatal("no Database Description was sent over the real interface")
	}
}

// RFC requirement: RFC2328-10.2-1 negative -- the BadLSReq restart is confined to a request the
// database cannot satisfy: an LS Request naming an LSA that IS in the database is answered with
// an LS Update and leaves the adjacency in place (handleLSReq, lsreq.go:75-82).
func TestRFC2328KnownLSRequestDoesNotRestartExchange(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	sender := &fakeSender{}
	tbl.SetSender(sender)
	held := packet.LSA{Header: testHeader(t, types.InitialSequenceNumber)}
	tbl.SetLSDB(fakeLSDB{held.Header.Key(): held})
	peer := rid(t, "10.0.0.2")
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	driveNegotiation(t, tbl, cfg, peer)
	before, _ := tbl.Lookup(cfg.Name, peer)

	req := packet.LSReq{Requests: []packet.LSRequestEntry{{
		Type:              held.Header.Type,
		LinkStateID:       held.Header.LinkStateID,
		AdvertisingRouter: held.Header.AdvertisingRouter,
	}}}
	if reason := tbl.HandleLSReq(cfg.Name, peer, req); reason != "" {
		t.Fatalf("LS Request for a held LSA = %q, want accepted", reason)
	}

	after, _ := tbl.Lookup(cfg.Name, peer)
	if after.State != before.State {
		t.Fatalf("a satisfiable LS Request changed the neighbor state %s -> %s", before.State, after.State)
	}
	if after.State == "exstart" {
		t.Fatalf("a satisfiable LS Request must not drive the adjacency back to ExStart")
	}
}
