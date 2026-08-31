// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS wire format
// RFC: rfc/short/rfc5176.md -- Section 3.4 Message-Authenticator on a CoA-Request
// Related: packet.go -- VerifyCoAMessageAuthenticator, VerifyCoARequestAuth
//
// VALIDATES: that the two authenticators of a CoA-Request or Disconnect-Request
// are verified in the order RFC 5176 Section 3.4 fixes -- the HMAC-MD5 over a
// packet whose Request Authenticator and Message-Authenticator are each sixteen
// octets of zero, and the Request Authenticator MD5 over the finished
// Message-Authenticator value.
// PREVENTS: the defect found on 2026-08-31. Ze had the two inverted:
// VerifyMessageAuthenticator hashed the Request Authenticator as it stood on the
// wire, and VerifyCoARequestAuth zeroed the Message-Authenticator before the
// MD5. A conformant Dynamic Authorization Client could not authenticate to Ze at
// all, and only a client with Ze's own inverted order could.
//
// Every fixture here is signed from the RFC's own recipe, never by calling a
// producer under test, so a producer that reverts to the inverted order fails.

package radius

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // RFC 5176 Section 3.4 mandates HMAC-MD5
	"encoding/binary"
	"testing"
)

// rfc5176UnsignedRequest encodes a CoA-Request or Disconnect-Request carrying a
// Message-Authenticator whose value is still sixteen octets of zero. The Request
// Authenticator field is zero too, because Packet.EncodeTo writes the zero
// value of Packet.Authenticator.
func rfc5176UnsignedRequest(t *testing.T, code uint8) []byte {
	t.Helper()
	pkt := &Packet{
		Code:       code,
		Identifier: 47,
		Attrs: []Attr{
			{Type: AttrUserName, Value: AttrString("dave")},
			{Type: AttrAcctSessionID, Value: AttrString("sess-13")},
			{Type: AttrMessageAuthenticator, Value: make([]byte, AuthenticatorLen)},
			{Type: AttrFilterID, Value: AttrString("10mbit")},
		},
	}
	buf := make([]byte, MaxPacketLen)
	n, err := pkt.EncodeTo(buf, 0)
	if err != nil {
		t.Fatalf("EncodeTo: %v", err)
	}
	return bytes.Clone(buf[:n])
}

// rfc5176MessageAuthenticatorOffset walks the attribute list itself rather than
// calling the package helper, so a defect in that helper cannot hide here.
func rfc5176MessageAuthenticatorOffset(t *testing.T, wire []byte) int {
	t.Helper()
	pktLen := int(binary.BigEndian.Uint16(wire[2:4]))
	for off := HeaderLen; off < pktLen; {
		attrLen := int(wire[off+1])
		if attrLen < 2 || off+attrLen > pktLen {
			t.Fatalf("invalid attribute length %d at offset %d", attrLen, off)
		}
		if wire[off] == AttrMessageAuthenticator {
			return off + 2
		}
		off += attrLen
	}
	t.Fatal("fixture carries no Message-Authenticator")
	return 0
}

// rfc5176SignConformant signs wire in place the way RFC 5176 Section 3.4 fixes.
//
// RFC 5176 Section 3.4: "When the HMAC-MD5 message integrity check is calculated
// the Request Authenticator field and Message-Authenticator Attribute MUST each
// be considered to be sixteen octets of zero.  The Message-Authenticator
// Attribute is calculated and inserted in the packet before the Request
// Authenticator is calculated". Both halves are performed here in that order.
func rfc5176SignConformant(t *testing.T, wire, secret []byte) {
	t.Helper()
	maOff := rfc5176MessageAuthenticatorOffset(t, wire)

	hashed := bytes.Clone(wire)
	clear(hashed[4 : 4+AuthenticatorLen])
	clear(hashed[maOff : maOff+AuthenticatorLen])
	mac := hmac.New(md5.New, secret) //nolint:gosec // RFC 5176 Section 3.4 mandates HMAC-MD5
	mac.Write(hashed)
	copy(wire[maOff:maOff+AuthenticatorLen], mac.Sum(nil))

	rfc5176SignRequestAuthenticator(wire, secret)
}

