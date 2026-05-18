package dhcpserver

import (
	"encoding/binary"
	"net"
	"net/netip"
	"testing"
)

func newTestServer(t *testing.T) *dhcpHandler {
	t.Helper()
	sub := subnetConfig{
		Prefix:        netip.MustParsePrefix("192.168.1.0/24"),
		Ranges:        []addressRange{{Name: "pool", Start: netip.MustParseAddr("192.168.1.100"), Stop: netip.MustParseAddr("192.168.1.200")}},
		LeaseTimeSec:  3600,
		DefaultRouter: netip.MustParseAddr("192.168.1.1"),
		DNSServers:    []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("8.8.4.4")},
		DomainName:    "home.lan",
	}
	serverIP := netip.MustParseAddr("192.168.1.1")
	return newDHCPHandler(sub, serverIP)
}

func newTestServerWithStatic(t *testing.T) *dhcpHandler {
	t.Helper()
	sub := subnetConfig{
		Prefix:        netip.MustParsePrefix("192.168.1.0/24"),
		Ranges:        []addressRange{{Name: "pool", Start: netip.MustParseAddr("192.168.1.100"), Stop: netip.MustParseAddr("192.168.1.200")}},
		LeaseTimeSec:  3600,
		DefaultRouter: netip.MustParseAddr("192.168.1.1"),
		DNSServers:    []netip.Addr{netip.MustParseAddr("8.8.8.8")},
		StaticMappings: []staticMapping{
			{
				Name: "printer",
				MAC:  net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
				IP:   netip.MustParseAddr("192.168.1.10"),
			},
		},
	}
	serverIP := netip.MustParseAddr("192.168.1.1")
	return newDHCPHandler(sub, serverIP)
}

func buildDiscover(mac net.HardwareAddr, xid uint32) []byte {
	pkt := make([]byte, 300)
	pkt[0] = opRequest
	pkt[1] = htypeEthernet
	pkt[2] = hlenEthernet
	binary.BigEndian.PutUint32(pkt[4:8], xid)
	copy(pkt[28:34], mac)
	binary.BigEndian.PutUint32(pkt[236:240], magicCookie)
	pkt[240] = optMessageType
	pkt[241] = 1
	pkt[242] = msgDiscover
	pkt[243] = optEnd
	return pkt
}

func buildRequest(mac net.HardwareAddr, xid uint32, requestedIP, serverID netip.Addr) []byte {
	pkt := make([]byte, 320)
	pkt[0] = opRequest
	pkt[1] = htypeEthernet
	pkt[2] = hlenEthernet
	binary.BigEndian.PutUint32(pkt[4:8], xid)
	copy(pkt[28:34], mac)
	binary.BigEndian.PutUint32(pkt[236:240], magicCookie)
	off := 240
	pkt[off] = optMessageType
	pkt[off+1] = 1
	pkt[off+2] = msgRequest
	off += 3
	if requestedIP.IsValid() {
		pkt[off] = optRequestedIP
		pkt[off+1] = 4
		ip4 := requestedIP.As4()
		copy(pkt[off+2:off+6], ip4[:])
		off += 6
	}
	if serverID.IsValid() {
		pkt[off] = optServerID
		pkt[off+1] = 4
		ip4 := serverID.As4()
		copy(pkt[off+2:off+6], ip4[:])
		off += 6
	}
	pkt[off] = optEnd
	return pkt
}

func buildRelease(mac net.HardwareAddr, xid uint32, clientIP netip.Addr) []byte {
	pkt := make([]byte, 300)
	pkt[0] = opRequest
	pkt[1] = htypeEthernet
	pkt[2] = hlenEthernet
	binary.BigEndian.PutUint32(pkt[4:8], xid)
	ip4 := clientIP.As4()
	copy(pkt[12:16], ip4[:])
	copy(pkt[28:34], mac)
	binary.BigEndian.PutUint32(pkt[236:240], magicCookie)
	pkt[240] = optMessageType
	pkt[241] = 1
	pkt[242] = msgRelease
	pkt[243] = optEnd
	return pkt
}

func getResponseMsgType(pkt []byte) byte {
	opts := pkt[240:]
	for i := 0; i < len(opts)-2; {
		if opts[i] == optEnd {
			break
		}
		if opts[i] == optPad {
			i++
			continue
		}
		if opts[i] == optMessageType && opts[i+1] == 1 {
			return opts[i+2]
		}
		i += 2 + int(opts[i+1])
	}
	return 0
}

