package dnsserver

import (
	"net"
	"net/netip"
	"strconv"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// recursiveQuery returns an RD=1 query for name, carrying an OPT that offers
// size octets when size is non-zero.
func recursiveQuery(name string, size uint16) *dns.Msg {
	q := new(dns.Msg)
	q.SetQuestion(name, dns.TypeA)
	if size != 0 {
		q.SetEdns0(size, false)
	}
	return q
}

// VALIDATES: every reply Recursive writes has QR=1, RA=1, AA=0, Z=0 and AD=0,
// and carries the query's CD and RD, whatever the answer func set: fn sets
// AA, AD and Z, clears RA, and flips CD, and none of it reaches the wire.
// METHOD: drive the handler with a fake writer, once with CD clear and once
// with CD set, and read the written message.
// PREVENTS: a resolver answer func claiming authority (AA), validation (AD),
// or no recursion (RA=0), or dropping the CD bit a security-aware stub set.
func TestRecursiveShape(t *testing.T) {
	for _, cd := range []bool{false, true} {
		q := recursiveQuery("example.test.", 0)
		q.CheckingDisabled = cd

		var before *dns.Msg
		handler := Recursive(nil, func(msg, r *dns.Msg, p Peer) bool {
			before = msg.Copy()
			msg.Authoritative = true
			msg.AuthenticatedData = true
			msg.Zero = true
			msg.RecursionAvailable = false
			msg.CheckingDisabled = !cd
			msg.Rcode = dns.RcodeServerFailure
			return true
		}, func(Peer, int) { t.Errorf("cd=%v: a standard query was reported rejected", cd) })

		fw := &fakeResponseWriter{}
		handler(fw, q)

		if before == nil {
			t.Fatalf("cd=%v: fn was not called", cd)
		}
		if !before.RecursionAvailable {
			t.Errorf("cd=%v: fn saw RA=0, want the recursive shape applied before fn", cd)
		}
		got := fw.written
		if got == nil {
			t.Fatalf("cd=%v: no reply written", cd)
		}
		if !got.Response {
			t.Errorf("cd=%v: QR=0, want 1", cd)
		}
		if !got.RecursionAvailable {
			t.Errorf("cd=%v: RA=0, want 1 on every resolver reply", cd)
		}
		if got.Authoritative {
			t.Errorf("cd=%v: AA=1, want 0: a forwarding resolver is no authority", cd)
		}
		if got.AuthenticatedData {
			t.Errorf("cd=%v: AD=1, want 0: this resolver validates nothing", cd)
		}
		if got.Zero {
			t.Errorf("cd=%v: Z=1, want 0", cd)
		}
		if got.CheckingDisabled != cd {
			t.Errorf("cd=%v: CD=%v, want the query's CD copied", cd, got.CheckingDisabled)
		}
		if !got.RecursionDesired {
			t.Errorf("cd=%v: RD=0, want the query's RD copied", cd)
		}
		if got.Rcode != dns.RcodeServerFailure {
			t.Errorf("cd=%v: rcode %s, want the SERVFAIL fn chose", cd, dns.RcodeToString[got.Rcode])
		}
	}

	dropped := Recursive(nil, func(msg, r *dns.Msg, p Peer) bool { return false }, nil)
	fw := &fakeResponseWriter{}
	dropped(fw, recursiveQuery("example.test.", 0))
	if fw.written != nil {
		t.Error("a reply was written for a query the answer func dropped (send=false), want none")
	}

	guard, logs := swapQueryGuard(t)
	panicking := Recursive(nil, func(msg, r *dns.Msg, p Peer) bool { panic("boom") }, nil)
	fw = &fakeResponseWriter{}
	panicking(fw, recursiveQuery("example.test.", 0))
	requireQueryContained(t, guard, logs, taskRecursiveQuery)
	if fw.written != nil {
		t.Error("a reply was written for a query whose answer func panicked, want none")
	}
}

// VALIDATES: a query whose opcode is not QUERY draws Not Implemented from
// Recursive without reaching the answer func, in the recursive shape (RA=1,
// AA=0) (AC-26).
// METHOD: send IQUERY, STATUS, NOTIFY and UPDATE through a handler whose
// answer func fails the test if called, and whose rejected func must be
// told of exactly one NOTIMP reply.
// PREVENTS: an inverse query or an update reaching the resolution path, where
// it would be forwarded or answered as if Ze implemented it.
func TestRecursiveOpcode(t *testing.T) {
	for _, opcode := range []int{dns.OpcodeIQuery, dns.OpcodeStatus, dns.OpcodeNotify, dns.OpcodeUpdate} {
		q := recursiveQuery("example.test.", 0)
		q.Opcode = opcode
		var rejected []int
		handler := Recursive(nil, func(msg, r *dns.Msg, p Peer) bool {
			t.Errorf("opcode %s reached the answer func", dns.OpcodeToString[opcode])
			return true
		}, func(_ Peer, rcode int) { rejected = append(rejected, rcode) })
		fw := &fakeResponseWriter{}
		handler(fw, q)
		if len(rejected) != 1 || rejected[0] != dns.RcodeNotImplemented {
			t.Errorf("opcode %s: rejected reported %v, want one NOTIMP", dns.OpcodeToString[opcode], rejected)
		}
		if fw.written == nil {
			t.Fatalf("opcode %s: no reply written", dns.OpcodeToString[opcode])
		}
		if fw.written.Rcode != dns.RcodeNotImplemented {
			t.Errorf("opcode %s: rcode %s, want NOTIMP", dns.OpcodeToString[opcode], dns.RcodeToString[fw.written.Rcode])
		}
		if !fw.written.RecursionAvailable {
			t.Errorf("opcode %s: RA=0, want the recursive shape", dns.OpcodeToString[opcode])
		}
		if fw.written.Authoritative {
			t.Errorf("opcode %s: AA=1, want 0", dns.OpcodeToString[opcode])
		}
	}
}

// VALIDATES: AC-13 and AC-14. A reply to an EDNS query carries an OPT that
// advertises RecursiveUDPSize with DO clear; a datagram reply larger than the
// smaller of the client's offer and RecursiveUDPSize is cut to it with TC=1;
// the same answer on a stream is written whole; and a listener built with
// Options.UDPSize reads a 1232-octet query that the default 512-octet read
// buffer would cut short.
// METHOD: fake writers for the bound, then a real loopback Manager that
// exchanges a padded query with and without Options.UDPSize.
// PREVENTS: a resolver reply past the fragmentation-safe size, a reply
// without the OPT the client asked for, or a listener that drops every large
// EDNS query.
func TestRecursiveUDPSize(t *testing.T) {
	handler := Recursive(nil, answerN(200), nil)

	for _, offer := range []uint16{1232, 4096} {
		w := udpWriter()
		handler(w, recursiveQuery("big.example.test.", offer))
		got := w.written
		if got == nil {
			t.Fatalf("offer %d: no reply written", offer)
		}
		opt := got.IsEdns0()
		if opt == nil {
			t.Fatalf("offer %d: reply carries no OPT, want one answering the query's", offer)
		}
		if opt.UDPSize() != RecursiveUDPSize {
			t.Errorf("offer %d: reply OPT advertises %d, want %d", offer, opt.UDPSize(), RecursiveUDPSize)
		}
		if opt.Do() {
			t.Errorf("offer %d: reply OPT has DO=1, want 0 from a resolver that is not security-aware", offer)
		}
		if !got.Truncated {
			t.Errorf("offer %d: TC=0 on a reply that cannot fit, want 1", offer)
		}
		if n := packedLen(t, got); n > RecursiveUDPSize {
			t.Errorf("offer %d: datagram reply is %d octets, want at most %d", offer, n, RecursiveUDPSize)
		}
	}

	w := udpWriter()
	handler(w, recursiveQuery("big.example.test.", 0))
	if w.written.IsEdns0() != nil {
		t.Error("reply to a query without OPT carries one, want none (RFC 6891 Section 6.1.1)")
	}
	if n := packedLen(t, w.written); n > dns.MinMsgSize {
		t.Errorf("reply to a plain query is %d octets, want at most %d", n, dns.MinMsgSize)
	}

	s := tcpWriter()
	handler(s, recursiveQuery("big.example.test.", 1232))
	if s.written.Truncated {
		t.Error("stream reply TC=1, want the whole answer")
	}
	if len(s.written.Answer) != 200 {
		t.Errorf("stream reply carries %d records, want 200", len(s.written.Answer))
	}

	if answered := padded1232Answered(t, Options{UDPSize: RecursiveUDPSize}); !answered {
		t.Error("listener with Options.UDPSize did not answer a 1232-octet query")
	}
	if answered := padded1232Answered(t, Options{}); answered {
		t.Error("listener without Options.UDPSize answered a 1232-octet query; the test cannot tell the option from the default")
	}
}

// padded1232Answered binds a loopback listener with opts and reports whether a
// UDP query padded to 1232 octets drew a reply.
func padded1232Answered(t *testing.T, opts Options) bool {
	t.Helper()
	port := freePort(t)
	mgr := New(testLogger(), Recursive(nil, func(msg, r *dns.Msg, p Peer) bool { return true }, nil), opts)
	if err := mgr.reconcile(true, Listeners{Plain: []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}}}, BindLenient); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	defer mgr.Stop()

	q := recursiveQuery("pad.example.test.", RecursiveUDPSize)
	opt := q.IsEdns0()
	opt.Option = append(opt.Option, &dns.EDNS0_PADDING{Padding: make([]byte, 0)})
	pad := RecursiveUDPSize - packedLen(t, q)
	opt.Option[0] = &dns.EDNS0_PADDING{Padding: make([]byte, pad)}
	if n := packedLen(t, q); n != RecursiveUDPSize {
		t.Fatalf("padded query is %d octets, want %d", n, RecursiveUDPSize)
	}

	c := &dns.Client{Net: "udp", Timeout: time.Second, UDPSize: RecursiveUDPSize}
	resp, _, err := c.Exchange(q, net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port))))
	if err != nil {
		return false
	}
	// A query cut short by the read buffer does not unpack, and miekg's
	// server answers it FORMERR without calling the handler, so only NOERROR
	// (what the handler above answers) shows the query was read whole.
	return resp.Rcode == dns.RcodeSuccess
}

