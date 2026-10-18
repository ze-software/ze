// Design: docs/features/interfaces.md -- DHCPv4 client lifecycle
// Related: dhcp_v4_linux.go -- v4NextAttempt, v4LeaseFromAck, v4LeaseTimers

//go:build linux

package ifacedhcp

import (
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

// TestV4RetransmitSchedule walks the pure retransmission function.
//
// VALIDATES: AC-9 -- each wait is half the time left in the phase, never less
// than 60 s, and a wait that reaches the phase end moves to the phase end:
// RENEWING of a 1000 s lease transmits at 500, 687.5, 781.25, 841.25 s and
// hands over at 875 s; REBINDING transmits at 875, 937.5, 997.5 s and ends at
// 1000 s. Boundaries: 120 s left waits 60 s, exactly 60 s left and less go to
// the phase end.
func TestV4RetransmitSchedule(t *testing.T) {
	start := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	walk := func(from, end time.Duration) []time.Duration {
		var out []time.Duration
		for at := from; at < end; {
			out = append(out, at)
			at = v4NextAttempt(start.Add(at), start.Add(end)).Sub(start)
		}
		return out
	}
	assertTimes(t, "RENEWING", walk(seconds(500), seconds(875)), silentRenewTimes)
	assertTimes(t, "REBINDING", walk(seconds(875), seconds(1000)), silentRebindTimes)

	cases := []struct {
		left, want time.Duration
	}{
		{left: seconds(120), want: seconds(60)},
		{left: seconds(121), want: seconds(60.5)},
		{left: seconds(60), want: seconds(60)},
		{left: seconds(59), want: seconds(59)},
		{left: seconds(1), want: seconds(1)},
	}
	for _, tc := range cases {
		if got := v4NextAttempt(start, start.Add(tc.left)).Sub(start); got != tc.want {
			t.Errorf("%v left: next attempt after %v, want %v", tc.left, got, tc.want)
		}
	}
}

// TestV4LeaseTimersFromSendTime measures the deadlines from the REQUEST send
// instant, not from the ACK arrival.
//
// VALIDATES: AC-14 -- an ACK arriving 3 s after its REQUEST, lease 100 s: T1
// 50 s, T2 87.5 s, expiry 100 s after the send; through the loop the first
// RENEW goes 50 s after the DORA REQUEST, not 53 s. AC-18 through the loop:
// options 58 = 30 s and 59 = 70 s put the RENEW at 30 s and the REBIND at 70 s.
// PREVENTS: R-3, expiry late by the round trip.
func TestV4LeaseTimersFromSendTime(t *testing.T) {
	sent := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	lease, refusal := v4LeaseFromAck(testAck(t, loopServerA, 100*time.Second, nil), sent, nil)
	if refusal != nil {
		t.Fatalf("v4LeaseFromAck refused: %v", refusal)
	}
	if lease.renewAt.Sub(sent) != seconds(50) || lease.rebindAt.Sub(sent) != seconds(87.5) || lease.expireAt.Sub(sent) != seconds(100) {
		t.Errorf("deadlines T1 %v T2 %v expiry %v, want 50s 87.5s 100s", lease.renewAt.Sub(sent), lease.rebindAt.Sub(sent), lease.expireAt.Sub(sent))
	}

	r := newV4Rig(t, dHCPConfig{})
	r.exch.acquireDelay = 3 * time.Second
	r.exch.acquireAnswer = firstAckThenFail(testAck(t, loopServerA, 100*time.Second, nil))
	r.startWorker()
	r.runUntil(seconds(51))
	assertTimes(t, "RENEW", r.extendTimes(v4DeliveryUnicast), []time.Duration{seconds(50)})
	r.stop()

	r = newV4Rig(t, dHCPConfig{})
	r.exch.acquireAnswer = firstAckThenFail(testAck(t, loopServerA, 100*time.Second, nil,
		dhcpv4.WithOption(dhcpv4.OptRenewTimeValue(30*time.Second)),
		dhcpv4.WithOption(dhcpv4.OptRebindingTimeValue(70*time.Second))))
	r.startWorker()
	r.runUntil(seconds(71))
	renews := r.extendTimes(v4DeliveryUnicast)
	rebinds := r.extendTimes(v4DeliveryBroadcast)
	if len(renews) == 0 || renews[0] != seconds(30) || len(rebinds) == 0 || rebinds[0] != seconds(70) {
		t.Errorf("RENEW at %v, REBIND at %v; want the first at 30 s and 70 s", renews, rebinds)
	}
}

// TestV4ServerTimersHonoredWhenOrdered judges options 58 and 59 against the
// RFC 2131 Section 4.4.5 order T1 < T2 < lease.
//
// VALIDATES: AC-18 -- ordered values are used; AC-19 -- any order violation,
// by both options or by one against the default for the other, gives both
// defaults and reports the rejection.
func TestV4ServerTimersHonoredWhenOrdered(t *testing.T) {
	be := func(s uint32) []byte { return []byte{byte(s >> 24), byte(s >> 16), byte(s >> 8), byte(s)} }
	lease := 100 * time.Second
	cases := []struct {
		name           string
		t1, t2         []byte
		wantT1, wantT2 time.Duration
		rejected       bool
	}{
		{name: "none", wantT1: seconds(50), wantT2: seconds(87.5)},
		{name: "30 and 70", t1: be(30), t2: be(70), wantT1: seconds(30), wantT2: seconds(70)},
		{name: "T2 one below the lease", t1: be(30), t2: be(99), wantT1: seconds(30), wantT2: seconds(99)},
		{name: "T1 one below T2", t1: be(69), t2: be(70), wantT1: seconds(69), wantT2: seconds(70)},
		{name: "58 alone, ordered", t1: be(30), wantT1: seconds(30), wantT2: seconds(87.5)},
		{name: "59 alone, ordered", t2: be(95), wantT1: seconds(50), wantT2: seconds(95)},
		{name: "reversed", t1: be(80), t2: be(40), wantT1: seconds(50), wantT2: seconds(87.5), rejected: true},
		{name: "equal", t1: be(70), t2: be(70), wantT1: seconds(50), wantT2: seconds(87.5), rejected: true},
		{name: "T2 at the lease", t1: be(30), t2: be(100), wantT1: seconds(50), wantT2: seconds(87.5), rejected: true},
		{name: "T1 zero", t1: be(0), t2: be(70), wantT1: seconds(50), wantT2: seconds(87.5), rejected: true},
		{name: "58 alone past the default T2", t1: be(90), wantT1: seconds(50), wantT2: seconds(87.5), rejected: true},
		{name: "59 alone below the default T1", t2: be(40), wantT1: seconds(50), wantT2: seconds(87.5), rejected: true},
		{name: "58 malformed", t1: []byte{0, 30}, t2: be(70), wantT1: seconds(50), wantT2: seconds(87.5), rejected: true},
	}
	for _, tc := range cases {
		t1, t2, rejected := v4LeaseTimers(lease, tc.t1, tc.t2)
		if t1 != tc.wantT1 || t2 != tc.wantT2 || rejected != tc.rejected {
			t.Errorf("%s: T1 %v T2 %v rejected %v, want %v %v %v", tc.name, t1, t2, rejected, tc.wantT1, tc.wantT2, tc.rejected)
		}
	}
}
