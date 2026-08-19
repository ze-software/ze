package as112

import (
	"net"
	"testing"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/dnsserver"
)

// captureWriter is a dns.ResponseWriter that keeps the single reply the
// dnsserver harness writes, so a test can assert on the header bits the harness
// owns (AA) as well as on the answer content answerQuestions builds.
type captureWriter struct {
	dns.ResponseWriter
	written *dns.Msg
}

func (c *captureWriter) WriteMsg(m *dns.Msg) error { c.written = m; return nil }
func (c *captureWriter) RemoteAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP("203.0.113.1"), Port: 53000}
}

// as112Handler publishes an enabled as112 state and returns the full harness
// handler (dnsserver.Authoritative over answerQuery), the composition
// newServerManager binds to the listeners. Driving a query through it is the
// only way to see the AA bit: shapeAuthoritative owns that bit and
// answerQuestions never touches it.
func as112Handler(t *testing.T) dns.HandlerFunc {
	t.Helper()
	resetAS112State(t)
	storeState(buildState(as112Config{Enabled: true}, 1))
	return dnsserver.Authoritative(nil, answerQuery, nil)
}

// askAS112 drives one question through the harness and returns the reply.
func askAS112(t *testing.T, h dns.HandlerFunc, qname string, qtype uint16) *dns.Msg {
	t.Helper()
	r := new(dns.Msg)
	r.SetQuestion(qname, qtype)
	w := &captureWriter{}
	h(w, r)
	if w.written == nil {
		t.Fatalf("no reply written for %s %s", qname, dns.TypeToString[qtype])
	}
	return w.written
}

// VALIDATES: the 19 RFC 7534 reverse zones plus EMPTY.AS112.ARPA plus the two
// hostname zones are all present, exactly once.
func TestServedZones_CompleteList(t *testing.T) {
	zones := servedZones()
	if len(zones) != 22 {
		t.Fatalf("servedZones() has %d entries, want 22 (19 reverse + empty.as112.arpa + 2 hostname)", len(zones))
	}
	want := map[string]bool{
		"10.in-addr.arpa.": true, "168.192.in-addr.arpa.": true, "254.169.in-addr.arpa.": true,
		"empty.as112.arpa.": true, "hostname.as112.net.": true, "hostname.as112.arpa.": true,
	}
	for n := 16; n <= 31; n++ {
		want[dns.Fqdn(itoa(n)+".172.in-addr.arpa")] = true
	}
	seen := map[string]bool{}
	for _, z := range zones {
		if seen[z.Name] {
			t.Errorf("zone %q listed more than once", z.Name)
		}
		seen[z.Name] = true
		if !want[z.Name] {
			t.Errorf("unexpected zone %q", z.Name)
		}
	}
	for z := range want {
		if !seen[z] {
			t.Errorf("missing expected zone %q", z)
		}
	}
}

// VALIDATES: AC-2 -- a query at the apex of a Direct-Delegation reverse zone,
// for a type the zone holds no record of, gets NOERROR, empty Answer, zone SOA
// in Authority (RFC 1035 NODATA).
//
// RFC requirement: RFC7534-3.5-1 positive -- the AS112 nameserver answers authoritatively for a
// zone delegated to it: a query at the apex of 10.in-addr.arpa. is answered from that zone
// (NOERROR, the zone's own SOA in Authority).
// RFC requirement: RFC7534-3.5-2 positive -- a Direct-Delegation zone contains no records beyond
// SOA and NS: a PTR query at the apex, where the zone's records live, returns NODATA (empty
// Answer, SOA in Authority), never a PTR record.
func TestZoneAnswer_ReverseZoneNoData(t *testing.T) {
	r := new(dns.Msg)
	r.SetQuestion("10.in-addr.arpa.", dns.TypePTR)
	msg := new(dns.Msg)
	msg.SetReply(r)

	answerQuestions(msg, r, 1, "", "", "")

	if msg.Rcode != dns.RcodeSuccess {
		t.Fatalf("Rcode = %v, want NOERROR", dns.RcodeToString[msg.Rcode])
	}
	if len(msg.Answer) != 0 {
		t.Fatalf("Answer = %v, want empty", msg.Answer)
	}
	if len(msg.Ns) != 1 {
		t.Fatalf("Ns = %v, want exactly the zone SOA", msg.Ns)
	}
	soa, ok := msg.Ns[0].(*dns.SOA)
	if !ok || soa.Hdr.Name != "10.in-addr.arpa." {
		t.Fatalf("Ns[0] = %v, want SOA for 10.in-addr.arpa.", msg.Ns[0])
	}
}

