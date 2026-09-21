// Design: docs/architecture/ospf/ospf-10-as-external-asbr.md -- Type-7 acceptance by area type.
// RFC: rfc/short/rfc3101.md -- Appendix D (ExternalRoutingCapability accepts Type-7 in an NSSA).
package lsdb

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// nssaLSA builds a received Type-7 NSSA-LSA for 203.0.113.0/24 from adv.
func nssaLSA(t *testing.T, adv types.RouterID, seq types.LSSequenceNumber) packet.LSA {
	t.Helper()
	body := packet.ExternalLSA{NetworkMask: ip4("255.255.255.0"), Metric: 20, ForwardingAddr: ip4("10.0.0.2")}
	lsa := packet.LSA{Header: packet.LSAHeader{Age: 0, Options: types.OptionNP, Type: types.LSTypeNSSA, LinkStateID: lsid("203.0.113.0"), AdvertisingRouter: adv, Sequence: seq}, External: &body}
	return encodeDecodeLSA(t, lsa)
}

// receiveType7 delivers one Type-7 LSA on eth0 in an area of the given type and reports
// whether the LSDB installed it.
func receiveType7(t *testing.T, areaType string) bool {
	t.Helper()
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	a := area("0.0.0.7")
	db.SetAreaTypes(map[types.AreaID]string{a: areaType})
	db.SetTopology(func() []InterfaceInfo {
		return []InterfaceInfo{{
			Name: "eth0", AreaID: a, AreaType: areaType,
			NetworkType: types.NetworkPointToPoint, State: InterfaceStateDR, RouterID: rid("1.1.1.1"), TransmitDelay: 1,
			Neighbors: []NeighborInfo{{RouterID: rid("2.2.2.2"), Address: naddr4("10.0.0.2"), State: NeighborStateFull}},
		}}
	})
	lsa := nssaLSA(t, rid("2.2.2.2"), types.InitialSequenceNumber)
	db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: a, RouterID: rid("2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}})
	_, ok := db.LookupLSA(a, lsa.Header.Key())
	return ok
}

// RFC requirement: RFC3101-x-4 positive -- an area whose type is configured as NSSA has its
// ExternalRoutingCapability set to accept Type-7 external routes: a Type-7 NSSA-LSA received
// on that area's interface is installed into the area store (shouldDropByArea, flooding.go,
// keeps LSTypeNSSA when the area type is nssa).
func TestRFC3101NSSAAreaAcceptsType7(t *testing.T) {
	if !receiveType7(t, types.AreaTypeNSSA) {
		t.Fatalf("Type-7 NSSA-LSA was not installed in an NSSA area")
	}
}

// RFC requirement: RFC3101-x-4 negative -- the capability is what the area type configures,
// not a blanket: the same Type-7 NSSA-LSA received on a normal area's interface is discarded
// and never enters the area store (shouldDropByArea, flooding.go, drops LSTypeNSSA when the
// area type is not nssa).
func TestRFC3101NormalAreaRejectsType7(t *testing.T) {
	if receiveType7(t, types.AreaTypeNormal) {
		t.Fatalf("Type-7 NSSA-LSA was installed in a normal area")
	}
}
