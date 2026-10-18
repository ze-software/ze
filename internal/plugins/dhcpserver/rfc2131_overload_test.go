// Design: docs/architecture/provisioning/dhcp-server.md -- RFC 2131 conformance coverage
//
// Proves RFC 2131 Section 4.1 option overload on receive: the options field is
// interpreted first, so the option overload option (52) found there decides
// whether the 'file' field and then the 'sname' field carry further options.

package dhcpserver

import (
	"encoding/binary"
	"net"
	"net/netip"
	"testing"
)

// overloadedMsg builds a BOOTREQUEST whose option bytes are spread over the
// options, file and sname fields. Each non-empty field gets an End option after
// its bytes; the rest of file and sname stays zero, which is the Pad option.
// No option is added implicitly, so a test controls every option byte.
func overloadedMsg(t *testing.T, mac net.HardwareAddr, xid uint32, options, file, sname []byte) []byte {
	t.Helper()
	if len(file) >= 128 {
		t.Fatalf("file bytes %d do not fit the 128-octet field with its End option", len(file))
	}
	if len(sname) >= 64 {
		t.Fatalf("sname bytes %d do not fit the 64-octet field with its End option", len(sname))
	}
	pkt := make([]byte, 240+len(options)+1+8)
	pkt[0] = opRequest
	pkt[1] = htypeEthernet
	pkt[2] = hlenEthernet
	binary.BigEndian.PutUint32(pkt[4:8], xid)
	copy(pkt[28:34], mac)
	if len(sname) > 0 {
		copy(pkt[snameStart:], sname)
		pkt[snameStart+len(sname)] = optEnd
	}
	if len(file) > 0 {
		copy(pkt[fileStart:], file)
		pkt[fileStart+len(file)] = optEnd
	}
	binary.BigEndian.PutUint32(pkt[236:240], magicCookie)
	copy(pkt[240:], options)
	pkt[240+len(options)] = optEnd
	return pkt
}

func cat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// TestOverloadFieldsCarryOptions places the message type (53) and requested
// address (50) of an INIT-REBOOT DHCPREQUEST in the file and sname fields under
// each overload value, and drives the packet through h.handle, the function the
// receive loop calls.
// VALIDATES: RFC 2131 Section 4.1 -- the file field is interpreted when option 52
// says 1 or 3, the sname field when it says 2 or 3, so the request is classified
// and answered with the requested address.
// PREVENTS: a reader that walks the options field alone and drops a client whose
// options spill into file or sname.
func TestOverloadFieldsCarryOptions(t *testing.T) {
	t.Parallel()

	want := netip.MustParseAddr("192.168.1.150")
	msgType := tlv(optMessageType, msgRequest)
	requested := ipOpt(optRequestedIP, want)

	for i, tc := range []struct {
		name     string
		overload byte
		file     []byte
		sname    []byte
	}{
		{"file", overloadFile, cat(msgType, requested), nil},
		{"sname", overloadSname, nil, cat(msgType, requested)},
		{"both", overloadBoth, msgType, requested},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newTestServer(t)
			defer h.leases.stop()

			mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x52, byte(i)}
			pkt := overloadedMsg(t, mac, 0x5200+uint32(i), tlv(optOverload, tc.overload), tc.file, tc.sname)

			// RFC requirement: RFC2131-4.1-15 positive -- under option 52 value 1 the file field, under 2 the sname field, and under 3 both are read for options: the message type (53) and requested address (50) placed there are found.
			if got := parseMsgType(pkt); got != msgRequest {
				t.Fatalf("message type = %d, want %d (REQUEST) read from the overloaded field", got, msgRequest)
			}
			if got := parseOptionAddr(pkt, optRequestedIP); got != want {
				t.Fatalf("requested address = %v, want %v read from the overloaded field", got, want)
			}
			// RFC requirement: RFC2131-4.1-7 positive -- the option overload option (52) in the options field is interpreted, so a DHCPREQUEST whose message type and requested address sit in the overloaded field is answered with a DHCPACK for that address.
			resp := h.handle(pkt)
			if resp == nil {
				t.Fatal("no reply to a DHCPREQUEST whose options sit in an overloaded field")
			}
			if got := getResponseMsgType(resp); got != msgAck {
				t.Fatalf("reply message type = %d, want %d (ACK)", got, msgAck)
			}
			if got := netip.AddrFrom4([4]byte(resp[16:20])); got != want {
				t.Errorf("yiaddr = %v, want %v", got, want)
			}
		})
	}
}