// VALIDATES: AC-3 -- a name inside empty.as112.arpa gets NXDOMAIN with the zone
// SOA in Authority. The zone holds its SOA and NS at the apex and nothing below
// it (RFC 7534 Section 3.5's db.dr-empty), so a redirected name landing on the
// node does not exist.
func TestZoneAnswer_EmptyAS112Arpa(t *testing.T) {
	r := new(dns.Msg)
	r.SetQuestion("foo.empty.as112.arpa.", dns.TypeA)
	msg := new(dns.Msg)
	msg.SetReply(r)

	answerQuestions(msg, r, 1, "", "", "")

	if msg.Rcode != dns.RcodeNameError {
		t.Fatalf("Rcode = %v, want NXDOMAIN", dns.RcodeToString[msg.Rcode])
	}
	if len(msg.Answer) != 0 || len(msg.Ns) != 1 {
		t.Fatalf("Answer=%v Ns=%v, want empty Answer + SOA in Ns", msg.Answer, msg.Ns)
	}
	soa, ok := msg.Ns[0].(*dns.SOA)
	if !ok || soa.Hdr.Name != "empty.as112.arpa." || soa.Ns != dnameRedirectionMName {
		t.Fatalf("Ns[0] = %v, want empty.as112.arpa SOA with MNAME %q", msg.Ns[0], dnameRedirectionMName)
	}
}

// VALIDATES: AC-4 -- HOSTNAME.AS112.NET TXT answer includes the configured
// hostname as a distinct TXT string.
func TestZoneAnswer_HostnameTXTIncludesHostname(t *testing.T) {
	r := new(dns.Msg)
	r.SetQuestion("hostname.as112.net.", dns.TypeTXT)
	msg := new(dns.Msg)
	msg.SetReply(r)

	answerQuestions(msg, r, 1, "node1.example", "", "")

	if len(msg.Answer) != 1 {
		t.Fatalf("Answer = %v, want exactly one TXT record", msg.Answer)
	}
	txt, ok := msg.Answer[0].(*dns.TXT)
	if !ok {
		t.Fatalf("Answer[0] = %v, want *dns.TXT", msg.Answer[0])
	}
	found := false
	for _, s := range txt.Txt {
		if s == "node1.example" {
			found = true
		}
	}
	if !found {
		t.Fatalf("TXT strings = %v, want one of them to be the configured hostname %q", txt.Txt, "node1.example")
	}
}

// VALIDATES: AC-5 -- a name outside every served zone is REFUSED, with AA
// clear.
//
// RFC requirement: RFC7534-3.5-1 negative -- authoritative answering is confined to the delegated
// zones: for a name outside every served zone (example.com.) the nameserver answers no zone data
// and makes no authority claim (REFUSED, AA clear), so it answers only for the zones delegated to
// it.
func TestZoneAnswer_OutOfZoneRefused(t *testing.T) {
	reply := askAS112(t, as112Handler(t), "example.com.", dns.TypeA)

	if reply.Rcode != dns.RcodeRefused {
		t.Fatalf("Rcode = %v, want REFUSED", dns.RcodeToString[reply.Rcode])
	}
	if reply.Authoritative {
		t.Fatal("AA set on a name under no served zone; the node claims authority it does not have")
	}
	if len(reply.Answer) != 0 || len(reply.Ns) != 0 {
		t.Fatalf("Answer=%v Ns=%v, want both empty for a refused name", reply.Answer, reply.Ns)
	}
}

// VALIDATES: zone-boundary matching is label-aware, not a raw string suffix
// match. A sibling name that merely ENDS WITH a served zone's characters
// (e.g. "evil10.in-addr.arpa." ends with "10.in-addr.arpa.") is NOT inside
// that zone and must get REFUSED with AA clear, never treated as in-bailiwick.
func TestZoneAnswer_SiblingNameNotInZone_Refused(t *testing.T) {
	handler := as112Handler(t)
	cases := []string{
		"evil10.in-addr.arpa.",
		"not168.192.in-addr.arpa.",
		"xempty.as112.arpa.",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			reply := askAS112(t, handler, name, dns.TypeA)
			if reply.Rcode != dns.RcodeRefused {
				t.Fatalf("Rcode = %v, want REFUSED for out-of-zone sibling name %q", dns.RcodeToString[reply.Rcode], name)
			}
			if reply.Authoritative {
				t.Fatalf("AA set for out-of-zone sibling name %q", name)
			}
		})
	}
}

