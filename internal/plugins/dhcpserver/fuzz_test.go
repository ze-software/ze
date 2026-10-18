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
		overloadedSeed(mac),     // option overload 3: options in file and sname
	}
}

// overloadedSeed builds a DHCPDISCOVER whose options field carries option
// overload value 3, a requested address (50) in the file field, and a server
// identifier (54) in the sname field whose length octet runs past the field,
// so the fuzzer starts from the overload walk and a boundary overrun.
func overloadedSeed(mac net.HardwareAddr) []byte {
	pkt := buildDiscover(mac, 0x52525252)
	copy(pkt[243:], []byte{optOverload, 1, overloadBoth, optEnd})
	copy(pkt[fileStart:], []byte{optRequestedIP, 4, 192, 168, 1, 150, optEnd})
	copy(pkt[fileStart-2:], []byte{optServerID, 4})
	return pkt
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

// FuzzFindOption feeds arbitrary bytes and option codes into findOption, the one
// walk every option reader uses, and checks where the answer came from.
//
// The returned slice aliases pkt, so its offset is cap(pkt) - cap(data). The
// answer MUST lie entirely inside one field: the options field, or the file or
// sname field when the options field's option overload (52) opens it. A walk
// that reads across a field boundary, past the packet end, or into a field the
// overload value did not open fails here.
//
// VALIDATES: RFC 2131 Section 4.1 field containment and overload gating under
// adversarial input; no panic for any length octet.
// PREVENTS: a hostile length octet carrying the walk from file into the magic
// cookie, or from sname into file.
func FuzzFindOption(f *testing.F) {
	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	for _, seed := range dhcpSeeds() {
		f.Add(seed, byte(optMessageType))
	}
	f.Add(overloadedSeed(mac), byte(optRequestedIP))
	f.Add(overloadedSeed(mac), byte(optServerID))

	f.Fuzz(func(t *testing.T, pkt []byte, code byte) {
		data, ok := findOption(pkt, code)
		if !ok {
			if data != nil {
				t.Fatalf("not found, yet data = %v", data)
			}
			return
		}
		offset := cap(pkt) - cap(data)
		end := offset + len(data)
		switch {
		case offset >= optionsStart:
			if end > len(pkt) {
				t.Fatalf("option data [%d:%d] runs past the packet end %d", offset, end, len(pkt))
			}
		case offset >= fileStart:
			if end > fileEnd {
				t.Fatalf("option data [%d:%d] runs past the file field end %d", offset, end, fileEnd)
			}
			if v := overloadValue(pkt[optionsStart:]); v != overloadFile && v != overloadBoth {
				t.Fatalf("option read from file under overload value %d", v)
			}
		case offset >= snameStart:
			if end > fileStart {
				t.Fatalf("option data [%d:%d] runs past the sname field end %d", offset, end, fileStart)
			}
			if v := overloadValue(pkt[optionsStart:]); v != overloadSname && v != overloadBoth {
				t.Fatalf("option read from sname under overload value %d", v)
			}
		default:
			t.Fatalf("option data at offset %d lies in the fixed header", offset)
		}
	})
}
