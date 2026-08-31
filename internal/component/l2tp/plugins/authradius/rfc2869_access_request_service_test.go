// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS NAS attribute set
// Related: handler.go -- buildAuthAttrs, the only Access-Request builder
//
// VALIDATES: what an Access-Request Ze's NAS builds names as the service, and
// that it never carries two kinds of authentication credential.
// PREVENTS: a future auth method appending its credential attribute beside an
// existing one instead of inside the switch, which RFC 2869 Section 5.19 Note 1
// forbids and which a RADIUS server answers by picking whichever it prefers.

package l2tpauthradius

import (
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/radius"
)

// rfc2869CredentialAttrs is the set RFC 2869 Section 5.19 Note 1 names: an
// Access-Request may carry at most one KIND of these four.
//
// RFC 2865 Section 5 assigns 2 User-Password and 3 CHAP-Password; RFC 2869
// Section 5 assigns 70 ARAP-Password and 79 EAP-Message.
var rfc2869CredentialAttrs = map[uint8]string{
	2:  "User-Password",
	3:  "CHAP-Password",
	70: "ARAP-Password",
	79: "EAP-Message",
}

// rfc2869AuthRequests returns one auth request per method the NAS supports,
// each carrying enough material for buildAuthAttrs to reach its credential
// branch. MS-CHAPv2 needs 40 or more response octets for its two VSAs.
func rfc2869AuthRequests() map[string]ppp.EventAuthRequest {
	return map[string]ppp.EventAuthRequest{
		"pap": {
			TunnelID: 1, SessionID: 2, Method: ppp.AuthMethodPAP,
			Username: "alice", Response: []byte("secret"),
		},
		"chap-md5": {
			TunnelID: 1, SessionID: 3, Method: ppp.AuthMethodCHAPMD5, Identifier: 7,
			Username: "bob", Challenge: make([]byte, 16), Response: make([]byte, 16),
		},
		"mschapv2": {
			TunnelID: 1, SessionID: 4, Method: ppp.AuthMethodMSCHAPv2, Identifier: 9,
			Username: "carol", Challenge: make([]byte, 16), Response: make([]byte, 40),
		},
		"none": {
			TunnelID: 1, SessionID: 5, Method: ppp.AuthMethodNone, Username: "dave",
		},
	}
}

// TestRFC2869AccessRequestNamesTheServiceZeOffers reads the Service-Type and
// Framed-Protocol of every Access-Request the NAS can build.
//
// RFC requirement: RFC2869-1.1-2 positive -- every Access-Request names
// Service-Type Framed and Framed-Protocol PPP, the one service Ze's NAS
// provides (buildAuthAttrs, handler.go).
func TestRFC2869AccessRequestNamesTheServiceZeOffers(t *testing.T) {
	for name, req := range rfc2869AuthRequests() {
		t.Run(name, func(t *testing.T) {
			attrs, ok := buildAuthAttrs(req, "test-nas", nil)
			if name == "none" {
				// RFC 2865 Section 4.1: "An Access-Request MUST contain either a
				// User-Password or a CHAP-Password or a State." A peer that
				// authenticated with nothing supplies none of the three, so no
				// Access-Request is built and there is no service to name.
				if ok {
					t.Fatal("AuthMethodNone MUST NOT build an Access-Request")
				}
				return
			}
			if !ok {
				t.Fatalf("%s MUST build an Access-Request", name)
			}
			pkt := &radius.Packet{Attrs: attrs}

			service := pkt.FindAttr(radius.AttrServiceType)
			if len(service) != 4 || service[3] != byte(radius.ServiceTypeFramed) {
				t.Fatalf("Service-Type = %v, want Framed (%d)", service, radius.ServiceTypeFramed)
			}
			proto := pkt.FindAttr(radius.AttrFramedProtocol)
			if len(proto) != 4 || proto[3] != byte(radius.FramedProtocolPPP) {
				t.Fatalf("Framed-Protocol = %v, want PPP (%d)", proto, radius.FramedProtocolPPP)
			}
		})
	}
}