// rfc5176SignRequestAuthenticator writes the Request Authenticator over the
// packet as it now stands, attributes included, with sixteen zero octets in
// place of the authenticator field.
//
// RFC 5176 Section 2.3: "The Request Authenticator is calculated the same way as
// for an Accounting-Request, specified in [RFC2866]". RFC 2866 Section 3 gives
// MD5 over Code + Identifier + Length + 16 zero octets + attributes + secret.
func rfc5176SignRequestAuthenticator(wire, secret []byte) {
	h := md5.New() //nolint:gosec // RFC 2866 Section 3 mandates MD5
	h.Write(wire[:4])
	h.Write(make([]byte, AuthenticatorLen))
	h.Write(wire[HeaderLen:])
	h.Write(secret)
	copy(wire[4:4+AuthenticatorLen], h.Sum(nil))
}

// rfc5176SignInverted signs wire the way Ze signed and verified it before
// 2026-08-31: the Request Authenticator computed first over a zeroed
// Message-Authenticator, then the HMAC computed over that finished Request
// Authenticator. It is the fixture every negative case below rejects.
func rfc5176SignInverted(t *testing.T, wire, secret []byte) {
	t.Helper()
	maOff := rfc5176MessageAuthenticatorOffset(t, wire)

	rfc5176SignRequestAuthenticator(wire, secret)

	hashed := bytes.Clone(wire)
	clear(hashed[maOff : maOff+AuthenticatorLen])
	mac := hmac.New(md5.New, secret) //nolint:gosec // the inverted order under test
	mac.Write(hashed)
	copy(wire[maOff:maOff+AuthenticatorLen], mac.Sum(nil))
}

// RFC requirement: RFC5176-3.4-1 positive -- VerifyCoAMessageAuthenticator
// accepts a CoA-Request whose Message-Authenticator is the HMAC-MD5 over a
// packet with the Request Authenticator field and the Message-Authenticator
// Attribute each replaced by sixteen octets of zero (packet.go
// VerifyCoAMessageAuthenticator).
func TestRFC5176MessageAuthenticatorZeroesBothFields(t *testing.T) {
	secret := []byte("coa-ma-secret")
	wire := rfc5176UnsignedRequest(t, CodeCoARequest)
	rfc5176SignConformant(t, wire, secret)

	if !VerifyCoAMessageAuthenticator(wire, secret) {
		t.Fatal("a conformantly signed CoA-Request was refused: a Dynamic Authorization Client cannot authenticate to Ze")
	}

	// The Request Authenticator field is not zero on the wire, so a producer
	// that skipped the substitution would have hashed different octets.
	var zeros [AuthenticatorLen]byte
	if bytes.Equal(wire[4:4+AuthenticatorLen], zeros[:]) {
		t.Fatal("the fixture carries a zero Request Authenticator; the assertion above cannot see a missing substitution")
	}
}

