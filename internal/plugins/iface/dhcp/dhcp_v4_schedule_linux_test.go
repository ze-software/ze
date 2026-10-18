// Design: docs/features/interfaces.md -- DHCPv4 client lifecycle
// Related: dhcp_v4_linux.go -- v4LeaseFromAck
// Related: rfc2131_dhcp_v4_timers_linux_test.go -- the RFC 2131 Section 4.4.5 timer and schedule tests

//go:build linux

package ifacedhcp

import (
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

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

	r := newV4Rig(t)
	r.exch.acquireDelay = 3 * time.Second
	r.exch.acquireAnswer = firstAckThenFail(testAck(t, loopServerA, 100*time.Second, nil))
	r.startWorker()
	r.runUntil(seconds(51))
	assertTimes(t, "RENEW", r.extendTimes(v4DeliveryUnicast), []time.Duration{seconds(50)})
	r.stop()

	r = newV4Rig(t)
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
