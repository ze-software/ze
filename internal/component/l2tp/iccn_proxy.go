// Design: docs/architecture/l2tp/followup-l2tp-call.md -- realm relay, proxy LCP and authentication AVPs
// RFC: rfc/short/rfc2661.md -- RFC 2661 Section 4.4.5 (Proxy LCP and Authentication AVPs), Section 4.3 (hiding)
// Related: session_initiator.go -- writeICCNBody calls writeICCNProxyAVPs
// Related: hidden.go -- HiddenEncrypt, the Section 4.3 cipher AVP 33 uses for PAP

package l2tp

import (
	"crypto/rand"

	"github.com/ze-software/ze/internal/core/callsink"
)

// Proxy Authen Type values (RFC 2661 Section 4.4.5) Ze sends.
const (
	proxyAuthenTypeCHAP uint16 = 2
	proxyAuthenTypePAP  uint16 = 3
)

// randomVectorOctets is the Random Vector length Ze writes. RFC 2661 Section
// 4.4.1: "a random vector of at least 16 octets is recommended."
const randomVectorOctets = 16

// iccnProxyOctetsMax bounds the proxy AVPs one ICCN carries, so the body
// always fits the 1500-octet control buffer beside the header and the three
// required AVPs. A subscriber controls the LCP option bodies, so the bound is
// checked before writing, never assumed.
const iccnProxyOctetsMax = 1200

// Proxy authentication outcomes a realm call records for `show l2tp session`.
const (
	proxyAuthReasonNoSharedSecret = "no-shared-secret"
	proxyAuthReasonNoAuth         = "no-authentication"
	proxyAuthReasonTooLarge       = "proxy-data-too-large"
)

// iccnProxy is the proxy data a realm-relayed call carries in its ICCN. lcp
// is always written; auth only when sendAuth is true. secret is the remote's
// tunnel secret, present exactly when hiding AVP 33 for PAP is possible.
type iccnProxy struct {
	name     string
	lcp      callsink.ProxyLCP
	auth     callsink.ProxyAuth
	sendAuth bool
	secret   []byte
}

// newICCNProxy decides what proxy data a realm call sends, from the
// subscriber's exchange and the remote's tunnel secret. It returns the proxy
// value, or nil with a reason when the data cannot fit an ICCN. authReason is
// empty when proxy authentication is sent, and names why it is not otherwise.
func newICCNProxy(username string, lcp callsink.ProxyLCP, auth callsink.ProxyAuth, secret string) (p *iccnProxy, authReason string, fits bool) {
	p = &iccnProxy{name: username, lcp: lcp, auth: auth}
	switch auth.Kind {
	case callsink.AuthKindCHAP:
		p.sendAuth = true
	case callsink.AuthKindPAP:
		// RFC 2661 Section 4.4.5: "In the case of cleartext passwords, AVP
		// hiding is recommended." Ze sends a PAP password only hidden, and
		// hiding needs the tunnel secret (Section 4.3), so a remote without
		// one gets no proxy authentication and the LNS restarts it.
		if secret == "" {
			authReason = proxyAuthReasonNoSharedSecret
			break
		}
		p.sendAuth = true
		p.secret = []byte(secret)
	case callsink.AuthKindUnspecified:
		authReason = proxyAuthReasonNoAuth
	default:
		// The set is open to a future AuthKind this writer does not know:
		// send no proxy authentication rather than a wrong type.
		authReason = proxyAuthReasonNoAuth
	}
	if !p.fits() {
		return nil, authReason, false
	}
	return p, authReason, true
}

// fits reports whether every proxy AVP stays within one AVP's length and the
// whole set within iccnProxyOctetsMax.
func (p *iccnProxy) fits() bool {
	valueMax := AVPMaxLen - AVPHeaderLen
	values := [...][]byte{p.lcp.InitialReceived, p.lcp.LastSent, p.lcp.LastReceived}
	total := 0
	for _, v := range values {
		if len(v) > valueMax {
			return false
		}
		total += AVPHeaderLen + len(v)
	}
	if p.sendAuth {
		// The hidden subformat adds the 2-octet Original Length.
		for _, v := range [...][]byte{p.auth.Challenge, p.auth.Response} {
			if len(v)+hiddenLenField > valueMax {
				return false
			}
			total += AVPHeaderLen + hiddenLenField + len(v)
		}
		// AVPs 29 (2 octets), 30 (the name), 32 (2 octets), 36 (random).
		total += 4*AVPHeaderLen + 2 + len(p.name) + 2 + randomVectorOctets
		if len(p.name) > valueMax {
			return false
		}
	}
	return total <= iccnProxyOctetsMax
}