func getResponseOption(pkt []byte, code byte) []byte {
	opts := pkt[240:]
	for i := 0; i < len(opts)-1; {
		if opts[i] == optEnd {
			break
		}
		if opts[i] == optPad {
			i++
			continue
		}
		l := int(opts[i+1])
		if opts[i] == code {
			return opts[i+2 : i+2+l]
		}
		i += 2 + l
	}
	return nil
}

func TestDHCPDiscoverOffer(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	discover := buildDiscover(mac, 0x12345678)
	resp := h.handle(discover)
	if resp == nil {
		t.Fatal("expected DHCPOFFER, got nil")
	}

	if resp[0] != opReply {
		t.Errorf("op = %d, want %d (BOOTREPLY)", resp[0], opReply)
	}
	if getResponseMsgType(resp) != msgOffer {
		t.Errorf("message type = %d, want %d (OFFER)", getResponseMsgType(resp), msgOffer)
	}

	yiaddr := netip.AddrFrom4([4]byte(resp[16:20]))
	if !h.subnet.Prefix.Contains(yiaddr) {
		t.Errorf("offered address %v not in subnet", yiaddr)
	}

	leaseOpt := getResponseOption(resp, optLeaseTime)
	if leaseOpt == nil {
		t.Error("missing lease-time option")
	} else {
		lt := binary.BigEndian.Uint32(leaseOpt)
		if lt != 3600 {
			t.Errorf("lease-time = %d, want 3600", lt)
		}
	}

	routerOpt := getResponseOption(resp, optRouter)
	if routerOpt == nil {
		t.Error("missing router option")
	}

	dnsOpt := getResponseOption(resp, optDNS)
	if dnsOpt == nil {
		t.Error("missing DNS option")
	}
}

func TestDHCPRequestAck(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}

	discover := buildDiscover(mac, 0xABCD)
	offer := h.handle(discover)
	if offer == nil {
		t.Fatal("no OFFER")
	}
	offeredIP := netip.AddrFrom4([4]byte(offer[16:20]))

	request := buildRequest(mac, 0xABCD, offeredIP, h.serverIP)
	ack := h.handle(request)
	if ack == nil {
		t.Fatal("no ACK")
	}

	if getResponseMsgType(ack) != msgAck {
		t.Errorf("message type = %d, want %d (ACK)", getResponseMsgType(ack), msgAck)
	}

	assignedIP := netip.AddrFrom4([4]byte(ack[16:20]))
	if assignedIP != offeredIP {
		t.Errorf("ACK yiaddr = %v, want %v", assignedIP, offeredIP)
	}

	leaseOpt := getResponseOption(ack, optLeaseTime)
	if leaseOpt == nil {
		t.Error("missing lease-time in ACK")
	}

	serverOpt := getResponseOption(ack, optServerID)
	if serverOpt == nil {
		t.Error("missing server-identifier in ACK")
	}
}

func TestDHCPStaticMapping(t *testing.T) {
	t.Parallel()

	h := newTestServerWithStatic(t)
	defer h.leases.stop()

	mac := net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	discover := buildDiscover(mac, 0x1111)
	offer := h.handle(discover)
	if offer == nil {
		t.Fatal("no OFFER for static MAC")
	}

	offeredIP := netip.AddrFrom4([4]byte(offer[16:20]))
	expectedIP := netip.MustParseAddr("192.168.1.10")
	if offeredIP != expectedIP {
		t.Errorf("static mapping: offered %v, want %v", offeredIP, expectedIP)
	}
}

func TestDHCPRelease(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}

	discover := buildDiscover(mac, 0x2222)
	offer := h.handle(discover)
	if offer == nil {
		t.Fatal("no OFFER")
	}
	offeredIP := netip.AddrFrom4([4]byte(offer[16:20]))

	request := buildRequest(mac, 0x2222, offeredIP, h.serverIP)
	ack := h.handle(request)
	if ack == nil {
		t.Fatal("no ACK")
	}

	release := buildRelease(mac, 0x2222, offeredIP)
	resp := h.handle(release)
	if resp != nil {
		t.Error("RELEASE should produce no response")
	}

	if l := h.leases.lookup(mac); l != nil {
		t.Error("lease should be gone after RELEASE")
	}
}