// VALIDATES: the RCODE, the AA bit and the Authority section a query draws,
// against where its name sits relative to the served zones -- at a zone apex,
// below one, or outside every one.
// PREVENTS: the three answers collapsing into one. A test reading the RCODE
// alone passes against a REFUSED reply that still claims authority, and a test
// reading AA alone passes against an NXDOMAIN that denies a name Ze does serve.
func TestZoneAnswer_ResponseCodeByNamePosition(t *testing.T) {
	handler := as112Handler(t)

	for _, tc := range []struct {
		what      string
		qname     string
		qtype     uint16
		rcode     int
		aa        bool
		answers   bool // records expected in the Answer section
		soaInAuth bool // the zone SOA expected in the Authority section
	}{
		{"apex, a type the zone holds", "10.in-addr.arpa.", dns.TypeSOA, dns.RcodeSuccess, true, true, false},
		{"apex, a type the zone does not hold", "10.in-addr.arpa.", dns.TypePTR, dns.RcodeSuccess, true, false, true},
		{"below the apex, a name the zone does not own", "1.0.10.in-addr.arpa.", dns.TypePTR, dns.RcodeNameError, true, false, true},
		{"under no served zone", "example.com.", dns.TypeA, dns.RcodeRefused, false, false, false},
	} {
		t.Run(tc.what, func(t *testing.T) {
			reply := askAS112(t, handler, tc.qname, tc.qtype)

			// RFC requirement: RFC1035-4.1.1-3 positive -- "3               Name Error - Meaningful
			// only for responses from an authoritative name server, this code signifies that the
			// domain name referenced in the query does not exist." 1.0.10.in-addr.arpa. is inside a
			// zone this node is authoritative for, and that zone owns no node below its apex, so the
			// name does not exist and the reply carries RCODE 3.
			// RFC requirement: RFC1035-4.1.1-3 negative -- the same code is withheld from the other
			// three rows. The apex EXISTS, so a type it holds no record of is NOERROR with no data,
			// never a name error; and example.com. draws RCODE 5 rather than RCODE 3, because RCODE
			// 3 is "meaningful only for responses from an authoritative name server" and this node
			// is not an authority for it.
			// RFC requirement: RFC7534-3.5-3 positive -- records for RFC 1918 resources are not
			// hosted on the AS112 nameserver itself: an RFC 1918 reverse name such as
			// 1.0.10.in-addr.arpa. draws a name error with no PTR anywhere in the reply.
			if reply.Rcode != tc.rcode {
				t.Errorf("Rcode = %s, want %s", dns.RcodeToString[reply.Rcode], dns.RcodeToString[tc.rcode])
			}
			// RFC requirement: RFC1035-4.1.1-2 negative -- "AA              Authoritative Answer -
			// this bit is valid in responses, and specifies that the responding name server is an
			// authority for the domain name in question section." The bit is withheld from the one
			// name in the table this node is not an authority for.
			if reply.Authoritative != tc.aa {
				t.Errorf("AA = %v, want %v", reply.Authoritative, tc.aa)
			}
			if got := len(reply.Answer) > 0; got != tc.answers {
				t.Errorf("Answer section non-empty = %v, want %v (%v)", got, tc.answers, reply.Answer)
			}
			// RFC 2308 Section 3: "Name servers authoritative for a zone MUST
			// include the SOA record of the zone in the authority section of the
			// response when reporting an NXDOMAIN or indicating that no data of the
			// requested type exists."
			if got := hasSOAIn(reply.Ns); got != tc.soaInAuth {
				t.Errorf("SOA in Authority = %v, want %v (%v)", got, tc.soaInAuth, reply.Ns)
			}
			for _, rr := range append(append([]dns.RR{}, reply.Answer...), reply.Extra...) {
				if rr.Header().Rrtype == dns.TypePTR {
					t.Errorf("reply carries a PTR record (%v); this node hosts no RFC 1918 reverse data", rr)
				}
			}
		})
	}
}

// hasSOAIn reports whether rrs holds an SOA record.
func hasSOAIn(rrs []dns.RR) bool {
	for _, rr := range rrs {
		if _, ok := rr.(*dns.SOA); ok {
			return true
		}
	}
	return false
}

