// Design: docs/features/interfaces.md -- DHCPv4 client lifecycle
// Related: dhcp_v4_linux.go -- v4NextAttempt, v4LeaseTimers
// Related: dhcp_v4_schedule_linux_test.go -- the send-instant deadline test

//go:build linux

package ifacedhcp

import (
	"testing"
	"time"
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
	// RFC requirement: RFC2131-4.4.5-8 positive -- with no answer, the next RENEWING DHCPREQUEST waits half the time left until T2 and the next REBINDING one half the lease time left: a 1000 s lease transmits at 500, 687.5, 781.25 and 841.25 s, then at 875, 937.5 and 997.5 s.
	assertTimes(t, "RENEWING", walk(seconds(500), seconds(875)), silentRenewTimes)
	assertTimes(t, "REBINDING", walk(seconds(875), seconds(1000)), silentRebindTimes)

	// RFC requirement: RFC2131-4.4.5-8 negative -- half the time left is not used below 60 s: with 120 s left the next attempt waits 60 s, and with 60 s or less left no attempt is scheduled before the phase ends.
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
	// RFC requirement: RFC2131-4.4.5-2 positive -- a server T1 (option 58) earlier than T2 is used as sent: 30 s and 69 s against a T2 of 70 s.
	// RFC requirement: RFC2131-4.4.5-2 negative -- a server T1 not earlier than T2 is not used, and both timers fall back to the defaults: 80 s against 40 s, 70 s against 70 s, and 90 s alone against the default T2 of 87.5 s.
	// RFC requirement: RFC2131-4.4.5-3 positive -- a server T2 (option 59) earlier than the lease expiry is used as sent: 99 s and 95 s in a 100 s lease.
	// RFC requirement: RFC2131-4.4.5-3 negative -- a server T2 at the lease expiry is not used, and both timers fall back to the defaults: 100 s in a 100 s lease.
	for _, tc := range cases {
		t1, t2, rejected := v4LeaseTimers(lease, tc.t1, tc.t2)
		if t1 != tc.wantT1 || t2 != tc.wantT2 || rejected != tc.rejected {
			t.Errorf("%s: T1 %v T2 %v rejected %v, want %v %v %v", tc.name, t1, t2, rejected, tc.wantT1, tc.wantT2, tc.rejected)
		}
	}
}