// RFC 2131 Section 4.3.1: server remains silent when no address available.
func TestDHCPPoolExhaustionSilent(t *testing.T) {
	t.Parallel()

	sub := subnetConfig{
		Prefix:       netip.MustParsePrefix("10.0.0.0/30"),
		Ranges:       []addressRange{{Name: "pool", Start: netip.MustParseAddr("10.0.0.1"), Stop: netip.MustParseAddr("10.0.0.2")}},
		LeaseTimeSec: 3600,
	}
	h := newDHCPHandler(sub, netip.MustParseAddr("10.0.0.1"))
	defer h.leases.stop()

	for i := range 2 {
		mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, byte(i)}
		discover := buildDiscover(mac, uint32(i+1))
		offer := h.handle(discover)
		if offer == nil {
			t.Fatalf("no OFFER for client %d", i)
		}
		offeredIP := netip.AddrFrom4([4]byte(offer[16:20]))
		req := buildRequest(mac, uint32(i+1), offeredIP, h.serverIP)
		ack := h.handle(req)
		if ack == nil {
			t.Fatalf("no ACK for client %d", i)
		}
	}

	exhaustMAC := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0xFF}
	discover := buildDiscover(exhaustMAC, 0xFF)
	resp := h.handle(discover)
	if resp != nil {
		t.Errorf("expected silence on pool exhaustion, got message type %d", getResponseMsgType(resp))
	}
}

func TestDHCPStaticOnlySubnet(t *testing.T) {
	t.Parallel()

	sub := subnetConfig{
		Prefix:       netip.MustParsePrefix("192.168.1.0/24"),
		LeaseTimeSec: 3600,
		StaticMappings: []staticMapping{
			{
				Name: "printer",
				MAC:  net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
				IP:   netip.MustParseAddr("192.168.1.10"),
			},
		},
	}
	h := newDHCPHandler(sub, netip.MustParseAddr("192.168.1.1"))
	defer h.leases.stop()

	staticMAC := net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	discover := buildDiscover(staticMAC, 0x3333)
	offer := h.handle(discover)
	if offer == nil {
		t.Fatal("expected OFFER for static MAC on static-only subnet")
	}
	offeredIP := netip.AddrFrom4([4]byte(offer[16:20]))
	if offeredIP != netip.MustParseAddr("192.168.1.10") {
		t.Errorf("static-only: offered %v, want 192.168.1.10", offeredIP)
	}

	request := buildRequest(staticMAC, 0x3333, offeredIP, h.serverIP)
	ack := h.handle(request)
	if ack == nil {
		t.Fatal("expected ACK for static MAC on static-only subnet")
	}
	if getResponseMsgType(ack) != msgAck {
		t.Errorf("expected ACK, got message type %d", getResponseMsgType(ack))
	}
	assignedIP := netip.AddrFrom4([4]byte(ack[16:20]))
	if assignedIP != offeredIP {
		t.Errorf("ACK yiaddr = %v, want %v", assignedIP, offeredIP)
	}

	unknownMAC := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	discover2 := buildDiscover(unknownMAC, 0x4444)
	resp := h.handle(discover2)
	if resp != nil {
		t.Error("expected silence for unknown MAC on static-only subnet")
	}
}

func TestDHCPInitReboot(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	reqIP := netip.MustParseAddr("192.168.1.150")

	request := buildRequest(mac, 0x5555, reqIP, netip.Addr{})
	ack := h.handle(request)
	if ack == nil {
		t.Fatal("expected ACK for INIT-REBOOT with valid subnet address")
	}
	if getResponseMsgType(ack) != msgAck {
		t.Errorf("expected ACK, got %d", getResponseMsgType(ack))
	}
	assignedIP := netip.AddrFrom4([4]byte(ack[16:20]))
	if assignedIP != reqIP {
		t.Errorf("INIT-REBOOT: yiaddr = %v, want %v", assignedIP, reqIP)
	}
}

func TestDHCPInitRebootWrongSubnet(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}

	// Wrong-subnet IP is filtered by matchesSubnet, so handler returns nil.
	// In serveMulti, no handler matches, resulting in silence.
	wrongIP := netip.MustParseAddr("10.0.0.50")
	request := buildRequest(mac, 0x6666, wrongIP, netip.Addr{})
	resp := h.handle(request)
	if resp != nil {
		t.Error("expected nil for wrong-subnet INIT-REBOOT (filtered by matchesSubnet)")
	}

	// An IP inside the subnet but outside the pool range still gets ACK
	// (commitBinding reserves it). This matches RFC 2131: server with
	// record should respond, server without should be silent.
	inSubnetIP := netip.MustParseAddr("192.168.1.50")
	request2 := buildRequest(mac, 0x6667, inSubnetIP, netip.Addr{})
	resp2 := h.handle(request2)
	if resp2 == nil {
		t.Fatal("expected ACK for in-subnet INIT-REBOOT")
	}
	if getResponseMsgType(resp2) != msgAck {
		t.Errorf("expected ACK, got %d", getResponseMsgType(resp2))
	}
}

