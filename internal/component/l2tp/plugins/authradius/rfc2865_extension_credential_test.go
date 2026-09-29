// Design: docs/guide/l2tp.md -- RADIUS authentication of L2TP subscribers
// RFC: rfc/short/rfc2865.md -- Section 4.1 credential attributes and their extensions
// Related: handler.go -- doRADIUS builds the Access-Request for each auth method
// Related: rfc2865_nas_obligations_test.go -- setupCapturingAuth, decodeCaptured

// RFC 2865 Section 4.1 lets an extension carry authentication information "in an
// Access-Request instead of User-Password or CHAP-Password". MS-CHAPv2 is such an
// extension (RFC 2548). This test drives a well-formed MS-CHAPv2 response through
// the real auth handler and reads the Access-Request the server received.
package l2tpauthradius

import (
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/radius"
)

// RFC requirement: RFC2865-4.1-3 positive -- a well-formed MS-CHAPv2 response
// is sent as an Access-Request carrying the RFC 2548 MS-CHAP2-Response vendor
// attribute, the extension credential that stands instead of User-Password or
// CHAP-Password, and carrying neither of those two.
func TestRFC2865MSCHAPv2AccessRequestCarriesTheExtensionCredential(t *testing.T) {
	a, srv, resp := setupCapturingAuth(t, []byte("testing123"))
	a.handle(ppp.EventAuthRequest{
		TunnelID: 1, SessionID: 2, Method: ppp.AuthMethodMSCHAPv2,
		Username: "alice", Identifier: 7,
		Challenge: make([]byte, 16), Response: make([]byte, 49),
	}, resp.respond)
	resp.waitOne(t)
	req := decodeCaptured(t, srv)

	found := false
	for _, raw := range req.FindAllAttr(radius.AttrVendorSpecific) {
		vendorID, vendorType, _, err := radius.DecodeVSA(raw)
		if err != nil {
			t.Fatalf("Vendor-Specific does not decode: %v", err)
		}
		if vendorID == radius.VendorMicrosoft && vendorType == radius.MSCHAP2Response {
			found = true
		}
	}
	if !found {
		t.Fatal("an MS-CHAPv2 Access-Request MUST carry the MS-CHAP2-Response credential")
	}
	if req.FindAttr(radius.AttrUserPassword) != nil {
		t.Error("an MS-CHAPv2 Access-Request carries a User-Password beside the extension credential")
	}
	if req.FindAttr(radius.AttrCHAPPassword) != nil {
		t.Error("an MS-CHAPv2 Access-Request carries a CHAP-Password beside the extension credential")
	}
}