// VALIDATES: AC-13 / finding M1 -- SOA timers match the RFC 7534 db.dd-empty
// / db.dr-empty example zone files exactly (refresh 1W, retry 1M=60s,
// expire 1W, minimum 1W), and MNAME/NS match the canonical names per kind.
//
// RFC requirement: RFC7534-3.5-2 negative -- a Direct-Delegation zone does contain SOA and NS (the
// "no records beyond SOA and NS" rule is not "no records at all"): the zone has an SOA with the
// RFC-mandated parameters and exactly the two blackhole-{1,2}.iana.org. NS records.
func TestSOA_RFCMandatedParameters(t *testing.T) {
	dd := buildSOA("10.in-addr.arpa.", kindDirectDelegation, 42)
	if dd.Refresh != 604800 || dd.Retry != 60 || dd.Expire != 604800 || dd.Minttl != 604800 {
		t.Fatalf("Direct-Delegation SOA timers = %+v, want refresh=604800 retry=60 expire=604800 minttl=604800", dd)
	}
	if dd.Ns != "prisoner.iana.org." || dd.Mbox != "hostmaster.root-servers.org." {
		t.Fatalf("Direct-Delegation SOA MNAME/RNAME = %q/%q, want prisoner.iana.org./hostmaster.root-servers.org.", dd.Ns, dd.Mbox)
	}
	if dd.Hdr.Ttl != 604800 {
		t.Fatalf("Direct-Delegation SOA record TTL = %d, want 604800 ($TTL 1W)", dd.Hdr.Ttl)
	}

	msg := new(dns.Msg)
	appendNS(msg, "10.in-addr.arpa.", kindDirectDelegation, false)
	if len(msg.Answer) != 2 {
		t.Fatalf("Direct-Delegation NS records = %v, want exactly blackhole-1 and blackhole-2", msg.Answer)
	}
	names := map[string]bool{}
	for _, rr := range msg.Answer {
		ns, ok := rr.(*dns.NS)
		if !ok {
			t.Fatalf("NS record = %v, want *dns.NS", rr)
		}
		names[ns.Ns] = true
	}
	if !names["blackhole-1.iana.org."] || !names["blackhole-2.iana.org."] {
		t.Fatalf("NS names = %v, want blackhole-1.iana.org. and blackhole-2.iana.org.", names)
	}

	dr := buildSOA("empty.as112.arpa.", kindDNAMERedirection, 42)
	if dr.Ns != "blackhole.as112.arpa." || dr.Mbox != "noc.dns.icann.org." {
		t.Fatalf("DNAME-Redirection SOA MNAME/RNAME = %q/%q, want blackhole.as112.arpa./noc.dns.icann.org.", dr.Ns, dr.Mbox)
	}

	msg2 := new(dns.Msg)
	appendNS(msg2, "empty.as112.arpa.", kindDNAMERedirection, false)
	if len(msg2.Answer) != 1 {
		t.Fatalf("DNAME-Redirection NS records = %v, want exactly blackhole.as112.arpa.", msg2.Answer)
	}
	if ns, ok := msg2.Answer[0].(*dns.NS); !ok || ns.Ns != "blackhole.as112.arpa." {
		t.Fatalf("NS record = %v, want blackhole.as112.arpa.", msg2.Answer[0])
	}
}

// VALIDATES: AC-4 / finding M3 -- the ASSEMBLED UDP response (all TXT
// strings + NS + SOA-equivalent overhead) with hostname/facility/location
// all at their max YANG length fits within 512 octets with TC=0.
func TestHostnameTXT_TotalResponseUnder512(t *testing.T) {
	maxHostname := repeatString("h", maxHostnameLen)
	maxFacility := repeatString("f", maxFacilityLen)
	maxLocation := repeatString("l", maxLocationLen)

	r := new(dns.Msg)
	r.SetQuestion("hostname.as112.net.", dns.TypeTXT)
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Compress = false

	answerQuestions(msg, r, 1, maxHostname, maxFacility, maxLocation)

	packed, err := msg.Pack()
	if err != nil {
		t.Fatalf("Pack() error: %v", err)
	}
	if len(packed) > 512 {
		t.Fatalf("assembled response = %d octets, want <= 512 (max-length hostname/facility/location)", len(packed))
	}
	if msg.Truncated {
		t.Fatalf("msg.Truncated = true, want false (TC=0) at max field lengths")
	}
}

func repeatString(s string, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = s[0]
	}
	return string(b)
}
