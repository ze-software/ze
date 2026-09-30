package dns

import (
	"slices"
	"sync/atomic"
	"testing"

	mdns "github.com/miekg/dns"
)

// VALIDATES: RFC 4035 section 4.1 in BOTH security-aware modes (permissive and
// strict): every query carries an EDNS OPT with DO set, and the 4096 octets its
// sender's UDP payload size advertises is what the resolver then accepts over
// UDP -- a 2900-octet answer (above the 512 and 1220 floors) comes back whole
// and untruncated.
// PREVENTS: a mode that drops DO, or an advertised size the receive path does
// not honor, so the upstream sends a size the stub then truncates or refuses.
func TestRFC4035SecurityAwareQueryInEveryValidatingMode(t *testing.T) {
	const answers = 110 // 110 uncompressed A records of 26 octets each: about 2900 octets on the wire
	var last atomic.Pointer[mdns.Msg]
	addr, cleanup := testDNSServer(t, rfc4035Upstream(&last, func(r *mdns.Msg) *mdns.Msg {
		m := new(mdns.Msg)
		m.SetReply(r)
		m.Compress = false
		for i := range answers {
			m.Answer = append(m.Answer, aRecord(r.Question[0].Name, []byte{198, 51, byte(i / 250), byte(i%250 + 1)}))
		}
		return m
	}))
	defer cleanup()

	for _, mode := range []string{"permissive", "strict"} {
		t.Run(mode, func(t *testing.T) {
			r := NewResolver(ResolverConfig{Server: addr, DNSSECValidation: mode})
			defer r.Close()
			records, err := r.ResolveA("large.test")
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			q := last.Load()
			if q == nil {
				t.Fatal("upstream saw no query")
			}
			opt := q.IsEdns0()
			// RFC requirement: RFC4035-4.1-1 positive -- in permissive and in strict mode the query carries an EDNS OPT pseudo-RR with the DO bit set.
			if opt == nil {
				t.Fatalf("%s query carries no EDNS OPT pseudo-RR", mode)
			}
			if !opt.Do() {
				t.Errorf("%s query has the DO bit clear", mode)
			}
			// RFC requirement: RFC4035-4.1-4 positive -- in both modes the OPT advertises 4096 octets, and a 2900-octet UDP answer is then accepted whole: every one of its 110 records is returned.
			if got := opt.UDPSize(); got != 4096 {
				t.Errorf("%s query advertises %d octets, want 4096", mode, got)
			}
			if len(records) != answers {
				t.Errorf("%s: %d of the %d records in the 2900-octet answer came back", mode, len(records), answers)
			}
		})
	}
}

// VALIDATES: RFC 4035 section 4.6 -- over the plain UDP channel the resolver
// uses, the CD and AD bits of a RESPONSE change nothing: a NOERROR answer with
// AD and CD set returns exactly the records of the same answer with both clear,
// and a SERVFAIL is refused in strict mode whether it carries AD, CD or both.
// PREVENTS: a stub that trusts an AD bit anyone on the path could set, or reads
// a response's CD bit as permission to skip the validation failure.
func TestRFC4035ResponseCDAndADBitsChangeNothing(t *testing.T) {
	var last atomic.Pointer[mdns.Msg]
	addr, cleanup := testDNSServer(t, rfc4035Upstream(&last, func(r *mdns.Msg) *mdns.Msg {
		m := new(mdns.Msg)
		m.SetReply(r)
		name := r.Question[0].Name
		switch name {
		case "fail-ad.test.", "fail-cd.test.", "fail-both.test.":
			m.SetRcode(r, mdns.RcodeServerFailure)
		default:
			m.Answer = append(m.Answer, aRecord(name, []byte{203, 0, 113, 9}))
		}
		m.AuthenticatedData = name == "ok-both.test." || name == "fail-ad.test." || name == "fail-both.test."
		m.CheckingDisabled = name == "ok-both.test." || name == "fail-cd.test." || name == "fail-both.test."
		return m
	}))
	defer cleanup()

	r := NewResolver(ResolverConfig{Server: addr, DNSSECValidation: "strict"})
	defer r.Close()

	// RFC requirement: RFC4035-4.6-3 positive -- a NOERROR answer with AD=1 and CD=1 returns exactly the records of the same answer with AD=0 and CD=0: neither bit changes the result.
	plain, err := r.ResolveA("ok-none.test")
	if err != nil {
		t.Fatalf("AD=0 CD=0 answer: %v", err)
	}
	flagged, err := r.ResolveA("ok-both.test")
	if err != nil {
		t.Fatalf("AD=1 CD=1 answer rejected: %v", err)
	}
	if !slices.Equal(plain, flagged) || len(plain) != 1 {
		t.Errorf("AD/CD-set answer %v differs from the clear answer %v", flagged, plain)
	}

	// RFC requirement: RFC4035-4.6-3 negative -- a SERVFAIL is refused under strict validation whether the response sets AD, CD or both: neither bit, received without a secure channel, makes the resolver accept it.
	for _, name := range []string{"fail-ad.test", "fail-cd.test", "fail-both.test"} {
		if records, err := r.ResolveA(name); err == nil {
			t.Errorf("%s: SERVFAIL accepted (%v) because of a response header bit", name, records)
		}
	}
}
