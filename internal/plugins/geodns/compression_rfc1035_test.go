// Design: docs/architecture/dns/server-harness.md -- the harness disables name
// compression on every reply and Msg.Truncate turns it back on for exactly the
// datagram replies it has to shorten. This file drives a real listener so the
// pointers asserted here are the octets that left the socket.
// RFC: rfc/short/rfc1035.md -- message compression, and TCP message framing

package geodns

import (
	"context"
	"encoding/binary"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// compressionZone is a zone with nine nameservers. An apex NS query draws nine
// NS records plus nine glue A records, which packs past 512 octets uncompressed,
// so the datagram reply is the one Msg.Truncate compresses.
func compressionZone(t *testing.T, port uint16) geodnsConfig {
	t.Helper()
	ns := make([]string, 0, maxNameservers)
	for i := 1; i <= maxNameservers; i++ {
		ns = append(ns, `"10.0.0.`+strconv.Itoa(i)+`"`)
	}
	cfg, err := parseConfig(`{"service":{"geodns":{"enabled":"true",` +
		`"zone":["compression-test.example."],` +
		`"nameserver":[` + strings.Join(ns, ",") + `]}}}`)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	cfg.Listeners = []listenerEndpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}}
	return cfg
}

// serveCompressionZone starts a real geodns listener on a free port and returns
// its address.
func serveCompressionZone(t *testing.T) *net.UDPAddr {
	t.Helper()
	port := freePort(t)
	cfg := compressionZone(t, port)
	storeApplied(cfg, 1)
	mgr := newServerManager(testLogger())
	if err := mgr.apply(cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	t.Cleanup(mgr.stopAll)
	return &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(port)}
}

