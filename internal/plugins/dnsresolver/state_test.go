package dnsresolver

import (
	"net/netip"
	"testing"

	"github.com/miekg/dns"
)

// TestNarrowedReloadWindow proves the window a reload rebinds its listeners
// in answers only the sources both the running and the candidate allow-from
// admit, and that restore puts the running policy back. Method: apply a
// running config allowing 127.0.1.0/24 and 127.0.2.0/24, narrow to a
// candidate allowing 127.0.2.0/24 and 127.0.3.0/24, and query through answer
// from one source of each /24: only the shared one is answered; after
// restore the running-only source is answered again and the candidate-only
// one is still refused.
func TestNarrowedReloadWindow(t *testing.T) {
	up := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		return answerWith(q, "n.test. 300 IN A 192.0.2.1")
	})
	running := testConfig(up)
	running.AllowFrom = []netip.Prefix{netip.MustParsePrefix("127.0.1.0/24"), netip.MustParsePrefix("127.0.2.0/24")}
	next := testConfig(up)
	next.AllowFrom = []netip.Prefix{netip.MustParsePrefix("127.0.2.0/24"), netip.MustParsePrefix("127.0.3.0/24")}
	r := newTestResolver(t, running, newTestClock())

	rcode := func(source string) int {
		t.Helper()
		reply := ask(r, query("n.test", dns.TypeA), source, false)
		if reply == nil {
			t.Fatalf("query from %s dropped", source)
		}
		return reply.Rcode
	}
	want := func(source string, rc int) {
		t.Helper()
		if got := rcode(source); got != rc {
			t.Errorf("from %s rcode %s, want %s", source, dns.RcodeToString[got], dns.RcodeToString[rc])
		}
	}

	saved := r.narrow(&next)
	want("127.0.1.9", dns.RcodeRefused) // only the running config admits it
	want("127.0.3.9", dns.RcodeRefused) // only the candidate admits it
	want("127.0.2.9", dns.RcodeSuccess) // both admit it

	r.restore(saved)
	want("127.0.1.9", dns.RcodeSuccess)
	want("127.0.3.9", dns.RcodeRefused)
	want("127.0.2.9", dns.RcodeSuccess)
}
