package dhcpserver

import (
	"net"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// FuzzDHCPHandle feeds arbitrary bytes into the two receive-path consumers of a
// DHCP datagram: h.handle (the decoder) and logExchange (which re-parses the
// request for logging). Bytes arrive as on-link, unauthenticated UDP. Driving
// both consumers -- rather than the sub-parsers below their 244-byte guard --
// matches how serveMulti feeds them in production (register.go:202,208).
//
// A fresh handler is built per iteration so pool/lease state does not
// accumulate across inputs (R-2). The decoder must never panic, any non-nil
// reply must be a BOOTREPLY (resp[0] == opReply), and logExchange on the same
// input must never panic. Seed corpus covers zero-length, truncated (<244),
// exactly-244, oversized, and malformed option streams.
//
// VALIDATES: handle + logExchange bounds under adversarial input (AC-3).
// PREVENTS: regression where a future edit drops a bound and a crafted packet
// or option stream panics on the on-link UDP path.
func FuzzDHCPHandle(f *testing.F) {
	for _, seed := range dhcpSeeds() {
		f.Add(seed)
	}

	serverIP := netip.MustParseAddr("192.168.1.1")
	serverIPs := []netip.Addr{serverIP}

	f.Fuzz(func(t *testing.T, data []byte) {
		h := newFuzzDHCPHandler()
		defer h.leases.stop()

		resp := h.handle(data)
		if resp != nil {
			if len(resp) == 0 {
				t.Fatal("accepted packet: non-nil but empty reply")
			}
			if resp[0] != opReply {
				t.Fatalf("accepted packet: reply op = %d, want %d (BOOTREPLY)", resp[0], opReply)
			}
		}
		// logExchange re-parses the request behind its own len guard; it must
		// tolerate the same arbitrary input without panicking.
		logExchange(slogutil.DiscardLogger(), data, resp, serverIPs)
	})
}

// newFuzzDHCPHandler builds a handler over a small IPv4 pool, matching the
// in-package test shim (handler_test.go newTestServer) without a *testing.T.
func newFuzzDHCPHandler() *dhcpHandler {
	sub := subnetConfig{
		Prefix:        netip.MustParsePrefix("192.168.1.0/24"),
		Ranges:        []addressRange{{Name: "pool", Start: netip.MustParseAddr("192.168.1.100"), Stop: netip.MustParseAddr("192.168.1.200")}},
		LeaseTimeSec:  3600,
		DefaultRouter: netip.MustParseAddr("192.168.1.1"),
	}
	return newDHCPHandler(sub, netip.MustParseAddr("192.168.1.1"), pxeConfig{})
}

// dhcpSeeds returns valid and malformed DHCP datagrams for the fuzzer, reusing
// the in-package builders for the deep accept paths (DISCOVER, REQUEST).
func dhcpSeeds() [][]byte {
	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	discover := buildDiscover(mac, 0x12345678)
	request := buildRequest(mac, 0x12345678, netip.MustParseAddr("192.168.1.150"), netip.MustParseAddr("192.168.1.1"))

	return [][]byte{
		discover,                // valid DISCOVER (reaches buildReply)
		request,                 // valid REQUEST (INIT-REBOOT, reaches commitBinding)
		discover[:minPacketLen], // exactly minPacketLen (244) valid DISCOVER
		{},                      // zero-length
		make([]byte, 100),       // truncated below minPacketLen (dropped)
		make([]byte, 1500),      // oversized, all-zero (bad magic cookie, dropped)
		malformedOptions(mac),   // valid header, cookie, then a garbage option stream
	}
}

// malformedOptions builds a 300-byte packet with a valid fixed header and magic
// cookie but an option field of oversized/truncated option lengths, exercising
// the option loop in parseMsgType/parseOptionAddr without tripping the header
// guards.
func malformedOptions(mac net.HardwareAddr) []byte {
	pkt := make([]byte, 300)
	pkt[0] = opRequest
	pkt[1] = htypeEthernet
	pkt[2] = hlenEthernet
	copy(pkt[28:34], mac)
	// magic cookie so handle proceeds past the cookie guard into option parsing.
	pkt[236], pkt[237], pkt[238], pkt[239] = 0x63, 0x82, 0x53, 0x63
	// option code 53 (message type) claiming length 200 (overruns) then trailing
	// codes with runaway lengths.
	pkt[240] = optMessageType
	pkt[241] = 200
	pkt[299] = 0xff
	return pkt
}
