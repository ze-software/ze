// Design: docs/architecture/provisioning/dhcp-server.md -- segment resolution and subnet selection
// Related: segment.go -- resolveSegment and the selection these tests drive

package dhcpserver

import (
	"net"
	"net/netip"
	"testing"
)

// handle serves pkt from a segment that holds h alone, which is what one
// listener serving one subnet does: the shape the one-subnet tests drive.
func (h *dhcpHandler) handle(pkt []byte) []byte {
	if !validRequest(pkt) {
		return nil
	}
	return segment{subnets: []*dhcpHandler{h}}.serve(pkt)
}

var (
	macReservedA = net.HardwareAddr{0x02, 0, 0, 0, 0, 0x01} // reserved 192.168.50.20
	macReservedB = net.HardwareAddr{0x02, 0, 0, 0, 0, 0x02} // reserved 10.99.0.20
	macPlain     = net.HardwareAddr{0x02, 0, 0, 0, 0, 0x03} // reserved nowhere
	macOther     = net.HardwareAddr{0x02, 0, 0, 0, 0, 0x04} // reserved nowhere
)

// lanAddrs are the addresses the LAN bridge holds in sharedLAN: the
// household subnet's first, so it is the primary address.
var lanAddrs = []netip.Addr{netip.MustParseAddr("192.168.50.1"), netip.MustParseAddr("10.99.0.1")}

// sharedLANConfig is the home CPE of the spec: one shared network "lan" with
// the household subnet 192.168.50.0/24 and the device subnet 10.99.0.0/24,
// each with a reservation, and a second shared network "lab" on another
// segment. poolA is the household range.
func sharedLANConfig(poolA addressRange) serverConfig {
	return serverConfig{
		Enabled: true,
		SharedNetworks: []sharedNetwork{
			{Name: "lab", Subnets: []subnetConfig{{
				Prefix:        netip.MustParsePrefix("172.20.0.0/24"),
				Ranges:        []addressRange{{Name: "lab", Start: netip.MustParseAddr("172.20.0.100"), Stop: netip.MustParseAddr("172.20.0.199")}},
				LeaseTimeSec:  3600,
				DefaultRouter: netip.MustParseAddr("172.20.0.1"),
			}}},
			{Name: "lan", Subnets: []subnetConfig{
				{
					Prefix:         netip.MustParsePrefix("192.168.50.0/24"),
					Ranges:         []addressRange{poolA},
					LeaseTimeSec:   3600,
					DefaultRouter:  netip.MustParseAddr("192.168.50.1"),
					DNSServers:     []netip.Addr{netip.MustParseAddr("192.168.50.1")},
					StaticMappings: []staticMapping{{Name: "laptop", MAC: macReservedA, IP: netip.MustParseAddr("192.168.50.20")}},
				},
				{
					Prefix:         netip.MustParsePrefix("10.99.0.0/24"),
					Ranges:         []addressRange{{Name: "devices", Start: netip.MustParseAddr("10.99.0.50"), Stop: netip.MustParseAddr("10.99.0.199")}},
					LeaseTimeSec:   3600,
					DefaultRouter:  netip.MustParseAddr("10.99.0.1"),
					DNSServers:     []netip.Addr{netip.MustParseAddr("10.99.0.1")},
					StaticMappings: []staticMapping{{Name: "printer", MAC: macReservedB, IP: netip.MustParseAddr("10.99.0.20")}},
				},
			}},
		},
	}
}

// householdPool is the 192.168.50.0/24 range sharedLANConfig serves by default.
var householdPool = addressRange{Name: "household", Start: netip.MustParseAddr("192.168.50.100"), Stop: netip.MustParseAddr("192.168.50.199")}

// sharedLAN builds the handlers of sharedLANConfig and stops their lease
// tables when the test ends.
func sharedLAN(t *testing.T, poolA addressRange) []*dhcpHandler {
	t.Helper()
	handlers := buildHandlers(sharedLANConfig(poolA), nil)
	t.Cleanup(func() {
		for _, h := range handlers {
			h.leases.stop()
		}
	})
	return handlers
}

// lanSegment resolves the segment the LAN bridge serves, failing the test
// when it resolves none.
func lanSegment(t *testing.T, handlers []*dhcpHandler) segment {
	t.Helper()
	seg, ok := resolveSegment(handlers, netip.IPv4Unspecified(), lanAddrs)
	if !ok {
		t.Fatal("the LAN bridge's addresses resolved no segment")
	}
	return seg
}

// segmentPrefixes lists the subnets of seg in order.
func segmentPrefixes(seg segment) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(seg.subnets))
	for _, h := range seg.subnets {
		prefixes = append(prefixes, h.subnet.Prefix)
	}
	return prefixes
}

// samePrefixes reports whether got lists exactly the prefixes of want, in order.
func samePrefixes(got []netip.Prefix, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != netip.MustParsePrefix(want[i]) {
			return false
		}
	}
	return true
}

