// Design: docs/architecture/dns/server-harness.md -- the harness owns the single
// wire write. RFC 1035 section 4.2.1's datagram size bound and the Not
// Implemented reply are pinned here, at the one place both responders funnel
// through.
// RFC: rfc/short/rfc1035.md -- UDP message size, the TC bit, inverse queries

package dnsserver

import (
	"bytes"
	"errors"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// rfc1035Writer is a dns.ResponseWriter that captures the reply and reports a
// caller-chosen remote address, which is what tells the harness a datagram
// transport from a stream one.
type rfc1035Writer struct {
	dns.ResponseWriter
	remote  net.Addr
	written *dns.Msg
	// writeErr is what WriteMsg returns, so a test can drive the transport
	// refusing the reply. Nil is a write that succeeded.
	writeErr error
}

func (w *rfc1035Writer) WriteMsg(m *dns.Msg) error { w.written = m; return w.writeErr }
func (w *rfc1035Writer) RemoteAddr() net.Addr      { return w.remote }

func udpWriter() *rfc1035Writer {
	return &rfc1035Writer{remote: &net.UDPAddr{IP: net.ParseIP("192.0.2.1"), Port: 5353}}
}

func tcpWriter() *rfc1035Writer {
	return &rfc1035Writer{remote: &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 5353}}
}

// answerN returns an AnswerFunc that appends n A records to the reply. Fifty
// records pack to about 1400 octets uncompressed and about 800 compressed, so
// n=50 exceeds the 512-octet bound either way and n=3 stays well under it.
func answerN(n int) AnswerFunc {
	return func(msg, r *dns.Msg, p Peer) bool {
		for i := range n {
			msg.Answer = append(msg.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   net.IPv4(10, 0, byte(i/256), byte(i%256)),
			})
		}
		return true
	}
}

func questionFor(name string) *dns.Msg {
	q := new(dns.Msg)
	q.SetQuestion(dns.Fqdn(name), dns.TypeA)
	return q
}

// packedLen returns the wire length of msg, which is the octet count RFC 1035
// section 4.2.1 bounds. Asserting on it rather than on the record count is what
// makes the bound test measure the obligation instead of an implementation
// detail.
func packedLen(t *testing.T, msg *dns.Msg) int {
	t.Helper()
	wire, err := msg.Pack()
	if err != nil {
		t.Fatalf("pack reply: %v", err)
	}
	return len(wire)
}

// VALIDATES: a reply that would exceed 512 octets on a datagram transport is
// shortened to fit and carries TC, while a reply that already fits goes out
// whole with TC clear. Both halves run the same answer func through the same
// handler and differ only in how many records the reply carries, so a TC bit
// hard-coded either way fails one of them.
// PREVENTS: an oversized UDP answer going out whole with TC clear, which leaves
// a resolver caching a silently short answer and never retrying over TCP. Also
// a small answer marked truncated, which forces a needless TCP retry.
func TestRFC1035_UDPReplyBoundedAndTruncated(t *testing.T) {
	t.Parallel()

	over := udpWriter()
	Authoritative(nil, answerN(50), nil)(over, questionFor("big.test"))
	if over.written == nil {
		t.Fatal("no reply written for the oversized query")
	}
	// RFC requirement: RFC1035-4.2.1-1 positive -- "Messages carried by UDP are
	// restricted to 512 bytes (not counting the IP or UDP headers)."
	if n := packedLen(t, over.written); n > dns.MinMsgSize {
		t.Errorf("UDP reply packs to %d octets, want at most %d", n, dns.MinMsgSize)
	}
	// RFC requirement: RFC1035-2.3.4-2 positive -- the size limit for "UDP
	// messages" is "512 octets or less".
	if n := packedLen(t, over.written); n > dns.MinMsgSize {
		t.Errorf("UDP reply exceeds the section 2.3.4 size limit: %d octets", n)
	}
	// RFC requirement: RFC1035-4.2.1-2 positive -- "Longer messages are
	// truncated and the TC bit is set in the header."
	if !over.written.Truncated {
		t.Error("oversized UDP reply has TC clear, want TC set")
	}
	if len(over.written.Answer) >= 50 {
		t.Errorf("oversized UDP reply kept all %d answers; nothing was truncated", len(over.written.Answer))
	}
	if len(over.written.Answer) == 0 {
		t.Error("oversized UDP reply kept no answer at all; truncation must drop only the records that do not fit")
	}

	under := udpWriter()
	Authoritative(nil, answerN(3), nil)(under, questionFor("small.test"))
	if under.written == nil {
		t.Fatal("no reply written for the small query")
	}
	if n := packedLen(t, under.written); n > dns.MinMsgSize {
		t.Fatalf("the small reply packs to %d octets, so it is not under the bound this half tests", n)
	}
	// RFC requirement: RFC1035-4.2.1-2 negative -- a message that is not longer
	// than the bound is not truncated and the TC bit stays clear.
	if under.written.Truncated {
		t.Error("a reply that fits in 512 octets has TC set, want TC clear")
	}
	// RFC requirement: RFC1035-2.3.4-2 negative -- a UDP message already within
	// the "512 octets or less" size limit is emitted whole, with every record
	// the answer func built.
	if len(under.written.Answer) != 3 {
		t.Errorf("reply carries %d answers, want all 3 (nothing may be dropped under the bound)", len(under.written.Answer))
	}
}

