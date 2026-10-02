// VALIDATES: RFC 2328 Section 13.4 through ReceiveUpdate: a received LSA is detected as
// self-originated when its Advertising Router is this router's Router ID, or when it is a
// network-LSA whose Link State ID is one of this router's interface addresses; a detected
// LSA with no local copy is flushed (installed and flooded at MaxAge).
// PREVENTS: a stale network-LSA from this router's previous Router ID living on in the
// routing domain, and a foreign LSA being flushed because it resembles an own one.
package lsdb

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func selfDetectNetworkLSA(t *testing.T, lsID string, adv types.RouterID) packet.LSA {
	t.Helper()
	body := packet.NetworkLSA{NetworkMask: ip4("255.255.255.0"), AttachedRouters: []types.RouterID{adv, rid("3.3.3.3")}}
	lsa := packet.LSA{Header: packet.LSAHeader{Options: types.OptionE, Type: types.LSTypeNetwork, LinkStateID: lsid(lsID), AdvertisingRouter: adv, Sequence: types.InitialSequenceNumber.Next()}, Network: &body}
	return encodeDecodeLSA(t, lsa)
}

func selfDetectExternalLSA(t *testing.T, lsID string, adv types.RouterID) packet.LSA {
	t.Helper()
	body := packet.ExternalLSA{NetworkMask: ip4("255.255.255.255"), Metric: 20}
	lsa := packet.LSA{Header: packet.LSAHeader{Options: types.OptionE, Type: types.LSTypeASExternal, LinkStateID: lsid(lsID), AdvertisingRouter: adv, Sequence: types.InitialSequenceNumber.Next()}, External: &body}
	return encodeDecodeLSA(t, lsa)
}

// selfDetectReceive delivers lsa on eth0 (floodTopology: this router 1.1.1.1, eth0
// 10.0.0.1, eth1 10.0.1.1) and returns the database copy and whether an LS Update
// carried the key at MaxAge.
func selfDetectReceive(t *testing.T, lsa packet.LSA) (packet.LSAHeader, bool) {
	t.Helper()
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodTopology)
	receiveOnEth0(t, db, lsa)
	got, ok := db.Lookup(area("0.0.0.0"), lsa.Header.Key())
	if !ok {
		t.Fatalf("no database copy of %v after receive", lsa.Header.Key())
	}
	flushed := false
	for i := range tx.sends {
		if tx.sends[i].pkt.LSUpdate == nil {
			continue
		}
		for _, l := range tx.sends[i].pkt.LSUpdate.LSAs {
			if l.Header.Key() == lsa.Header.Key() && l.Header.Age.IsMaxAge() {
				flushed = true
			}
		}
	}
	return got, flushed
}

// RFC requirement: RFC2328-13.4-1 positive -- both detection rules fire on the receive path: a router-LSA advertised by this router's Router ID 1.1.1.1, and a network-LSA advertised by 9.9.9.9 whose Link State ID is this router's eth1 address 10.0.1.1 (or eth0 address 10.0.0.1), are each treated as self-originated: with no local copy each is installed at MaxAge with a higher sequence and flooded at MaxAge (handleSelfReceived, origination.go).
func TestRFC2328SelfOriginatedDetectedByRouterIDOrInterfaceAddress(t *testing.T) {
	cases := []struct {
		name string
		lsa  packet.LSA
	}{
		{name: "advertising-router-is-self", lsa: routerLSA(t, rid("1.1.1.1"), types.InitialSequenceNumber.Next(), 10)},
		{name: "network-lsid-is-eth1-address", lsa: selfDetectNetworkLSA(t, "10.0.1.1", rid("9.9.9.9"))},
		{name: "network-lsid-is-eth0-address", lsa: selfDetectNetworkLSA(t, "10.0.0.1", rid("9.9.9.9"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, flushed := selfDetectReceive(t, tc.lsa)
			if !got.Age.IsMaxAge() || got.Sequence != tc.lsa.Header.Sequence.Next() {
				t.Fatalf("database copy age %v sequence %v, want MaxAge at %v: not detected as self-originated", got.Age, got.Sequence, tc.lsa.Header.Sequence.Next())
			}
			if !flushed {
				t.Fatal("the self-originated LSA was not flooded at MaxAge")
			}
		})
	}
}

// RFC requirement: RFC2328-13.4-1 negative -- LSAs matching neither rule are not self-originated: a network-LSA from 9.9.9.9 whose Link State ID 10.0.1.9 is not one of this router's addresses, a router-LSA from 4.4.4.4, and an AS-external-LSA from 9.9.9.9 whose Link State ID equals this router's eth1 address (rule 2 covers network-LSAs only) are each installed as received, below MaxAge with the received sequence, and never flooded at MaxAge (handleSelfReceived, origination.go).
func TestRFC2328ForeignLSANotTakenAsSelfOriginated(t *testing.T) {
	cases := []struct {
		name string
		lsa  packet.LSA
	}{
		{name: "network-lsid-not-ours", lsa: selfDetectNetworkLSA(t, "10.0.1.9", rid("9.9.9.9"))},
		{name: "router-lsa-of-another-router", lsa: routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber.Next(), 10)},
		{name: "external-lsid-is-eth1-address", lsa: selfDetectExternalLSA(t, "10.0.1.1", rid("9.9.9.9"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, flushed := selfDetectReceive(t, tc.lsa)
			if got.Age.IsMaxAge() || got.Sequence != tc.lsa.Header.Sequence {
				t.Fatalf("database copy age %v sequence %v, want the received instance %v below MaxAge", got.Age, got.Sequence, tc.lsa.Header.Sequence)
			}
			if flushed {
				t.Fatal("a foreign LSA was flushed as self-originated")
			}
		})
	}
}
