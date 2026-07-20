// Design: (none -- test-only file pinning the RFC 4035 stub-resolver behavior of
// internal/component/resolve/dns)
// RFC: rfc/short/rfc4035.md -- DNSSEC protocol modifications, security-aware
// stub-resolver side (EDNS0 OPT/DO, the AD and CD header bits, DNSSEC RR types
// appearing in a response)

package dns

import (
	"sync/atomic"
	"testing"

	mdns "github.com/miekg/dns"
)

// rfc4035Upstream is a fake upstream that records a copy of every query it
// receives (so a test can assert on the exact bytes ze emits) and replies with
// whatever reply builds.
func rfc4035Upstream(last *atomic.Pointer[mdns.Msg], reply func(r *mdns.Msg) *mdns.Msg) mdns.Handler {
	return mdns.HandlerFunc(func(w mdns.ResponseWriter, r *mdns.Msg) {
		last.Store(r.Copy())
		_ = w.WriteMsg(reply(r))
	})
}

// aRecord builds an A record for name with a fixed 300-second TTL.
func aRecord(name string, ip []byte) *mdns.A {
	return &mdns.A{
		Hdr: mdns.RR_Header{Name: name, Rrtype: mdns.TypeA, Class: mdns.ClassINET, Ttl: 300},
		A:   ip,
	}
}

// VALIDATES: the query ze emits as a security-aware stub resolver carries an
// EDNS0 OPT pseudo-RR advertising 4096 octets with the DO bit set, the CD bit
// clear (so the validating upstream does validate), and the AD bit clear.
// PREVENTS: a validation mode that never asks the upstream for DNSSEC handling,
// an advertised buffer below the 1220-octet floor, or a query that asserts AD.
func TestRFC4035_QueryCarriesEDNS0DOAndClearAD(t *testing.T) {
	var last atomic.Pointer[mdns.Msg]
	addr, cleanup := testDNSServer(t, rfc4035Upstream(&last, func(r *mdns.Msg) *mdns.Msg {
		m := new(mdns.Msg)
		m.SetReply(r)
		m.Answer = append(m.Answer, aRecord(r.Question[0].Name, []byte{203, 0, 113, 7}))
		return m
	}))
	defer cleanup()

	r := NewResolver(ResolverConfig{Server: addr, DNSSECValidation: "strict"})
	defer r.Close()
	if _, err := r.ResolveA("edns.test"); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	q := last.Load()
	if q == nil {
		t.Fatal("upstream saw no query")
	}
	opt := q.IsEdns0()
	// RFC requirement: RFC4035-4.1-1 positive -- the query carries an EDNS OPT
	// pseudo-RR with the DO bit set (resolver.go SetEdns0(4096, validating)).
	// RFC requirement: RFC4035-4.9.1-2 positive -- ze's stub sets the DO bit when
	// dnssec-validation is on, so the upstream returns DNSSEC-processed answers.
	if opt == nil {
		t.Fatal("query carries no EDNS0 OPT pseudo-RR")
	}
	if !opt.Do() {
		t.Error("query OPT has DO clear, want DO set under dnssec-validation strict")
	}
	// RFC requirement: RFC4035-4.1-4 positive -- the sender's UDP payload size
	// field advertises the message size ze accepts (4096, above the 1220 floor).
	if got := opt.UDPSize(); got != 4096 {
		t.Errorf("advertised UDP payload size = %d, want 4096", got)
	}
	// RFC requirement: RFC4035-4.6-2 positive -- the composed query has the AD bit
	// clear; ze never asserts authenticated data on a query it sends.
	if q.AuthenticatedData {
		t.Error("query has the AD bit set, want it clear")
	}
	if q.CheckingDisabled {
		t.Error("query has the CD bit set; ze must let the upstream validate (CD=0)")
	}
}

// VALIDATES: a UDP response well above the 1220-octet floor is received and
// parsed whole, because the resolver's advertised EDNS0 buffer (4096) sizes the
// receive buffer (miekg client.go co.UDPSize).
// PREVENTS: a silently truncated read that would drop records from a large
// DNSSEC-sized answer.
func TestRFC4035_LargeUDPResponseAccepted(t *testing.T) {
	const count = 80
	var wireLen atomic.Int64
	var last atomic.Pointer[mdns.Msg]
	addr, cleanup := testDNSServer(t, rfc4035Upstream(&last, func(r *mdns.Msg) *mdns.Msg {
		m := new(mdns.Msg)
		m.SetReply(r)
		for i := range count {
			m.Answer = append(m.Answer, aRecord(r.Question[0].Name, []byte{203, 0, 113, byte(i)}))
		}
		if packed, err := m.Pack(); err == nil {
			wireLen.Store(int64(len(packed)))
		}
		return m
	}))
	defer cleanup()

	r := NewResolver(ResolverConfig{Server: addr, DNSSECValidation: "strict"})
	defer r.Close()
	recs, err := r.ResolveA("bigresponse.test")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if wireLen.Load() <= 1220 {
		t.Fatalf("test response is %d octets, want a response above the 1220-octet floor", wireLen.Load())
	}
	// RFC requirement: RFC4035-4.1-2 positive -- a response of more than 1220
	// octets arrives complete over UDP; every one of the 80 answers is returned.
	if len(recs) != count {
		t.Errorf("got %d records from a %d-octet response, want %d (the response was truncated on read)", len(recs), wireLen.Load(), count)
	}
}

