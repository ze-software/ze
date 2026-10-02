// VALIDATES: RFC 2328 Section 13 at the LSDB flood path: an AS-external-LSA is not flooded
// into a stub area (send side, received and self-originated) and not throughout one
// (an AS-external-LSA arriving on a stub interface goes nowhere).
// PREVENTS: a flood path that stops consulting the stub/NSSA filter leaking Type-5 LSAs
// into a stub area, which the helper-only unit TestOSPFStubFloodFilter cannot see.
package lsdb

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// stubExternalTopology is an area border router 1.1.1.1 with one Full neighbor per area:
// eth0 backbone (2.2.2.2), eth1 stub 0.0.0.1 (3.3.3.3), eth2 NSSA 0.0.0.2 (5.5.5.5) and
// eth3 normal 0.0.0.3 (4.4.4.4). eth3 is the control: a Type-5 MUST reach it.
func stubExternalTopology() []InterfaceInfo {
	one := func(name, areaID, areaType, addr, nbr, nbrAddr string) InterfaceInfo {
		return InterfaceInfo{
			Name: name, AreaID: area(areaID), AreaType: areaType,
			NetworkType: types.NetworkBroadcast, State: InterfaceStateDR,
			Address: ip4(addr), NetworkMask: ip4("255.255.255.0"), RouterID: rid("1.1.1.1"), DR: rid("1.1.1.1"), TransmitDelay: 1,
			Neighbors: []NeighborInfo{{RouterID: rid(nbr), Address: naddr4(nbrAddr), State: NeighborStateFull}},
		}
	}
	return []InterfaceInfo{
		one("eth0", "0.0.0.0", types.AreaTypeNormal, "10.0.0.1", "2.2.2.2", "10.0.0.2"),
		one("eth1", "0.0.0.1", types.AreaTypeStub, "10.0.1.1", "3.3.3.3", "10.0.1.3"),
		one("eth2", "0.0.0.2", types.AreaTypeNSSA, "10.0.2.1", "5.5.5.5", "10.0.2.5"),
		one("eth3", "0.0.0.3", types.AreaTypeNormal, "10.0.3.1", "4.4.4.4", "10.0.3.4"),
	}
}

func newStubExternalDB(t *testing.T) (*LSDB, *txRecorder) {
	t.Helper()
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetAreaTypes(map[types.AreaID]string{area("0.0.0.1"): types.AreaTypeStub, area("0.0.0.2"): types.AreaTypeNSSA})
	db.SetTopology(stubExternalTopology)
	return db, tx
}

// lsaFloodedOn reports whether an LS Update carrying key left interface iface.
func lsaFloodedOn(tx *txRecorder, iface string, key types.LSAKey) bool {
	for i := range tx.sends {
		if tx.sends[i].iface != iface || tx.sends[i].pkt.LSUpdate == nil {
			continue
		}
		for j := range tx.sends[i].pkt.LSUpdate.LSAs {
			if tx.sends[i].pkt.LSUpdate.LSAs[j].Header.Key() == key {
				return true
			}
		}
	}
	return false
}

// RFC requirement: RFC2328-13-2 positive -- an AS-external-LSA, whether received from the backbone neighbor or originated by this router, is flooded in an LS Update out the normal-area interface eth3 and never out the stub interface eth1 (nor the NSSA interface eth2), and no stub or NSSA neighbor gets it on its retransmission list (floodExcept/eligibleInterface, flooding.go).
func TestRFC2328ASExternalNotFloodedIntoStubArea(t *testing.T) {
	// Goal: the send side of the stub rule on the real flood path, both entry points.
	// Method: four-area ABR; the Type-5 enters by ReceiveUpdate on eth0, or by
	// OriginateExternal; the decoded sends and the retransmission lists are read.
	cases := []struct {
		name   string
		inject func(t *testing.T, db *LSDB) types.LSAKey
	}{
		{"received", func(t *testing.T, db *LSDB) types.LSAKey {
			lsa := externalLSA(t, rid("6.6.6.6"), types.InitialSequenceNumber)
			reason := db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: area("0.0.0.0"), RouterID: rid("2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}})
			if reason != "" {
				t.Fatalf("ReceiveUpdate reason = %q", reason)
			}
			return lsa.Header.Key()
		}},
		{"self-originated", func(t *testing.T, db *LSDB) types.LSAKey {
			h, ok, err := db.OriginateExternal(rid("1.1.1.1"), [4]byte{203, 0, 113, 0}, [4]byte{255, 255, 255, 0}, types.OptionE, true, 20, [4]byte{}, 0)
			if err != nil {
				t.Fatalf("OriginateExternal: %v", err)
			}
			if !ok {
				t.Fatalf("OriginateExternal installed nothing")
			}
			return h.Key()
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, tx := newStubExternalDB(t)
			key := tc.inject(t, db)
			if !lsaFloodedOn(tx, "eth3", key) {
				t.Fatalf("setup: the AS-external-LSA was not flooded out the normal-area interface eth3: %+v", tx.sends)
			}
			for _, iface := range []string{"eth1", "eth2"} {
				if lsaFloodedOn(tx, iface, key) {
					t.Errorf("an LS Update carrying the AS-external-LSA was flooded out %s, into a stub/NSSA area", iface)
				}
			}
			for _, nk := range []NeighborKey{{Interface: "eth1", RouterID: rid("3.3.3.3")}, {Interface: "eth2", RouterID: rid("5.5.5.5")}} {
				if db.retransmit[nk][key] != nil {
					t.Errorf("the AS-external-LSA is on %s's retransmission list", nk.RouterID)
				}
			}
		})
	}
}

// RFC requirement: RFC2328-13-2 negative -- an AS-external-LSA arriving from the stub neighbor on eth1 (or the NSSA neighbor on eth2) is refused: it is not installed in the AS-external database and no LS Update carries it out of any other interface, so it is not flooded throughout the area nor beyond it (shouldDropByArea, flooding.go).
func TestRFC2328ASExternalFromStubInterfaceGoesNowhere(t *testing.T) {
	// Goal: the receive side of the stub rule with somewhere to flood to.
	// Method: the same four-area ABR; the Type-5 arrives on the stub or NSSA interface.
	cases := []struct {
		iface, areaID, nbr, src string
	}{
		{"eth1", "0.0.0.1", "3.3.3.3", "10.0.1.3"},
		{"eth2", "0.0.0.2", "5.5.5.5", "10.0.2.5"},
	}
	for _, tc := range cases {
		t.Run(tc.iface, func(t *testing.T) {
			db, tx := newStubExternalDB(t)
			lsa := externalLSA(t, rid("6.6.6.6"), types.InitialSequenceNumber)
			db.ReceiveUpdate(ReceiveInput{Interface: tc.iface, AreaID: area(tc.areaID), RouterID: rid(tc.nbr), Src: netip.MustParseAddr(tc.src), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}})
			for _, a := range []string{"0.0.0.0", tc.areaID} {
				if _, ok := db.Lookup(area(a), lsa.Header.Key()); ok {
					t.Errorf("the AS-external-LSA from the %s neighbor was installed (lookup in area %s)", tc.iface, a)
				}
			}
			for _, out := range []string{"eth0", "eth1", "eth2", "eth3"} {
				if lsaFloodedOn(tx, out, lsa.Header.Key()) {
					t.Errorf("the AS-external-LSA received on %s was flooded out %s", tc.iface, out)
				}
			}
		})
	}
}
