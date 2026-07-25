// VALIDATES: spec-ospf-ext-6-ti-lfa AC-9 (remote Adj-SID Q-segment) on the PRODUCTION
// path -- srTILFAResolver.AdjSIDLabel(from != self) resolves a REMOTE router's Adj-SID
// from its RFC 7684 Extended Link Opaque LSA (Opaque Type 8) / RFC 8665 §6.1 Adj-SID
// (point-to-point, matched by the Extended Link TLV's Link ID) and §6.2 LAN-Adj-SID
// (transit, matched by the sub-TLV's Neighbor ID), returning the advertising router's
// absolute local label verbatim. This is the reachable Adj-SID Q-segment: every
// reachable TI-LFA Adj-SID repair is a remote-node Adj-SID (see zz_empirical_test.go
// finding), so this is the case AC-9 actually exercises.
// PREVENTS: returning false for a remote P-node (the old gap that left the repair
// unresolved), matching the wrong link's Adj-SID (a forwarding bug), or emitting an
// index-form Adj-SID as if it were an absolute label.
package ospf

import (
	"net/netip"
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// recvExtLink floods an Extended Link Opaque LSA (Opaque Type 8) advertised by adv into
// eng's area LSDB, so srRemoteAdjSID can scan it back. adv is set up as a Full neighbor
// by extRecvInto so the flooded self-LSA is accepted.
func recvExtLink(t *testing.T, eng *engine, adv types.RouterID, opaqueID uint32, tlv packet.ExtLinkTLV) {
	t.Helper()
	extRecvInto(t, eng, adv)
	body := packet.EncodeExtLinkLSA(tlv)
	lsa := opaqueLSAForTest(t, types.LSTypeOpaqueArea, packet.ExtLinkOpaqueType, opaqueID, adv, types.InitialSequenceNumber, body)
	if reason := eng.lsdb.ReceiveUpdate(ospflsdb.ReceiveInput{
		Interface: "eth0", AreaID: mustBackboneArea(t), RouterID: adv,
		Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}},
	}); reason != "" {
		t.Fatalf("ReceiveUpdate(ext-link): %q", reason)
	}
}

func TestSRTILFAResolverRemoteAdjSIDPointToPoint(t *testing.T) {
	// AC-9: a remote P-node (adv = 3.3.3.3, != self 1.1.1.1) advertises a point-to-point
	// Extended Link LSA whose Link ID is the neighbor Router ID (4.4.4.4) and whose
	// Adj-SID sub-TLV (type 2) carries the absolute local label 24003. The resolver must
	// return that label for AdjSIDLabel(3.3.3.3, 4.4.4.4).
	eng, _ := extFnRegister(t)
	adv := mustRouterID(t, "3.3.3.3")
	nbr := mustRouterID(t, "4.4.4.4")
	adj := sr.AdjSID{Flags: sr.AdjSIDFlags{V: true, L: true}, Label: 24003, IsLabel: true}
	recvExtLink(t, eng, adv, 1, packet.ExtLinkTLV{
		LinkType: packet.RouterLinkTypeP2P,
		LinkID:   [4]byte(nbr),
		LinkData: [4]byte{10, 0, 34, 3},
		SubTLVs:  []packet.ExtSubTLV{{Type: sr.V4TypeAdjSID, Value: sr.EncodeAdjSIDValue(adj)}},
	})

	label, ok := srTILFAResolver{e: eng}.AdjSIDLabel(adv, nbr)
	if !ok {
		t.Fatalf("AdjSIDLabel(remote P-node, neighbor) did not resolve; remote Extended-Link Adj-SID decode missing")
	}
	if label != 24003 {
		t.Fatalf("resolved Adj-SID label = %d, want 24003 (advertising router's absolute local label)", label)
	}
	// A non-adjacent router must NOT resolve to this link's Adj-SID (precise Link ID match).
	if _, ok := (srTILFAResolver{e: eng}).AdjSIDLabel(adv, mustRouterID(t, "9.9.9.9")); ok {
		t.Fatalf("AdjSIDLabel resolved a neighbor the P2P Link ID does not point at (forwarding bug)")
	}
}

func TestSRTILFAResolverRemoteAdjSIDLAN(t *testing.T) {
	// AC-9 (transit/LAN): a remote P-node advertises a transit Extended Link LSA whose
	// LAN-Adj-SID sub-TLV (type 3) carries the neighbor Router ID (4.4.4.4) explicitly
	// (the transit Link ID is the DR interface, NOT the neighbor). The resolver must
	// match the LAN-Adj-SID Neighbor ID field, not the Link ID.
	eng, _ := extFnRegister(t)
	adv := mustRouterID(t, "3.3.3.3")
	nbr := mustRouterID(t, "4.4.4.4")
	adj := sr.AdjSID{Flags: sr.AdjSIDFlags{V: true, L: true}, Label: 24009, IsLabel: true, IsLAN: true, NeighborID: [4]byte(nbr)}
	recvExtLink(t, eng, adv, 2, packet.ExtLinkTLV{
		LinkType: packet.RouterLinkTypeTransit,
		LinkID:   [4]byte{10, 0, 50, 1}, // DR interface address, not the neighbor
		LinkData: [4]byte{10, 0, 50, 3},
		SubTLVs:  []packet.ExtSubTLV{{Type: sr.V4TypeLANAdjSID, Value: sr.EncodeLANAdjSIDValue(adj)}},
	})

	label, ok := srTILFAResolver{e: eng}.AdjSIDLabel(adv, nbr)
	if !ok || label != 24009 {
		t.Fatalf("LAN-Adj-SID resolve = (%d,%v), want (24009,true) matched by Neighbor ID", label, ok)
	}
}

func TestSRTILFAResolverRemoteAdjSIDIndexFormSkipped(t *testing.T) {
	// An index-form Adj-SID (V=0/L=0) is NOT an absolute label; the advertising router's
	// SRLB is not read here, so the resolver returns false rather than emit a wrong label.
	eng, _ := extFnRegister(t)
	adv := mustRouterID(t, "3.3.3.3")
	nbr := mustRouterID(t, "4.4.4.4")
	adj := sr.AdjSID{Flags: sr.AdjSIDFlags{V: false, L: false}, Index: 42} // index form
	recvExtLink(t, eng, adv, 3, packet.ExtLinkTLV{
		LinkType: packet.RouterLinkTypeP2P,
		LinkID:   [4]byte(nbr),
		SubTLVs:  []packet.ExtSubTLV{{Type: sr.V4TypeAdjSID, Value: sr.EncodeAdjSIDValue(adj)}},
	})
	if _, ok := (srTILFAResolver{e: eng}).AdjSIDLabel(adv, nbr); ok {
		t.Fatalf("index-form Adj-SID must not resolve to a label (no SRLB read here)")
	}
}

func TestSRTILFAResolverLocalAdjStillResolves(t *testing.T) {
	// The from == self path is unchanged: a locally allocated Adj-SID still resolves
	// through srAdj.adjLabelForRouter (RFC 8665 §6.1 local SRLB label).
	eng, _ := extFnRegister(t)
	self := eng.cfg.RouterID
	nbr := mustRouterID(t, "2.2.2.2")
	eng.srAdj = &srAdjManager{self: self, labels: map[srAdjKey]srAdjRecord{
		{iface: "eth0", router: nbr}: {label: 40002},
	}}
	label, ok := srTILFAResolver{e: eng}.AdjSIDLabel(self, nbr)
	if !ok || label != 40002 {
		t.Fatalf("local Adj-SID resolve = (%d,%v), want (40002,true)", label, ok)
	}
}
