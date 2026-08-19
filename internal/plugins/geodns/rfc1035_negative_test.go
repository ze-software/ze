// Design: docs/architecture/dns/geodns.md -- the answer policy's three response
// codes, and which of them a query draws
// RFC: rfc/short/rfc1035.md -- RCODE 3 (Name Error), RCODE 5 (Refused), the AA
// bit. RFC 2308 Section 3 is cited in prose: it is not enrolled and has no
// summary, so it carries no requirement id to tag.

package geodns

import (
	"net"
	"testing"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/dnsserver"
)

// negWriter is a dns.ResponseWriter keeping the single reply the harness writes,
// with the client address as a field so one handler can be driven from several
// source prefixes. The AA bit lives on this path and nowhere else:
// shapeAuthoritative owns it, and answerQuestions never touches it.
type negWriter struct {
	dns.ResponseWriter
	client  string
	written *dns.Msg
}

func (n *negWriter) WriteMsg(m *dns.Msg) error { n.written = m; return nil }
func (n *negWriter) RemoteAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP(n.client), Port: 53000}
}

// negZone publishes a two-host-set configuration and returns the full harness
// handler, the composition register.go binds to the listeners.
//
// The two host sets are what makes the no-data case reachable: "eu-only" is
// configured in one of them and not the other, so the same name has an address
// for a client in 203.0.113.0/24 and none for a client in 198.51.100.0/24.
// "a.b.t.example." is configured so that "b.t.example." exists as an interior
// node owning no record of its own.
func negZone(t *testing.T) dns.HandlerFunc {
	t.Helper()
	cfg, err := parseConfig(`{"service":{"geodns":{"enabled":"true","zone":["t.example."],` +
		`"nameserver":["10.0.0.1"],` +
		`"host-set":{` +
		`"us":{"host":{"www.t.example.":{"address":["10.0.0.5"]},"a.b.t.example.":{"address":["10.0.0.7"]}}},` +
		`"eu":{"host":{"www.t.example.":{"address":["10.0.0.6"]},"eu-only.t.example.":{"address":["10.0.0.8"]}}}},` +
		`"source":{"198.51.100.0/24":{"host-set":"us"},"203.0.113.0/24":{"host-set":"eu"}}}}}`)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	storeApplied(cfg, 1)
	return dnsserver.Authoritative(nil, answerQuery, nil)
}

// askFrom drives one question through the harness as a query from client.
func askFrom(t *testing.T, h dns.HandlerFunc, client, qname string, qtype uint16) *dns.Msg {
	t.Helper()
	r := new(dns.Msg)
	r.SetQuestion(qname, qtype)
	w := &negWriter{client: client}
	h(w, r)
	if w.written == nil {
		t.Fatalf("no reply written for %s %s from %s", qname, dns.TypeToString[qtype], client)
	}
	return w.written
}

// VALIDATES: the RCODE, the AA bit and the Authority section a query draws,
// against where its name sits and what the client's own host set holds for it.
// PREVENTS: the three answers collapsing into one, in either direction. A test
// reading the RCODE alone passes against a REFUSED reply that still claims
// authority. A name error for a name configured in ANOTHER host set breaks what
// geodns exists to do -- it tells every client the name does not exist because
// their own view of it is empty.
func TestRFC1035_ResponseCodeByNameAndClient(t *testing.T) {
	handler := negZone(t)

	const us = "198.51.100.7" // maps to host set "us"

	for _, tc := range []struct {
		what      string
		client    string
		qname     string
		qtype     uint16
		rcode     int
		aa        bool
		answers   bool // records expected in the Answer section
		soaInAuth bool // the zone SOA expected in the Authority section
	}{
		{"in zone, this client has data", us, "www.t.example.", dns.TypeA, dns.RcodeSuccess, true, true, false},
		{"in zone, the zone owns no such name", us, "nope.t.example.", dns.TypeA, dns.RcodeNameError, true, false, true},
		{"in zone, configured for another client only", us, "eu-only.t.example.", dns.TypeA, dns.RcodeSuccess, true, false, true},
		{"in zone, an interior node owning no record", us, "b.t.example.", dns.TypeA, dns.RcodeSuccess, true, false, true},
		{"in zone, a type the name holds no record of", us, "www.t.example.", dns.TypeMX, dns.RcodeSuccess, true, false, true},
		{"under no served zone", us, "elsewhere.invalid.", dns.TypeA, dns.RcodeRefused, false, false, false},
		{"a sibling of the served zone", us, "evilt.example.", dns.TypeA, dns.RcodeRefused, false, false, false},
	} {
		t.Run(tc.what, func(t *testing.T) {
			reply := askFrom(t, handler, tc.client, tc.qname, tc.qtype)

			// RFC requirement: RFC1035-4.1.1-3 positive -- "3               Name Error - Meaningful
			// only for responses from an authoritative name server, this code signifies that the
			// domain name referenced in the query does not exist." nope.t.example. is inside a zone
			// geodns is authoritative for and no host set configures it, so it does not exist and
			// the reply carries RCODE 3.
			// RFC requirement: RFC1035-4.1.1-3 negative -- the code is withheld from every name
			// that DOES exist, whatever this client can see of it. eu-only.t.example. is configured
			// in another host set, b.t.example. is the interior node a.b.t.example. hangs from, and
			// www.t.example. holds no MX; all three exist, so all three are NOERROR with no data.
			// It is withheld from elsewhere.invalid. too, for the opposite reason: RCODE 3 is
			// "meaningful only for responses from an authoritative name server", and geodns is not
			// an authority for that name.
			if reply.Rcode != tc.rcode {
				t.Errorf("Rcode = %s, want %s", dns.RcodeToString[reply.Rcode], dns.RcodeToString[tc.rcode])
			}
			// RFC requirement: RFC1035-4.1.1-2 negative -- "AA              Authoritative Answer -
			// this bit is valid in responses, and specifies that the responding name server is an
			// authority for the domain name in question section." The bit is withheld from the two
			// names in the table geodns serves no zone for, including the sibling name whose
			// characters end with the zone's while its labels do not nest inside it.
			if reply.Authoritative != tc.aa {
				t.Errorf("AA = %v, want %v", reply.Authoritative, tc.aa)
			}
			if got := len(reply.Answer) > 0; got != tc.answers {
				t.Errorf("Answer section non-empty = %v, want %v (%v)", got, tc.answers, reply.Answer)
			}
			// RFC 2308 Section 3: "Name servers authoritative for a zone MUST
			// include the SOA record of the zone in the authority section of the
			// response when reporting an NXDOMAIN or indicating that no data of the
			// requested type exists.  This is required so that the response may be
			// cached."
			if got := hasSOA(reply.Ns); got != tc.soaInAuth {
				t.Errorf("SOA in Authority = %v, want %v (%v)", got, tc.soaInAuth, reply.Ns)
			}
		})
	}
}