// VALIDATES: the 512-octet bound is the floor, not a constant. The identical
// oversized reply is truncated for a requestor that advertises no EDNS0 buffer
// and goes out whole for one advertising 4096 octets.
// PREVENTS: hard-coding 512 for every datagram reply. That truncates answers a
// requestor said it can reassemble, and forces a TCP retry per query.
func TestRFC1035_UDPBoundFollowsAdvertisedEDNSSize(t *testing.T) {
	t.Parallel()

	plain := udpWriter()
	Authoritative(nil, answerN(50), nil)(plain, questionFor("edns.test"))
	if plain.written == nil {
		t.Fatal("no reply written for the query without an OPT record")
	}
	if !plain.written.Truncated {
		t.Fatal("the reply for a query with no OPT record is not truncated, so this test cannot show the advertised size raising the bound")
	}

	advertised := questionFor("edns.test")
	advertised.SetEdns0(4096, false)
	large := udpWriter()
	Authoritative(nil, answerN(50), nil)(large, advertised)
	if large.written == nil {
		t.Fatal("no reply written for the EDNS0 query")
	}
	// RFC requirement: RFC1035-4.2.1-1 negative -- the 512-octet restriction is
	// the bound only for a requestor that has advertised nothing larger. A
	// requestor offering a 4096-octet buffer is answered up to that size.
	if large.written.Truncated {
		t.Error("reply to a query advertising a 4096-octet buffer has TC set, want the whole answer")
	}
	if len(large.written.Answer) != 50 {
		t.Errorf("reply carries %d answers, want all 50 within the advertised 4096-octet buffer", len(large.written.Answer))
	}
	if n := packedLen(t, large.written); n <= dns.MinMsgSize {
		t.Errorf("reply packs to %d octets, so it never tested a bound above 512", n)
	}
}

// VALIDATES: a stream transport carries the whole reply with TC clear, however
// long it is. The reply asserted here is the same one the datagram half of
// TestRFC1035_UDPReplyBoundedAndTruncated proves is shortened.
// PREVENTS: applying the section 4.2.1 UDP bound to TCP, DoT or DoH, which
// truncates answers on transports that have no such limit and breaks the TCP
// retry a truncated UDP answer tells a resolver to make.
func TestRFC1035_StreamTransportNotTruncated(t *testing.T) {
	t.Parallel()

	w := tcpWriter()
	Authoritative(nil, answerN(50), nil)(w, questionFor("stream.test"))
	if w.written == nil {
		t.Fatal("no reply written")
	}
	if n := packedLen(t, w.written); n <= dns.MinMsgSize {
		t.Fatalf("the stream reply packs to %d octets, so it is not one the UDP bound would have shortened", n)
	}
	// RFC requirement: RFC1035-4.2.1-2 negative -- section 4.2.1 is headed "UDP
	// usage". A message carried on a stream transport is neither truncated nor
	// marked with the TC bit.
	if w.written.Truncated {
		t.Error("reply on a stream transport has TC set, want TC clear")
	}
	if len(w.written.Answer) != 50 {
		t.Errorf("stream reply carries %d answers, want all 50", len(w.written.Answer))
	}
}

