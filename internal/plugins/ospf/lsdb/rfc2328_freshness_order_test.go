// VALIDATES: RFC 2328 Section 13.1: the steps that decide which of two instances is more
// recent apply in order (sequence, then checksum as an unsigned 16-bit value, then MaxAge,
// then an age difference above MaxAgeDiff), and the sequence comparison uses the signed
// space of Section 12.1.6.
// PREVENTS: a comparator that checks a later step first, or compares sequence numbers or
// checksums with the wrong signedness; the one-field-at-a-time matrix cannot see either.
package lsdb

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC2328-13.1-1 positive -- each case makes an earlier step and a later step disagree, and CompareHeaders answers by the earlier one in both argument orders: a newer sequence wins against a larger checksum at MaxAge; within one sequence the larger checksum as an unsigned 16-bit value (0x8001 over 0x7FFF) wins against MaxAge; within one checksum MaxAge wins against a younger age; then the younger by more than MaxAgeDiff wins; and MaxSequenceNumber is newer than InitialSequenceNumber in the signed sequence space (entry.go).
func TestRFC2328FreshnessStepsAppliedInOrder(t *testing.T) {
	// Goal: discriminate the ORDER of the steps and the signedness of two comparisons.
	// Method: pairs where the newer instance by the earlier step loses every later step.
	base := packet.LSAHeader{Age: 10, Type: types.LSTypeRouter, LinkStateID: lsid("1.1.1.1"), AdvertisingRouter: rid("1.1.1.1"), Sequence: types.InitialSequenceNumber, Checksum: 0x1000, Length: types.LSAHeaderLen}
	maxAge := types.LSAge(types.MaxAge)
	cases := []struct {
		name         string
		newer, older packet.LSAHeader
	}{
		{"sequence before checksum and MaxAge",
			with(base, func(h *packet.LSAHeader) { h.Sequence = base.Sequence.Next(); h.Checksum = 0x0001; h.Age = 0 }),
			with(base, func(h *packet.LSAHeader) { h.Checksum = 0xFFFF; h.Age = maxAge })},
		{"unsigned checksum before MaxAge",
			with(base, func(h *packet.LSAHeader) { h.Checksum = 0x8001; h.Age = 1 }),
			with(base, func(h *packet.LSAHeader) { h.Checksum = 0x7FFF; h.Age = maxAge })},
		{"MaxAge before age difference",
			with(base, func(h *packet.LSAHeader) { h.Age = maxAge }),
			with(base, func(h *packet.LSAHeader) { h.Age = 1 })},
		{"younger by more than MaxAgeDiff",
			with(base, func(h *packet.LSAHeader) { h.Age = 1 }),
			with(base, func(h *packet.LSAHeader) { h.Age = types.LSAge(types.MaxAgeDiff + 2) })},
		{"signed sequence space",
			with(base, func(h *packet.LSAHeader) { h.Sequence = types.MaxSequenceNumber }),
			with(base, func(h *packet.LSAHeader) { h.Sequence = types.InitialSequenceNumber })},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CompareHeaders(tc.newer, tc.older); got != Newer {
				t.Errorf("CompareHeaders(newer, older) = %v, want Newer", got)
			}
			if got := CompareHeaders(tc.older, tc.newer); got != Older {
				t.Errorf("CompareHeaders(older, newer) = %v, want Older", got)
			}
		})
	}
}

// RFC requirement: RFC2328-13.1-1 negative -- through ReceiveUpdate, an instance that is less recent by an earlier step is refused even when a later step favors it: a lower sequence at MaxAge, and the same sequence with the smaller LS checksum at MaxAge, each leave the database copy in place (same sequence, same checksum, not MaxAge) (CompareHeaders, entry.go; ReceiveUpdate, flooding.go).
func TestRFC2328LessRecentByEarlierStepRefused(t *testing.T) {
	// Goal: the order of the steps decides what the database keeps.
	// Method: install a copy by flooding, wait past MinLSArrival, then receive an instance
	// at MaxAge that loses on sequence or on checksum.
	seq := types.InitialSequenceNumber.Next()
	a, b := routerLSA(t, rid("4.4.4.4"), seq, 10), routerLSA(t, rid("4.4.4.4"), seq, 11)
	if a.Header.Checksum == b.Header.Checksum {
		t.Fatalf("setup: metrics 10 and 11 gave the same LS checksum")
	}
	larger, smaller := a, b
	if smaller.Header.Checksum > larger.Header.Checksum {
		larger, smaller = b, a
	}
	cases := []struct {
		name           string
		database, recv packet.LSA
	}{
		{"lower sequence at MaxAge", larger, agedLSA(t, routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 12))},
		{"smaller checksum at MaxAge", larger, agedLSA(t, smaller)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clock := &fakeClock{now: time.Unix(0, 0)}
			db := newTestDB(clock)
			tx := &txRecorder{}
			db.SetTx(tx.Send)
			db.SetTopology(floodTopology)
			receiveOnEth0(t, db, tc.database)
			clock.Add(2 * time.Second)
			receiveOnEth0(t, db, tc.recv)
			got, ok := db.Lookup(area("0.0.0.0"), tc.database.Header.Key())
			if !ok {
				t.Fatalf("the database copy was removed")
			}
			if got.Sequence != tc.database.Header.Sequence || got.Checksum != tc.database.Header.Checksum {
				t.Fatalf("database holds seq %v checksum %#x, want the copy's seq %v checksum %#x", got.Sequence, got.Checksum, tc.database.Header.Sequence, tc.database.Header.Checksum)
			}
			if got.Age.IsMaxAge() {
				t.Fatalf("the database copy was replaced by the MaxAge instance")
			}
		})
	}
}

func with(h packet.LSAHeader, edit func(*packet.LSAHeader)) packet.LSAHeader {
	edit(&h)
	return h
}

// agedLSA re-encodes lsa with LS age MaxAge; the age is outside the LS checksum, so the
// checksum is unchanged.
func agedLSA(t *testing.T, lsa packet.LSA) packet.LSA {
	t.Helper()
	aged := lsa
	aged.Header.Age = types.LSAge(types.MaxAge)
	aged.RawBytes = nil
	out := encodeDecodeLSA(t, aged)
	if out.Header.Checksum != lsa.Header.Checksum || !out.Header.Age.IsMaxAge() {
		t.Fatalf("setup: aged copy checksum %#x age %v, want %#x at MaxAge", out.Header.Checksum, out.Header.Age, lsa.Header.Checksum)
	}
	return out
}
