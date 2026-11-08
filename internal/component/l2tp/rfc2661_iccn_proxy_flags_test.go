package l2tp

// Design: docs/architecture/wire/l2tp.md -- RFC 2661 AVP flag proofs for the proxy ICCN
//
// Tagged proofs that the M and H bits of every AVP a realm call's ICCN
// carries follow RFC 2661 Section 4.4. TestRFC2661WrittenAVPFlags walks the
// bodies writeICCNBody emits with no proxy data; a PAP realm call toward a
// remote with a shared secret adds a Random Vector AVP and a hidden AVP 33,
// which that walk never reaches.
// Related: iccn_proxy.go, session_initiator.go, rfc2661_message_test.go.

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/core/callsink"
)

// rfc2661ProxyMandatoryBit is the M-bit RFC 2661 Section 4.4 fixes for each
// AVP a proxy ICCN appends after the required ICCN AVPs.
//
// RFC 2661 Section 4.4.1, Random Vector: "The M-bit for this AVP MUST be set
// to 1."
// RFC 2661 Section 4.4.5, AVPs 26 to 33: "The M-bit for this AVP MUST be set
// to 0."
var rfc2661ProxyMandatoryBit = map[AVPType]bool{
	AVPInitialReceivedLCPConfReq: false,
	AVPLastSentLCPConfReq:        false,
	AVPLastReceivedLCPConfReq:    false,
	AVPProxyAuthenType:           false,
	AVPProxyAuthenName:           false,
	AVPProxyAuthenChallenge:      false,
	AVPProxyAuthenID:             false,
	AVPProxyAuthenResponse:       false,
	AVPRandomVector:              true,
}

// RFC requirement: RFC2661-4.4-1 positive -- every AVP of the ICCN a PAP
// realm call writes toward a remote with a shared secret, and of the ICCN a
// CHAP realm call writes, whose Section 4.4 definition fixes M at 1 carries
// M=1, the Random Vector AVP included.
// RFC requirement: RFC2661-4.4-3 positive -- in those two ICCNs no AVP other
// than Proxy Authen Response (AVP 33) carries H=1, so the Random Vector AVP
// and every other AVP Section 4.4 forbids hiding is sent with H=0.
//
// TestRFC2661ProxyICCNAVPFlags walks every AVP of both proxy ICCN bodies and
// checks its M bit against the Section 4.4 definition of that AVP, and its H
// bit against the one AVP Ze hides.
//
// RFC 2661 Section 4.4.1, Random Vector: "The M-bit for this AVP MUST be set
// to 1.  This AVP MUST NOT be hidden (the H-bit MUST be 0)."
// RFC 2661 Section 4.4.5, Proxy Authen Response: "This AVP may be hidden (the
// H-bit may be 0 or 1)."
//
// VALIDATES: each AVP's M bit equals the bit its Section 4.4 definition
// fixes; only AVP 33 of the PAP ICCN carries H=1; the PAP ICCN carries the
// Random Vector and the hidden AVP 33 the walk is there to reach.
// PREVENTS: a Random Vector written with M=0 or H=1, or any AVP other than
// AVP 33 hidden, on the PAP proxy path TestRFC2661WrittenAVPFlags skips.
func TestRFC2661ProxyICCNAVPFlags(t *testing.T) {
	pap := callsink.ProxyAuth{Kind: callsink.AuthKindPAP, Identifier: 3, Response: []byte("s3cret-pass")}
	chap := callsink.ProxyAuth{Kind: callsink.AuthKindCHAP, Identifier: 7,
		Challenge: bytes.Repeat([]byte{0x11}, 16), Response: bytes.Repeat([]byte{0x22}, 16)}
	cases := []struct {
		name       string
		auth       callsink.ProxyAuth
		secret     string
		wantHidden int
	}{
		{"PAP with shared secret", pap, "tunnel-secret", 1},
		{"CHAP", chap, "", 0},
	}
	for _, c := range cases {
		proxy, reason, fits := newICCNProxy("cpe@example.net", testProxyLCP, c.auth, c.secret)
		if !fits {
			t.Fatalf("%s: proxy data does not fit", c.name)
		}
		if reason != "" {
			t.Fatalf("%s: proxy authentication not sent: %s", c.name, reason)
		}
		buf := make([]byte, BufSize)
		n := writeICCNBody(buf, 64000, 1, proxy)

		seen := map[AVPType]bool{}
		hidden := 0
		it := NewAVPIterator(buf[:n])
		for {
			vendorID, attrType, flags, _, ok := it.Next()
			if !ok {
				break
			}
			if vendorID != 0 {
				t.Fatalf("%s: vendor AVP %d emitted", c.name, vendorID)
			}
			seen[attrType] = true
			if flags&FlagHidden != 0 {
				if attrType != AVPProxyAuthenResponse {
					t.Fatalf("%s: AVP %d emitted with H=1, only AVP 33 may be hidden", c.name, attrType)
				}
				hidden++
			}
			want, known := rfc2661MandatoryBit[attrType]
			if !known {
				want, known = rfc2661ProxyMandatoryBit[attrType]
			}
			if !known {
				t.Fatalf("%s: AVP %d has no expected M-bit in the test tables", c.name, attrType)
			}
			if got := flags&FlagMandatory != 0; got != want {
				t.Fatalf("%s: AVP %d M=%v, RFC 2661 Section 4.4 wants %v", c.name, attrType, got, want)
			}
		}
		if err := it.Err(); err != nil {
			t.Fatalf("%s: iterate: %v", c.name, err)
		}
		if hidden != c.wantHidden {
			t.Fatalf("%s: %d hidden AVPs, want %d", c.name, hidden, c.wantHidden)
		}
		if !seen[AVPProxyAuthenResponse] {
			t.Fatalf("%s: no Proxy Authen Response AVP, the walk missed the proxy data", c.name)
		}
		if c.wantHidden > 0 && !seen[AVPRandomVector] {
			t.Fatalf("%s: no Random Vector AVP before the hidden AVP 33", c.name)
		}
	}
}
