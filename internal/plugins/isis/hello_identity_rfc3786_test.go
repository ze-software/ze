// Design: docs/architecture/isis/isis-5-adjacency.md -- circuit and LSP identity.
// Goal: the identity used to establish a neighbor is the identity advertising it.
// Method: use the live dispatcher, Hello transmitter, originator, and config reload.

package isis

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

func assertNormalHelloIdentity(t *testing.T, eng *engine, backend *protocolBackend, want types.SystemID) {
	t.Helper()
	c := protocolLiveCircuit(t, eng, "eth0")
	peer := types.SystemID{0, 0, 0, 0, 0, 2}
	eng.dispatch.dispatch(transport.RawFrame{IfIndex: c.IfIndex(), PDU: p2pHelloPDU(t, peer, eng.cfg.NETs[0].AreaID())})
	if err := c.SendHello(adjacency.Level1); err != nil {
		t.Fatal(err)
	}
	eng.originate()
	entry := eng.lsdb.Lookup(lsdb.Level1, types.NewLSPID(types.NewSourceID(want, 0), 0))
	if entry == nil {
		t.Fatalf("missing normal LSP for %s", want)
	}
	lsp, err := entry.Decode()
	if err != nil {
		t.Fatal(err)
	}
	defer packet.ReleaseTLVs(lsp.TLVs)
	foundNeighbor := false
	for _, tlv := range lsp.TLVs {
		if tlv.Type != packet.TLVExtendedISReach {
			continue
		}
		reach, err := packet.DecodeExtendedISReachTLV(tlv.Value)
		if err != nil {
			t.Fatal(err)
		}
		for _, neighbor := range reach.Entries {
			if neighbor.Neighbor == types.NewSourceID(peer, 0) {
				foundNeighbor = true
			}
		}
	}
	if !foundNeighbor {
		t.Fatal("normal LSP does not contain the neighbor whose Hello identity is being checked")
	}
	capture := protocolCaptureFor(t, backend, "eth0")
	capture.mu.Lock()
	defer capture.mu.Unlock()
	var sawISH, sawIIH bool
	for _, wire := range capture.sent {
		if len(wire) < 5 {
			continue
		}
		if wire[0] == packet.ESISProtocolDiscriminator {
			h, err := packet.DecodeISH(wire)
			if err != nil {
				t.Fatal(err)
			}
			packet.ReleaseTLVs(h.TLVs)
			if h.NET.SystemID() != lsp.LSPID.SystemID() || !h.NET.Equal(eng.cfg.NETs[0]) {
				t.Fatalf("ISH identity %s disagrees with containing LSP %s or current NET", h.NET, lsp.LSPID)
			}
			sawISH = true
			continue
		}
		if packet.PDUType(wire[4]&0x1f) != packet.PDUTypeP2PHello {
			continue
		}
		p, err := packet.DecodePDU(wire)
		if err != nil {
			t.Fatal(err)
		}
		packet.ReleaseTLVs(p.P2PHello.TLVs)
		if p.P2PHello.SystemID != lsp.LSPID.SystemID() {
			t.Fatalf("IIH identity %s disagrees with containing LSP %s", p.P2PHello.SystemID, lsp.LSPID)
		}
		sawIIH = true
	}
	if !sawISH || !sawIIH {
		t.Fatalf("missing adjacency-establishment output: ISH=%v IIH=%v", sawISH, sawIIH)
	}
}

// RFC requirement: RFC3786-6-1 positive -- actual outgoing IS and IS-IS Hellos
// use the normal System ID of the production LSP containing their neighbor.
func TestRFC3786NormalHelloMatchesContainingLSP(t *testing.T) {
	eng, backend := protocolEngine(t, protocolP2PConfig)
	assertNormalHelloIdentity(t, eng, backend, types.SystemID{0, 0, 0, 0, 0, 1})
}

// RFC requirement: RFC3786-6-1 negative -- reloading the node's System ID or NET
// cannot leave a retained circuit sending its old identity beside the newly
// originated normal LSP. Both the IIH System ID and ISH NET follow the reload.
func TestRFC3786NormalHelloIdentityReload(t *testing.T) {
	eng, backend := protocolEngine(t, protocolP2PConfig)
	assertNormalHelloIdentity(t, eng, backend, types.SystemID{0, 0, 0, 0, 0, 1})
	for _, net := range []string{"49.0001.0000.0000.0009.00", "49.0002.0000.0000.0009.00"} {
		reconcileTo(t, eng, `{"isis":{"net":"`+net+`","interfaces":{"interface":{
"eth0":{"level":"l1","hello-interval":"3600","circuit-type":"point-to-point"}}}}}`)
		assertNormalHelloIdentity(t, eng, backend, types.SystemID{0, 0, 0, 0, 0, 9})
	}
}
