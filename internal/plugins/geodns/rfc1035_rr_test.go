// Design: docs/architecture/dns/geodns.md -- geodns stamps every RR header
// through hdr and bounds every configurable TTL in parseConfig. This file reads
// the resulting TTL and RDLENGTH off the packed octets of geodns's own reply.
// RFC: rfc/short/rfc1035.md -- the RR fields of section 4.1.3 and the TTL size
// limit of section 2.3.4

package geodns

import (
	"encoding/binary"
	"net/netip"
	"strings"
	"testing"

	"github.com/miekg/dns"
)

// wireRR is one resource record as it appears on the wire, with the two fields
// RFC 1035 section 4.1.3 gives a width and a meaning.
type wireRR struct {
	name     string
	rrtype   uint16
	ttl      uint32
	rdlength uint16
	rdata    []byte
}

// walkRRs reads every record of msg's three record sections straight out of the
// packed octets. It exists so a TTL assertion measures the field a resolver
// reads, not the dns.RR_Header struct geodns filled in.
func walkRRs(t *testing.T, msg *dns.Msg) []wireRR {
	t.Helper()
	wire := packReply(t, msg)
	if len(wire) < 12 {
		t.Fatalf("packed reply is %d octets, shorter than the header", len(wire))
	}
	off := 12
	for range int(binary.BigEndian.Uint16(wire[4:6])) { // QDCOUNT
		_, next := walkName(t, wire, off)
		off = next + 4 // QTYPE and QCLASS
	}
	total := int(binary.BigEndian.Uint16(wire[6:8])) + // ANCOUNT
		int(binary.BigEndian.Uint16(wire[8:10])) + // NSCOUNT
		int(binary.BigEndian.Uint16(wire[10:12])) // ARCOUNT
	out := make([]wireRR, 0, total)
	for range total {
		labels, next := walkName(t, wire, off)
		off = next
		if off+10 > len(wire) {
			t.Fatalf("record header at offset %d runs past the end of a %d-octet message", off, len(wire))
		}
		rr := wireRR{
			name:     strings.Join(labels, ".") + ".",
			rrtype:   binary.BigEndian.Uint16(wire[off : off+2]),
			ttl:      binary.BigEndian.Uint32(wire[off+4 : off+8]),
			rdlength: binary.BigEndian.Uint16(wire[off+8 : off+10]),
		}
		off += 10
		if off+int(rr.rdlength) > len(wire) {
			t.Fatalf("RDLENGTH %d at offset %d runs past the end of a %d-octet message", rr.rdlength, off, len(wire))
		}
		rr.rdata = wire[off : off+int(rr.rdlength)]
		off += int(rr.rdlength)
		out = append(out, rr)
	}
	if off != len(wire) {
		t.Errorf("walking every record left %d octets unread; a length field disagrees with the message", len(wire)-off)
	}
	return out
}

// firstOfType returns the first record of rrtype in rrs.
func firstOfType(t *testing.T, rrs []wireRR, rrtype uint16) wireRR {
	t.Helper()
	for _, rr := range rrs {
		if rr.rrtype == rrtype {
			return rr
		}
	}
	t.Fatalf("no record of type %d in the reply", rrtype)
	return wireRR{}
}

// ttlState serves one zone whose three hosts pin the ends of the TTL range and
// the middle of it.
func ttlState(t *testing.T) *resolverState {
	t.Helper()
	return answerState(t, `{"service":{"geodns":{"enabled":"true","zone":["t.example."],`+
		`"nameserver":["10.0.0.1"],"soa":{"minimum":"300"},`+
		`"host-set":{"web":{"host":{`+
		`"low.t.example.":{"ttl":"120","address":["10.0.0.5"]},`+
		`"max.t.example.":{"ttl":"2147483647","address":["10.0.0.6"]},`+
		`"zero.t.example.":{"ttl":"0","address":["10.0.0.7"]},`+
		`"six.t.example.":{"address":["2001:db8::1"]}}}},`+
		`"source":{"0.0.0.0/0":{"host-set":"web"}}}}}`)
}

// VALIDATES: the configured TTL reaches the wire as a 32-bit unsigned value, at
// 120, at the 2147483647 ceiling and at 0. Three configured values produce three
// different octet quads, so no constant satisfies the assertions.
// PREVENTS: a TTL that is silently defaulted, clamped or sign-extended. A
// resolver caches for exactly the number this field carries, so an emitted TTL
// that is not the configured one is a caching decision the operator did not
// make and cannot see.
func TestRFC1035_RecordTTLIsA32BitUnsignedSecondCount(t *testing.T) {
	t.Parallel()
	st := ttlState(t)

	// RFC requirement: RFC1035-4.1.3-1 positive -- "TTL             a 32 bit
	// unsigned integer that specifies the time interval that the resource record
	// may be cached before the source of the information should again be
	// consulted." RFC 1035 carries no capitalised RFC 2119 keyword anywhere, so
	// this quoted sentence is the whole anchor.
	for _, tc := range []struct {
		host string
		want uint32
	}{
		{"low.t.example.", 120},
		{"max.t.example.", maxTTL},
	} {
		rr := firstOfType(t, walkRRs(t, answerA(st, tc.host)), dns.TypeA)
		if rr.ttl != tc.want {
			t.Errorf("%s TTL on the wire = %d, want the configured %d", tc.host, rr.ttl, tc.want)
		}
	}

	// RFC requirement: RFC1035-4.1.3-1 negative -- the field is UNSIGNED, so zero
	// is a value rather than an absence. A configured 0 must reach the wire as 0
	// and must not be replaced by the zone default of 300.
	zero := firstOfType(t, walkRRs(t, answerA(st, "zero.t.example.")), dns.TypeA)
	if zero.ttl != 0 {
		t.Errorf("a host configured with ttl 0 serves TTL %d; zero is a legal unsigned value, not an unset one", zero.ttl)
	}
}

