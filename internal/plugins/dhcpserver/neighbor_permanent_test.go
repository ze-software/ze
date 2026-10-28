package dhcpserver

import (
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestOfferKeepsPermanentNeighbor proves replyDestination never overwrites a
// PERMANENT neighbor entry an operator wrote for yiaddr. Method: drive a
// DISCOVER through the handler, model the neighbor table with neighborRead,
// and record every neighborInstall call. A permanent entry for another MAC
// means a unicast would reach the wrong host, so the OFFER is broadcast; a
// permanent entry for chaddr already delivers the unicast, so the OFFER goes
// to yiaddr; a table that cannot be read leaves the entry unknown, so the
// OFFER is broadcast. None of the three writes an entry.
func TestOfferKeepsPermanentNeighbor(t *testing.T) {
	calls := recordNeighbors(t, nil)

	h := newTestServer(t)
	defer h.leases.stop()
	log := slogutil.DiscardLogger()

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x44}
	other := net.HardwareAddr{0x02, 0x00, 0x00, 0x00, 0x00, 0x99}
	discover := buildMsg(msgDiscover, mac, 0x4401, netip.Addr{}, 0, nil)
	offer := h.handle(discover)
	if offer == nil {
		t.Fatal("expected DHCPOFFER, got nil")
	}
	yiaddr := netip.AddrFrom4([4]byte(offer[16:20]))

	cases := []struct {
		name      string
		entry     neighborEntry
		readErr   error
		broadcast bool
	}{
		{"permanent entry for another MAC", neighborEntry{permanent: true, hardware: other}, nil, true},
		{"permanent entry for chaddr", neighborEntry{permanent: true, hardware: mac}, nil, false},
		{"unreadable table", neighborEntry{}, errors.New("netlink dump refused"), true},
	}
	for _, tc := range cases {
		var asked []netip.Addr
		neighborRead = func(iface string, addr netip.Addr) (neighborEntry, error) {
			asked = append(asked, addr)
			return tc.entry, tc.readErr
		}
		*calls = (*calls)[:0]
		dst := replyDestination("eth7", discover, offer, log)
		if tc.broadcast {
			if !dst.IP.Equal(net.IPv4bcast) || dst.Port != 68 {
				t.Errorf("%s: destination = %s, want 255.255.255.255:68", tc.name, dst)
			}
		} else if dst.IP.String() != yiaddr.String() || dst.Port != 68 {
			t.Errorf("%s: destination = %s, want %s:68", tc.name, dst, yiaddr)
		}
		if len(*calls) != 0 {
			t.Errorf("%s: neighbor entries installed %+v, want none", tc.name, *calls)
		}
		if len(asked) != 1 || asked[0] != yiaddr {
			t.Errorf("%s: neighbor table read for %v, want once for %s", tc.name, asked, yiaddr)
		}
	}
}
