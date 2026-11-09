package dhcpprobe

import (
	"encoding/binary"
	"net"
	"net/netip"
	"strings"
	"testing"
)

// VALIDATES: a request the probe builds carries exactly the fields the step
// named, at the offsets RFC 2131 Section 2 gives them, inside a valid IPv4
// header, and a reply built the way the server builds one parses back to the
// type, yiaddr, options and destination it carries.
// PREVENTS: a probe whose own framing is wrong, which would read as a DHCP
// server defect in every dhcp .ci that uses it.
// The method: build a relayed SELECTING request and read each field back;
// turn it into a reply by hand and parse it.
func TestRequestFrameFields(t *testing.T) {
	mac := net.HardwareAddr{0x02, 0, 0, 0, 0, 0x02}
	req := Request{
		Type:      MsgRequest,
		XID:       0x01020304,
		MAC:       mac,
		Broadcast: true,
		GIAddr:    netip.MustParseAddr("172.16.5.1"),
		Requested: netip.MustParseAddr("172.16.5.100"),
		ServerID:  netip.MustParseAddr("192.168.50.1"),
	}
	frame, err := buildFrame(req)
	if err != nil {
		t.Fatalf("buildFrame: %v", err)
	}
	ip := frame[ethHeaderLen : ethHeaderLen+ipv4HeaderLen]
	if got := checksum(0, ip); got != 0xffff {
		t.Errorf("IPv4 header checksum does not verify: sum %#04x", got)
	}
	udp := frame[ethHeaderLen+ipv4HeaderLen:]
	if got := binary.BigEndian.Uint16(udp[0:2]); got != portServer {
		t.Errorf("a relayed request leaves port %d, want %d", got, portServer)
	}
	msg := udp[udpHeaderLen:]
	if len(msg) < bootpMinLen {
		t.Errorf("message is %d octets, want at least %d", len(msg), bootpMinLen)
	}
	if msg[0] != opRequest || msg[3] != 1 {
		t.Errorf("op %d hops %d, want op 1 hops 1", msg[0], msg[3])
	}
	if got := binary.BigEndian.Uint32(msg[4:8]); got != req.XID {
		t.Errorf("xid %#x, want %#x", got, req.XID)
	}
	if got := binary.BigEndian.Uint16(msg[10:12]); got != flagBroadcast {
		t.Errorf("flags %#x, want the BROADCAST bit", got)
	}
	if got := netip.AddrFrom4([4]byte(msg[24:28])); got != req.GIAddr {
		t.Errorf("giaddr %s, want %s", got, req.GIAddr)
	}
	if got := netip.AddrFrom4([4]byte(msg[12:16])); !got.IsUnspecified() {
		t.Errorf("ciaddr %s, want 0.0.0.0", got)
	}
	if got := net.HardwareAddr(msg[28:34]); got.String() != mac.String() {
		t.Errorf("chaddr %s, want %s", got, mac)
	}
	options := string(msg[bootpLen+cookieLen:])
	for name, want := range map[string]string{
		"message type": string([]byte{optMsgType, 1, byte(MsgRequest)}),
		"requested":    string([]byte{optRequested, 4, 172, 16, 5, 100}),
		"server id":    string([]byte{optServerID, 4, 192, 168, 50, 1}),
	} {
		if !strings.Contains(options, want) {
			t.Errorf("the options field does not carry the %s option % x", name, want)
		}
	}

	// The server answers with op 2 and its own options, sent to the relay.
	reply := append([]byte(nil), frame...)
	rmsg := reply[ethHeaderLen+ipv4HeaderLen+udpHeaderLen:]
	rmsg[0] = opReply
	copy(rmsg[16:20], []byte{172, 16, 5, 100})
	copy(reply[ethHeaderLen+16:ethHeaderLen+20], []byte{172, 16, 5, 1})
	opts := rmsg[bootpLen+cookieLen:]
	for i := range opts {
		opts[i] = optPad
	}
	copy(opts, []byte{
		optMsgType, 1, byte(MsgAck),
		optServerID, 4, 192, 168, 50, 1,
		optRouter, 8, 172, 16, 5, 1, 172, 16, 5, 2,
		optMask, 4, 255, 255, 255, 0,
		optDNS, 8, 172, 16, 5, 53, 172, 16, 5, 54,
		optLease, 4, 0, 0, 14, 16,
		optEnd,
	})
	got, ok := parseReply(reply)
	if !ok {
		t.Fatal("parseReply refused the reply")
	}
	want := Reply{
		Type:     MsgAck,
		XID:      req.XID,
		YIAddr:   netip.MustParseAddr("172.16.5.100"),
		ServerID: netip.MustParseAddr("192.168.50.1"),
		Router:   netip.MustParseAddr("172.16.5.1"),
		Mask:     netip.MustParseAddr("255.255.255.0"),
		DNS:      netip.MustParseAddr("172.16.5.53"),
		DNSCount: 2,
		LeaseSec: 3600,
		Dest:     netip.MustParseAddrPort("172.16.5.1:67"),
	}
	want.LeaseCarried = true
	if got != want {
		t.Errorf("parseReply = %+v, want %+v", got, want)
	}

	// The probe's own request is a BOOTREQUEST, never read as a reply.
	if _, ok := parseReply(frame); ok {
		t.Error("parseReply read the probe's own BOOTREQUEST as a reply")
	}
}