func TestDHCPRenew(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	clientIP := netip.MustParseAddr("192.168.1.150")

	pkt := make([]byte, 300)
	pkt[0] = opRequest
	pkt[1] = htypeEthernet
	pkt[2] = hlenEthernet
	binary.BigEndian.PutUint32(pkt[4:8], 0x7777)
	ip4 := clientIP.As4()
	copy(pkt[12:16], ip4[:])
	copy(pkt[28:34], mac)
	binary.BigEndian.PutUint32(pkt[236:240], magicCookie)
	pkt[240] = optMessageType
	pkt[241] = 1
	pkt[242] = msgRequest
	pkt[243] = optEnd

	ack := h.handle(pkt)
	if ack == nil {
		t.Fatal("expected ACK for RENEWING request")
	}
	if getResponseMsgType(ack) != msgAck {
		t.Errorf("expected ACK, got %d", getResponseMsgType(ack))
	}
	ciaddr := netip.AddrFrom4([4]byte(ack[12:16]))
	if ciaddr != clientIP {
		t.Errorf("ACK ciaddr = %v, want %v (echoed from request)", ciaddr, clientIP)
	}
}

func TestMatchesSubnet(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	t.Cleanup(h.leases.stop)

	tests := []struct {
		name   string
		giaddr netip.Addr
		ciaddr netip.Addr
		reqIP  netip.Addr
		want   bool
	}{
		{"giaddr in subnet", netip.MustParseAddr("192.168.1.1"), netip.Addr{}, netip.Addr{}, true},
		{"giaddr outside", netip.MustParseAddr("10.0.0.1"), netip.Addr{}, netip.Addr{}, false},
		{"ciaddr in subnet", netip.Addr{}, netip.MustParseAddr("192.168.1.50"), netip.Addr{}, true},
		{"ciaddr outside", netip.Addr{}, netip.MustParseAddr("10.0.0.50"), netip.Addr{}, false},
		{"requestedIP in subnet", netip.Addr{}, netip.Addr{}, netip.MustParseAddr("192.168.1.100"), true},
		{"requestedIP outside", netip.Addr{}, netip.Addr{}, netip.MustParseAddr("10.0.0.100"), false},
		{"no hints (fallback true)", netip.Addr{}, netip.Addr{}, netip.Addr{}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pkt := make([]byte, 300)
			pkt[0] = opRequest
			pkt[1] = htypeEthernet
			pkt[2] = hlenEthernet
			binary.BigEndian.PutUint32(pkt[236:240], magicCookie)

			if tc.giaddr.IsValid() {
				gi := tc.giaddr.As4()
				copy(pkt[24:28], gi[:])
			}
			if tc.ciaddr.IsValid() {
				ci := tc.ciaddr.As4()
				copy(pkt[12:16], ci[:])
			}

			off := 240
			pkt[off] = optMessageType
			pkt[off+1] = 1
			pkt[off+2] = msgDiscover
			off += 3
			if tc.reqIP.IsValid() {
				pkt[off] = optRequestedIP
				pkt[off+1] = 4
				rip := tc.reqIP.As4()
				copy(pkt[off+2:off+6], rip[:])
				off += 6
			}
			pkt[off] = optEnd

			got := h.matchesSubnet(pkt)
			if got != tc.want {
				t.Errorf("matchesSubnet = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDHCPDecline(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}

	discover := buildDiscover(mac, 0x8888)
	offer := h.handle(discover)
	if offer == nil {
		t.Fatal("no OFFER")
	}
	offeredIP := netip.AddrFrom4([4]byte(offer[16:20]))

	request := buildRequest(mac, 0x8888, offeredIP, h.serverIP)
	ack := h.handle(request)
	if ack == nil {
		t.Fatal("no ACK")
	}

	decline := make([]byte, 300)
	decline[0] = opRequest
	decline[1] = htypeEthernet
	decline[2] = hlenEthernet
	binary.BigEndian.PutUint32(decline[4:8], 0x8888)
	copy(decline[28:34], mac)
	binary.BigEndian.PutUint32(decline[236:240], magicCookie)
	off := 240
	decline[off] = optMessageType
	decline[off+1] = 1
	decline[off+2] = msgDecline
	off += 3
	decline[off] = optRequestedIP
	decline[off+1] = 4
	decIP := offeredIP.As4()
	copy(decline[off+2:off+6], decIP[:])
	off += 6
	decline[off] = optEnd

	resp := h.handle(decline)
	if resp != nil {
		t.Error("DECLINE should produce no response")
	}

	otherMAC := net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x01}
	discover2 := buildDiscover(otherMAC, 0x9999)
	offer2 := h.handle(discover2)
	if offer2 == nil {
		t.Fatal("no OFFER after DECLINE")
	}
	newIP := netip.AddrFrom4([4]byte(offer2[16:20]))
	if newIP == offeredIP {
		t.Error("declined address should not be re-allocated")
	}
}

func TestMalformedPackets(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	t.Cleanup(h.leases.stop)

	tests := []struct {
		name string
		pkt  []byte
	}{
		{"too short", make([]byte, 100)},
		{"wrong opcode", func() []byte {
			p := make([]byte, 300)
			p[0] = opReply
			p[1] = htypeEthernet
			p[2] = hlenEthernet
			binary.BigEndian.PutUint32(p[236:240], magicCookie)
			p[240] = optMessageType
			p[241] = 1
			p[242] = msgDiscover
			p[243] = optEnd
			return p
		}()},
		{"bad magic cookie", func() []byte {
			p := make([]byte, 300)
			p[0] = opRequest
			p[1] = htypeEthernet
			p[2] = hlenEthernet
			binary.BigEndian.PutUint32(p[236:240], 0xDEADBEEF)
			p[240] = optMessageType
			p[241] = 1
			p[242] = msgDiscover
			p[243] = optEnd
			return p
		}()},
		{"no message type", func() []byte {
			p := make([]byte, 300)
			p[0] = opRequest
			p[1] = htypeEthernet
			p[2] = hlenEthernet
			binary.BigEndian.PutUint32(p[236:240], magicCookie)
			p[240] = optEnd
			return p
		}()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp := h.handle(tc.pkt)
			if resp != nil {
				t.Errorf("expected nil for malformed packet, got response type %d", getResponseMsgType(resp))
			}
		})
	}
}

func TestResponseAddr(t *testing.T) {
	t.Parallel()

	makeReq := func(giaddr, ciaddr netip.Addr, broadcast bool) []byte {
		pkt := make([]byte, 300)
		if giaddr.IsValid() {
			gi := giaddr.As4()
			copy(pkt[24:28], gi[:])
		}
		if ciaddr.IsValid() {
			ci := ciaddr.As4()
			copy(pkt[12:16], ci[:])
		}
		if broadcast {
			pkt[10] = 0x80
		}
		return pkt
	}
	makeResp := func(yiaddr netip.Addr) []byte {
		resp := make([]byte, 300)
		if yiaddr.IsValid() {
			yi := yiaddr.As4()
			copy(resp[16:20], yi[:])
		}
		return resp
	}

	tests := []struct {
		name     string
		req      []byte
		resp     []byte
		wantIP   string
		wantPort int
	}{
		{
			"giaddr set: relay port 67",
			makeReq(netip.MustParseAddr("10.0.0.1"), netip.Addr{}, false),
			makeResp(netip.Addr{}),
			"10.0.0.1", 67,
		},
		{
			"ciaddr set: unicast port 68",
			makeReq(netip.Addr{}, netip.MustParseAddr("192.168.1.50"), false),
			makeResp(netip.Addr{}),
			"192.168.1.50", 68,
		},
		{
			"broadcast flag: broadcast port 68",
			makeReq(netip.Addr{}, netip.Addr{}, true),
			makeResp(netip.MustParseAddr("192.168.1.100")),
			"255.255.255.255", 68,
		},
		{
			"unicast to yiaddr: port 68",
			makeReq(netip.Addr{}, netip.Addr{}, false),
			makeResp(netip.MustParseAddr("192.168.1.100")),
			"192.168.1.100", 68,
		},
		{
			"fallback broadcast",
			makeReq(netip.Addr{}, netip.Addr{}, false),
			makeResp(netip.Addr{}),
			"255.255.255.255", 68,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dst := responseAddr(tc.req, tc.resp)
			if dst.IP.String() != tc.wantIP {
				t.Errorf("IP = %s, want %s", dst.IP.String(), tc.wantIP)
			}
			if dst.Port != tc.wantPort {
				t.Errorf("Port = %d, want %d", dst.Port, tc.wantPort)
			}
		})
	}
}
