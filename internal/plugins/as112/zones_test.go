package as112

import (
	"testing"

	"github.com/miekg/dns"
)

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

// VALIDATES: AC-2 -- a query for a name within a Direct-Delegation reverse
// zone gets NOERROR, empty Answer, zone SOA in Authority (RFC 1035 NODATA).
//
// RFC requirement: RFC7534-3.5-1 positive -- the AS112 nameserver answers authoritatively for a
// zone delegated to it: a query within 10.in-addr.arpa. is answered from that zone (NOERROR, the
// zone's own SOA in Authority).
// RFC requirement: RFC7534-3.5-2 positive -- a Direct-Delegation zone contains no records beyond
// SOA and NS: a PTR query returns NODATA (empty Answer, SOA in Authority), never a PTR record.
// RFC requirement: RFC7534-3.5-3 positive -- records for RFC 1918 resources are not hosted on the
// nameserver: the RFC 1918 reverse name 1.0.10.in-addr.arpa. yields NODATA, not a hosted PTR.
func TestZoneAnswer_ReverseZoneNoData(t *testing.T) {
	r := new(dns.Msg)
	r.SetQuestion("1.0.10.in-addr.arpa.", dns.TypePTR)
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

// VALIDATES: AC-3 -- a query within empty.as112.arpa gets NOERROR, empty
// Answer, zone SOA in Authority.
func TestZoneAnswer_EmptyAS112Arpa(t *testing.T) {
	r := new(dns.Msg)
	r.SetQuestion("foo.empty.as112.arpa.", dns.TypeA)
	msg := new(dns.Msg)
	msg.SetReply(r)

	answerQuestions(msg, r, 1, "", "", "")

	if msg.Rcode != dns.RcodeSuccess {
		t.Fatalf("Rcode = %v, want NOERROR", dns.RcodeToString[msg.Rcode])
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

// VALIDATES: AC-5 -- a name outside every served zone is NXDOMAIN.
//
// RFC requirement: RFC7534-3.5-1 negative -- authoritative answering is confined to the delegated
// zones: a name outside every served zone (example.com.) is not answered with authoritative data
// (NXDOMAIN), so the nameserver answers only for the zones delegated to it.
func TestZoneAnswer_OutOfZoneNXDOMAIN(t *testing.T) {
	r := new(dns.Msg)
	r.SetQuestion("example.com.", dns.TypeA)
	msg := new(dns.Msg)
	msg.SetReply(r)

	answerQuestions(msg, r, 1, "", "", "")

	if msg.Rcode != dns.RcodeNameError {
		t.Fatalf("Rcode = %v, want NXDOMAIN", dns.RcodeToString[msg.Rcode])
	}
}

// VALIDATES: zone-boundary matching is label-aware, not a raw string suffix
// match. A sibling name that merely ENDS WITH a served zone's characters
// (e.g. "evil10.in-addr.arpa." ends with "10.in-addr.arpa.") is NOT inside
// that zone and must get NXDOMAIN, never treated as in-bailiwick NODATA.
func TestZoneAnswer_SiblingNameNotInZone_NXDOMAIN(t *testing.T) {
	cases := []string{
		"evil10.in-addr.arpa.",
		"not168.192.in-addr.arpa.",
		"xempty.as112.arpa.",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			r := new(dns.Msg)
			r.SetQuestion(name, dns.TypeA)
			msg := new(dns.Msg)
			msg.SetReply(r)

			answerQuestions(msg, r, 1, "", "", "")

			if msg.Rcode != dns.RcodeNameError {
				t.Fatalf("Rcode = %v, want NXDOMAIN for out-of-zone sibling name %q", dns.RcodeToString[msg.Rcode], name)
			}
		})
	}
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