// TestOverloadInterpretationOrder plants the same option in more than one field
// and reads which copy wins.
// VALIDATES: RFC 2131 Section 4.1 -- options first, then file, then sname; and
// only the option 52 found in the options field decides which fields are read.
// PREVENTS: a walk in another order, or one that honors an overload option
// planted inside an overloaded field.
func TestOverloadInterpretationOrder(t *testing.T) {
	t.Parallel()

	inOptions := netip.MustParseAddr("192.168.1.170")
	inFile := netip.MustParseAddr("192.168.1.150")
	inSname := netip.MustParseAddr("192.168.1.160")
	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x53, 0x01}

	first := overloadedMsg(t, mac, 0x5301,
		cat(tlv(optMessageType, msgRequest), ipOpt(optRequestedIP, inOptions), tlv(optOverload, overloadBoth)),
		ipOpt(optRequestedIP, inFile), ipOpt(optRequestedIP, inSname))
	if got := parseOptionAddr(first, optRequestedIP); got != inOptions {
		t.Errorf("requested address = %v, want %v from the options field, interpreted first", got, inOptions)
	}

	fileFirst := overloadedMsg(t, mac, 0x5302,
		cat(tlv(optMessageType, msgRequest), tlv(optOverload, overloadBoth)),
		ipOpt(optRequestedIP, inFile), ipOpt(optRequestedIP, inSname))
	// RFC requirement: RFC2131-4.1-15 positive -- under option 52 value 3 the file field is interpreted before the sname field: an option present in both is taken from file.
	if got := parseOptionAddr(fileFirst, optRequestedIP); got != inFile {
		t.Errorf("requested address = %v, want %v from file, interpreted before sname", got, inFile)
	}

	// The options field says only file is overloaded. An option 52 planted inside
	// the file field claims sname as well; it is not the options field's overload
	// option, so sname stays unread and its requested address is not taken.
	nested := overloadedMsg(t, mac, 0x5303,
		cat(tlv(optMessageType, msgRequest), tlv(optOverload, overloadFile)),
		tlv(optOverload, overloadBoth), ipOpt(optRequestedIP, inSname))
	// RFC requirement: RFC2131-4.1-7 negative -- only the option overload option found in the options field, interpreted first, decides the overloaded fields: an option 52 inside the file field does not open the sname field, so no requested address is read from sname.
	if got := parseOptionAddr(nested, optRequestedIP); got.IsValid() {
		t.Errorf("requested address = %v read from sname, which only an option 52 inside file claimed", got)
	}
}

// TestOverloadAbsentFieldsIgnored fills file and sname with option bytes the
// options field never declares.
// VALIDATES: RFC 2131 Section 4.1 -- file and sname carry options only when the
// options field's option 52 says so.
// PREVENTS: a reader that scans file or sname unconditionally, taking a boot file
// name or server host name for options.
func TestOverloadAbsentFieldsIgnored(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	addr := netip.MustParseAddr("192.168.1.150")
	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x54, 0x01}
	body := cat(tlv(optMessageType, msgRequest), ipOpt(optRequestedIP, addr))

	noOverload := overloadedMsg(t, mac, 0x5401, nil, body, body)
	// RFC requirement: RFC2131-4.1-15 negative -- with no option overload (52) in the options field, option bytes in the file and sname fields are not interpreted: no message type is read, and the request draws no reply.
	if got := parseMsgType(noOverload); got != 0 {
		t.Errorf("message type = %d read from file or sname with no option 52", got)
	}
	if resp := h.handle(noOverload); resp != nil {
		t.Errorf("reply type %d to a request whose only message type sits in a field option 52 never declared", getResponseMsgType(resp))
	}

	fileOnly := overloadedMsg(t, mac, 0x5402,
		cat(tlv(optMessageType, msgRequest), tlv(optOverload, overloadFile)), nil, ipOpt(optRequestedIP, addr))
	// RFC requirement: RFC2131-4.1-15 negative -- under option 52 value 1 the sname field is not interpreted: its requested address is not read, and the request (no server identifier, no ciaddr) draws a DHCPNAK.
	if got := parseOptionAddr(fileOnly, optRequestedIP); got.IsValid() {
		t.Errorf("requested address = %v read from sname under overload value 1 (file only)", got)
	}
	resp := h.handle(fileOnly)
	if resp == nil {
		t.Fatal("expected a DHCPNAK, got no reply")
	}
	if got := getResponseMsgType(resp); got != msgNak {
		t.Errorf("reply message type = %d, want %d (NAK)", got, msgNak)
	}
}

// TestOverloadMalformedFields feeds truncated and runaway option lengths inside
// overloaded fields.
// VALIDATES: an option whose length runs past the end of its field is not read,
// and an option 52 of the wrong length or an unknown value opens no field.
// PREVENTS: a read across the file/sname boundary or past the packet end.
func TestOverloadMalformedFields(t *testing.T) {
	t.Parallel()

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x01}
	opts := cat(tlv(optMessageType, msgRequest), tlv(optOverload, overloadBoth))

	// file's last option claims four octets but only two remain in the field;
	// the bytes after the field boundary (the magic cookie) must not be read.
	pkt := overloadedMsg(t, mac, 0x5501, opts, nil, nil)
	pkt[fileEnd-4] = optRequestedIP
	pkt[fileEnd-3] = 4
	pkt[fileEnd-2] = 0xc0
	pkt[fileEnd-1] = 0xa8
	for i := fileStart; i < fileEnd-4; i++ {
		pkt[i] = optPad
	}
	if got := parseOptionAddr(pkt, optRequestedIP); got.IsValid() {
		t.Errorf("read %v from an option overrunning the file field", got)
	}

	for _, value := range []byte{0, 4, 255} {
		bad := overloadedMsg(t, mac, 0x5502, cat(tlv(optMessageType, msgRequest), tlv(optOverload, value)),
			ipOpt(optRequestedIP, netip.MustParseAddr("192.168.1.150")), nil)
		if got := parseOptionAddr(bad, optRequestedIP); got.IsValid() {
			t.Errorf("overload value %d opened the file field: read %v", value, got)
		}
	}
	wrongLen := overloadedMsg(t, mac, 0x5503, cat(tlv(optMessageType, msgRequest), tlv(optOverload, overloadFile, 0)),
		ipOpt(optRequestedIP, netip.MustParseAddr("192.168.1.150")), nil)
	if got := parseOptionAddr(wrongLen, optRequestedIP); got.IsValid() {
		t.Errorf("a two-octet option 52 opened the file field: read %v", got)
	}

	// A packet cut inside the fixed header reads no option at all.
	if got := parseMsgType(pkt[:200]); got != 0 {
		t.Errorf("message type = %d from a packet shorter than the options field offset", got)
	}
}