// yiaddrOf answers the address a reply offers or acknowledges.
func yiaddrOf(resp []byte) netip.Addr {
	return netip.AddrFrom4([4]byte(resp[16:20]))
}

// expectReply fails the test unless resp is a reply of msgType.
func expectReply(t *testing.T, what string, resp []byte, msgType byte) {
	t.Helper()
	if resp == nil {
		t.Fatalf("%s: no reply, want %s", what, msgTypeName(msgType))
	}
	if got := getResponseMsgType(resp); got != msgType {
		t.Fatalf("%s: reply %s, want %s", what, msgTypeName(got), msgTypeName(msgType))
	}
}

// TestSegmentForDeviceAddresses resolves the segment of a request no relay
// forwarded from the receiving device's addresses.
// VALIDATES: the segment is every subnet of each shared network holding a
// device address, the subnet of the earliest address first; a device that
// holds no address in any configured subnet resolves no segment.
// PREVENTS: a listener that answers for every configured subnet whatever
// interface the request arrived on.
func TestSegmentForDeviceAddresses(t *testing.T) {
	t.Parallel()
	handlers := sharedLAN(t, householdPool)
	any4 := netip.IPv4Unspecified()

	seg, ok := resolveSegment(handlers, any4, lanAddrs)
	if !ok || !samePrefixes(segmentPrefixes(seg), "192.168.50.0/24", "10.99.0.0/24") {
		t.Errorf("device %v: segment %v (resolved %v), want [192.168.50.0/24 10.99.0.0/24]", lanAddrs, segmentPrefixes(seg), ok)
	}

	second := []netip.Addr{netip.MustParseAddr("10.99.0.1")}
	seg, ok = resolveSegment(handlers, any4, second)
	if !ok || !samePrefixes(segmentPrefixes(seg), "10.99.0.0/24", "192.168.50.0/24") {
		t.Errorf("device %v: segment %v (resolved %v), want [10.99.0.0/24 192.168.50.0/24]", second, segmentPrefixes(seg), ok)
	}

	for _, device := range [][]netip.Addr{nil, {netip.MustParseAddr("192.0.2.1")}} {
		if seg, ok := resolveSegment(handlers, any4, device); ok {
			t.Errorf("device %v holds no address in a configured subnet, resolved segment %v", device, segmentPrefixes(seg))
		}
	}
}

// TestSegmentForGiaddr resolves the segment of a relayed request.
// VALIDATES: AC-12: giaddr selects the shared network holding a subnet that
// contains it, that subnet first, and the receiving device's addresses are
// not read; a giaddr no subnet holds resolves no segment.
// PREVENTS: a relayed request served from the receiving interface's segment.
func TestSegmentForGiaddr(t *testing.T) {
	t.Parallel()
	handlers := sharedLAN(t, householdPool)

	seg, ok := resolveSegment(handlers, netip.MustParseAddr("10.99.0.254"), lanAddrs)
	if !ok || !samePrefixes(segmentPrefixes(seg), "10.99.0.0/24", "192.168.50.0/24") {
		t.Errorf("giaddr 10.99.0.254: segment %v (resolved %v), want [10.99.0.0/24 192.168.50.0/24]", segmentPrefixes(seg), ok)
	}
	seg, ok = resolveSegment(handlers, netip.MustParseAddr("172.20.0.1"), lanAddrs)
	if !ok || !samePrefixes(segmentPrefixes(seg), "172.20.0.0/24") {
		t.Errorf("giaddr 172.20.0.1 received on the LAN: segment %v (resolved %v), want [172.20.0.0/24]", segmentPrefixes(seg), ok)
	}
	if seg, ok := resolveSegment(handlers, netip.MustParseAddr("198.51.100.1"), lanAddrs); ok {
		t.Errorf("giaddr 198.51.100.1 is in no configured subnet, resolved segment %v", segmentPrefixes(seg))
	}
}

// TestSelectReservationAcrossSubnets offers each reserved client its
// reservation, whichever subnet of the segment holds it.
// VALIDATES: AC-1 and AC-2: a DISCOVER with no hint from a MAC reserved in the
// second subnet is offered that reservation with that subnet's server
// identifier, router and mask; one reserved in the first, the first's.
// PREVENTS: the first subnet's pool claiming a client reserved in the second.
func TestSelectReservationAcrossSubnets(t *testing.T) {
	t.Parallel()
	seg := lanSegment(t, sharedLAN(t, householdPool))

	for _, tc := range []struct {
		mac    net.HardwareAddr
		yiaddr string
		router string
	}{
		{macReservedB, "10.99.0.20", "10.99.0.1"},
		{macReservedA, "192.168.50.20", "192.168.50.1"},
	} {
		offer := seg.serve(buildDiscover(tc.mac, 0x5001))
		expectReply(t, "DISCOVER from "+tc.mac.String(), offer, msgOffer)
		if got := yiaddrOf(offer); got != netip.MustParseAddr(tc.yiaddr) {
			t.Errorf("%s: yiaddr %v, want the reservation %s", tc.mac, got, tc.yiaddr)
		}
		if got := parseOptionAddr(offer, optServerID); got != netip.MustParseAddr(tc.router) {
			t.Errorf("%s: server identifier %v, want %s", tc.mac, got, tc.router)
		}
		if got := parseOptionAddr(offer, optRouter); got != netip.MustParseAddr(tc.router) {
			t.Errorf("%s: router %v, want %s", tc.mac, got, tc.router)
		}
		if got := parseOptionAddr(offer, optSubnetMask); got != netip.MustParseAddr("255.255.255.0") {
			t.Errorf("%s: mask %v, want 255.255.255.0", tc.mac, got)
		}
	}
}