// VALIDATES: every opcode other than a standard query draws RCODE 4 without the
// answer func ever running, and the query opcode still reaches it.
// PREVENTS: an inverse query, a server status request, a notify or an update
// falling through to the answer policy, where a name inside a served zone draws
// a normal answer and a name outside one draws NXDOMAIN. Either reply claims Ze
// acted on a request it never implemented.
func TestRFC1035_UnsupportedOpcodeReturnsNotImplemented(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		opcode int
	}{
		{"inverse query", dns.OpcodeIQuery},
		{"server status request", dns.OpcodeStatus},
		{"notify", dns.OpcodeNotify},
		{"update", dns.OpcodeUpdate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ran := false
			h := Authoritative(nil, func(msg, r *dns.Msg, p Peer) bool {
				ran = true
				msg.SetRcode(r, dns.RcodeNameError)
				return true
			}, nil)

			q := questionFor("iquery.test")
			q.Opcode = tc.opcode
			w := udpWriter()
			h(w, q)

			if w.written == nil {
				t.Fatal("no reply written")
			}
			// RFC requirement: RFC1035-6.4-1 positive -- "If a name server
			// receives an inverse query that it does not support, it returns an
			// error response with the "Not Implemented" error set in the
			// header.  While inverse query support is optional, all name servers
			// must be at least able to return the error response."
			if w.written.Rcode != dns.RcodeNotImplemented {
				t.Errorf("rcode = %s, want %s", dns.RcodeToString[w.written.Rcode], dns.RcodeToString[dns.RcodeNotImplemented])
			}
			if ran {
				t.Error("the answer func ran for an opcode Ze does not serve; the reply must cost no zone lookup")
			}
			if w.written.Opcode != tc.opcode {
				t.Errorf("reply opcode = %d, want %d copied from the query", w.written.Opcode, tc.opcode)
			}
			if !w.written.Authoritative || w.written.RecursionAvailable {
				t.Error("the Not Implemented reply skipped the authoritative shape")
			}
		})
	}
}

// VALIDATES: a standard query is unaffected by the opcode check -- the answer
// func runs and the rcode it chose reaches the wire.
// PREVENTS: the Not Implemented path swallowing the queries Ze exists to
// answer, which the positive test alone cannot detect because a handler that
// always replies Not Implemented passes it.
func TestRFC1035_QueryOpcodeAnsweredNormally(t *testing.T) {
	t.Parallel()

	ran := false
	h := Authoritative(nil, func(msg, r *dns.Msg, p Peer) bool {
		ran = true
		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.IPv4(10, 0, 0, 1),
		})
		return true
	}, nil)

	q := questionFor("query.test")
	w := udpWriter()
	h(w, q)

	if w.written == nil {
		t.Fatal("no reply written")
	}
	if !ran {
		t.Error("the answer func did not run for a standard query")
	}
	// RFC requirement: RFC1035-6.4-1 negative -- the Not Implemented reply is
	// owed for an unsupported query type only. A standard query is answered.
	if w.written.Rcode == dns.RcodeNotImplemented {
		t.Error("a standard query drew Not Implemented")
	}
	if len(w.written.Answer) != 1 {
		t.Errorf("reply carries %d answers, want the 1 the answer func built", len(w.written.Answer))
	}
}

// countingRegistry is a metrics.Registry that counts increments per
// "name{label,label}" key, so a test can assert both that a counter moved and
// which label values it moved under. Only the counter surfaces record; the
// harness defines nothing else.
type countingRegistry struct {
	mu   sync.Mutex
	hits map[string]float64
}

