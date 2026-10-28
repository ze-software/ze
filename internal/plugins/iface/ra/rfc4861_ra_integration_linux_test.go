//go:build linux

// Design: docs/features/interfaces.md -- Router Advertisement sender (Linux)
// Related: ra_integration_linux_test.go -- the veth fixture, capture and helpers these tests share.
//
// The RFC 4861 tagged integration tests of the sender: the source address of
// an advertisement on the wire, and the answer to a solicitation sent to the
// all-routers group. Each runs in its own user, network and mount namespaces.
// They need no host privilege; refused setup fails the test.

package ifacera

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/ndp"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestRASenderWireFormat reads one advertisement off the wire and checks every
// field a receiver depends on.
//
// VALIDATES: RFC 4861 Section 4.2 (Hop Limit 255, type 134, code 0, a
// link-local source), Section 4.6.1 (the Source Link-layer Address option
// carries the sending interface's MAC), and Section 4.6.2 (the Prefix
// Information option carries the advertised prefix with the L and A flags).
// PREVENTS: the sender falling back to the kernel default multicast hop limit
// of 1, which Section 6.1.2 makes every conforming host discard, and prevents a
// silently dropped or malformed option.
//
// RFC requirement: RFC4861-4.2-1 positive -- the advertisement a sender started on a veth emits is received on the peer end with a link-local unicast source address.
func TestRASenderWireFormat(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	const router, host = "zrb0", "zrb1"
	withRAVeth(t, router, host, func() {
		capture := openRACapture(t, host)
		routerMAC := linkHardwareAddress(t, router)

		prefix := netip.MustParsePrefix("2001:db8:a02::/64")
		sender := startSender(t, raSpecFor(router, prefix, 200*time.Millisecond, 400*time.Millisecond))
		defer sender.Stop()

		got, err := capture.next(10 * time.Second)
		if err != nil {
			t.Fatalf("no advertisement on %s within 10s: %v", host, err)
		}

		// RFC 4861 Section 4.2. The literal is deliberate: reading the
		// sender's own constant would agree with any value it chose.
		if got.hopLimit != 255 {
			t.Errorf("received Hop Limit = %d, want 255 (RFC 4861 Section 4.2)", got.hopLimit)
		}
		if got.source == nil || !got.source.IsLinkLocalUnicast() {
			t.Errorf("source address = %v, want a link-local address (RFC 4861 Section 4.2)", got.source)
		}
		if got.message.messageType != ndp.ICMPv6TypeRouterAdvertisement {
			t.Errorf("ICMPv6 type = %d, want %d", got.message.messageType, ndp.ICMPv6TypeRouterAdvertisement)
		}
		if got.message.code != 0 {
			t.Errorf("ICMPv6 code = %d, want 0", got.message.code)
		}
		if got.message.curHopLimit != 64 {
			t.Errorf("Cur Hop Limit = %d, want 64", got.message.curHopLimit)
		}
		if got.message.routerLifetime != 1800 {
			t.Errorf("Router Lifetime = %d, want 1800", got.message.routerLifetime)
		}

		if got.message.sourceLLA.String() != routerMAC.String() {
			t.Errorf("Source Link-layer Address option = %v, want the %s MAC %v (RFC 4861 Section 4.6.1)",
				got.message.sourceLLA, router, routerMAC)
		}

		if len(got.message.prefixes) != 1 {
			t.Fatalf("Prefix Information options = %d, want 1", len(got.message.prefixes))
		}
		pio := got.message.prefixes[0]
		if pio.prefix != prefix {
			t.Errorf("advertised prefix = %s, want %s", pio.prefix, prefix)
		}
		if !pio.onLink {
			t.Error("Prefix Information L flag is clear, want set (RFC 4861 Section 4.6.2)")
		}
		if !pio.autonomous {
			t.Error("Prefix Information A flag is clear, want set (RFC 4861 Section 4.6.2)")
		}
		if pio.validLifetime != 7200 {
			t.Errorf("Valid Lifetime = %d, want 7200", pio.validLifetime)
		}
		if pio.preferredLifetime != 3600 {
			t.Errorf("Preferred Lifetime = %d, want 3600", pio.preferredLifetime)
		}

		// The control. Stop the sender, clear its final advertisements, then
		// put one hand-built advertisement with Hop Limit 1 on the same wire.
		// The capture must report 1. Without this, a capture that always
		// reported 255 would credit the sender with a value it never set.
		sender.Stop()
		capture.drain()
		sendCraftedRA(t, router, 1)
		control, err := capture.next(5 * time.Second)
		if err != nil {
			t.Fatalf("the control advertisement did not arrive: %v", err)
		}
		if len(control.message.prefixes) != 0 {
			t.Fatalf("the control read a sender advertisement, not the crafted one")
		}
		if control.hopLimit != 1 {
			t.Errorf("control Hop Limit = %d, want 1: the capture does not read the wire, so the 255 above proves nothing",
				control.hopLimit)
		}
	})
}

