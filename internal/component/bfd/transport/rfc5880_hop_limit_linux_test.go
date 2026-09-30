//go:build linux

package transport

import (
	"net/netip"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/bfd/api"
)

// RFC requirement: RFC5880-9-2 positive -- the IPv6 half of "the TTL or Hop
// Count MUST be set to the maximum on transmit": a single-hop transport bound
// to an IPv6 address has IPV6_UNICAST_HOPS 255 on its socket after Start, so
// every Control packet it sends leaves with the maximum Hop Limit.
//
// VALIDATES: applySocketOptionsV6 sets the unicast Hop Limit to 255.
// PREVENTS: an IPv6 single-hop session transmitting with the kernel's default
// Hop Limit (64), which the peer's receive check discards.
func TestRFC5880SingleHopTransmitHopLimitIsMaximumIPv6(t *testing.T) {
	u := &UDP{
		Bind: netip.MustParseAddrPort("[::1]:0"),
		Mode: api.SingleHop,
	}
	if err := u.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		if err := u.Stop(); err != nil {
			t.Errorf("Stop: %v", err)
		}
	}()

	if got := rfc5880SockoptInt(t, u, unix.IPPROTO_IPV6, unix.IPV6_UNICAST_HOPS); got != 255 {
		t.Fatalf("IPV6_UNICAST_HOPS = %d, want the maximum 255", got)
	}
}
