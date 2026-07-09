package dns

import (
	"strings"
	"sync/atomic"
	"testing"

	mdns "github.com/miekg/dns"
)

// dnssecUpstream is a fake validating upstream: it records whether the last
// query carried the EDNS0 DO bit, returns SERVFAIL for "bogus.test." (a
// validating resolver's response to a broken DNSSEC chain), and NOERROR with
// AuthenticatedData set for "secure.test." (a validated answer).
func dnssecUpstream(sawDO *atomic.Bool) mdns.Handler {
	return mdns.HandlerFunc(func(w mdns.ResponseWriter, r *mdns.Msg) {
		if opt := r.IsEdns0(); opt != nil && opt.Do() {
			sawDO.Store(true)
		}
		m := new(mdns.Msg)
		m.SetReply(r)
		if len(r.Question) > 0 && strings.EqualFold(r.Question[0].Name, "bogus.test.") {
			m.SetRcode(r, mdns.RcodeServerFailure)
			_ = w.WriteMsg(m)
			return
		}
		// secure.test.: a validated A answer.
		m.AuthenticatedData = true
		if len(r.Question) > 0 {
			m.Answer = append(m.Answer, &mdns.A{
				Hdr: mdns.RR_Header{Name: r.Question[0].Name, Rrtype: mdns.TypeA, Class: mdns.ClassINET, Ttl: 300},
				A:   []byte{203, 0, 113, 7},
			})
		}
		_ = w.WriteMsg(m)
	})
}

// VALIDATES: AC-5 -- with dnssec-validation strict, a broken chain (upstream
// SERVFAIL) is REJECTED as an error, not returned as an empty result.
func TestDNSSECStrictRejectsBogus(t *testing.T) {
	var sawDO atomic.Bool
	addr, cleanup := testDNSServer(t, dnssecUpstream(&sawDO))
	defer cleanup()

	r := NewResolver(ResolverConfig{Server: addr, DNSSECValidation: "strict"})
	defer r.Close()
	if _, err := r.ResolveA("bogus.test"); err == nil {
		t.Fatal("strict dnssec-validation accepted a bogus (SERVFAIL) answer, want error")
	}
	if !sawDO.Load() {
		t.Fatal("query did not set the EDNS0 DO bit under validation")
	}
}

// VALIDATES: AC-5 -- with dnssec-validation permissive, a broken chain is logged
// (not fatal): the call returns without an error and with an empty result.
func TestDNSSECPermissiveLogsBogus(t *testing.T) {
	var sawDO atomic.Bool
	addr, cleanup := testDNSServer(t, dnssecUpstream(&sawDO))
	defer cleanup()

	r := NewResolver(ResolverConfig{Server: addr, DNSSECValidation: "permissive"})
	defer r.Close()
	recs, err := r.ResolveA("bogus.test")
	if err != nil {
		t.Fatalf("permissive dnssec-validation returned an error, want soft handling: %v", err)
	}
	if len(recs) != 0 {
		t.Fatalf("permissive dnssec-validation returned records for a SERVFAIL: %v", recs)
	}
}

// VALIDATES: AC-5 -- a secure (validated) answer resolves normally under strict.
func TestDNSSECStrictResolvesSecure(t *testing.T) {
	var sawDO atomic.Bool
	addr, cleanup := testDNSServer(t, dnssecUpstream(&sawDO))
	defer cleanup()

	r := NewResolver(ResolverConfig{Server: addr, DNSSECValidation: "strict"})
	defer r.Close()
	recs, err := r.ResolveA("secure.test")
	if err != nil {
		t.Fatalf("strict dnssec-validation rejected a secure answer: %v", err)
	}
	if len(recs) != 1 || recs[0] != "203.0.113.7" {
		t.Fatalf("unexpected records for secure answer: %v", recs)
	}
}

// VALIDATES: AC-5 -- default (off) does not set the DO bit and treats SERVFAIL
// as today (empty result, no error).
func TestDNSSECOffIsUnchanged(t *testing.T) {
	var sawDO atomic.Bool
	addr, cleanup := testDNSServer(t, dnssecUpstream(&sawDO))
	defer cleanup()

	r := NewResolver(ResolverConfig{Server: addr}) // default off
	defer r.Close()
	recs, err := r.ResolveA("bogus.test")
	if err != nil {
		t.Fatalf("off mode returned an error for SERVFAIL: %v", err)
	}
	if len(recs) != 0 {
		t.Fatalf("off mode returned records for SERVFAIL: %v", recs)
	}
	if sawDO.Load() {
		t.Fatal("off mode set the EDNS0 DO bit, want it clear")
	}
}

// VALIDATES: AC-5 -- the pure decision function: strict rejects SERVFAIL,
// permissive warns, off is silent; a secure (AD) answer passes in every mode.
func TestDNSSECDecision(t *testing.T) {
	cases := []struct {
		mode       string
		rcode      int
		ad         bool
		wantReject bool
		wantWarn   bool
	}{
		{"off", mdns.RcodeServerFailure, false, false, false},
		{"strict", mdns.RcodeServerFailure, false, true, false},
		{"permissive", mdns.RcodeServerFailure, false, false, true},
		{"strict", mdns.RcodeSuccess, true, false, false},
		{"permissive", mdns.RcodeSuccess, true, false, false},
		{"strict", mdns.RcodeSuccess, false, false, false}, // insecure/unsigned zone: accepted
	}
	for _, tc := range cases {
		warn, reject := dnssecDecision(tc.rcode, tc.ad, tc.mode)
		if (reject != nil) != tc.wantReject {
			t.Errorf("mode=%s rcode=%d ad=%v: reject=%v, wantReject=%v", tc.mode, tc.rcode, tc.ad, reject, tc.wantReject)
		}
		if (warn != "") != tc.wantWarn {
			t.Errorf("mode=%s rcode=%d ad=%v: warn=%q, wantWarn=%v", tc.mode, tc.rcode, tc.ad, warn, tc.wantWarn)
		}
	}
}