// VALIDATES: an option whose length runs past the options field ends the
// walk instead of reading past the frame.
// PREVENTS: a probe that panics, or invents a field, on a truncated reply.
func TestReplyTruncatedOption(t *testing.T) {
	frame, err := buildFrame(Request{Type: MsgDiscover, XID: 7, MAC: net.HardwareAddr{2, 0, 0, 0, 0, 1}, Broadcast: true})
	if err != nil {
		t.Fatalf("buildFrame: %v", err)
	}
	msg := frame[ethHeaderLen+ipv4HeaderLen+udpHeaderLen:]
	msg[0] = opReply
	opts := msg[bootpLen+cookieLen:]
	copy(opts, []byte{optMsgType, 1, byte(MsgOffer), optServerID, 200, 10})
	frame = frame[:len(frame)-len(opts)+6]
	got, ok := parseReply(frame)
	if !ok {
		t.Fatal("parseReply refused a reply whose message type was readable")
	}
	if got.ServerID.IsValid() {
		t.Errorf("server identifier %s read from an option that runs past the field", got.ServerID)
	}
}

// VALIDATES: ParseStep reads each verb, defaults the BROADCAST bit from
// ciaddr, and refuses an exchange that names no expectation or an unknown key.
// PREVENTS: a .ci whose typo silently drops an assertion.
func TestParseStep(t *testing.T) {
	step, err := ParseStep("x,renew,type=request,mac=02-00-00-00-00-03,ciaddr=192.168.50.30,expect=nak,sid=192.168.50.1")
	if err != nil {
		t.Fatalf("ParseStep: %v", err)
	}
	ex := step.Exchange
	if step.Kind != StepExchange || ex.Request.Type != MsgRequest || ex.Want.Type != MsgNak {
		t.Errorf("parsed %+v", step)
	}
	if ex.Request.Broadcast {
		t.Error("a request with ciaddr set asked for a broadcast")
	}
	if ex.Want.ServerID != netip.MustParseAddr("192.168.50.1") {
		t.Errorf("sid %s", ex.Want.ServerID)
	}
	silent, err := ParseStep("x,quiet,type=request,mac=02-00-00-00-00-04,requested=192.168.50.77,expect=none")
	if err != nil {
		t.Fatalf("ParseStep: %v", err)
	}
	if !silent.Exchange.Silent || !silent.Exchange.Request.Broadcast {
		t.Errorf("parsed %+v", silent)
	}
	counted, err := ParseStep("x,short,type=discover,mac=02-00-00-00-00-05,expect=offer,dns-count=0,lease=60")
	if err != nil {
		t.Fatalf("ParseStep: %v", err)
	}
	cx := counted.Exchange
	if !cx.DNSCountChecked || cx.Want.DNSCount != 0 || !cx.LeaseChecked || cx.Want.LeaseSec != 60 {
		t.Errorf("parsed %+v", counted)
	}
	// A reply carrying one option 6 address and a 3600-second lease fails
	// both checks; one carrying neither option fails the lease check alone.
	if err := cx.Check(Reply{Type: MsgOffer, DNSCount: 1, LeaseSec: 3600, LeaseCarried: true}); err == nil {
		t.Error("Check accepted option 6 and lease 3600 against dns-count=0,lease=60")
	}
	if err := cx.Check(Reply{Type: MsgOffer}); err == nil {
		t.Error("Check accepted a reply with no option 51 against lease=60")
	}
	if err := cx.Check(Reply{Type: MsgOffer, LeaseSec: 60, LeaseCarried: true}); err != nil {
		t.Errorf("Check refused the reply it names: %v", err)
	}
	for _, bad := range []string{
		"x,a,type=discover,mac=02-00-00-00-00-01,expect=offer,dns-count=-1",
		"x,a,type=discover,mac=02-00-00-00-00-01,expect=offer,lease=4294967296",
		"x,a,type=discover,mac=02-00-00-00-00-01",
		"x,a,type=discover,mac=02-00-00-00-00-01,expect=offer,yiadr=10.0.0.1",
		"x,a,mac=02-00-00-00-00-01,expect=offer",
		"addr-add,10.0.0.1",
		"frobnicate",
	} {
		if _, err := ParseStep(bad); err == nil {
			t.Errorf("ParseStep(%q) accepted", bad)
		}
	}
}
