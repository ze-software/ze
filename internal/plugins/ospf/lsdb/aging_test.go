package lsdb

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func setOwnSequenceForTest(db *LSDB, areaID types.AreaID, key types.LSAKey, seq types.LSSequenceNumber) {
	db.mu.Lock()
	defer db.mu.Unlock()
	own := db.own[areaID]
	if own == nil {
		own = make(map[types.LSAKey]ownRecord)
		db.own[areaID] = own
	}
	own[key] = ownRecord{sequence: seq}
}

func TestOSPFLSDBAgeDecrement(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	lsa := routerLSA(t, rid("2.2.2.2"), types.InitialSequenceNumber, 10)
	db.Install(area("0.0.0.0"), lsa)
	clock.Add(7 * time.Second)
	h, _ := db.Lookup(area("0.0.0.0"), lsa.Header.Key())
	if h.Age.Age() != 7 {
		t.Fatalf("age = %d", h.Age.Age())
	}
}

// RFC requirement: RFC2328-14-1 positive -- LS age advances with elapsed time and stops at MaxAge, at which point the LSA is reflooded to flush it (Entry.age/LSAge.Add entry.go:87-101 and types/lsage.go:54-63, Tick lsdb/aging.go:28-37).
// NOT an RFC coverage claim for RFC2328-14-2: both neighbors are acked before the
// deletion is asserted, so every retransmission list is already empty and the
// retransmit-list guard (flooding.go:793-798) is never the reason for a decision here.
// The discriminating positive lives on TestOSPFASExternalPurgeRetainedAcrossAreas.
func TestOSPFLSDBAgeToPurge(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodTopology)
	lsa := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	db.Install(area("0.0.0.0"), lsa)
	clock.Add(time.Duration(types.MaxAge) * time.Second)
	res := db.Tick(clock.Now())
	if res.Purged != 1 {
		t.Fatalf("purged = %d", res.Purged)
	}
	h, ok := db.Lookup(area("0.0.0.0"), lsa.Header.Key())
	if !ok || !h.Age.IsMaxAge() {
		t.Fatalf("purge not retained: h=%+v ok=%v", h, ok)
	}
	if len(tx.sends) == 0 {
		t.Fatalf("purge was not flooded")
	}
	db.ReceiveAck(AckInput{Interface: "eth1", AreaID: area("0.0.0.0"), RouterID: rid("3.3.3.3"), Ack: packetAck(lsa.Header)})
	db.ReceiveAck(AckInput{Interface: "eth0", AreaID: area("0.0.0.0"), RouterID: rid("2.2.2.2"), Ack: packetAck(lsa.Header)})
	db.deletePurgedIfAcked(area("0.0.0.0"), lsa.Header.Key())
	if _, ok := db.Lookup(area("0.0.0.0"), lsa.Header.Key()); ok {
		t.Fatalf("purge retained after all acks")
	}
}

func TestOSPFLSRefresh(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	db.SetTopology(floodTopology)
	h, ok := db.OriginateRouter(OriginInput{AreaID: area("0.0.0.0"), RouterID: rid("1.1.1.1"), Options: types.OptionE, Interfaces: originTopology()})
	if !ok {
		t.Fatalf("originate false")
	}
	clock.Add(time.Duration(types.LSRefreshTime) * time.Second)
	if n := db.RefreshSelf(clock.Now()); n != 1 {
		t.Fatalf("refresh count = %d", n)
	}
	got, _ := db.Lookup(area("0.0.0.0"), h.Key())
	if got.Sequence != h.Sequence.Next() || got.Age.Age() != 0 || got.Checksum == h.Checksum {
		t.Fatalf("refresh header = %+v old=%+v", got, h)
	}
}

func TestOSPFLSRefreshExternal(t *testing.T) {
	// Regression: self-originated Type 5 AS-External LSAs live in the AS-wide store, not
	// in d.areas. RefreshSelf must re-stamp them too -- otherwise Tick MaxAge-purges every
	// redistributed external and the default route ~LSRefreshTime after origination.
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	db.SetTopology(floodTopology)
	router := rid("1.1.1.1")
	h, ok, err := db.OriginateExternal(router, ip4("10.50.0.0"), ip4("255.255.255.0"), types.OptionE, true, 20, ip4("0.0.0.0"), 7)
	if err != nil {
		t.Fatalf("originate external error: %v", err)
	}
	if !ok {
		t.Fatalf("originate external false")
	}
	key := h.Key()
	clock.Add(time.Duration(types.LSRefreshTime) * time.Second)
	if n := db.RefreshSelf(clock.Now()); n != 1 {
		t.Fatalf("external refresh count = %d, want 1", n)
	}
	got, found := db.Lookup(area("0.0.0.0"), key)
	if !found || got.Sequence != h.Sequence.Next() || got.Age.Age() != 0 || got.Checksum == h.Checksum {
		t.Fatalf("external refresh header = %+v old=%+v found=%v", got, h, found)
	}
	// The refreshed external survives the next Tick instead of being MaxAge-purged.
	if res := db.Tick(clock.Now()); res.Purged != 0 {
		t.Fatalf("refreshed external purged by Tick: %d", res.Purged)
	}
}

func TestOSPFSequenceWraparound(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	key := types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(rid("1.1.1.1")), AdvertisingRouter: rid("1.1.1.1")}
	setOwnSequenceForTest(db, area("0.0.0.0"), key, types.MaxSequenceNumber)
	h, ok := db.OriginateRouter(OriginInput{AreaID: area("0.0.0.0"), RouterID: rid("1.1.1.1"), Options: types.OptionE, Interfaces: originTopology()})
	if !ok || h.Sequence != types.MaxSequenceNumber || !h.Age.IsMaxAge() {
		t.Fatalf("wrap purge h=%+v ok=%v", h, ok)
	}
	db.deletePurgedIfAcked(area("0.0.0.0"), key)
	clock.Add(5 * time.Second)
	h, ok = db.OriginateRouter(OriginInput{AreaID: area("0.0.0.0"), RouterID: rid("1.1.1.1"), Options: types.OptionE, Interfaces: originTopology()})
	if !ok || h.Sequence != types.InitialSequenceNumber {
		t.Fatalf("wrap restart h=%+v ok=%v", h, ok)
	}
}

func packetAck(h packet.LSAHeader) packet.LSAck { return packet.LSAck{Headers: []packet.LSAHeader{h}} }