// VALIDATES: an edns0-only configuration, asked by a client that sent no
// client-subnet option, still refuses a name it serves no zone for.
// PREVENTS: the client-resolution failure short-circuiting the answer policy.
// Which zone owns a name does not depend on the client, so a path that returns
// before consulting the zones answers every name alike -- and the harness then
// stamps AA on an empty NOERROR for a namespace Ze holds no zone for, which is
// the claim the refused answer exists to withhold.
func TestRFC1035_EDNS0OnlyWithoutSubnetStillRefusesOutOfZone(t *testing.T) {
	cfg, err := parseConfig(`{"service":{"geodns":{"enabled":"true","zone":["t.example."],` +
		`"nameserver":["10.0.0.1"],"client-ip-source":"edns0",` +
		`"host-set":{"any":{"host":{"www.t.example.":{"address":["10.0.0.5"]}}}},` +
		`"source":{"0.0.0.0/0":{"host-set":"any"}}}}}`)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	storeApplied(cfg, 1)
	handler := dnsserver.Authoritative(nil, answerQuery, nil)

	out := askFrom(t, handler, "198.51.100.7", "elsewhere.invalid.", dns.TypeA)
	if out.Rcode != dns.RcodeRefused {
		t.Errorf("out-of-zone rcode = %s, want REFUSED", dns.RcodeToString[out.Rcode])
	}
	if out.Authoritative {
		t.Error("out-of-zone reply has AA set with no client resolved")
	}

	// The in-zone name still exists, so it is a no-data answer rather than a name
	// error: only the host-set selection failed, not the zone lookup.
	in := askFrom(t, handler, "198.51.100.7", "www.t.example.", dns.TypeA)
	if in.Rcode != dns.RcodeSuccess || len(in.Answer) != 0 || !hasSOA(in.Ns) {
		t.Errorf("in-zone rcode=%s answer=%v ns=%v, want NOERROR + empty answer + SOA",
			dns.RcodeToString[in.Rcode], in.Answer, in.Ns)
	}

	absent := askFrom(t, handler, "198.51.100.7", "nope.t.example.", dns.TypeA)
	if absent.Rcode != dns.RcodeNameError {
		t.Errorf("in-zone absent rcode = %s, want NXDOMAIN", dns.RcodeToString[absent.Rcode])
	}
}

// VALIDATES: the same name draws data for one client and a no-data answer for
// another, and neither answer is a name error.
// PREVENTS: the existence test being read from the client's own host set. That
// reading gives geodns's whole purpose the wrong response code: a name split
// across host sets would be declared non-existent to every client but one.
func TestRFC1035_NameExistenceIsNotPerClient(t *testing.T) {
	handler := negZone(t)

	eu := askFrom(t, handler, "203.0.113.9", "eu-only.t.example.", dns.TypeA)
	if eu.Rcode != dns.RcodeSuccess || firstA(eu) != "10.0.0.8" {
		t.Fatalf("client in the eu prefix: rcode=%s A=%q, want NOERROR 10.0.0.8", dns.RcodeToString[eu.Rcode], firstA(eu))
	}

	us := askFrom(t, handler, "198.51.100.7", "eu-only.t.example.", dns.TypeA)
	if us.Rcode != dns.RcodeSuccess {
		t.Errorf("client in the us prefix: rcode = %s, want NOERROR (the name exists, this client has no data for it)", dns.RcodeToString[us.Rcode])
	}
	if len(us.Answer) != 0 {
		t.Errorf("client in the us prefix: Answer = %v, want empty", us.Answer)
	}
	if !hasSOA(us.Ns) {
		t.Error("client in the us prefix: no SOA in Authority; the no-data answer must carry it to be cacheable")
	}

	// A client no source prefix covers sees the same shape: the name exists, it
	// has no data for them.
	none := askFrom(t, handler, "192.0.2.5", "eu-only.t.example.", dns.TypeA)
	if none.Rcode != dns.RcodeSuccess || len(none.Answer) != 0 || !hasSOA(none.Ns) {
		t.Errorf("client under no source prefix: rcode=%s answer=%v ns=%v, want NOERROR + empty answer + SOA",
			dns.RcodeToString[none.Rcode], none.Answer, none.Ns)
	}
}
