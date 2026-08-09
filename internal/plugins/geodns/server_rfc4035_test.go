// Design: docs/architecture/dns/geodns.md -- geodns answer policy; the
// dnsserver harness owns the reply's authoritative shape
// RFC: rfc/short/rfc4035.md -- DNSSEC protocol modifications, authoritative
// name-server side (a DS query at a zone cut, and the absence of any DNSSEC
// additional processing on an unsigned zone)

package geodns

import (
	"net"
	"testing"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/dnsserver"
)

// rfc4035Writer is a dns.ResponseWriter capturing the single reply the
// dnsserver harness writes, so a test can assert on the header bits the harness
// owns (AA, AD) as well as on geodns's answer content.
type rfc4035Writer struct {
	dns.ResponseWriter
	written *dns.Msg
}

func (f *rfc4035Writer) WriteMsg(m *dns.Msg) error { f.written = m; return nil }
func (f *rfc4035Writer) RemoteAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP("203.0.113.7"), Port: 5353}
}

// rfc4035Zone publishes a one-zone geodns configuration and returns the full
// harness handler (dnsserver.Authoritative over geodns's answerQuery), the same
// composition register.go binds to the listeners.
func rfc4035Zone(t *testing.T) dns.HandlerFunc {
	t.Helper()
	cfg, err := parseConfig(`{"service":{"geodns":{"enabled":"true","zone":["t.example."],` +
		`"host-set":{"web":{"host":{"www.t.example.":{"address":["10.0.0.5"]}}}},` +
		`"source":{"203.0.113.0/24":{"host-set":"web"}}}}}`)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	storeApplied(cfg, 1)
	return dnsserver.Authoritative(answerQuery, nil)
}

// hasDNSSECRR reports whether any section of msg carries a DNSSEC RR type.
func hasDNSSECRR(msg *dns.Msg) bool {
	for _, section := range [][]dns.RR{msg.Answer, msg.Ns, msg.Extra} {
		for _, rr := range section {
			switch rr.Header().Rrtype {
			case dns.TypeRRSIG, dns.TypeNSEC, dns.TypeNSEC3, dns.TypeDNSKEY, dns.TypeDS:
				return true
			}
		}
	}
	return false
}

// VALIDATES: a DS query for a name inside a zone geodns serves draws an
// authoritative no-data response (NOERROR, AA set, empty Answer, the zone SOA in
// the Authority section), while a DS query for a name geodns serves no zone for
// draws NXDOMAIN.
// PREVENTS: a DS query at a zone cut being answered NXDOMAIN (which a validating
// resolver reads as "the name does not exist" rather than "no DS here"), or a
// blanket no-data answer for names geodns is not authoritative for.
func TestRFC4035_DSQueryIsAuthoritativeNoData(t *testing.T) {
	handler := rfc4035Zone(t)

	inZone := new(dns.Msg)
	inZone.SetQuestion("child.t.example.", dns.TypeDS)
	fw := &rfc4035Writer{}
	handler(fw, inZone)
	if fw.written == nil {
		t.Fatal("no reply written for the in-zone DS query")
	}

	// RFC requirement: RFC4035-3.1.4.1-1 positive -- geodns is authoritative for
	// the child zone, is not the parent, and offers no recursion (the harness
	// clears RecursionAvailable), so a DS query at the cut returns an
	// authoritative no-data response: NOERROR, AA set, no Answer, SOA in Authority.
	if fw.written.Rcode != dns.RcodeSuccess {
		t.Errorf("DS query rcode = %s, want NOERROR (an authoritative no-data answer)", dns.RcodeToString[fw.written.Rcode])
	}
	if !fw.written.Authoritative {
		t.Error("DS reply has AA clear, want an authoritative answer")
	}
	if fw.written.RecursionAvailable {
		t.Error("DS reply advertises recursion; the no-data rule applies to a server not offering recursion")
	}
	if len(fw.written.Answer) != 0 {
		t.Errorf("DS reply Answer = %v, want empty (no data)", fw.written.Answer)
	}
	if !hasSOA(fw.written.Ns) {
		t.Error("DS reply has no SOA in the Authority section; a no-data answer must carry it")
	}

	outOfZone := new(dns.Msg)
	outOfZone.SetQuestion("elsewhere.invalid.", dns.TypeDS)
	fwOut := &rfc4035Writer{}
	handler(fwOut, outOfZone)
	if fwOut.written == nil {
		t.Fatal("no reply written for the out-of-zone DS query")
	}
	// RFC requirement: RFC4035-3.1.4.1-1 negative -- the no-data answer is not
	// handed out unconditionally: a DS query for a name under no served zone draws
	// NXDOMAIN, so the positive case above is authoritative-for-the-child data.
	if fwOut.written.Rcode != dns.RcodeNameError {
		t.Errorf("out-of-zone DS query rcode = %s, want NXDOMAIN", dns.RcodeToString[fwOut.written.Rcode])
	}
}

// VALIDATES: geodns performs no DNSSEC additional processing -- a query with the
// DO bit set and the same query without an EDNS OPT at all draw the same answer,
// and neither reply carries an RRSIG, NSEC, NSEC3, DNSKEY, or DS record, nor
// asserts the AD bit.
// PREVENTS: an unsigned synthetic zone claiming authenticated data, or DNSSEC
// records appearing in a reply ze cannot sign.
func TestRFC4035_NoDNSSECAdditionalProcessing(t *testing.T) {
	handler := rfc4035Zone(t)

	plain := new(dns.Msg)
	plain.SetQuestion("www.t.example.", dns.TypeA)
	fwPlain := &rfc4035Writer{}
	handler(fwPlain, plain)

	withDO := new(dns.Msg)
	withDO.SetQuestion("www.t.example.", dns.TypeA)
	withDO.SetEdns0(4096, true)
	fwDO := &rfc4035Writer{}
	handler(fwDO, withDO)

	if fwPlain.written == nil || fwDO.written == nil {
		t.Fatal("a reply is missing")
	}
	if got := firstA(fwPlain.written); got != "10.0.0.5" {
		t.Fatalf("A for the OPT-less query = %q, want 10.0.0.5", got)
	}
	// RFC requirement: RFC4035-3-6 positive -- a query carrying no EDNS OPT (and
	// so no DO bit) receives an answer with no DNSSEC additional processing: the
	// reply is identical to the DO-set one and carries no DNSSEC RR of any type.
	if hasDNSSECRR(fwPlain.written) {
		t.Error("reply to an OPT-less query carries a DNSSEC RR; no DNSSEC additional processing may happen")
	}
	if hasDNSSECRR(fwDO.written) {
		t.Error("reply to a DO-set query carries a DNSSEC RR; geodns signs nothing")
	}
	if firstA(fwPlain.written) != firstA(fwDO.written) {
		t.Errorf("DO-set answer %q differs from the OPT-less answer %q", firstA(fwDO.written), firstA(fwPlain.written))
	}

	// RFC requirement: RFC4035-3.1.6-2 positive -- geodns never sets the AD bit;
	// it authenticates no RRset in the Answer or Authority sections.
	// RFC requirement: RFC4035-3.1.6-4 positive -- the answer is local
	// authoritative-zone data obtained from the running configuration rather than
	// by secure means, and it is served with AD clear.
	// RFC requirement: RFC4035-3.1.6-5 positive -- ze has no setting that marks
	// local zone data authentic, so the AD bit stays clear for it in every case.
	if fwPlain.written.AuthenticatedData || fwDO.written.AuthenticatedData {
		t.Error("a geodns reply asserts AD for unsigned local zone data")
	}
}
