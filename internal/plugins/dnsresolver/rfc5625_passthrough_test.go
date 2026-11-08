// Design: docs/architecture/dns/lan-resolver.md -- a query carrying TSIG or TKEY is forwarded unchanged
// Related: rfc5625_resolver_test.go -- the other RFC 5625 rows on the resolver path

package dnsresolver

import (
	"context"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// passKey is the TSIG key the test client and the test upstream share. The
// resolver does not hold it.
const passKey = "lan-key.example."

var passSecret = map[string]string{passKey: "c2VjcmV0LXNoYXJlZC1ieS1jbGllbnQtYW5kLXVwc3RyZWFt"}

// tsigUpstream is an upstream that holds passKey: the DNS library verifies
// each signed query it reads against the bytes that arrived, and signs its
// reply. handle builds the reply; verdicts records each query's TSIG check.
type tsigUpstream struct {
	addr     netip.AddrPort
	mu       sync.Mutex
	verdicts []error
	signed   []bool
	extra    [][]dns.RR
}

func newTSIGUpstream(t *testing.T, handle func(q *dns.Msg) *dns.Msg) *tsigUpstream {
	t.Helper()
	u := &tsigUpstream{}
	var lc net.ListenConfig
	pc, err := lc.ListenPacket(context.Background(), "udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln, err := lc.Listen(context.Background(), "tcp", pc.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	u.addr = netip.MustParseAddrPort(pc.LocalAddr().String())
	h := dns.HandlerFunc(func(w dns.ResponseWriter, q *dns.Msg) {
		u.mu.Lock()
		u.verdicts = append(u.verdicts, w.TsigStatus())
		u.signed = append(u.signed, q.IsTsig() != nil)
		u.extra = append(u.extra, q.Extra)
		u.mu.Unlock()
		reply := handle(q)
		if t := q.IsTsig(); t != nil {
			reply.SetTsig(t.Hdr.Name, t.Algorithm, 300, time.Now().Unix())
		}
		_ = w.WriteMsg(reply)
	})
	udp := &dns.Server{PacketConn: pc, Handler: h, TsigSecret: passSecret}
	tcp := &dns.Server{Listener: ln, Handler: h, TsigSecret: passSecret}
	go func() { _ = udp.ActivateAndServe() }()
	go func() { _ = tcp.ActivateAndServe() }()
	t.Cleanup(func() {
		_ = udp.Shutdown()
		_ = tcp.Shutdown()
	})
	return u
}

func (u *tsigUpstream) seen() (verdicts []error, signed []bool, extra [][]dns.RR) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]error(nil), u.verdicts...), append([]bool(nil), u.signed...), append([][]dns.RR(nil), u.extra...)
}

// listeningResolver binds a resolver forwarding to up on a loopback listener
// and returns the listener's address.
func listeningResolver(t *testing.T, up netip.AddrPort) (*resolver, string) {
	t.Helper()
	cfg := defaultConfig()
	cfg.Enabled = true
	cfg.AllowFrom = []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}
	cfg.Forward = []upstream{{IP: up.Addr(), Port: up.Port()}}
	cfg.Timeout = 2 * time.Second
	listen := listenerEndpoint{Name: "lan", IP: netip.MustParseAddr("127.0.0.1"), Port: freeLoopbackPort(t)}
	cfg.Listeners = []listenerEndpoint{listen}
	res := newTestResolver(t, cfg, newTestClock())
	log := slogutil.DiscardLogger()
	in := &resolverInstaller{srv: newResolverServer(log, res), res: res, log: log}
	t.Cleanup(in.srv.stop)
	if err := in.install(cfg); err != nil {
		t.Fatalf("install: %v", err)
	}
	return res, net.JoinHostPort(listen.IP.String(), strconv.Itoa(int(listen.Port)))
}

// tkeyRR is a GSS-API mode TKEY record owned by passKey, as RFC 2930
// Section 4.3 has a resolver send.
func tkeyRR() *dns.TKEY {
	now := uint32(time.Now().Unix())
	return &dns.TKEY{
		Hdr:       dns.RR_Header{Name: passKey, Rrtype: dns.TypeTKEY, Class: dns.ClassANY},
		Algorithm: "gss-tsig.", Inception: now, Expiration: now + 3600, Mode: 3,
		KeySize: 4, Key: "01020304",
	}
}