func newCountingRegistry() *countingRegistry { return &countingRegistry{hits: map[string]float64{}} }

func (r *countingRegistry) add(key string) {
	r.mu.Lock()
	r.hits[key]++
	r.mu.Unlock()
}

func (r *countingRegistry) get(key string) float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.hits[key]
}

func (r *countingRegistry) Counter(name, _ string) metrics.Counter { return &countingMetric{r, name} }
func (r *countingRegistry) Gauge(string, string) metrics.Gauge {
	return metrics.NopRegistry{}.Gauge("", "")
}
func (r *countingRegistry) Histogram(name, help string, buckets []float64) metrics.Histogram {
	return metrics.NopRegistry{}.Histogram(name, help, buckets)
}

func (r *countingRegistry) CounterVec(name, _ string, _ []string) metrics.CounterVec {
	return &countingCounterVec{r, name}
}

func (r *countingRegistry) GaugeVec(name, help string, labels []string) metrics.GaugeVec {
	return metrics.NopRegistry{}.GaugeVec(name, help, labels)
}

func (r *countingRegistry) HistogramVec(name, help string, buckets []float64, labels []string) metrics.HistogramVec {
	return metrics.NopRegistry{}.HistogramVec(name, help, buckets, labels)
}

type countingMetric struct {
	r   *countingRegistry
	key string
}

func (m *countingMetric) Inc()          { m.r.add(m.key) }
func (m *countingMetric) Add(v float64) { m.r.add(m.key) }

type countingCounterVec struct {
	r    *countingRegistry
	name string
}

func (v *countingCounterVec) With(labelValues ...string) metrics.Counter {
	var tb textbuf.Buffer
	tb.Str(v.name).Byte('{')
	for i, lv := range labelValues {
		if i > 0 {
			tb.Byte(',')
		}
		tb.Str(lv)
	}
	tb.Byte('}')
	return &countingMetric{v.r, tb.String()}
}

func (v *countingCounterVec) Delete(...string) bool { return false }

// VALIDATES: a reply the transport refuses is counted under the transport it
// was refused on and logged, on a datagram write and on a stream write alike.
// The two halves run the same handler and differ only in the remote address, so
// a hard-coded transport label fails one of them.
// PREVENTS: the write error going back to being discarded (`_ = w.WriteMsg`),
// which leaves an operator with a server that answers nothing and reports
// nothing -- no counter to alert on and no line to grep. It is the one failure
// on the query path with no other witness: the querier sees a timeout, and Ze
// sees success.
//
// Not parallel: it swaps the package metric set, which every other test in this
// package shares.
func TestWriteFailureLoggedAndCounted(t *testing.T) {
	rec := newCountingRegistry()
	SetMetricsRegistry(rec)
	t.Cleanup(func() { SetMetricsRegistry(metrics.NopRegistry{}) })

	refused := errors.New("sendmsg: no buffer space available")

	for _, tc := range []struct {
		name      string
		writer    *rfc1035Writer
		transport string
	}{
		{name: "datagram", writer: udpWriter(), transport: "datagram"},
		{name: "stream", writer: tcpWriter(), transport: "stream"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logged bytes.Buffer
			log := slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug}))
			tc.writer.writeErr = refused

			Authoritative(log, answerN(1), nil)(tc.writer, questionFor("refused.test"))

			key := "ze_dns_reply_write_failure_total{" + tc.transport + "}"
			if got := rec.get(key); got != 1 {
				t.Errorf("%s = %v, want 1 after one refused write", key, got)
			}
			line := logged.String()
			if !strings.Contains(line, "reply write failed") {
				t.Errorf("log = %q, want it to name the failed write", line)
			}
			if !strings.Contains(line, refused.Error()) {
				t.Errorf("log = %q, want it to carry the transport error %q", line, refused)
			}
			if !strings.Contains(line, "level=DEBUG") {
				t.Errorf("log = %q, want DEBUG: one line per query is a flood at any higher level", line)
			}
		})
	}
}