// writeICCNProxyAVPs writes the proxy LCP AVPs and, when p.sendAuth, the
// proxy authentication AVPs into buf at off, and returns the octets written.
// The caller MUST have checked p with fits.
//
// Wire layout, appended after the required ICCN AVPs (each AVP is a 6-octet
// header: M|H|rsvd|Length(10) at 0, Vendor ID at 2, Attribute Type at 4):
//
//	AVP 26 Initial Received LCP CONFREQ   M=0 H=0  options of the first CONFREQ received
//	AVP 27 Last Sent LCP CONFREQ          M=0 H=0  options of Ze's Acked CONFREQ
//	AVP 28 Last Received LCP CONFREQ      M=0 H=0  options of the subscriber's Acked CONFREQ
//	AVP 29 Proxy Authen Type              M=0 H=0  2 octets: 2 (CHAP) or 3 (PAP)
//	AVP 30 Proxy Authen Name              M=0 H=0  the username
//	AVP 31 Proxy Authen Challenge         M=0 H=0  CHAP only: the challenge sent
//	AVP 32 Proxy Authen ID                M=0 H=0  2 octets: 0, then the ID
//	AVP 36 Random Vector                  M=1 H=0  PAP only: 16 random octets
//	AVP 33 Proxy Authen Response          M=0 H=1 for PAP (hidden), H=0 for CHAP
func writeICCNProxyAVPs(buf []byte, off int, p *iccnProxy) int {
	start := off
	// RFC 2661 Section 4.4.5: "LCP CONFREQ is a copy of the body of the
	// initial CONFREQ received, starting at the first option within the body
	// of the LCP message."
	// RFC 2661 Section 4.4.5: "The M-bit for this AVP MUST be set to 0."
	off += WriteAVPBytes(buf, off, false, 0, AVPInitialReceivedLCPConfReq, p.lcp.InitialReceived)
	// RFC 2661 Section 4.4.5: "The LCP CONFREQ is a copy of the body of the
	// final CONFREQ sent to the client to complete LCP negotiation, starting
	// at the first option within the body of the LCP message."
	// RFC 2661 Section 4.4.5: "The M-bit for this AVP MUST be set to 0."
	off += WriteAVPBytes(buf, off, false, 0, AVPLastSentLCPConfReq, p.lcp.LastSent)
	// RFC 2661 Section 4.4.5: "The LCP CONFREQ is a copy of the body of the
	// final CONFREQ received from the client to complete LCP negotiation,
	// starting at the first option within the body of the LCP message."
	// RFC 2661 Section 4.4.5: "The M-bit for this AVP MUST be set to 0."
	off += WriteAVPBytes(buf, off, false, 0, AVPLastReceivedLCPConfReq, p.lcp.LastReceived)
	if !p.sendAuth {
		return off - start
	}
	if p.auth.Kind == callsink.AuthKindPAP {
		return off - start + writeICCNProxyPAP(buf, off, p)
	}
	return off - start + writeICCNProxyCHAP(buf, off, p)
}

// writeICCNProxyCHAP writes AVPs 29 to 33 for a CHAP subscriber, none hidden.
func writeICCNProxyCHAP(buf []byte, off int, p *iccnProxy) int {
	start := off
	// RFC 2661 Section 4.4.5: "This AVP MUST be present if proxy
	// authentication is to be utilized."
	// RFC 2661 Section 4.4.5: "The M-bit for this AVP MUST be set to 0."
	off += WriteAVPUint16(buf, off, false, AVPProxyAuthenType, proxyAuthenTypeCHAP)
	// RFC 2661 Section 4.4.5: "This AVP MUST be present in messages
	// containing a Proxy Authen Type AVP with an Authen Type of 1, 2, 3 or 5."
	off += WriteAVPString(buf, off, false, AVPProxyAuthenName, p.name)
	// RFC 2661 Section 4.4.5: "This AVP MUST be present for Proxy Authen
	// Types 2 and 5."
	off += WriteAVPBytes(buf, off, false, 0, AVPProxyAuthenChallenge, p.auth.Challenge)
	// RFC 2661 Section 4.4.5: "The Proxy Authen ID AVP MUST be present for
	// Proxy authen types 2, 3 and 5."
	off += writeAVPProxyAuthenID(buf, off, ProxyAuthenIDValue{ChapID: p.auth.Identifier})
	// RFC 2661 Section 4.4.5: "This AVP MUST be present for Proxy authen
	// types 1, 2, 3 and 5."
	off += WriteAVPBytes(buf, off, false, 0, AVPProxyAuthenResponse, p.auth.Response)
	return off - start
}

