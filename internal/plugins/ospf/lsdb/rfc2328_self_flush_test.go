// Design: docs/architecture/ospf/README.md -- self-originated LSA handling
// Related: origination.go -- handleSelfReceived, the RFC 2328 Section 13.4 special action.
//
// VALIDATES: RFC 2328 Section 13.4 through ReceiveUpdate: a received self-originated LSA
// newer than the router's own instance is flushed (LS age MaxAge, reflooded) when the
// router no longer wishes to originate it, and is answered by a new instance below MaxAge
// when the router still originates it.
// PREVENTS: a withdrawn summary-LSA coming back to life at LS age 0 because a neighbor
// still floods a stale copy, and a live LSA being flushed instead of updated.
package lsdb

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// selfFlushMaxAgeFlooded reports whether an LS Update carried key at MaxAge.
func selfFlushMaxAgeFlooded(tx *txRecorder, key types.LSAKey) bool {
	for i := range tx.sends {
		if tx.sends[i].pkt.LSUpdate == nil {
			continue
		}
		for _, l := range tx.sends[i].pkt.LSUpdate.LSAs {
			if l.Header.Key() == key && l.Header.Age.IsMaxAge() {
				return true
			}
		}
	}
	return false
}

// RFC requirement: RFC2328-13.4-3 positive -- a received self-originated LSA the router no longer wishes to originate is flushed by setting its LS age to MaxAge and reflooding, not updated: a summary-LSA this router originated and then withdrew, received again at a higher sequence below MaxAge, is installed at MaxAge with the received LS sequence number left alone (RFC 2328 Section 14.1) and flooded at MaxAge; and a network-LSA whose Link State ID is this router's eth1 address but whose Advertising Router is another router is installed and flooded at MaxAge (handleSelfReceived, origination.go).
func TestRFC2328SelfOriginatedNoLongerWantedIsFlushed(t *testing.T) {
	t.Run("withdrawn-summary", func(t *testing.T) {
		clock := &fakeClock{now: time.Unix(0, 0)}
		db := newTestDB(clock)
		db.SetTopology(floodTopology)
		a0 := area("0.0.0.0")
		h, ok := db.OriginateSummary(a0, rid("1.1.1.1"), types.OptionE, types.LSTypeSummaryNetwork, lsid("10.20.0.0"), ip4("255.255.255.0"), 100)
		if !ok {
			t.Fatal("OriginateSummary false")
		}
		withdrawn, ok := db.WithdrawSelf(a0, h.Key())
		if !ok || !withdrawn.Age.IsMaxAge() {
			t.Fatalf("WithdrawSelf = %+v ok=%v, want a MaxAge flush", withdrawn, ok)
		}
		clock.now = clock.now.Add(10 * time.Second)
		tx := &txRecorder{}
		db.SetTx(tx.Send)
		stale := packet.LSA{Header: packet.LSAHeader{Options: types.OptionE, Type: types.LSTypeSummaryNetwork, LinkStateID: lsid("10.20.0.0"), AdvertisingRouter: rid("1.1.1.1"), Sequence: withdrawn.Sequence.Next().Next()}, Summary: &packet.SummaryLSA{NetworkMask: ip4("255.255.255.0"), Metric: 100}}
		stale = encodeDecodeLSA(t, stale)
		receiveOnEth0(t, db, stale)
		got, ok := db.Lookup(a0, h.Key())
		if !ok {
			t.Fatal("no database copy after receiving the stale self-originated summary-LSA")
		}
		if !got.Age.IsMaxAge() {
			t.Fatalf("database copy age %v sequence %v, want MaxAge: the withdrawn LSA was re-originated instead of flushed", got.Age, got.Sequence)
		}
		if got.Sequence != stale.Header.Sequence {
			t.Fatalf("database copy sequence %v, want %v: RFC 2328 Section 14.1 flushes leaving the received LS sequence number alone", got.Sequence, stale.Header.Sequence)
		}
		if !selfFlushMaxAgeFlooded(tx, h.Key()) {
			t.Fatal("the flush was not reflooded at MaxAge")
		}
	})
	t.Run("network-lsid-own-address-other-advertising-router", func(t *testing.T) {
		got, flushed := selfDetectReceive(t, selfDetectNetworkLSA(t, "10.0.1.1", rid("9.9.9.9")))
		if !got.Age.IsMaxAge() {
			t.Fatalf("database copy age %v, want MaxAge: the network-LSA was not flushed", got.Age)
		}
		if !flushed {
			t.Fatal("the network-LSA was not reflooded at MaxAge")
		}
	})
}

// RFC requirement: RFC2328-13.4-3 negative -- a received self-originated LSA the router still originates is not flushed: a router-LSA this router originated, received at a higher sequence, is answered by a new instance below MaxAge whose sequence is one past the received one, and no LS Update carries it at MaxAge (handleSelfReceived, origination.go).
func TestRFC2328SelfOriginatedStillWantedIsNotFlushed(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	db.SetTopology(floodTopology)
	in := OriginInput{AreaID: area("0.0.0.0"), RouterID: rid("1.1.1.1"), Options: types.OptionE, Interfaces: originTopology()}
	h, ok := db.OriginateRouter(in)
	if !ok {
		t.Fatal("OriginateRouter false")
	}
	clock.now = clock.now.Add(10 * time.Second)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	newer := routerLSA(t, rid("1.1.1.1"), h.Sequence.Next().Next(), 10)
	receiveOnEth0(t, db, newer)
	got, ok := db.Lookup(in.AreaID, h.Key())
	if !ok {
		t.Fatal("no database copy of the router-LSA")
	}
	if got.Age.IsMaxAge() {
		t.Fatalf("database copy at MaxAge, sequence %v: a router-LSA this router still originates was flushed", got.Sequence)
	}
	if got.Sequence != newer.Header.Sequence.Next() {
		t.Fatalf("database copy sequence %v, want %v, one past the received instance", got.Sequence, newer.Header.Sequence.Next())
	}
	if selfFlushMaxAgeFlooded(tx, h.Key()) {
		t.Fatal("a router-LSA this router still originates was flooded at MaxAge")
	}
}