// TestRFC5625SignedQueryForwardedUnchanged proves RFC 5625 Section 4.5 and
// RFC 8945 Section 5.5 on the resolver: a TSIG-signed query whose key the
// resolver does not hold reaches the upstream unchanged, and the upstream's
// signed reply reaches the client unchanged. Method: a client and an
// upstream that share a key, the resolver between them on a real listener,
// over UDP and over TCP. The upstream's DNS library verifies the MAC over
// the bytes that arrived, and the client's verifies the reply's. A second
// case signs a TKEY query whose TKEY owner the client compresses against the
// question, so a resolver that packs the parsed query again sends other
// bytes than the client signed. The upstream answers with AA and RA set, so
// a reply rewritten into the recursive shape fails the client's check. The
// same signed question is asked twice and reaches the upstream twice: no
// signed answer is served from the cache.
func TestRFC5625SignedQueryForwardedUnchanged(t *testing.T) {
	up := newTSIGUpstream(t, func(q *dns.Msg) *dns.Msg {
		m := new(dns.Msg)
		m.SetReply(q)
		m.Authoritative = true
		m.RecursionAvailable = true
		if q.Question[0].Qtype == dns.TypeA {
			m.Answer = append(m.Answer, mustRR(q.Question[0].Name+" 300 IN A 192.0.2.7"))
		}
		return m
	})
	_, addr := listeningResolver(t, up.addr)

	asked := 0
	for _, network := range []string{"udp", "tcp"} {
		client := &dns.Client{Net: network, Timeout: 3 * time.Second, TsigSecret: passSecret}
		for range 2 {
			q := query("signed.example.", dns.TypeA)
			q.SetTsig(passKey, dns.HmacSHA256, 300, time.Now().Unix())
			reply, _, err := client.Exchange(q, addr)
			asked++
			// RFC requirement: RFC5625-4.5-1 positive -- "DNS proxies MUST
			// implement Section 4.7 of [RFC2845] and either forward packets
			// unchanged (as recommended above) or fully implement TSIG." The
			// client verifies the upstream's signature on the reply, and the
			// upstream verified the client's on the query it received.
			if err != nil {
				t.Fatalf("%s: signed exchange through the resolver: %v", network, err)
			}
			if len(reply.Answer) != 1 || !reply.Authoritative {
				t.Fatalf("%s: reply %v, want the upstream's own AA reply with one A record", network, reply)
			}
		}

		q := new(dns.Msg)
		q.SetQuestion(passKey, dns.TypeTKEY)
		q.Question[0].Qclass = dns.ClassANY
		q.Extra = append(q.Extra, tkeyRR())
		q.Compress = true
		q.SetTsig(passKey, dns.HmacSHA256, 300, time.Now().Unix())
		_, _, err := client.Exchange(q, addr)
		asked++
		// RFC requirement: RFC5625-4.5-1 positive -- a signed query whose
		// names the client compressed reaches the upstream as signed.
		if err != nil {
			t.Fatalf("%s: signed TKEY exchange through the resolver: %v", network, err)
		}
	}
	verdicts, signed, _ := up.seen()
	if len(verdicts) != asked {
		t.Fatalf("upstream saw %d queries, want %d: a signed answer came from the cache", len(verdicts), asked)
	}
	for i, err := range verdicts {
		if !signed[i] {
			t.Errorf("query %d reached the upstream without its TSIG", i)
		}
		if err != nil {
			t.Errorf("query %d: the upstream's TSIG check failed: %v", i, err)
		}
	}
}

// TestRFC5625TKEYQueryProxied proves RFC 5625 Section 4.5 on the resolver: an
// unsigned TKEY query, RD clear as RFC 2930 Section 4 asks, reaches the
// upstream with its TKEY record, and the upstream's TKEY answer reaches the
// client. Method: a real listener between a client and an upstream that
// echoes the TKEY record in its answer, over UDP and TCP.
func TestRFC5625TKEYQueryProxied(t *testing.T) {
	up := newTSIGUpstream(t, func(q *dns.Msg) *dns.Msg {
		m := new(dns.Msg)
		m.SetReply(q)
		for _, rr := range q.Extra {
			if rr.Header().Rrtype == dns.TypeTKEY {
				m.Answer = append(m.Answer, dns.Copy(rr))
			}
		}
		return m
	})
	_, addr := listeningResolver(t, up.addr)

	for _, network := range []string{"udp", "tcp"} {
		client := &dns.Client{Net: network, Timeout: 3 * time.Second}
		q := new(dns.Msg)
		q.SetQuestion(passKey, dns.TypeTKEY)
		q.Question[0].Qclass = dns.ClassANY
		q.RecursionDesired = false
		q.Extra = append(q.Extra, tkeyRR())
		reply, _, err := client.Exchange(q, addr)
		if err != nil {
			t.Fatalf("%s: TKEY exchange: %v", network, err)
		}
		// RFC requirement: RFC5625-4.5-2 positive -- "As per Section 4.3, DNS
		// proxies MUST be capable of proxying packets containing TKEY
		// [RFC2930] Resource Records." The TKEY record reaches the upstream
		// and the upstream's TKEY answer reaches the client.
		if reply.Rcode != dns.RcodeSuccess || len(reply.Answer) != 1 || reply.Answer[0].Header().Rrtype != dns.TypeTKEY {
			t.Fatalf("%s: reply %v, want NOERROR with the upstream's TKEY answer", network, reply)
		}
	}
	_, _, extra := up.seen()
	for i, rrs := range extra {
		found := false
		for _, rr := range rrs {
			if rr.Header().Rrtype == dns.TypeTKEY {
				found = true
			}
		}
		if !found {
			t.Errorf("query %d reached the upstream without its TKEY record", i)
		}
	}
}
