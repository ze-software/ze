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