// TestRASolicitedResponse sends a Router Solicitation from the host end and
// waits for the advertisement that answers it.
//
// VALIDATES: RFC 4861 Section 6.2.6. A router answers a solicitation with an
// advertisement delayed by 0 to MAX_RA_DELAY_TIME, and consecutive multicast
// advertisements stay at least MIN_DELAY_BETWEEN_RAS apart.
// PREVENTS: a router that ignores solicitations, leaving a joining host to wait
// for the next unsolicited advertisement, and prevents a router that answers
// at once and lets a solicitation flood become an advertisement flood.
//
// The advertising interface runs with IPv6 forwarding off, and the test
// asserts it. With forwarding on, the kernel joins ff02::2 for the interface
// itself, and the sender's raw socket would receive the solicitation whether or
// not the sender joined the group, so the answer would prove nothing about the
// join.
//
// RFC requirement: RFC4861-6.2.2-2 positive -- with IPv6 forwarding off on the advertising interface, so the kernel has not joined ff02::2 for it, a Router Solicitation sent to the all-routers address ff02::2 on that interface is answered with an advertisement, which reaches the sender only through the all-routers group it joined.
func TestRASolicitedResponse(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	const router, host = "zrc0", "zrc1"
	withRAVeth(t, router, host, func() {
		requireRAForwardingOff(t, router)
		capture := openRACapture(t, host)

		// The unsolicited interval is far longer than the test, so any second
		// advertisement is an answer to the solicitation and nothing else.
		const unsolicited = 10 * time.Minute
		prefix := netip.MustParsePrefix("2001:db8:a03::/64")
		sender := startSender(t, raSpecFor(router, prefix, unsolicited, unsolicited))
		defer sender.Stop()

		first, err := capture.next(10 * time.Second)
		if err != nil {
			t.Fatalf("no initial advertisement on %s within 10s: %v", host, err)
		}

		sendRouterSolicitation(t, host)

		answer, err := capture.next(ndp.MinDelayBetweenRAs + ndp.MaxRADelayTime + 5*time.Second)
		if err != nil {
			t.Fatalf("no advertisement answered the Router Solicitation: %v", err)
		}
		if answer.message.messageType != ndp.ICMPv6TypeRouterAdvertisement {
			t.Fatalf("answer ICMPv6 type = %d, want %d", answer.message.messageType, ndp.ICMPv6TypeRouterAdvertisement)
		}
		if len(answer.message.prefixes) != 1 || answer.message.prefixes[0].prefix != prefix {
			t.Errorf("answer carries prefixes %v, want %s", answer.message.prefixes, prefix)
		}

		// The rate limit is a floor, so assert the gap is not below it. The
		// timestamps are the arrival of each message, which is never earlier
		// than its send, so a measured gap above the floor proves a real gap
		// above the floor. A tolerance covers the scheduling cost of the two
		// reads, and no upper instant is asserted.
		const readTolerance = 200 * time.Millisecond
		if gap := answer.at.Sub(first.at); gap+readTolerance < ndp.MinDelayBetweenRAs {
			t.Errorf("answer arrived %s after the previous advertisement, want at least %s (RFC 4861 Section 6.2.6)",
				gap, ndp.MinDelayBetweenRAs)
		}
	})
}

// requireRAForwardingOff fails the test unless IPv6 forwarding is off on the
// advertising device, read through the same reader the doctor check uses.
//
// The solicited-response test proves the sender's all-routers join only while
// the kernel has not joined ff02::2 for the device on its own, which it does
// when forwarding is on. A forwarding state that cannot be read is a failure
// too, because the proof would then rest on an unknown.
func requireRAForwardingOff(t *testing.T, device string) {
	t.Helper()
	on, known := readIPv6Forwarding(device)
	if !known {
		t.Fatalf("cannot read net.ipv6.conf.%s.forwarding, so the all-routers join cannot be told from the kernel's own", device)
	}
	if on {
		t.Fatalf("net.ipv6.conf.%s.forwarding is on: the kernel joins ff02::2 itself, so a received solicitation would not prove the sender's join", device)
	}
}