// TestSelectPrimaryPoolThenOthers allocates an unreserved client from the
// subnet of the device's primary address, then from the others.
// VALIDATES: AC-3: the household pool serves first; AC-4: with it exhausted,
// the device subnet's pool serves.
// PREVENTS: an exhausted home pool leaving a client with no address while the
// segment has free ones, or the second pool serving ahead of the first.
func TestSelectPrimaryPoolThenOthers(t *testing.T) {
	t.Parallel()
	seg := lanSegment(t, sharedLAN(t, householdPool))
	offer := seg.serve(buildDiscover(macPlain, 0x5002))
	expectReply(t, "DISCOVER", offer, msgOffer)
	if got := yiaddrOf(offer); !netip.MustParsePrefix("192.168.50.0/24").Contains(got) {
		t.Errorf("yiaddr %v, want an address of the household pool", got)
	}

	one := addressRange{Name: "one", Start: netip.MustParseAddr("192.168.50.100"), Stop: netip.MustParseAddr("192.168.50.100")}
	seg = lanSegment(t, sharedLAN(t, one))
	expectReply(t, "DISCOVER that takes the last household address", seg.serve(buildDiscover(macOther, 0x5003)), msgOffer)
	offer = seg.serve(buildDiscover(macPlain, 0x5004))
	expectReply(t, "DISCOVER with the household pool exhausted", offer, msgOffer)
	if got := yiaddrOf(offer); !netip.MustParsePrefix("10.99.0.0/24").Contains(got) {
		t.Errorf("yiaddr %v, want an address of the 10.99.0.0/24 pool", got)
	}
	if got := parseOptionAddr(offer, optServerID); got != netip.MustParseAddr("10.99.0.1") {
		t.Errorf("server identifier %v, want 10.99.0.1, the subnet that offered", got)
	}
}

// TestDiscoverReservationBeatsRequestedIP sends a DISCOVER whose requested
// address lies in the other subnet of the segment.
// VALIDATES: AC-14: the reservation is offered, so a hint never ends the
// selection before the reservation is read.
// PREVENTS: a client that once held a household lease never reaching its
// reservation in the device subnet.
func TestDiscoverReservationBeatsRequestedIP(t *testing.T) {
	t.Parallel()
	seg := lanSegment(t, sharedLAN(t, householdPool))
	discover := buildMsg(msgDiscover, macReservedB, 0x5005, netip.Addr{}, 0, ipOpt(optRequestedIP, netip.MustParseAddr("192.168.50.77")))
	offer := seg.serve(discover)
	expectReply(t, "DISCOVER asking 192.168.50.77", offer, msgOffer)
	if got := yiaddrOf(offer); got != netip.MustParseAddr("10.99.0.20") {
		t.Errorf("yiaddr %v, want the reservation 10.99.0.20", got)
	}
}

// TestDiscoverRequestedIPIsCandidate sends DISCOVERs whose requested address
// is free, and held by another client.
// VALIDATES: AC-15: a free requested address in a segment subnet's pool is
// offered, even outside the home subnet; one offered to another client is
// not, and the home pool serves instead.
// PREVENTS: the requested address ignored, or handed to a second client.
func TestDiscoverRequestedIPIsCandidate(t *testing.T) {
	t.Parallel()
	seg := lanSegment(t, sharedLAN(t, householdPool))
	asked := netip.MustParseAddr("10.99.0.50")
	offer := seg.serve(buildMsg(msgDiscover, macPlain, 0x5006, netip.Addr{}, 0, ipOpt(optRequestedIP, asked)))
	expectReply(t, "DISCOVER asking 10.99.0.50", offer, msgOffer)
	if got := yiaddrOf(offer); got != asked {
		t.Errorf("yiaddr %v, want the requested %v", got, asked)
	}

	offer = seg.serve(buildMsg(msgDiscover, macOther, 0x5007, netip.Addr{}, 0, ipOpt(optRequestedIP, asked)))
	expectReply(t, "second client asking 10.99.0.50", offer, msgOffer)
	if got := yiaddrOf(offer); got == asked {
		t.Errorf("yiaddr %v was already offered to %s", got, macPlain)
	}
	if got := yiaddrOf(offer); !netip.MustParsePrefix("192.168.50.0/24").Contains(got) {
		t.Errorf("yiaddr %v, want the household pool when the requested address is taken", got)
	}
}