// TestRFC2869AccessRequestNeverRequestsAnUnavailableService refuses any
// Framed-Protocol other than PPP. ARAP is Framed-Protocol 3, and Ze's PPP stack
// (internal/component/l2tp/ppp) offers no ARAP, so requesting it would ask a
// server to authorize a service the NAS cannot then deliver.
//
// RFC 2869 Section 1.1: "A NAS MUST treat a RADIUS access-request requesting an
// unavailable service as an access-reject instead."
//
// RFC requirement: RFC2869-1.1-2 negative -- no Access-Request names a
// Framed-Protocol other than PPP, and none appears more than once
// (buildAuthAttrs, handler.go).
func TestRFC2869AccessRequestNeverRequestsAnUnavailableService(t *testing.T) {
	for name, req := range rfc2869AuthRequests() {
		t.Run(name, func(t *testing.T) {
			attrs, ok := buildAuthAttrs(req, "test-nas", nil)
			if name == "none" {
				// RFC 2865 Section 4.1: "An Access-Request MUST contain either a
				// User-Password or a CHAP-Password or a State." A peer that
				// authenticated with nothing supplies none of the three, so no
				// Access-Request is built and there is no service to name.
				if ok {
					t.Fatal("AuthMethodNone MUST NOT build an Access-Request")
				}
				return
			}
			if !ok {
				t.Fatalf("%s MUST build an Access-Request", name)
			}
			pkt := &radius.Packet{Attrs: attrs}

			values := pkt.FindAllAttr(radius.AttrFramedProtocol)
			if len(values) != 1 {
				t.Fatalf("Framed-Protocol appears %d time(s), want exactly 1", len(values))
			}
			if values[0][3] != byte(radius.FramedProtocolPPP) {
				t.Fatalf("Framed-Protocol = %d, want PPP (%d); Ze's NAS offers no other service",
					values[0][3], radius.FramedProtocolPPP)
			}
		})
	}
}

// TestRFC2869AccessRequestCarriesTheCredentialOfItsMethod proves the builder
// does emit the credential the negotiated method needs, so the exclusivity case
// below is asserting over a packet that actually carries one.
//
// RFC requirement: RFC2869-5.19-1 positive -- a PAP Access-Request carries
// User-Password and a CHAP-MD5 Access-Request carries CHAP-Password
// (buildAuthAttrs, handler.go).
func TestRFC2869AccessRequestCarriesTheCredentialOfItsMethod(t *testing.T) {
	reqs := rfc2869AuthRequests()

	papAttrs, ok := buildAuthAttrs(reqs["pap"], "test-nas", nil)
	if !ok {
		t.Fatal("a PAP request MUST build an Access-Request")
	}
	pap := &radius.Packet{Attrs: papAttrs}
	if pap.FindAttr(radius.AttrUserPassword) == nil {
		t.Error("a PAP Access-Request carries no User-Password")
	}

	chapAttrs, ok := buildAuthAttrs(reqs["chap-md5"], "test-nas", nil)
	if !ok {
		t.Fatal("a CHAP-MD5 request MUST build an Access-Request")
	}
	chap := &radius.Packet{Attrs: chapAttrs}
	if chap.FindAttr(radius.AttrCHAPPassword) == nil {
		t.Error("a CHAP-MD5 Access-Request carries no CHAP-Password")
	}
}

// TestRFC2869AccessRequestCarriesOneKindOfCredential counts how many of the
// four credential attribute types appear in one Access-Request.
//
// RFC 2869 Section 5.19 Note 1: "An Access-Request that contains either a
// User-Password or CHAP-Password or ARAP-Password or one or more EAP-Message
// attributes MUST NOT contain more than one type of those four attributes."
//
// RFC requirement: RFC2869-5.19-1 negative -- no Access-Request carries two
// kinds of credential, so a CHAP request carries no User-Password and a PAP
// request carries no CHAP-Password (buildAuthAttrs, handler.go).
func TestRFC2869AccessRequestCarriesOneKindOfCredential(t *testing.T) {
	for name, req := range rfc2869AuthRequests() {
		t.Run(name, func(t *testing.T) {
			attrs, ok := buildAuthAttrs(req, "test-nas", nil)
			if name == "none" {
				// RFC 2865 Section 4.1: "An Access-Request MUST contain either a
				// User-Password or a CHAP-Password or a State." A peer that
				// authenticated with nothing supplies none of the three, so no
				// Access-Request is built and there is no service to name.
				if ok {
					t.Fatal("AuthMethodNone MUST NOT build an Access-Request")
				}
				return
			}
			if !ok {
				t.Fatalf("%s MUST build an Access-Request", name)
			}
			pkt := &radius.Packet{Attrs: attrs}

			var present []string
			for code, attrName := range rfc2869CredentialAttrs {
				if pkt.FindAttr(code) != nil {
					present = append(present, attrName)
				}
			}
			if len(present) > 1 {
				t.Fatalf("Access-Request carries %d kinds of credential (%v); "+
					"RFC 2869 Section 5.19 Note 1 allows at most one", len(present), present)
			}
		})
	}
}