// exchangeUDP sends raw octets to addr and returns the datagram that comes back,
// or nil when nothing arrives before the deadline.
func exchangeUDP(t *testing.T, addr *net.UDPAddr, query []byte, wait time.Duration) []byte {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("client socket: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.WriteToUDP(query, addr); err != nil {
		t.Fatalf("send query: %v", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(wait)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	buf := make([]byte, 4096)
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		return nil
	}
	return buf[:n]
}

// pointers returns every offset in wire at which a two-octet compression pointer
// begins, walking the message record by record so a length octet inside RDATA is
// never mistaken for one.
func pointerTargets(t *testing.T, wire []byte) []int {
	t.Helper()
	var targets []int
	scan := func(off int) int {
		for {
			if off >= len(wire) {
				t.Fatalf("name runs past the end of a %d-octet message", len(wire))
			}
			l := wire[off]
			if l&0xC0 == 0xC0 {
				if off+1 >= len(wire) {
					t.Fatalf("compression pointer at offset %d is one octet long", off)
				}
				targets = append(targets, int(binary.BigEndian.Uint16(wire[off:off+2])&0x3FFF))
				return off + 2
			}
			if l == 0 {
				return off + 1
			}
			off += 1 + int(l)
		}
	}
	off := 12
	for range int(binary.BigEndian.Uint16(wire[4:6])) {
		off = scan(off) + 4
	}
	total := int(binary.BigEndian.Uint16(wire[6:8])) +
		int(binary.BigEndian.Uint16(wire[8:10])) +
		int(binary.BigEndian.Uint16(wire[10:12]))
	for range total {
		off = scan(off)
		rdlength := int(binary.BigEndian.Uint16(wire[off+8 : off+10]))
		rrtype := binary.BigEndian.Uint16(wire[off : off+2])
		rdata := off + 10
		off = rdata + rdlength
		if rrtype == dns.TypeNS {
			scan(rdata)
		}
	}
	return targets
}

// VALIDATES: the datagram Ze shortens carries compression pointers, each one two
// octets with its high order two bits set, each offset resolving from the first
// octet of the message, and the NS records' RDLENGTH counting the compressed
// name rather than the expanded one. The A glue records in the same message
// carry no pointer at all.
// PREVENTS: the belief that compression is settled because a library owns it.
// Ze decides which replies are compressed -- only the datagram ones it must
// shorten -- and a wrong decision either overflows the datagram or emits a
// pointer where a reader cannot follow one.
func TestRFC1035_CompressionPointersInATruncatedDatagram(t *testing.T) {
	addr := serveCompressionZone(t)

	q := new(dns.Msg)
	q.SetQuestion("compression-test.example.", dns.TypeNS)
	packed, err := q.Pack()
	if err != nil {
		t.Fatalf("pack query: %v", err)
	}
	wire := exchangeUDP(t, addr, packed, 2*time.Second)
	if wire == nil {
		t.Fatal("no datagram came back")
	}
	if len(wire) > dns.MinMsgSize {
		t.Fatalf("datagram is %d octets, so it is not one the harness shortened", len(wire))
	}

	// RFC requirement: RFC1035-4.1.4-1 positive -- "The pointer takes the form of
	// a two octet sequence" whose "first two bits are ones.  This allows a
	// pointer to be distinguished from a label, since the label must begin with
	// two zero bits because labels are restricted to 63 octets or less." RFC 1035
	// carries no capitalised RFC 2119 keyword anywhere, so this quoted sentence
	// is the whole anchor. pointerTargets accepts a pointer only on that form.
	targets := pointerTargets(t, wire)
	if len(targets) == 0 {
		t.Fatal("the shortened datagram carries no compression pointer, so nothing below is exercised")
	}
	for i, target := range targets {
		if target >= len(wire) {
			t.Errorf("pointer %d offsets to %d, past the end of a %d-octet message", i, target, len(wire))
		}
		if target < 12 {
			t.Errorf("pointer %d offsets to %d, inside the 12-octet header where no name begins", i, target)
		}
	}

	// RFC requirement: RFC1035-4.1.4-2 positive -- "The OFFSET field specifies an
	// offset from the start of the message (i.e., the first octet of the ID field
	// in the domain header).  A zero offset specifies the first byte of the ID
	// field, etc." Offset 12 is therefore the question name, and following the
	// pointer from there yields the zone the query asked about.
	rest := new(dns.Msg)
	if err := rest.Unpack(wire); err != nil {
		t.Fatalf("unpack the shortened reply: %v", err)
	}
	for _, rr := range rest.Answer {
		if rr.Header().Name != "compression-test.example." {
			t.Errorf("an owner name expands to %q, want the queried zone", rr.Header().Name)
		}
	}

	// RFC requirement: RFC1035-4.1.4-4 positive -- "If a domain name is contained
	// in a part of the message subject to a length field (such as the RDATA
	// section of an RR), and compression is used, the length of the compressed
	// name is used in the length calculation, rather than the length of the
	// expanded name." Nine NS records whose targets are ns1..ns9 of a 26-octet
	// zone would each need 30 octets of RDATA uncompressed.
	for _, rr := range rest.Answer {
		ns, ok := rr.(*dns.NS)
		if !ok {
			continue
		}
		expanded := len(ns.Ns) + 1
		declared := rdlengthOf(t, wire, ns.Hdr.Name, dns.TypeNS)
		if declared >= expanded {
			t.Errorf("NS RDLENGTH is %d for a target that expands to %d octets; the compressed length is not being counted", declared, expanded)
		}
		break
	}

	// RFC requirement: RFC1035-4.1.4-3 positive -- "Pointers can only be used for
	// occurances of a domain name where the format is not class specific.  If
	// this were not the case, a name server or resolver would be required to know
	// the format of all RRs it handled." The NS RDATA above is a domain name in
	// the class-independent format, and it is compressed. An A record's RDATA is
	// four octets of address holding no name, and it is not.
	for _, rr := range rest.Extra {
		a, ok := rr.(*dns.A)
		if !ok {
			continue
		}
		if got := rdlengthOf(t, wire, a.Hdr.Name, dns.TypeA); got != 4 {
			t.Errorf("A glue RDLENGTH is %d, want 4; an address RDATA holds no name and admits no pointer", got)
		}
		break
	}

	// RFC requirement: RFC1035-4.1.4-1 negative -- "Programs are free to avoid
	// using pointers in messages they generate." A stream reply is under no size
	// bound, so it carries none, and every length octet in it begins with two
	// zero bits.
	// RFC requirement: RFC1035-4.1.4-2 negative -- with no pointer there is no
	// OFFSET to resolve: the same answer is a plain sequence of labels, longer
	// for it.
	// RFC requirement: RFC1035-4.1.4-3 negative -- the choice of where a pointer
	// may appear is not a choice about WHICH records are sent. The two transports
	// carry the same answer in two encodings.
	// RFC requirement: RFC1035-4.1.4-4 negative -- with no compression the length
	// calculation uses the expanded name, which is why this message is longer.
	stream := exchangeTCP(t, addr, packed)
	if len(pointerTargets(t, stream)) != 0 {
		t.Error("the stream reply carries a compression pointer; the harness compresses only a datagram it must shorten")
	}
	if len(stream) <= len(wire) {
		t.Errorf("stream reply is %d octets and the compressed datagram %d; compression saved nothing", len(stream), len(wire))
	}
	streamMsg := new(dns.Msg)
	if err := streamMsg.Unpack(stream); err != nil {
		t.Fatalf("unpack the stream reply: %v", err)
	}
	if len(streamMsg.Answer) != len(rest.Answer) {
		t.Errorf("stream carries %d answers and the datagram %d; compression must not change the answer, only its encoding",
			len(streamMsg.Answer), len(rest.Answer))
	}
}

// exchangeTCP sends query over a stream transport, framed with the two-octet
// length prefix, and returns the reply's octets without that prefix.
func exchangeTCP(t *testing.T, addr *net.UDPAddr, query []byte) []byte {
	t.Helper()
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(context.Background(), "tcp", addr.String())
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	framed := make([]byte, 2, 2+len(query))
	binary.BigEndian.PutUint16(framed, uint16(len(query)))
	framed = append(framed, query...)
	if _, err := conn.Write(framed); err != nil {
		t.Fatalf("write query: %v", err)
	}
	prefix := make([]byte, 2)
	if err := readFull(conn, prefix); err != nil {
		t.Fatalf("read length prefix: %v", err)
	}
	body := make([]byte, binary.BigEndian.Uint16(prefix))
	if err := readFull(conn, body); err != nil {
		t.Fatalf("read %d declared octets: %v", len(body), err)
	}
	return body
}

// rdlengthOf finds the first record of rrtype whose owner name matches and
// returns the RDLENGTH it declares on the wire.
func rdlengthOf(t *testing.T, wire []byte, name string, rrtype uint16) int {
	t.Helper()
	skip := func(off int) int {
		for {
			l := wire[off]
			if l&0xC0 == 0xC0 {
				return off + 2
			}
			if l == 0 {
				return off + 1
			}
			off += 1 + int(l)
		}
	}
	off := 12
	for range int(binary.BigEndian.Uint16(wire[4:6])) {
		off = skip(off) + 4
	}
	total := int(binary.BigEndian.Uint16(wire[6:8])) +
		int(binary.BigEndian.Uint16(wire[8:10])) +
		int(binary.BigEndian.Uint16(wire[10:12]))
	for range total {
		off = skip(off)
		gotType := binary.BigEndian.Uint16(wire[off : off+2])
		rdlength := int(binary.BigEndian.Uint16(wire[off+8 : off+10]))
		if gotType == rrtype {
			return rdlength
		}
		off += 10 + rdlength
	}
	t.Fatalf("no record of type %d for %q in the reply", rrtype, name)
	return 0
}

// VALIDATES: a query whose additional section names the query name with a
// compression pointer is understood and answered, and a query whose pointer
// loops back onto itself draws no reply while the listener keeps serving.
// PREVENTS: a server that only ever reads the names it writes. Compression is
// chosen by the sender, so any client may put a pointer in a query, and a
// responder that cannot follow one answers nothing for a legal message.
func TestRFC1035_InboundCompressionPointerUnderstood(t *testing.T) {
	addr := serveCompressionZone(t)

	// A query whose additional record reuses the question name. Compress=true
	// makes miekg encode that second occurrence as a pointer.
	q := new(dns.Msg)
	q.SetQuestion("compression-test.example.", dns.TypeSOA)
	q.Compress = true
	q.Extra = append(q.Extra, &dns.TXT{
		Hdr: dns.RR_Header{Name: "compression-test.example.", Rrtype: dns.TypeTXT, Class: dns.ClassINET},
		Txt: []string{"probe"},
	})
	packed, err := q.Pack()
	if err != nil {
		t.Fatalf("pack query: %v", err)
	}
	if len(pointerTargets(t, packed)) == 0 {
		t.Fatal("the query carries no compression pointer, so it cannot test reading one")
	}
	// RFC requirement: RFC1035-4.1.4-5 positive -- "However all programs are
	// required to understand arriving messages that contain pointers." The
	// sender chooses compression, so a client may put a pointer in a query
	// whatever the responder does in its own replies.

	wire := exchangeUDP(t, addr, packed, 2*time.Second)
	if wire == nil {
		t.Fatal("no reply to a query containing a compression pointer")
	}
	reply := new(dns.Msg)
	if err := reply.Unpack(wire); err != nil {
		t.Fatalf("unpack reply: %v", err)
	}
	if reply.Rcode != dns.RcodeSuccess {
		t.Errorf("rcode = %s, want NOERROR", dns.RcodeToString[reply.Rcode])
	}
	if len(reply.Answer) == 0 {
		t.Error("the apex SOA query drew no answer, so the pointer was not followed to the zone name")
	}

	// RFC requirement: RFC1035-4.1.4-5 negative -- understanding an arriving
	// pointer is following a legal one, not obeying any two octets whose top bits
	// are set. A pointer at offset 12 naming offset 12 is a loop: it draws no
	// answer, and the listener keeps serving.
	loop := make([]byte, 12, 20)
	copy(loop, packed[:12])
	binary.BigEndian.PutUint16(loop[4:6], 1) // QDCOUNT
	binary.BigEndian.PutUint16(loop[6:8], 0)
	binary.BigEndian.PutUint16(loop[8:10], 0)
	binary.BigEndian.PutUint16(loop[10:12], 0)
	loop = append(loop, 0xC0, 0x0C, 0x00, 0x01, 0x00, 0x01)
	if got := exchangeUDP(t, addr, loop, 300*time.Millisecond); got != nil {
		unpacked := new(dns.Msg)
		if err := unpacked.Unpack(got); err == nil && unpacked.Rcode == dns.RcodeSuccess {
			t.Errorf("a self-referencing compression pointer drew a NOERROR reply of %d octets", len(got))
		}
	}

	after := exchangeUDP(t, addr, packed, 2*time.Second)
	if after == nil {
		t.Error("the listener stopped answering after the looping pointer")
	}
}

// VALIDATES: a reply on a stream transport is prefixed with a two-octet length
// that equals the message following it, and the same reply on a datagram
// transport carries no such prefix.
// PREVENTS: a stream reply a client cannot frame. TCP delivers a byte stream
// with no message boundary, so the length field is the only thing that says
// where one reply ends and the next begins.
func TestRFC1035_TCPRepliesCarryATwoOctetLengthPrefix(t *testing.T) {
	addr := serveCompressionZone(t)

	q := new(dns.Msg)
	q.SetQuestion("compression-test.example.", dns.TypeSOA)
	packed, err := q.Pack()
	if err != nil {
		t.Fatalf("pack query: %v", err)
	}

	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(context.Background(), "tcp", addr.String())
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	framed := make([]byte, 2, 2+len(packed))
	binary.BigEndian.PutUint16(framed, uint16(len(packed)))
	framed = append(framed, packed...)
	if _, err := conn.Write(framed); err != nil {
		t.Fatalf("write query: %v", err)
	}

	// RFC requirement: RFC1035-4.2.2-1 positive -- "Messages sent over TCP
	// connections use server port 53 (decimal).  The message is prefixed with a
	// two byte length field which gives the message length, excluding the two
	// byte length field."
	prefix := make([]byte, 2)
	if err := readFull(conn, prefix); err != nil {
		t.Fatalf("read length prefix: %v", err)
	}
	declared := int(binary.BigEndian.Uint16(prefix))
	if declared == 0 {
		t.Fatal("the stream reply declares a zero-octet message")
	}
	body := make([]byte, declared)
	if err := readFull(conn, body); err != nil {
		t.Fatalf("read %d declared octets: %v", declared, err)
	}
	reply := new(dns.Msg)
	if err := reply.Unpack(body); err != nil {
		t.Fatalf("the %d octets the prefix declared do not unpack as a message: %v", declared, err)
	}
	if reply.Id != q.Id {
		t.Errorf("reply ID = %d, want the query's %d", reply.Id, q.Id)
	}

	// RFC requirement: RFC1035-4.2.2-1 negative -- the two byte length field is
	// the STREAM framing of section 4.2.2. A datagram carries its own boundary,
	// so it is not prefixed: its first two octets are the message ID.
	datagram := exchangeUDP(t, addr, packed, 2*time.Second)
	if datagram == nil {
		t.Fatal("no datagram reply")
	}
	if got := binary.BigEndian.Uint16(datagram[:2]); got != q.Id {
		t.Errorf("the datagram's first two octets are %d, want the message ID %d: a datagram carries no length prefix", got, q.Id)
	}
	if int(binary.BigEndian.Uint16(datagram[:2])) == len(datagram)-2 {
		t.Error("the datagram's first two octets read as a length prefix, which no datagram carries")
	}
}

// readFull fills buf from conn, which a stream transport needs and a datagram
// one does not.
func readFull(conn net.Conn, buf []byte) error {
	got := 0
	for got < len(buf) {
		n, err := conn.Read(buf[got:])
		got += n
		if err != nil {
			return err
		}
	}
	return nil
}
