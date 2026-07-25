// VALIDATES: spec-ospf-ext-2 Traffic Engineering Database -- a received TE LSA (RFC 3630
// type 1 or RFC 5392 type 6) is parsed into a link-keyed TED with its router address;
// a withdraw/MaxAge removes the entry; the RFC 5250 sec 5 reachability gate marks a
// Type-11 inter-AS entry usable only when its originator is reachable while Type-10 is
// always usable; the TED is bounded; the Snapshot is value-typed; and reception does not
// perturb SPF/the route table.
// PREVENTS: a stored-but-never-parsed TE LSA, a stale entry after withdraw, the gate
// applied to Type-10, an unbounded TED, or reception touching route computation.
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

const teCfgJSON = `{"ospf":{"router-id":"1.1.1.1","opaque":true,"areas":{"area":{"0":{"area-id":"0"}}}}}`

func teLinkBody(t *testing.T) []byte {
	t.Helper()
	return packet.TELSA{IsLink: true, Link: packet.TELink{
		HasLinkType: true, LinkType: packet.TELinkTypePointToPoint,
		HasLinkID: true, LinkID: [4]byte{2, 2, 2, 2},
		LocalIPs: [][4]byte{{10, 0, 0, 1}}, RemoteIPs: [][4]byte{{10, 0, 0, 2}},
		HasTEMetric: true, TEMetric: 42,
		HasMaxBandwidth: true, MaxBandwidth: 1.25e9,
	}}.Encode()
}

func teReceived(scope OpaqueScope, opaqueType uint8, id uint32, adv types.RouterID, body []byte, reachable, withdrawn bool) opaqueReceived {
	return opaqueReceived{
		OpaqueType: opaqueType, OpaqueID: id, Scope: scope, Area: types.BackboneArea,
		AdvertisingRouter: adv, Body: body, Reachable: reachable, Withdrawn: withdrawn,
	}
}

func TestTEReceiveIntoTED(t *testing.T) {
	eng, _ := newRedistEngine(t, teCfgJSON)
	adv := mustRouterID(t, "2.2.2.2")
	// A Router-Address TLV upserts the originator's TE Router Address (Instance 0).
	ra := packet.TELSA{IsRouterAddress: true, RouterAddress: [4]byte{9, 9, 9, 9}}.Encode()
	eng.teOnReceive(teReceived(OpaqueScopeArea, packet.TEOpaqueType, 0, adv, ra, true, false))
	// A Link TLV upserts a TED link entry keyed by the link.
	eng.teOnReceive(teReceived(OpaqueScopeArea, packet.TEOpaqueType, 1, adv, teLinkBody(t), true, false))

	snap := eng.ted.Snapshot()
	if len(snap.RouterAddresses) != 1 || snap.RouterAddresses[0].Address != [4]byte{9, 9, 9, 9} {
		t.Fatalf("router addresses = %+v", snap.RouterAddresses)
	}
	if len(snap.Links) != 1 {
		t.Fatalf("links = %d, want 1", len(snap.Links))
	}
	l := snap.Links[0]
	if l.AdvertisingRouter != adv || !l.Link.HasLinkID || l.Link.LinkID != [4]byte{2, 2, 2, 2} {
		t.Fatalf("link entry = %+v", l)
	}
	if l.Link.TEMetric != 42 || l.Link.MaxBandwidth != float64(float32(1.25e9)) {
		t.Fatalf("attributes lost: metric=%d bw=%g", l.Link.TEMetric, l.Link.MaxBandwidth)
	}
	if !l.Usable {
		t.Fatalf("Type-10 link must be usable")
	}
	// The rsvpte-facing lookup finds the link by (adv, link-id, local-addr).
	got, ok := eng.ted.LookupLink(adv, [4]byte{2, 2, 2, 2}, [4]byte{10, 0, 0, 1})
	if !ok || got.Link.TEMetric != 42 {
		t.Fatalf("LookupLink = %+v ok=%v", got, ok)
	}
}

func TestInterAsTEReceiveIntoTED(t *testing.T) {
	eng, _ := newRedistEngine(t, teCfgJSON)
	adv := mustRouterID(t, "3.3.3.3")
	body := packet.TELSA{IsLink: true, Link: packet.TELink{
		HasLinkType: true, LinkType: packet.TELinkTypePointToPoint,
		HasRemoteAS: true, RemoteAS: 65001,
		HasRemoteASBRv4: true, RemoteASBRv4: [4]byte{203, 0, 113, 9},
	}}.Encode()
	eng.teOnReceive(teReceived(OpaqueScopeArea, packet.InterAsTEOpaqueType, 1, adv, body, true, false))
	snap := eng.ted.Snapshot()
	if len(snap.Links) != 1 {
		t.Fatalf("inter-as links = %d, want 1", len(snap.Links))
	}
	l := snap.Links[0]
	if !l.Link.IsInterAS() || l.Link.RemoteAS != 65001 || l.Link.RemoteASBRv4 != [4]byte{203, 0, 113, 9} {
		t.Fatalf("inter-as entry = %+v", l.Link)
	}
	if l.Link.HasLinkID {
		t.Fatalf("inter-as Link TLV must not carry a Link ID (RFC 5392 sec 3.2.1)")
	}
}

