// VALIDATES: spec-ospf-ext-1 AC-10/AC-12/R-6 -- the opaque delivery hook fires on a
// newer install only (not on an equal/older duplicate), and an opaque LSA is stored and
// reflooded per scope regardless of whether a delivery hook is wired (the store/flood
// path is consumer-agnostic).
// PREVENTS: re-delivering a duplicate opaque LSA to a consumer, and dropping an opaque
// LSA of an unregistered type instead of transiting it to downstream consumers.
package lsdb

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestOpaqueDeliveryOnNewerOnly(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	db.SetTx((&txRecorder{}).Send)
	db.SetTopology(opaqueTopology)
	a0 := area("0.0.0.0")

	var deliveries []OpaqueDelivery
	db.SetOpaqueDelivery(func(d OpaqueDelivery) { deliveries = append(deliveries, d) })

	first := opaqueLSA(t, types.LSTypeOpaqueArea, 1, 0x60, rid("2.2.2.2"), types.InitialSequenceNumber, []byte{0xaa, 0xbb, 0xcc, 0xdd})
	recv := func(lsa packet.LSA) {
		db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: a0, RouterID: rid("2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}})
	}

	recv(first)
	if len(deliveries) != 1 {
		t.Fatalf("newer install: delivered %d times, want 1", len(deliveries))
	}
	got := deliveries[0]
	if got.Scope != types.LSTypeOpaqueArea || got.OpaqueType != 1 || got.OpaqueID != 0x60 || got.AdvertisingRouter != rid("2.2.2.2") {
		t.Fatalf("bad delivery payload: %+v", got)
	}

	// The same instance again (Equal) must NOT re-deliver.
	clock.Add(2 * time.Second)
	recv(first)
	if len(deliveries) != 1 {
		t.Fatalf("equal duplicate re-delivered: %d", len(deliveries))
	}

	// A newer sequence delivers once more.
	clock.Add(2 * time.Second)
	newer := opaqueLSA(t, types.LSTypeOpaqueArea, 1, 0x60, rid("2.2.2.2"), types.InitialSequenceNumber.Next(), []byte{0x11, 0x22, 0x33, 0x44})
	recv(newer)
	if len(deliveries) != 2 {
		t.Fatalf("newer instance not delivered: %d", len(deliveries))
	}
}

func TestUnregisteredOpaqueReflooded(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(opaqueTopology)
	// No delivery hook wired: this models an opaque type with no registered consumer.
	// The LSDB must still store and reflood it per scope.
	a0 := area("0.0.0.0")
	lsa10 := opaqueLSA(t, types.LSTypeOpaqueArea, 200 /* private-use, unregistered */, 0x70, rid("2.2.2.2"), types.InitialSequenceNumber, []byte{1, 2, 3, 4})

	// Arrives on eth0; must be stored and reflooded out eth1 to its opaque-capable neighbor.
	db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: a0, RouterID: rid("2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa10}}})
	if _, ok := db.LookupLSA(a0, lsa10.Header.Key()); !ok {
		t.Fatalf("unregistered opaque LSA not stored")
	}
	if db.retransmit[NeighborKey{Interface: "eth1", RouterID: rid("3.3.3.3")}][lsa10.Header.Key()] == nil {
		t.Fatalf("unregistered opaque LSA not reflooded to a downstream opaque neighbor")
	}
}
