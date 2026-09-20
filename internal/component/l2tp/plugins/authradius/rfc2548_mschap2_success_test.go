// Related: handler.go -- decodeMSCHAP2Success and radiusAuth.handle
// Related: internal/component/l2tp/ppp/mschapv2.go -- the consumer of the blob

package l2tpauthradius

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// mschap2SuccessAttr builds one MS-CHAP2-Success attribute value the way RFC
// 2548 Section 2.3.3 defines it: an Ident octet, then the 42-octet
// authenticator string RFC 2759 Section 5 gives as "S=" and 40 hex digits.
func mschap2SuccessAttr(ident byte, hexDigits string) []byte {
	return append([]byte{ident, 'S', '='}, []byte(hexDigits)...)
}

// VALIDATES: the MS-CHAP2-Success attribute is decoded to the raw 20-octet
//
//	Authenticator Response its consumer requires.
//
// PREVENTS: the teardown on every RADIUS-backed MS-CHAPv2 session that stood
// here until 2026-09-20. extractMSCHAP2Success returned the attribute value
// verbatim, 43 octets of Ident plus ASCII, and runMSCHAPv2AuthPhase
// (internal/component/l2tp/ppp/mschapv2.go) fails the session for any length
// other than 20.
//
// RFC 2548 Section 2.3.3 gives the value two fields, "Ident: Identical to the
// PPP MS-CHAP v2 Identifier" and "String: The 42-octet authenticator string".
// RFC 2759 Section 5 says of that string: "The <auth_string> quantity is a 20
// octet number encoded in ASCII as 40 hexadecimal digits".
func TestMSCHAP2SuccessDecodesToTheRawAuthenticatorResponse(t *testing.T) {
	const digits = "0123456789ABCDEF0123456789ABCDEF01234567"
	want := []byte{
		0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF,
		0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF,
		0x01, 0x23, 0x45, 0x67,
	}

	got := decodeMSCHAP2Success(mschap2SuccessAttr(0x07, digits))

	if len(got) != mschapv2AuthenticatorResponseLen {
		t.Fatalf("decoded %d octets, want the %d runMSCHAPv2AuthPhase declares", len(got), mschapv2AuthenticatorResponseLen)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("decoded % x, want % x", got, want)
	}
}

// VALIDATES: a value that is not the RFC 2548 Section 2.3.3 format is refused
//
//	rather than passed on half-read.
//
// A blob ze could not read becomes an S= field the peer verifies and rejects,
// so refusing here is what lets the session fail with a reason instead of
// succeeding onto a Success packet nobody accepts. RFC 2759 Section 5: "If the
// authenticator response is either missing or incorrect, the peer MUST"
// terminate.
func TestMSCHAP2SuccessRefusesAValueItCannotRead(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"ident alone", []byte{0x07}},
		{"no S= prefix", append([]byte{0x07}, []byte("M=welcome")...)},
		{"hex too short", mschap2SuccessAttr(0x07, "0123456789ABCDEF")},
		{"not hexadecimal", mschap2SuccessAttr(0x07, "ZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ")},
		{"ident octet not stripped", []byte("S=0123456789ABCDEF0123456789ABCDEF01234567")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := decodeMSCHAP2Success(tc.data); got != nil {
				t.Fatalf("decoded % x from a value this format does not describe, want nil", got)
			}
		})
	}
}

// VALIDATES: a session the operator configured for no authentication is
//
//	admitted by the RADIUS handler, so a RADIUS server configured for
//	accounting alone does not refuse every session.
//
// PREVENTS: the refusal that stood here until 2026-09-20. activateRadiusConfig
// (register.go) claims the single auth slot for every RADIUS deployment,
// accounting-only included, and handle then sent an AuthMethodNone request
// into doRADIUS, where buildAccessRequestAttrs found no credential and denied.
// A wholesale LNS that bills and never authenticates could not bring one
// session up.
//
// RFC requirement: RFC2865-4.1-3 negative -- RFC 2865 Section 4.1: "An
// Access-Request MUST contain either a User-Password or a CHAP-Password or a
// State." A no-auth session carries none of the three, so handle (handler.go)
// sends no Access-Request at all rather than a non-conformant one, and the
// mock server below records no request.
func TestNoAuthIsAdmittedWithARADIUSClientConfigured(t *testing.T) {
	a, resp, cleanup := setupAuth(t, []byte("secret"), 3) // 3 = Access-Reject
	defer cleanup()

	result := a.handle(ppp.EventAuthRequest{
		TunnelID:  1,
		SessionID: 2,
		Method:    ppp.AuthMethodNone,
	}, func(accept bool, message string, blob []byte) error {
		return resp.respond(accept, message, blob)
	})

	if result.Handled {
		t.Fatal("the handler sent an Access-Request for a session that carries no credential; RFC 2865 Section 4.1 describes no such request")
	}
	if !result.Accept {
		t.Fatalf("a no-auth session was refused with %q; the operator already decided the question an Access-Request would have asked", result.Message)
	}
}

// VALIDATES: a method that DOES carry a credential still goes to the server,
//
//	so the admission above is scoped to AuthMethodNone and is not a blanket
//	accept.
//
// RFC requirement: RFC2865-4.1-3 positive -- a CHAP session carries a
// CHAP-Password, which is one of the three attributes Section 4.1 names, so
// handle (handler.go) builds the request and hands it to the server rather
// than answering it locally.
func TestCredentialBearingMethodStillReachesTheServer(t *testing.T) {
	a, resp, cleanup := setupAuth(t, []byte("secret"), 3) // 3 = Access-Reject
	defer cleanup()

	result := a.handle(ppp.EventAuthRequest{
		TunnelID:   1,
		SessionID:  2,
		Method:     ppp.AuthMethodCHAPMD5,
		Username:   "alice",
		Identifier: 1,
		Challenge:  bytes.Repeat([]byte{0xAA}, 16),
		Response:   bytes.Repeat([]byte{0xBB}, 16),
	}, func(accept bool, message string, blob []byte) error {
		return resp.respond(accept, message, blob)
	})

	if !result.Handled {
		t.Fatalf("a CHAP session was answered locally (%+v); its CHAP-Password is what RFC 2865 Section 4.1 asks an Access-Request to carry", result)
	}
	var _ l2tp.AuthResult = result
}
