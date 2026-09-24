//go:build integration && linux

// Design: docs/architecture/fib/fib-depth-4-srv6.md -- selected Service SID reaches the wire as IPv6 encapsulation.
package fibkernel

import (
	"encoding/binary"
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
)

func TestFIBServiceSIDEncapsulationOnWire(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		bed := newMPLSTestbed(t, h)
		link, err := h.LinkByName(mplsZeLink)
		require.NoError(t, err)
		mplsMTUReturnRoutes(t, h, link)
		sid := netip.MustParseAddr("2001:db8:1::2")
		sidBytes := sid.As16()
		entry := incomingChange{
			Action: routeaction.Add, Prefix: netip.MustParsePrefix("192.0.2.0/24"),
			NextHop: mplsNextHop, SRv6SID: sid,
			// These NLRI bits may encode a transposed SID function. They must
			// not select an MPLS push in the presence of the Service SID.
			Labels: []uint32{300},
		}
		writer := newFIBKernel(newTestBackend(h))
		writer.processEvent(makeSysribPayload([]incomingChange{entry}))
		packet := ipv4UDP(netip.MustParseAddr("192.0.2.10"), netip.MustParseAddr("192.0.2.20"), 4000, []byte("service-sid"))
		mplsMTUInject(bed, nil, packet, false)
		frame := bed.awaitForwarded("SRv6-encapsulated IPv4 packet", func(frame []byte) bool {
			return len(frame) >= ethernetHeaderLen+40 &&
				binary.BigEndian.Uint16(frame[12:14]) == unix.ETH_P_IPV6 &&
				slices.Equal(frame[ethernetHeaderLen+24:ethernetHeaderLen+40], sidBytes[:])
		})
		outer := frame[ethernetHeaderLen:]
		require.Equal(t, byte(unix.IPPROTO_ROUTING), outer[6])
		require.GreaterOrEqual(t, len(outer), 48)
		srh := outer[40:]
		require.Equal(t, byte(4), srh[2], "missing Segment Routing Header")
		require.Equal(t, byte(unix.IPPROTO_IPIP), srh[0])
		headerLength := (int(srh[1]) + 1) * 8
		require.GreaterOrEqual(t, len(srh), headerLength+len(packet))
		inner := srh[headerLength : headerLength+len(packet)]
		require.Equal(t, packet[8]-1, inner[8], "IPv4 forwarding must decrement TTL once")
		require.True(t, slices.Equal(packet[12:], inner[12:]), "encapsulation changed the inner addresses or payload")

		entry.Action = routeaction.Withdraw
		writer.processEvent(makeSysribPayload([]incomingChange{entry}))
		_, err = h.RouteGet(netip.MustParseAddr("192.0.2.20").AsSlice())
		require.Error(t, err, "withdrawal left the service route usable")
	})
}
