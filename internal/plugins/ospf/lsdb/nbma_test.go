// VALIDATES: IPv4 point-to-multipoint origination emits a per-neighbor Type-1 link
// plus a /32 host-route stub (never a subnet stub) and no Network-LSA; NBMA behaves
// like broadcast (transit link + Network-LSA when DR).
// PREVENTS: a PtMP interface leaking a subnet stub or a Network-LSA; NBMA failing to
// originate its DR Network-LSA.
package lsdb

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func ptmpInterface() InterfaceInfo {
	return InterfaceInfo{
		Name: "ptp0", AreaID: area("0.0.0.0"), AreaType: AreaTypeNormal,
		NetworkType: NetworkPointToMultipoint, State: "point-to-point",
		Address: ip4("10.0.0.1"), NetworkMask: ip4("255.255.255.0"), RouterID: rid("1.1.1.1"), Cost: 10,
		Neighbors: []NeighborInfo{{RouterID: rid("2.2.2.2"), Address: naddr4("10.0.0.2"), State: NeighborStateFull}},
	}
}

func TestOSPFPtMPHostRoute(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	in := OriginInput{AreaID: area("0.0.0.0"), RouterID: rid("1.1.1.1"), Options: types.OptionE, Interfaces: []InterfaceInfo{ptmpInterface()}}
	h, ok := db.OriginateRouter(in)
	if !ok {
		t.Fatalf("OriginateRouter returned false")
	}
	lsa, ok := db.LookupLSA(in.AreaID, h.Key())
	if !ok {
		t.Fatalf("Router-LSA not installed")
	}
	body, err := lsa.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	var haveP2P, haveHostRoute bool
	for _, l := range body.Links {
		switch l.Type {
		case packet.RouterLinkTypeP2P:
			if l.LinkID == types.LinkStateID(rid("2.2.2.2")) && l.LinkData == ip4("10.0.0.1") {
				haveP2P = true
			}
		case packet.RouterLinkTypeStub:
			if l.LinkID == types.LinkStateID(ip4("10.0.0.1")) {
				if l.LinkData != ip4("255.255.255.255") || l.Metric != 0 {
					t.Fatalf("host-route stub = %+v, want mask 255.255.255.255 metric 0", l)
				}
				haveHostRoute = true
			}
			// The subnet network address must NOT appear as a stub link.
			if l.LinkID == types.LinkStateID(ip4("10.0.0.0")) {
				t.Fatalf("PtMP emitted a subnet stub link %+v; want only the host route", l)
			}
		}
	}
	if !haveP2P {
		t.Fatalf("PtMP missing the per-neighbor Type-1 link; links=%+v", body.Links)
	}
	if !haveHostRoute {
		t.Fatalf("PtMP missing the /32 host-route stub; links=%+v", body.Links)
	}
}

func TestOSPFPtMPNoNetworkLSA(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(func() []InterfaceInfo { return []InterfaceInfo{ptmpInterface()} })
	db.OriginateFromTopology(rid("1.1.1.1"), false)
	for _, hdr := range db.Summary(area("0.0.0.0")) {
		if hdr.Type == types.LSTypeNetwork {
			t.Fatalf("PtMP originated a Network-LSA %+v; want none", hdr)
		}
	}
}

func TestOSPFNBMANetworkLSA(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	// An NBMA interface where this router is the DR: originate the Type-2 Network-LSA.
	nbma := InterfaceInfo{
		Name: "nb0", AreaID: area("0.0.0.0"), AreaType: AreaTypeNormal,
		NetworkType: NetworkNBMA, State: InterfaceStateDR,
		Address: ip4("10.0.0.1"), NetworkMask: ip4("255.255.255.0"), RouterID: rid("1.1.1.1"), DR: rid("1.1.1.1"), Cost: 10,
		Neighbors: []NeighborInfo{{RouterID: rid("2.2.2.2"), Address: naddr4("10.0.0.2"), State: NeighborStateFull}},
	}
	db.SetTopology(func() []InterfaceInfo { return []InterfaceInfo{nbma} })
	db.OriginateFromTopology(rid("1.1.1.1"), false)
	key := types.LSAKey{Type: types.LSTypeNetwork, LinkStateID: types.LinkStateID(ip4("10.0.0.1")), AdvertisingRouter: rid("1.1.1.1")}
	lsa, ok := db.LookupLSA(area("0.0.0.0"), key)
	if !ok {
		t.Fatalf("NBMA DR did not originate a Network-LSA")
	}
	body, err := lsa.DecodeNetwork()
	if err != nil {
		t.Fatalf("DecodeNetwork: %v", err)
	}
	if len(body.AttachedRouters) != 2 {
		t.Fatalf("NBMA Network-LSA attached = %d, want 2 (self + Full neighbor)", len(body.AttachedRouters))
	}
}