// VALIDATES: the AD bit of a response reaching ze over a plain UDP channel
// carries no weight in either direction -- an AD-clear NOERROR answer is
// accepted on its own merit, and an AD-set SERVFAIL is still rejected under
// strict validation.
// PREVENTS: ze treating an unsigned (Insecure) zone as a failure, or an
// attacker-set AD bit turning a broken chain into an accepted answer.
func TestRFC4035_ResponseADBitDisregarded(t *testing.T) {
	var last atomic.Pointer[mdns.Msg]
	addr, cleanup := testDNSServer(t, rfc4035Upstream(&last, func(r *mdns.Msg) *mdns.Msg {
		m := new(mdns.Msg)
		m.SetReply(r)
		if r.Question[0].Name == "adbogus.test." {
			m.SetRcode(r, mdns.RcodeServerFailure)
			m.AuthenticatedData = true // an AD bit an attacker could equally have set
			return m
		}
		m.AuthenticatedData = false // insecure / unsigned zone
		m.Answer = append(m.Answer, aRecord(r.Question[0].Name, []byte{203, 0, 113, 9}))
		return m
	}))
	defer cleanup()

	r := NewResolver(ResolverConfig{Server: addr, DNSSECValidation: "strict"})
	defer r.Close()

	// RFC requirement: RFC4035-4.6-3 positive -- ze disregards the AD bit's
	// meaning: an AD-clear NOERROR answer (an unsigned zone) is returned normally
	// rather than treated as unauthenticated data to reject.
	recs, err := r.ResolveA("adclear.test")
	if err != nil {
		t.Fatalf("AD-clear NOERROR answer rejected: %v", err)
	}
	if len(recs) != 1 || recs[0] != "203.0.113.9" {
		t.Fatalf("AD-clear answer records = %v, want [203.0.113.9]", recs)
	}

	// RFC requirement: RFC4035-4.6-3 negative -- an AD-set response over the same
	// unsecured channel confers no trust: a SERVFAIL carrying AD=1 is still
	// rejected under strict validation (dnssecDecision ignores its AD argument).
	if _, err := r.ResolveA("adbogus.test"); err == nil {
		t.Fatal("SERVFAIL carrying AD=1 was accepted, want rejection under strict validation")
	}
}

// VALIDATES: a stub-resolver response carrying DNSSEC RR types is handled
// without mishandling -- the ordinary records alongside them are returned, and a
// response made up only of DNSSEC RRs yields an empty result rather than an
// error or a bogus record.
// PREVENTS: a DO-set query drawing RRSIG/NSEC/DNSKEY records that break parsing
// or leak into the record list a caller treats as addresses.
func TestRFC4035_StubHandlesDNSSECRRTypes(t *testing.T) {
	rrsig := func(name string) *mdns.RRSIG {
		return &mdns.RRSIG{
			Hdr:         mdns.RR_Header{Name: name, Rrtype: mdns.TypeRRSIG, Class: mdns.ClassINET, Ttl: 300},
			TypeCovered: mdns.TypeA, Algorithm: 8, Labels: 2, OrigTtl: 300,
			Expiration: 2000000000, Inception: 1000000000, KeyTag: 1234,
			SignerName: "test.", Signature: "AAAA",
		}
	}
	var last atomic.Pointer[mdns.Msg]
	addr, cleanup := testDNSServer(t, rfc4035Upstream(&last, func(r *mdns.Msg) *mdns.Msg {
		m := new(mdns.Msg)
		m.SetReply(r)
		name := r.Question[0].Name
		if name == "onlydnssec.test." {
			m.Answer = append(m.Answer,
				rrsig(name),
				&mdns.NSEC{
					Hdr:        mdns.RR_Header{Name: name, Rrtype: mdns.TypeNSEC, Class: mdns.ClassINET, Ttl: 300},
					NextDomain: "next.test.", TypeBitMap: []uint16{mdns.TypeA, mdns.TypeRRSIG, mdns.TypeNSEC},
				},
				&mdns.DNSKEY{
					Hdr:   mdns.RR_Header{Name: "test.", Rrtype: mdns.TypeDNSKEY, Class: mdns.ClassINET, Ttl: 300},
					Flags: 256, Protocol: 3, Algorithm: 8, PublicKey: "AAAA",
				})
			return m
		}
		m.Answer = append(m.Answer, aRecord(name, []byte{203, 0, 113, 11}), rrsig(name))
		return m
	}))
	defer cleanup()

	r := NewResolver(ResolverConfig{Server: addr, DNSSECValidation: "strict"})
	defer r.Close()

	// RFC requirement: RFC4035-4.9-1 positive -- a response carrying an RRSIG
	// beside the A record is not mishandled: the A record is returned and the
	// DNSSEC RR is neither an error nor an entry in the record list.
	recs, err := r.ResolveA("signed.test")
	if err != nil {
		t.Fatalf("answer containing an RRSIG returned an error: %v", err)
	}
	if len(recs) != 1 || recs[0] != "203.0.113.11" {
		t.Fatalf("records for an A+RRSIG answer = %v, want [203.0.113.11]", recs)
	}

	// RFC requirement: RFC4035-4.9-1 negative -- a response whose Answer section
	// is nothing but DNSSEC RR types (RRSIG, NSEC, DNSKEY) yields no records and
	// no error; none of them is turned into a bogus address string.
	recs, err = r.ResolveA("onlydnssec.test")
	if err != nil {
		t.Fatalf("DNSSEC-only answer returned an error: %v", err)
	}
	if len(recs) != 0 {
		t.Fatalf("DNSSEC-only answer produced records %v, want none", recs)
	}
}