// VALIDATES: every configurable TTL leaf is bounded to 0..2147483647 at parse
// time, and the value one below the bound is accepted and served unchanged.
// PREVENTS: a TTL whose top bit is set reaching the wire. Read as the signed
// 32-bit number RFC 1035 section 2.3.4 describes, such a value is negative, and
// resolvers have historically treated it as anything from zero to a week.
func TestRFC1035_ConfiguredTTLBoundedToASigned32BitPositive(t *testing.T) {
	t.Parallel()

	// RFC requirement: RFC1035-2.3.4-1 positive -- the size limits of section
	// 2.3.4 include "TTL             positive values of a signed 32 bit number."
	for _, tc := range []struct{ what, cfg string }{
		{"record ttl", `"host-set":{"w":{"host":{"a.t.example.":{"ttl":"2147483648","address":["10.0.0.5"]}}}},"source":{"0.0.0.0/0":{"host-set":"w"}}`},
		{"default-ttl", `"default-ttl":"2147483648"`},
		{"soa minimum", `"soa":{"minimum":"2147483648"}`},
	} {
		cfg := `{"service":{"geodns":{"enabled":"true","zone":["t.example."],"nameserver":["10.0.0.1"],` + tc.cfg + `}}}`
		if _, err := parseConfig(cfg); err == nil {
			t.Errorf("%s of 2147483648 parsed without error", tc.what)
		} else if !strings.Contains(err.Error(), "2147483647") {
			t.Errorf("%s error %q does not name the 2147483647 bound", tc.what, err)
		}
	}

	// RFC requirement: RFC1035-2.3.4-1 negative -- 2147483647 is the largest
	// positive value of a signed 32 bit number, so it is inside the limit rather
	// than over it, and it reaches the wire unaltered.
	st := ttlState(t)
	rr := firstOfType(t, walkRRs(t, answerA(st, "max.t.example.")), dns.TypeA)
	if rr.ttl != maxTTL {
		t.Errorf("the ceiling TTL serves as %d, want %d", rr.ttl, maxTTL)
	}
}

// VALIDATES: RDLENGTH counts the octets of the RDATA that follows it, for a
// fixed-width type at two widths (A is 4, AAAA is 16) and for a variable-width
// one whose two instances differ. walkRRs consumes the whole message by
// following those lengths, so a wrong one leaves octets unread and fails.
// PREVENTS: a length field that agrees with nothing. RDLENGTH is how every
// reader finds the record after this one, so an RDATA that is not the length it
// declares desynchronises the rest of the message rather than corrupting one
// record.
func TestRFC1035_RDLengthCountsTheRDataOctets(t *testing.T) {
	t.Parallel()
	st := ttlState(t)

	// RFC requirement: RFC1035-4.1.3-2 positive -- "RDLENGTH        an unsigned
	// 16 bit integer that specifies the length in octets of the RDATA field."
	four := firstOfType(t, walkRRs(t, answerA(st, "low.t.example.")), dns.TypeA)
	if four.rdlength != 4 || len(four.rdata) != 4 {
		t.Errorf("A record declares RDLENGTH %d over %d octets, want 4 over 4", four.rdlength, len(four.rdata))
	}

	r := new(dns.Msg)
	r.SetQuestion("six.t.example.", dns.TypeAAAA)
	msg := new(dns.Msg)
	msg.SetReply(r)
	answerQuestions(msg, r, st, netip.MustParseAddr("203.0.113.7"))
	sixteen := firstOfType(t, walkRRs(t, msg), dns.TypeAAAA)
	if sixteen.rdlength != 16 || len(sixteen.rdata) != 16 {
		t.Errorf("AAAA record declares RDLENGTH %d over %d octets, want 16 over 16", sixteen.rdlength, len(sixteen.rdata))
	}
	if four.rdlength == sixteen.rdlength {
		t.Fatal("both record types declared the same RDLENGTH, so the field does not track the RDATA")
	}

	// RFC requirement: RFC1035-4.1.3-2 negative -- the field is a length, not a
	// per-type constant. The SOA of a negative answer carries two domain names in
	// its RDATA, so its RDLENGTH is neither 4 nor 16 and it still equals the
	// octets that follow.
	soa := firstOfType(t, walkRRs(t, answerA(st, "absent.t.example.")), dns.TypeSOA)
	if int(soa.rdlength) != len(soa.rdata) {
		t.Errorf("SOA declares RDLENGTH %d over %d octets", soa.rdlength, len(soa.rdata))
	}
	if soa.rdlength == 4 || soa.rdlength == 16 {
		t.Errorf("SOA RDLENGTH is %d, the same as a fixed-width type; this reply did not exercise a variable length", soa.rdlength)
	}
}
