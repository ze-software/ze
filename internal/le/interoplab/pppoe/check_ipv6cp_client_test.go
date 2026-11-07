// Design: docs/labs/pppoe-interop.md -- the IPv6CP client scenarios' trace
// readers, proved on accel-ppp 1.14.0 trace lines.
package pppoe

import (
	"strings"
	"testing"
)

// accelIPv6CPTrace is an accel-ppp 1.14.0 verbose trace of an IPv6CP exchange
// in which accel-ppp Naks Ze's EUI-64 identifier with 0:0:0:5a and Acks the
// request that adopts it. The line shapes follow ppp/ppp_ipv6cp.c
// (ipv6cp_recv_conf_req, send_conf_nak, send_conf_ack, send_conf_req) and
// ppp/ipv6cp_opt_intfid.c (ipaddr_print).
const accelIPv6CPTrace = `[2026-11-07 10:00:01.000]: info: ppp0: send [CHAP Success id=1 "Authentication succeeded"]
[2026-11-07 10:00:01.100]: info: ppp0: send [IPV6CP ConfReq id=1 <addr 0:0:0:1>]
[2026-11-07 10:00:01.110]: info: ppp0: recv [IPV6CP ConfReq id=7 <addr 42c:aff:fe0b:2>]
[2026-11-07 10:00:01.111]: info: ppp0: send [IPV6CP ConfNak id=7 <addr 0:0:0:5a>]
[2026-11-07 10:00:01.120]: info: ppp0: recv [IPV6CP ConfAck id=1 <addr 0:0:0:1>]
[2026-11-07 10:00:01.130]: info: ppp0: recv [IPV6CP ConfReq id=8 <addr 0:0:0:5a>]
[2026-11-07 10:00:01.131]: info: ppp0: send [IPV6CP ConfAck id=8]
`

// TestAccelIPv6CPNakAdoption checks that the trace reader accepts the
// exchange AC-1 and AC-2 need and refuses each way it can fall short: no
// request, a zero or pre-suggested identifier, a second option, a client that
// ignores the Nak, an unacknowledged adoption, and a client that never Acked
// accel-ppp's own request.
func TestAccelIPv6CPNakAdoption(t *testing.T) {
	if err := accelIPv6CPNakAdoption(accelIPv6CPTrace); err != nil {
		t.Fatalf("accelIPv6CPNakAdoption(good trace) = %v, want nil", err)
	}
	cases := []struct {
		name    string
		trace   string
		message string
	}{
		{"no request", "send [CHAP Success id=1]\n", "received no Configure-Request"},
		{"zero identifier",
			strings.Replace(accelIPv6CPTrace, "<addr 42c:aff:fe0b:2>", "<addr 0:0:0:0>", 1), "zero Interface-Identifier"},
		{"first already suggested",
			strings.Replace(accelIPv6CPTrace, "<addr 42c:aff:fe0b:2>", "<addr 0:0:0:5a>", 1), "no Nak was exercised"},
		{"second option",
			strings.Replace(accelIPv6CPTrace, "<addr 42c:aff:fe0b:2>]", "<addr 42c:aff:fe0b:2> < 3 4 0 0 >]", 1), "exactly one option"},
		{"Nak ignored",
			strings.Replace(accelIPv6CPTrace, "id=8 <addr 0:0:0:5a>", "id=8 <addr 42c:aff:fe0b:2>", 1), "not the suggested"},
		{"adoption not acked",
			strings.Replace(accelIPv6CPTrace, "send [IPV6CP ConfAck id=8]", "send [IPV6CP ConfNak id=8 <addr 0:0:0:5a>]", 1), "did not acknowledge Ze's request"},
		{"accel request not acked",
			strings.Replace(accelIPv6CPTrace, "recv [IPV6CP ConfAck id=1", "recv [IPV6CP ConfNak id=1", 1), "Ze did not acknowledge"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := accelIPv6CPNakAdoption(tc.trace)
			if err == nil {
				t.Fatal("accelIPv6CPNakAdoption = nil, want a refusal")
			}
			if !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("accelIPv6CPNakAdoption = %q, want it to contain %q", err, tc.message)
			}
		})
	}
}

// accelIPv6CPRejectedTrace is an accel-ppp 1.14.0 trace with ipv6=deny: Ze's
// one IPv6CP request draws one Protocol-Reject (ppp/ppp_ipv6cp.c, ipv6cp_recv,
// then ppp/ppp_lcp.c, lcp_send_proto_rej) and IPCP carries on.
const accelIPv6CPRejectedTrace = `[2026-11-07 10:00:01.000]: info: ppp0: send [CHAP Success id=1 "Authentication succeeded"]
[2026-11-07 10:00:01.100]: warn: ppp0: IPV6CP: discarding packet
[2026-11-07 10:00:01.101]: info: ppp0: send [LCP ProtoRej id=3 <8057>]
[2026-11-07 10:00:01.200]: info: ppp0: recv [IPCP ConfAck id=1 <addr 10.11.0.1>]
`

// TestAccelIPv6CPRejectedEvidence checks that the reader accepts one
// Protocol-Reject of IPv6CP on one session, and refuses a client that kept
// sending IPv6CP, a terminated session, a second session, and an IPv6CP
// negotiation.
func TestAccelIPv6CPRejectedEvidence(t *testing.T) {
	if err := accelIPv6CPRejectedEvidence(accelIPv6CPRejectedTrace); err != nil {
		t.Fatalf("accelIPv6CPRejectedEvidence(good trace) = %v, want nil", err)
	}
	cases := []struct {
		name    string
		trace   string
		message string
	}{
		{"no reject", strings.Replace(accelIPv6CPRejectedTrace, "<8057>", "<80fd>", 1), "sent 0 Protocol-Rejects"},
		{"kept sending", accelIPv6CPRejectedTrace + "info: ppp0: send [LCP ProtoRej id=4 <8057>]\n", "sent 2 Protocol-Rejects"},
		{"terminated", accelIPv6CPRejectedTrace + "info: ppp0: recv [LCP TermReq id=2]\n", "terminated"},
		{"second session", accelIPv6CPRejectedTrace + "info: ppp1: send [CHAP Success id=1]\n", "authenticated 2 sessions"},
		{"negotiated", accelIPv6CPRejectedTrace + "info: ppp0: send [IPV6CP ConfReq id=1 <addr 0:0:0:1>]\n", "negotiated IPv6CP"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := accelIPv6CPRejectedEvidence(tc.trace)
			if err == nil {
				t.Fatal("accelIPv6CPRejectedEvidence = nil, want a refusal")
			}
			if !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("accelIPv6CPRejectedEvidence = %q, want it to contain %q", err, tc.message)
			}
		})
	}
}
