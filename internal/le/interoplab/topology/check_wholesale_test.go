package topology

import (
	"strings"
	"testing"
)

// TestRenegotiatedVerdict proves the access-wholesale and pppd checkers refuse
// an LNS session that accepted the proxy LCP, never opened, or did not run
// the LNS's own CHAP.
func TestRenegotiatedVerdict(t *testing.T) {
	text := func(s string) *string { return &s }
	chap := text("chap-md5")
	for _, tc := range []struct {
		name   string
		detail proxyLCPDetail
		want   string
	}{
		{"renegotiated", proxyLCPDetail{l2tpSession: l2tpSession{Auth: chap}, ProxyLCP: text("renegotiated")}, ""},
		{"accepted", proxyLCPDetail{l2tpSession: l2tpSession{Auth: chap}, ProxyLCP: text("accepted")}, "want renegotiated"},
		{"none", proxyLCPDetail{l2tpSession: l2tpSession{Auth: chap}, ProxyLCP: text("none")}, "want renegotiated"},
		{"null", proxyLCPDetail{l2tpSession: l2tpSession{Auth: chap}}, "proxy-lcp null"},
		{"pap", proxyLCPDetail{l2tpSession: l2tpSession{Auth: text("pap")}, ProxyLCP: text("renegotiated")}, "want chap-md5"},
	} {
		err := renegotiatedVerdict(tc.detail)
		if tc.want == "" {
			if err != nil {
				t.Errorf("%s: %v", tc.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", tc.name, err, tc.want)
		}
	}
}

// TestPPPDRenegotiationVerdict proves the pppd log verdict needs a second LCP
// negotiation: a log with one, as an accepted proxy leaves, is refused.
func TestPPPDRenegotiationVerdict(t *testing.T) {
	once := "rcvd [LCP ConfReq id=0x1]\nsent [LCP ConfAck id=0x1]\nCHAP authentication succeeded\nlocal  IP address 203.0.113.1\n"
	twice := "rcvd [LCP ConfReq id=0x1]\nsent [LCP ConfAck id=0x1]\nrcvd [LCP ConfReq id=0x1]\nsent [LCP ConfAck id=0x1]\nCHAP authentication succeeded\nlocal  IP address 203.0.113.1\n"
	if err := pppdRenegotiationVerdict(twice); err != nil {
		t.Fatalf("two negotiations refused: %v", err)
	}
	if err := pppdRenegotiationVerdict(once); err == nil {
		t.Fatal("one negotiation accepted")
	}
	if err := pppdRenegotiationVerdict(strings.Replace(twice, "CHAP authentication succeeded", "", 1)); err == nil {
		t.Fatal("no CHAP success accepted")
	}
}

// TestAccelRenegotiationVerdict proves the access-wholesale-accel-lns checker
// needs every step of the renegotiation in accel-ppp's log: the CPE's own
// Configure-Request, its Configure-Ack of accel-ppp's request, a CHAP
// Response naming the subscriber and the Success. A log missing any one, as a
// CPE that ignored accel-ppp's Configure-Request leaves, is refused.
func TestAccelRenegotiationVerdict(t *testing.T) {
	lines := map[string]string{
		"confreq":  "[2026-11-07 10:00:01]: info: ppp0: recv [LCP ConfReq id=1 <mru 1492> <magic 1a2b3c4d>]",
		"confack":  "[2026-11-07 10:00:01]: info: ppp0: recv [LCP ConfAck id=1 <auth CHAP-md5> <mru 1492> <magic 0a0b0c0d>]",
		"response": `[2026-11-07 10:00:02]: info: ppp0: recv [CHAP Response id=1 <0123456789abcdef0123456789abcdef>, name="cpe@lab.example.net"]`,
		"success":  `[2026-11-07 10:00:02]: info: ppp0: send [CHAP Success id=1 "Authentication succeeded"]`,
	}
	order := []string{"confreq", "confack", "response", "success"}
	all := func(skip string) string {
		var out []string
		for _, name := range order {
			if name != skip {
				out = append(out, lines[name])
			}
		}
		return strings.Join(out, "\n")
	}
	if err := accelRenegotiationVerdict(all(""), subscriberUser); err != nil {
		t.Fatalf("a full renegotiation: %v", err)
	}
	want := map[string]string{
		"confreq":  "no LCP Configure-Request",
		"confack":  "no LCP Configure-Ack",
		"response": "no CHAP Response",
		"success":  "no CHAP Success",
	}
	for _, name := range order {
		err := accelRenegotiationVerdict(all(name), subscriberUser)
		if err == nil || !strings.Contains(err.Error(), want[name]) {
			t.Errorf("without %s: err = %v, want %q", name, err, want[name])
		}
	}
	other := strings.Replace(all(""), "cpe@lab.example.net", "other@lab.example.net", 1)
	if err := accelRenegotiationVerdict(other, subscriberUser); err == nil || !strings.Contains(err.Error(), "no CHAP Response") {
		t.Errorf("another user's CHAP Response: err = %v, want no CHAP Response", err)
	}
}