func TestTEWithdrawRemovesTEDEntry(t *testing.T) {
	eng, _ := newRedistEngine(t, teCfgJSON)
	adv := mustRouterID(t, "2.2.2.2")
	eng.teOnReceive(teReceived(OpaqueScopeArea, packet.TEOpaqueType, 1, adv, teLinkBody(t), true, false))
	if len(eng.ted.Snapshot().Links) != 1 {
		t.Fatalf("link not installed")
	}
	// A MaxAge/withdraw delivery removes the corresponding TED entry (RFC 2328 sec 14).
	eng.teOnReceive(teReceived(OpaqueScopeArea, packet.TEOpaqueType, 1, adv, teLinkBody(t), true, true))
	if len(eng.ted.Snapshot().Links) != 0 {
		t.Fatalf("withdraw left a stale TED entry: %+v", eng.ted.Snapshot().Links)
	}
}

func TestTEType10AlwaysUsable(t *testing.T) {
	eng, _ := newRedistEngine(t, teCfgJSON)
	// Reachability reports the originator unreachable, but a Type-10 (area) entry is always
	// usable: the RFC 5250 sec 5 gate is Type-11-only.
	eng.ted.setReachable(func(types.RouterID) bool { return false })
	adv := mustRouterID(t, "2.2.2.2")
	eng.teOnReceive(teReceived(OpaqueScopeArea, packet.TEOpaqueType, 1, adv, teLinkBody(t), true, false))
	if !eng.ted.Snapshot().Links[0].Usable {
		t.Fatalf("Type-10 entry must be usable regardless of originator reachability")
	}
}

func TestInterAsTEType11UnreachableUnusable(t *testing.T) {
	eng, _ := newRedistEngine(t, teCfgJSON)
	adv := mustRouterID(t, "3.3.3.3")
	reachable := false
	eng.ted.setReachable(func(types.RouterID) bool { return reachable })
	body := packet.TELSA{IsLink: true, Link: packet.TELink{HasLinkType: true, LinkType: packet.TELinkTypePointToPoint, HasRemoteAS: true, RemoteAS: 65001, HasRemoteASBRv4: true, RemoteASBRv4: [4]byte{203, 0, 113, 9}}}.Encode()
	// RFC 5250 sec 5: a Type-11 LSA whose originator is unreachable is present but unusable.
	eng.teOnReceive(teReceived(OpaqueScopeAS, packet.InterAsTEOpaqueType, 1, adv, body, false, false))
	if eng.ted.Snapshot().Links[0].Usable {
		t.Fatalf("Type-11 entry from an unreachable originator must be unusable")
	}
	if eng.ted.unreachableCount() != 1 {
		t.Fatalf("unreachable-originator count = %d, want 1", eng.ted.unreachableCount())
	}
	// When the originator becomes reachable, the entry is marked usable on the next read.
	reachable = true
	if !eng.ted.Snapshot().Links[0].Usable {
		t.Fatalf("Type-11 entry must become usable when its originator is reachable")
	}
}

func TestTEDBoundedByOpaqueStore(t *testing.T) {
	eng, _ := newRedistEngine(t, teCfgJSON)
	eng.ted.setMax(4)
	adv := mustRouterID(t, "2.2.2.2")
	for i := uint32(1); i <= 20; i++ {
		eng.teOnReceive(teReceived(OpaqueScopeArea, packet.TEOpaqueType, i, adv, teLinkBody(t), true, false))
	}
	if n := len(eng.ted.Snapshot().Links); n > 4 {
		t.Fatalf("TED grew to %d links, must be bounded at 4", n)
	}
}

func TestTEDSnapshotReadOnly(t *testing.T) {
	eng, _ := newRedistEngine(t, teCfgJSON)
	adv := mustRouterID(t, "2.2.2.2")
	eng.teOnReceive(teReceived(OpaqueScopeArea, packet.TEOpaqueType, 1, adv, teLinkBody(t), true, false))
	snap := eng.ted.Snapshot()
	// Mutating the returned value-typed snapshot must not affect the TED (no shared pointers).
	if len(snap.Links) == 1 {
		snap.Links[0].Link.TEMetric = 999
		snap.Links[0].Link.LocalIPs[0] = [4]byte{4, 4, 4, 4}
	}
	again := eng.ted.Snapshot()
	if again.Links[0].Link.TEMetric != 42 || again.Links[0].Link.LocalIPs[0] != [4]byte{10, 0, 0, 1} {
		t.Fatalf("snapshot is not a value copy: mutation leaked into the TED")
	}
}

func TestTEReceptionDoesNotTriggerSPF(t *testing.T) {
	eng, _ := newRedistEngine(t, teCfgJSON)
	before := len(eng.routeSnapshot())
	adv := mustRouterID(t, "2.2.2.2")
	eng.teOnReceive(teReceived(OpaqueScopeArea, packet.TEOpaqueType, 1, adv, teLinkBody(t), true, false))
	if after := len(eng.routeSnapshot()); after != before {
		t.Fatalf("TE reception changed the route table: %d -> %d", before, after)
	}
}