// writeICCNProxyPAP writes AVPs 29, 30, 32, a Random Vector and AVP 33
// hidden for a PAP subscriber. AVP 33 is the only hidden AVP in the message
// and the last one, so the Random Vector written just before it precedes
// every hidden AVP.
func writeICCNProxyPAP(buf []byte, off int, p *iccnProxy) int {
	start := off
	// RFC 2661 Section 4.4.5: "This AVP MUST be present if proxy
	// authentication is to be utilized."
	off += WriteAVPUint16(buf, off, false, AVPProxyAuthenType, proxyAuthenTypePAP)
	// RFC 2661 Section 4.4.5: "This AVP MUST be present in messages
	// containing a Proxy Authen Type AVP with an Authen Type of 1, 2, 3 or 5."
	off += WriteAVPString(buf, off, false, AVPProxyAuthenName, p.name)
	// RFC 2661 Section 4.4.5: "For 3, it is the Identifier value of the
	// Authenticate-Request."
	off += writeAVPProxyAuthenID(buf, off, ProxyAuthenIDValue{ChapID: p.auth.Identifier})
	// RFC 2661 Section 4.3: "If the H bit is set in any AVP(s) in a given
	// control message, a Random Vector AVP must also be present in the
	// message and MUST precede the first AVP having an H bit of 1."
	rvOff := off + AVPHeaderLen
	off += writeAVPRandomVector(buf, off)
	rv := buf[rvOff : rvOff+randomVectorOctets]
	// RFC 2661 Section 4.3: "The H bit MUST only be set if a shared secret
	// exists between the LAC and LNS." newICCNProxy sets sendAuth for PAP
	// only with p.secret non-empty.
	off += writeAVPHidden(buf, off, false, AVPProxyAuthenResponse, p.secret, rv, p.auth.Response)
	return off - start
}

// writeAVPRandomVector writes a Random Vector AVP with randomVectorOctets
// fresh random octets and returns its length. The octets are drawn into the
// message buffer itself, so the hidden AVP that follows reads them there.
func writeAVPRandomVector(buf []byte, off int) int {
	total := AVPHeaderLen + randomVectorOctets
	// RFC 2661 Section 4.4.1: "The M-bit for this AVP MUST be set to 1."
	// RFC 2661 Section 4.4.1: "This AVP MUST NOT be hidden (the H-bit MUST be
	// 0)."
	WriteAVPHeader(buf, off, FlagMandatory, 0, AVPRandomVector, total)
	// crypto/rand.Read never returns an error: it crashes the program
	// rather than hand back predictable octets (Go 1.24 crypto/rand).
	rand.Read(buf[off+AVPHeaderLen : off+total]) //nolint:errcheck // documented never to fail since Go 1.24
	return total
}

// writeAVPHidden writes an AVP with the H bit set, its value hidden with
// the Section 4.3 cipher under secret and rv, and returns its length. The
// caller MUST have written the Random Vector rv earlier in the same message
// and MUST pass a non-empty secret (HiddenEncrypt panics otherwise, a Ze
// defect alone can reach it).
func writeAVPHidden(buf []byte, off int, mandatory bool, attrType AVPType, secret, rv, plaintext []byte) int {
	out := HiddenEncrypt(buf[off+AVPHeaderLen:], attrType, secret, rv, plaintext)
	total := AVPHeaderLen + len(out)
	flags := FlagHidden
	if mandatory {
		flags |= FlagMandatory
	}
	WriteAVPHeader(buf, off, flags, 0, attrType, total)
	return total
}
