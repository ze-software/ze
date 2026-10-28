package dnsresolver

import (
	"net"
	"net/netip"
	"strconv"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestMalformedQueryShapedAndCounted proves the resolver's own handler, not
// the DNS library's header filter, answers a query with two questions and a
// query whose opcode Ze does not implement: the reply is FORMERR or NOTIMP in
// the recursive shape (RA=1, AA=0) and the query is counted.
// Method: bind a real listener, send each message over UDP and TCP, and read
// the reply and the resolver's query counter. Under the library's default
// accept function both replies are written before the handler runs, so they
// carry RA=0 and the counter does not move.
func TestMalformedQueryShapedAndCounted(t *testing.T) {
	up := aUpstream(t, 0)
	cfg := testConfig(up)
	listen := listenerEndpoint{Name: "lan", IP: netip.MustParseAddr("127.0.0.1"), Port: freeLoopbackPort(t)}
	cfg.Listeners = []listenerEndpoint{listen}
	res := newTestResolver(t, cfg, newTestClock())
	log := slogutil.DiscardLogger()
	in := &resolverInstaller{srv: newResolverServer(log, res), res: res, log: log}
	t.Cleanup(in.srv.stop)
	if err := in.install(cfg); err != nil {
		t.Fatalf("install: %v", err)
	}
	addr := net.JoinHostPort(listen.IP.String(), strconv.Itoa(int(listen.Port)))

	twoQuestions := query("a.example.", dns.TypeA)
	twoQuestions.Question = append(twoQuestions.Question, dns.Question{Name: "b.example.", Qtype: dns.TypeA, Qclass: dns.ClassINET})
	update := query("example.", dns.TypeSOA)
	update.Opcode = dns.OpcodeUpdate
	status := query("example.", dns.TypeA)
	status.Opcode = dns.OpcodeStatus
	ednsVersion1 := query("a.example.", dns.TypeA)
	ednsVersion1.SetEdns0(1232, false)
	ednsVersion1.IsEdns0().SetVersion(1)

	cases := []struct {
		name  string
		msg   *dns.Msg
		rcode int
	}{
		{"two questions", twoQuestions, dns.RcodeFormatError},
		{"UPDATE", update, dns.RcodeNotImplemented},
		{"STATUS", status, dns.RcodeNotImplemented},
		// RFC 6891 Section 6.1.3
		{"EDNS version 1", ednsVersion1, dns.RcodeBadVers},
	}
	for _, network := range []string{"udp", "tcp"} {
		client := &dns.Client{Net: network, Timeout: 2 * time.Second}
		for _, tc := range cases {
			before := res.stats.queries.Load()
			reply, _, err := client.Exchange(tc.msg.Copy(), addr)
			if err != nil {
				t.Fatalf("%s over %s: %v", tc.name, network, err)
			}
			if reply.Rcode != tc.rcode {
				t.Errorf("%s over %s: rcode %s, want %s", tc.name, network, rcodeName(reply.Rcode), rcodeName(tc.rcode))
			}
			if !reply.RecursionAvailable {
				t.Errorf("%s over %s: RA=0, want the recursive shape", tc.name, network)
			}
			if reply.Authoritative {
				t.Errorf("%s over %s: AA=1, want 0", tc.name, network)
			}
			if got := res.stats.queries.Load() - before; got != 1 {
				t.Errorf("%s over %s: counted %d queries, want 1", tc.name, network, got)
			}
		}
	}
}

// TestRcodeNameBadVers proves the query counter labels RCODE 16 BADVERS, the
// meaning this service writes, not miekg/dns's TSIG name BADSIG. Method: ask
// rcodeName for 16 and for two plain codes.
func TestRcodeNameBadVers(t *testing.T) {
	cases := map[int]string{dns.RcodeBadVers: "BADVERS", dns.RcodeSuccess: "NOERROR", dns.RcodeNotImplemented: "NOTIMP"}
	for rcode, want := range cases {
		if got := rcodeName(rcode); got != want {
			t.Errorf("rcodeName(%d) = %q, want %q", rcode, got, want)
		}
	}
}