// RFC requirement: RFC5176-3.4-1 negative -- VerifyCoAMessageAuthenticator
// refuses a CoA-Request whose Message-Authenticator was computed over any other
// octet stream: the wire Request Authenticator left in place, the attribute
// value left in place, an attribute rewritten after signing, or another shared
// secret (packet.go VerifyCoAMessageAuthenticator).
func TestRFC5176MessageAuthenticatorRefusesEveryOtherStream(t *testing.T) {
	secret := []byte("coa-ma-secret")

	cases := []struct {
		name string
		sign func(t *testing.T, wire []byte)
	}{
		{
			name: "the Request Authenticator hashed as it stands on the wire",
			sign: func(t *testing.T, wire []byte) {
				t.Helper()
				rfc5176SignInverted(t, wire, secret)
			},
		},
		{
			name: "the Message-Authenticator attribute not zeroed before the HMAC",
			sign: func(t *testing.T, wire []byte) {
				t.Helper()
				maOff := rfc5176MessageAuthenticatorOffset(t, wire)
				for i := maOff; i < maOff+AuthenticatorLen; i++ {
					wire[i] = 0x77
				}
				hashed := bytes.Clone(wire)
				clear(hashed[4 : 4+AuthenticatorLen])
				mac := hmac.New(md5.New, secret) //nolint:gosec // the stream under test
				mac.Write(hashed)
				copy(wire[maOff:maOff+AuthenticatorLen], mac.Sum(nil))
				rfc5176SignRequestAuthenticator(wire, secret)
			},
		},
		{
			name: "an attribute rewritten after signing",
			sign: func(t *testing.T, wire []byte) {
				t.Helper()
				rfc5176SignConformant(t, wire, secret)
				wire[HeaderLen+2]++
			},
		},
		{
			name: "another shared secret",
			sign: func(t *testing.T, wire []byte) {
				t.Helper()
				rfc5176SignConformant(t, wire, []byte("not-the-secret"))
			},
		},
		{
			name: "no Message-Authenticator value written at all",
			sign: func(t *testing.T, wire []byte) {
				t.Helper()
				rfc5176SignRequestAuthenticator(wire, secret)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wire := rfc5176UnsignedRequest(t, CodeDisconnectRequest)
			c.sign(t, wire)
			if VerifyCoAMessageAuthenticator(wire, secret) {
				t.Fatalf("a request signed with %s verified: the HMAC does not cover the stream RFC 5176 Section 3.4 names", c.name)
			}
		})
	}
}

// RFC requirement: RFC5176-3.4-2 positive -- VerifyCoARequestAuth accepts a
// CoA-Request whose Request Authenticator was computed last, over the finished
// Message-Authenticator value, because RFC 5176 Section 3.4 inserts that
// attribute before the Request Authenticator is calculated (packet.go
// VerifyCoARequestAuth).
func TestRFC5176RequestAuthenticatorCoversTheSignedMessageAuthenticator(t *testing.T) {
	secret := []byte("coa-reqauth-secret")
	wire := rfc5176UnsignedRequest(t, CodeCoARequest)
	rfc5176SignConformant(t, wire, secret)

	if !VerifyCoARequestAuth(wire, secret) {
		t.Fatal("a conformantly signed CoA-Request was refused: the Request Authenticator was computed over a Message-Authenticator the sender never sent")
	}

	maOff := rfc5176MessageAuthenticatorOffset(t, wire)
	var zeros [AuthenticatorLen]byte
	if bytes.Equal(wire[maOff:maOff+AuthenticatorLen], zeros[:]) {
		t.Fatal("the fixture carries a zero Message-Authenticator; the assertion above cannot see a producer that zeroes it")
	}
}

// RFC requirement: RFC5176-3.4-2 negative -- VerifyCoARequestAuth refuses a
// CoA-Request whose Request Authenticator was computed with the
// Message-Authenticator zeroed, which is the order RFC 5176 Section 3.4 forbids,
// and refuses one whose attribute stream changed after signing (packet.go
// VerifyCoARequestAuth).
func TestRFC5176RequestAuthenticatorRefusesTheInvertedOrder(t *testing.T) {
	secret := []byte("coa-reqauth-secret")

	inverted := rfc5176UnsignedRequest(t, CodeDisconnectRequest)
	rfc5176SignInverted(t, inverted, secret)
	if VerifyCoARequestAuth(inverted, secret) {
		t.Fatal("a request whose Request Authenticator was computed over a zeroed Message-Authenticator verified: the MD5 does not cover the attribute as sent")
	}

	tampered := rfc5176UnsignedRequest(t, CodeDisconnectRequest)
	rfc5176SignConformant(t, tampered, secret)
	maOff := rfc5176MessageAuthenticatorOffset(t, tampered)
	tampered[maOff]++
	if VerifyCoARequestAuth(tampered, secret) {
		t.Fatal("a request whose Message-Authenticator changed after signing verified: the Request Authenticator does not cover that attribute")
	}
}
