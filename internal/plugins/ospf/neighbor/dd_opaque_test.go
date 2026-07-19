// VALIDATES: spec-ospf-ext-1 AC-6/A-6 -- when opaque capability is enabled the outgoing
// Database Description packet sets the RFC 5250 O-bit, a neighbor whose DD carried the
// O-bit is reported OpaqueCapable to flooding, and the O-bit is a DD-only signal that is
// ignored in Hello adjacency matching (setting it does not block a legacy peer).
// PREVENTS: opaque flooding to a peer that never advertised the O-bit, and an adjacency
// regression where the O-bit is treated as part of the Hello E/N option match.
package neighbor

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

func TestDDSetsOpaqueBit(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	// Opaque enabled: the interface Options carry the O-bit (the engine ORs it in when
	// the `opaque` config leaf is set).
	cfg.Options = types.OptionE | types.OptionO
	tbl.ConfigureInterface(cfg)
	sender := &fakeSender{}
	tbl.SetSender(sender)

	peer := rid(t, "10.0.0.2")
	now := tbl.now()
	tbl.Hello(hello(cfg, peer, false, now))
	tbl.Hello(hello(cfg, peer, true, now)) // 2-way -> ExStart, sends the initial DD

	if len(sender.sent) == 0 {
		t.Fatalf("no DD packet sent on reaching ExStart")
	}
	p := sentPacket(t, sender, len(sender.sent)-1)
	if p.DBDesc == nil {
		t.Fatalf("last sent packet is not a DBDesc")
	}
	if !p.DBDesc.Options.Has(types.OptionO) {
		t.Fatalf("outgoing DD Options %v missing the O-bit", p.DBDesc.Options)
	}
}

func TestOpaqueBitIgnoredOutsideDD(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	cfg.Options = types.OptionE | types.OptionO
	tbl.ConfigureInterface(cfg)
	sender := &fakeSender{}
	tbl.SetSender(sender)
	peer := rid(t, "10.0.0.2")
	now := tbl.now()

	// A Hello carries no O-bit relevance: the adjacency forms and reaches ExStart even
	// though the peer's Hello never mentions the O-bit (it is a DD-only signal, §3.1).
	tbl.Hello(hello(cfg, peer, false, now))
	tbl.Hello(hello(cfg, peer, true, now))
	snap, ok := tbl.Lookup(cfg.Name, peer)
	if !ok {
		t.Fatalf("neighbor not created")
	}
	if snap.State == stateNameDown {
		t.Fatalf("adjacency did not progress with opaque enabled")
	}
	// Before any DD is exchanged the neighbor is not yet opaque-capable.
	// RFC requirement: RFC5250-3.1-5 negative -- the O-bit is a DD-only signal; a Hello exchange never confers opaque capability
	if snap.OpaqueCapable {
		t.Fatalf("neighbor reported opaque-capable before receiving an O-bit DD")
	}

	// The peer's DD sets the O-bit -> the neighbor becomes opaque-capable for flooding.
	dd := packet.DBDesc{InterfaceMTU: 1500, Options: types.OptionE | types.OptionO, Flags: packet.DDFlagInit | packet.DDFlagMore | packet.DDFlagMaster, DDSequence: 7}
	if reason := tbl.HandleDBDesc(cfg.Name, peer, dd); reason != "" {
		t.Fatalf("HandleDBDesc: %s", reason)
	}
	fns := tbl.FloodNeighbors(cfg.Name)
	// RFC requirement: RFC5250-3.1-5 positive -- opaque capability is learned from the DD Options O-bit
	if len(fns) != 1 || !fns[0].OpaqueCapable {
		t.Fatalf("neighbor not reported opaque-capable after an O-bit DD: %+v", fns)
	}
}