// TestRecursiveBadVersion proves an EDNS query of a version this service does
// not implement draws BADVERS, as RFC 6891 Section 6.1.3 requires, without
// reaching the answer func. Method: send queries whose OPT carries version 1
// and 255 through a handler whose answer func fails the test if called, pack
// the reply as it goes on the wire, and read back the extended RCODE and the
// reply OPT's version; a version 0 query must still reach the answer func.
// PREVENTS: a query of an unknown EDNS version answered as if version 0 were
// what the requestor asked for.
func TestRecursiveBadVersion(t *testing.T) {
	for _, version := range []uint8{1, 255} {
		q := recursiveQuery("example.test.", 1232)
		q.IsEdns0().SetVersion(version)
		var rejected []int
		handler := Recursive(nil, func(msg, r *dns.Msg, p Peer) bool {
			t.Errorf("EDNS version %d reached the answer func", version)
			return true
		}, func(_ Peer, rcode int) { rejected = append(rejected, rcode) })
		fw := &fakeResponseWriter{}
		handler(fw, q)
		if fw.written == nil {
			t.Fatalf("EDNS version %d: no reply written", version)
		}
		wire, err := fw.written.Pack()
		if err != nil {
			t.Fatalf("EDNS version %d: pack reply: %v", version, err)
		}
		reply := new(dns.Msg)
		if err := reply.Unpack(wire); err != nil {
			t.Fatalf("EDNS version %d: unpack reply: %v", version, err)
		}
		if reply.Rcode != dns.RcodeBadVers {
			t.Errorf("EDNS version %d: wire rcode %d, want BADVERS (16)", version, reply.Rcode)
		}
		opt := reply.IsEdns0()
		if opt == nil {
			t.Fatalf("EDNS version %d: reply carries no OPT, so BADVERS cannot be expressed", version)
		}
		if opt.Version() != 0 {
			t.Errorf("EDNS version %d: reply OPT version %d, want 0, the version this service implements", version, opt.Version())
		}
		if !reply.RecursionAvailable {
			t.Errorf("EDNS version %d: RA=0, want the recursive shape", version)
		}
		if len(rejected) != 1 || rejected[0] != dns.RcodeBadVers {
			t.Errorf("EDNS version %d: rejected reported %v, want one BADVERS", version, rejected)
		}
	}

	q := recursiveQuery("example.test.", 1232)
	reached := false
	handler := Recursive(nil, func(msg, r *dns.Msg, p Peer) bool {
		reached = true
		return true
	}, nil)
	handler(&fakeResponseWriter{}, q)
	if !reached {
		t.Errorf("EDNS version 0 query did not reach the answer func")
	}
}

