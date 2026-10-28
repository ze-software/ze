package dnsresolver

import (
	"context"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/dnsserver"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// fakeReply is one message a fake upstream sends for a query.
type fakeReply struct {
	msg       *dns.Msg
	fromOther bool          // send from another socket (another source port)
	delay     time.Duration // wait before sending
	wire      []byte        // sent as-is over UDP in place of msg, for a datagram that does not parse
}

// fakeUpstream is a scripted DNS upstream on 127.0.0.1, UDP and TCP on one
// port. handle returns the replies for one query; none means silence.
type fakeUpstream struct {
	t       *testing.T
	pc      net.PacketConn
	ln      net.Listener
	other   net.PacketConn
	handle  func(q *dns.Msg, tcp bool) []fakeReply
	queries atomic.Int32
	tcp     atomic.Int32

	mu   sync.Mutex
	seen []*dns.Msg
}

func newFakeUpstream(t *testing.T, handle func(q *dns.Msg, tcp bool) []fakeReply) *fakeUpstream {
	t.Helper()
	var pc net.PacketConn
	var ln net.Listener
	var lc net.ListenConfig
	ctx := context.Background()
	for range 20 {
		p, err := lc.ListenPacket(ctx, "udp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		l, err := lc.Listen(ctx, "tcp", p.LocalAddr().String())
		if err != nil {
			_ = p.Close()
			continue
		}
		pc, ln = p, l
		break
	}
	if pc == nil {
		t.Fatal("no free UDP and TCP port pair")
	}
	other, err := lc.ListenPacket(ctx, "udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	u := &fakeUpstream{t: t, pc: pc, ln: ln, other: other, handle: handle}
	go u.serveUDP()
	go u.serveTCP()
	t.Cleanup(func() {
		_ = pc.Close()
		_ = ln.Close()
		_ = other.Close()
	})
	return u
}

func (u *fakeUpstream) upstream() upstream {
	ap := netip.MustParseAddrPort(u.pc.LocalAddr().String())
	return upstream{IP: ap.Addr(), Port: ap.Port()}
}

func (u *fakeUpstream) record(q *dns.Msg, tcp bool) {
	u.queries.Add(1)
	if tcp {
		u.tcp.Add(1)
	}
	u.mu.Lock()
	u.seen = append(u.seen, q)
	u.mu.Unlock()
}

func (u *fakeUpstream) lastQuery() *dns.Msg {
	u.mu.Lock()
	defer u.mu.Unlock()
	if len(u.seen) == 0 {
		return nil
	}
	return u.seen[len(u.seen)-1]
}

func (u *fakeUpstream) serveUDP() {
	buf := make([]byte, 65535)
	for {
		n, from, err := u.pc.ReadFrom(buf)
		if err != nil {
			return
		}
		q := new(dns.Msg)
		if q.Unpack(buf[:n]) != nil {
			continue
		}
		u.record(q, false)
		for _, rep := range u.handle(q, false) {
			time.Sleep(rep.delay)
			wire := rep.wire
			if wire == nil {
				packed, err := rep.msg.Pack()
				if err != nil {
					u.t.Error(err)
					return
				}
				wire = packed
			}
			conn := u.pc
			if rep.fromOther {
				conn = u.other
			}
			_, _ = conn.WriteTo(wire, from)
		}
	}
}

func (u *fakeUpstream) serveTCP() {
	for {
		c, err := u.ln.Accept()
		if err != nil {
			return
		}
		co := &dns.Conn{Conn: c}
		q, err := co.ReadMsg()
		if err == nil {
			u.record(q, true)
			for _, rep := range u.handle(q, true) {
				time.Sleep(rep.delay)
				_ = co.WriteMsg(rep.msg)
			}
		}
		_ = c.Close()
	}
}

// answerWith builds a reply to q carrying rrs in the answer section.
func answerWith(q *dns.Msg, rrs ...string) *dns.Msg {
	m := new(dns.Msg)
	m.SetReply(q)
	for _, s := range rrs {
		m.Answer = append(m.Answer, mustRR(s))
	}
	return m
}

func mustRR(s string) dns.RR {
	rr, err := dns.NewRR(s)
	if err != nil {
		panic("BUG: test RR " + s + ": " + err.Error())
	}
	return rr
}

// testClock is a settable clock for cache expiry.
type testClock struct{ ns atomic.Int64 }

func newTestClock() *testClock {
	c := &testClock{}
	c.ns.Store(time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC).UnixNano())
	return c
}

func (c *testClock) now() time.Time          { return time.Unix(0, c.ns.Load()) }
func (c *testClock) advance(d time.Duration) { c.ns.Add(int64(d)) }

// testConfig is a valid enabled config forwarding to ups, allowing
// 127.0.0.0/8, with a short timeout.
func testConfig(ups ...*fakeUpstream) resolverConfig {
	cfg := defaultConfig()
	cfg.Enabled = true
	cfg.AllowFrom = []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}
	for _, u := range ups {
		cfg.Forward = append(cfg.Forward, u.upstream())
	}
	cfg.Timeout = 600 * time.Millisecond
	return cfg
}

func newTestResolver(t *testing.T, cfg resolverConfig, clock *testClock) *resolver {
	t.Helper()
	r := newResolver(clock.now)
	r.apply(&cfg)
	return r
}

// recorder is a dns.ResponseWriter that keeps the one message written.
type recorder struct {
	remote net.Addr
	msg    *dns.Msg
}

func (w *recorder) LocalAddr() net.Addr       { return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 53), Port: 53} }
func (w *recorder) RemoteAddr() net.Addr      { return w.remote }
func (w *recorder) WriteMsg(m *dns.Msg) error { w.msg = m; return nil }
func (w *recorder) Write([]byte) (int, error) { return 0, nil }
func (w *recorder) Close() error              { return nil }
func (w *recorder) TsigStatus() error         { return nil }
func (w *recorder) TsigTimersOnly(bool)       {}
func (w *recorder) Hijack()                   {}

// ask sends q through dnsserver.Recursive over r, as the listener does, from
// source over UDP (or TCP), and returns the reply, nil when dropped.
func ask(r *resolver, q *dns.Msg, source string, tcp bool) *dns.Msg {
	ip := net.ParseIP(source)
	var remote net.Addr = &net.UDPAddr{IP: ip, Port: 40000}
	if tcp {
		remote = &net.TCPAddr{IP: ip, Port: 40000}
	}
	w := &recorder{remote: remote}
	recursiveHandler(slogutil.DiscardLogger(), r).ServeDNS(w, q)
	return w.msg
}

// query builds an RD=1 query for name and qtype.
func query(name string, qtype uint16) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), qtype)
	m.RecursionDesired = true
	return m
}

var _ dnsserver.Peer = (*recorder)(nil)
