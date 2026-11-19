package dnsserver

import (
	"net"
	"net/http"
	"testing"

	"github.com/miekg/dns"
)

// negativeReplyHandler answers every query with rcode, the given answers, and
// an Authority SOA of TTL soaTTL and MINIMUM soaMinimum, the shape of an
// authoritative negative answer (RFC 2308 Section 3).
func negativeReplyHandler(rcode int, answers []dns.RR, soaTTL, soaMinimum uint32) dns.Handler {
	return dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetRcode(r, rcode)
		m.Answer = answers
		m.Ns = []dns.RR{&dns.SOA{
			Hdr:     dns.RR_Header{Name: "test.", Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: soaTTL},
			Ns:      "ns.test.",
			Mbox:    "hostmaster.test.",
			Serial:  1,
			Refresh: 3600,
			Retry:   600,
			Expire:  86400,
			Minttl:  soaMinimum,
		}}
		_ = w.WriteMsg(m)
	})
}

// dohCacheControl posts one query for name to a DoH listener serving handler
// and returns the response's Cache-Control header.
func dohCacheControl(t *testing.T, handler dns.Handler, name string) string {
	t.Helper()
	port, client := startDoH(t, handler)
	resp := dohPost(t, client, port, dohContentType, mustPack(t, name))
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: status = %d, want 200", name, resp.StatusCode)
	}
	return resp.Header.Get("Cache-Control")
}

// VALIDATES: a DoH reply with no Answer record and an Authority SOA gets an
// HTTP freshness lifetime no greater than the SOA MINIMUM field, for NODATA
// and for NXDOMAIN.
// METHOD: a handler answers with an empty Answer section and an SOA of TTL
// 3600 and MINIMUM 60; the DoH response's Cache-Control is read over HTTPS.
// PREVENTS: a negative answer sent with no Cache-Control at all, which lets an
// HTTP cache assign a heuristic freshness longer than the zone allows.
// RFC requirement: RFC8484-5.1-2 positive -- a NODATA and an NXDOMAIN DoH reply with no Answer records and an Authority SOA of TTL 3600 and MINIMUM 60 each carry Cache-Control max-age=60, not greater than the SOA MINIMUM field.
func TestDoHNegativeFreshnessCappedBySOAMinimum(t *testing.T) {
	for _, tc := range []struct {
		name  string
		rcode int
	}{
		{"nodata.test", dns.RcodeSuccess},
		{"nxdomain.test", dns.RcodeNameError},
	} {
		got := dohCacheControl(t, negativeReplyHandler(tc.rcode, nil, 3600, 60), tc.name)
		if got != "max-age=60" {
			t.Fatalf("%s: Cache-Control = %q, want %q (the SOA MINIMUM)", tc.name, got, "max-age=60")
		}
	}
}

// VALIDATES: the negative lifetime is the smaller of the SOA record's TTL and
// its MINIMUM field, so an SOA TTL below MINIMUM caps the freshness.
// METHOD: a NODATA reply with an Authority SOA of TTL 30 and MINIMUM 600.
// PREVENTS: a freshness of MINIMUM when the SOA record itself expires sooner.
func TestDoHNegativeFreshnessTakesSmallerSOATTL(t *testing.T) {
	got := dohCacheControl(t, negativeReplyHandler(dns.RcodeSuccess, nil, 30, 600), "soattl.test")
	if got != "max-age=30" {
		t.Fatalf("Cache-Control = %q, want %q (SOA TTL below MINIMUM, RFC 2308 Section 5)", got, "max-age=30")
	}
}

// VALIDATES: the SOA cap applies only when the Answer section is empty: a
// reply that carries an answer takes its freshness from the answer TTL even
// when its Authority SOA has a smaller MINIMUM.
// METHOD: a reply with one A record of TTL 300 and an Authority SOA of TTL 5
// and MINIMUM 5.
// PREVENTS: a positive answer cut to the negative-caching lifetime of its zone.
// RFC requirement: RFC8484-5.1-2 negative -- a DoH reply WITH an Answer record of TTL 300 and an Authority SOA of TTL 5 and MINIMUM 5 carries Cache-Control max-age=300, the Answer TTL, not the SOA MINIMUM: the SOA cap is tied to an empty Answer section.
func TestDoHAnswerFreshnessIgnoresSOAMinimum(t *testing.T) {
	answer := []dns.RR{&dns.A{
		Hdr: dns.RR_Header{Name: "answer.test.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
		A:   net.ParseIP("10.0.0.1"),
	}}
	got := dohCacheControl(t, negativeReplyHandler(dns.RcodeSuccess, answer, 5, 5), "answer.test")
	if got != "max-age=300" {
		t.Fatalf("Cache-Control = %q, want %q (the Answer TTL)", got, "max-age=300")
	}
}

// VALIDATES: a reply with no Answer record and no SOA, which has no lifetime
// to inherit, is sent with max-age=0 rather than with no Cache-Control.
// METHOD: a SERVFAIL reply with empty Answer and Authority sections.
// PREVENTS: an HTTP cache keeping a failure under a heuristic lifetime.
func TestDoHNoAnswerNoSOANotCached(t *testing.T) {
	handler := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeServerFailure)
		_ = w.WriteMsg(m)
	})
	if got := dohCacheControl(t, handler, "servfail.test"); got != "max-age=0" {
		t.Fatalf("Cache-Control = %q, want %q", got, "max-age=0")
	}
}