// TestRecursiveAccept pins every branch of the recursive header filter.
// Goal: a response is ignored, a header claiming sections the library would
// unpack at length is rejected, and a query the default filter would answer
// unshaped (a question count other than one, an opcode other than QUERY)
// reaches the handler. Method: one header per bound, at and one past it.
func TestRecursiveAccept(t *testing.T) {
	opcode := func(op int) uint16 { return uint16(op) << 11 }
	cases := []struct {
		name   string
		header dns.Header
		want   dns.MsgAcceptAction
	}{
		{"normal query", dns.Header{Qdcount: 1}, dns.MsgAccept},
		{"no question", dns.Header{Qdcount: 0}, dns.MsgAccept},
		{"two questions", dns.Header{Qdcount: 2}, dns.MsgAccept},
		{"questions at the bound", dns.Header{Qdcount: recursiveQuestionsMax}, dns.MsgAccept},
		{"questions past the bound", dns.Header{Qdcount: recursiveQuestionsMax + 1}, dns.MsgReject},
		{"response", dns.Header{Bits: headerQR, Qdcount: 1}, dns.MsgIgnore},
		{"response past every bound", dns.Header{Bits: headerQR, Qdcount: 1000, Ancount: 9}, dns.MsgIgnore},
		{"opcode STATUS", dns.Header{Bits: opcode(dns.OpcodeStatus), Qdcount: 1}, dns.MsgAccept},
		{"opcode NOTIFY", dns.Header{Bits: opcode(dns.OpcodeNotify), Qdcount: 1}, dns.MsgAccept},
		{"one answer record", dns.Header{Qdcount: 1, Ancount: 1}, dns.MsgAccept},
		{"two answer records", dns.Header{Qdcount: 1, Ancount: 2}, dns.MsgReject},
		{"one authority record", dns.Header{Qdcount: 1, Nscount: 1}, dns.MsgAccept},
		{"two authority records", dns.Header{Qdcount: 1, Nscount: 2}, dns.MsgReject},
		{"two additional records", dns.Header{Qdcount: 1, Arcount: 2}, dns.MsgAccept},
		{"three additional records", dns.Header{Qdcount: 1, Arcount: 3}, dns.MsgReject},
	}
	for _, c := range cases {
		if got := RecursiveAccept(c.header); got != c.want {
			t.Errorf("%s: RecursiveAccept(%+v) = %v, want %v", c.name, c.header, got, c.want)
		}
	}
}
