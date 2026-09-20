// VALIDATES: the socket address a UDP transport writes an Outbound to carries
// the link for a link-local peer, and carries no link for any other peer.
// PREVENTS: the send half of the two-forms defect. A session to fe80::1 on
// eth0 holds the link in its key and the address without one, so an address
// built from the peer alone reaches the kernel with no zone, which leaves it
// to pick a link or to refuse the write.
package transport

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
)

// TestSendKeepsTheZoneForALinkLocalPeer covers destination, the producer of
// the socket address Send writes to. The write itself is not exercised here:
// a link-local peer needs a real link with a real neighbor, which the QEMU
// functional path provides and a unit test cannot.
func TestSendKeepsTheZoneForALinkLocalPeer(t *testing.T) {
	u := &UDP{
		Bind: netip.AddrPortFrom(netip.MustParseAddr("::"), UDPPortSingleHopControl),
		Mode: api.SingleHop,
	}

	cases := []struct {
		name string
		out  Outbound
		zone string
	}{
		{
			name: "link-local peer takes the session's interface",
			out:  Outbound{To: netip.MustParseAddr("fe80::1"), Interface: "eth0"},
			zone: "eth0",
		},
		{
			name: "a zone already on the address is kept",
			out:  Outbound{To: netip.MustParseAddr("fe80::1%eth1"), Interface: "eth1"},
			zone: "eth1",
		},
		{
			name: "a routed peer is not scoped to a link",
			out:  Outbound{To: netip.MustParseAddr("2001:db8::1"), Interface: "eth0"},
			zone: "",
		},
		{
			name: "an IPv4 peer has no zone to carry",
			out:  Outbound{To: netip.MustParseAddr("203.0.113.1"), Interface: "eth0"},
			zone: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			addr := u.destination(c.out)
			if addr.Zone != c.zone {
				t.Fatalf("zone = %q, want %q", addr.Zone, c.zone)
			}
			if got := addr.IP.String(); got != c.out.To.WithZone("").String() {
				t.Fatalf("address = %s, want %s", got, c.out.To.WithZone(""))
			}
			if addr.Port != int(UDPPortSingleHopControl) {
				t.Fatalf("port = %d, want %d", addr.Port, UDPPortSingleHopControl)
			}
		})
	}
}
