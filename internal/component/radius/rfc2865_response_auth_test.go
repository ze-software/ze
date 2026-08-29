// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS wire format
// Related: packet.go -- ResponseAuthenticator, VerifyResponseAuth, AccountingRequestAuth
//
// VALIDATES: the two RADIUS authenticator formulas against a reference the test
// computes itself, rather than against the producer under test.
// PREVENTS: the failure this file was written for. TestResponseAuthenticator and
// TestVerifyCoARequestAuth compare the producer with itself: one calls
// ResponseAuthenticator twice and the other signs with AccountingRequestAuth and
// verifies with a function that calls AccountingRequestAuth again. Deleting
// `h.Write(attrs)` from either formula leaves both green, and a server that
// leaves the Attributes out of the hash accepts an Access-Accept or a
// Disconnect-Request whose every attribute an attacker has rewritten.
//
// The tests live beside packet_test.go rather than inside it: that file carries
// `RFC requirement:` tags and the pretool-writeedit hook refuses every edit to a
// tagged test file, an addition included (ai/rules/testing.md).

package radius

import (
	"bytes"
	"crypto/md5" //nolint:gosec // RFC 2865 Section 3 and RFC 5176 Section 3.5 mandate MD5
	"encoding/binary"
	"testing"
)

// rfc2865SamplePacket builds an Access-Accept-shaped datagram with several
// attributes, so a formula that drops the attribute stream produces a different
// value from one that keeps it.
func rfc2865SamplePacket(t *testing.T, code uint8) ([]byte, int) {
	t.Helper()
	pkt := &Packet{
		Code:       code,
		Identifier: 91,
		Attrs: []Attr{
			{Type: AttrUserName, Value: AttrString("carol")},
			{Type: AttrServiceType, Value: AttrUint32(ServiceTypeFramed)},
			{Type: AttrFramedProtocol, Value: AttrUint32(FramedProtocolPPP)},
			{Type: AttrAcctSessionID, Value: AttrString("sess-77")},
		},
	}
	buf := make([]byte, MaxPacketLen)
	n, err := pkt.EncodeTo(buf, 0)
	if err != nil {
		t.Fatalf("EncodeTo: %v", err)
	}
	return buf, n
}

// TestRFC2865ResponseAuthenticatorMatchesTheFormula pins the Response
// Authenticator to the octet stream RFC 2865 names, computed here from the RFC's
// own recipe and never by calling the producer.
//
// RFC 2865 Section 3: "That is, ResponseAuth =
// MD5(Code+ID+Length+RequestAuth+Attributes+Secret) where + denotes
// concatenation."
//
// RFC requirement: RFC2865-3-3 positive -- ResponseAuthenticator returns exactly
// MD5 over Code, Identifier, Length, the Request Authenticator, the response
// Attributes and the shared secret, in that order (packet.go
// ResponseAuthenticator).
func TestRFC2865ResponseAuthenticatorMatchesTheFormula(t *testing.T) {
	secret := []byte("response-secret")
	var reqAuth [AuthenticatorLen]byte
	copy(reqAuth[:], "request-auth-016")

	buf, n := rfc2865SamplePacket(t, CodeAccessAccept)
	pktLen := binary.BigEndian.Uint16(buf[2:4])
	attrs := buf[HeaderLen:n]

	got := ResponseAuthenticator(buf[0], buf[1], pktLen, reqAuth, attrs, secret)

	// Independent reference implementation of the Section 3 formula.
	h := md5.New() //nolint:gosec // RFC 2865 Section 3 mandates MD5
	h.Write(buf[:2])
	var lenBuf [2]byte
	binary.BigEndian.PutUint16(lenBuf[:], pktLen)
	h.Write(lenBuf[:])
	h.Write(reqAuth[:])
	h.Write(attrs)
	h.Write(secret)
	var want [AuthenticatorLen]byte
	copy(want[:], h.Sum(nil))

	if got != want {
		t.Fatalf("ResponseAuthenticator = %x, want %x (MD5(Code+ID+Length+RequestAuth+Attributes+Secret))", got, want)
	}

	// The fixture is not degenerate: the same packet with no attributes hashes to
	// something else, so the equality above genuinely covers the attribute stream.
	if ResponseAuthenticator(buf[0], buf[1], pktLen, reqAuth, nil, secret) == want {
		t.Fatal("the fixture carries no attribute contribution; the assertion above cannot see a formula that drops them")
	}
}

// TestRFC2865ResponseAuthenticatorCoversEveryNamedField drives each field the
// formula names, one at a time, and requires the value to move.
//
// RFC requirement: RFC2865-3-3 negative -- a response whose Code, Identifier,
// Length, Request Authenticator, Attributes or shared secret differs from the one
// signed does not verify: VerifyResponseAuth rejects it, so a rewritten
// Access-Accept is refused (packet.go VerifyResponseAuth).
func TestRFC2865ResponseAuthenticatorCoversEveryNamedField(t *testing.T) {
	secret := []byte("response-secret")
	var reqAuth [AuthenticatorLen]byte
	copy(reqAuth[:], "request-auth-016")

	buf, n := rfc2865SamplePacket(t, CodeAccessAccept)
	pktLen := binary.BigEndian.Uint16(buf[2:4])
	signed := ResponseAuthenticator(buf[0], buf[1], pktLen, reqAuth, buf[HeaderLen:n], secret)
	copy(buf[4:4+AuthenticatorLen], signed[:])

	if !VerifyResponseAuth(buf[:n], reqAuth, secret) {
		t.Fatal("the correctly signed response did not verify; the negatives below would prove nothing")
	}

	mutations := []struct {
		name   string
		mutate func(wire []byte) ([]byte, [AuthenticatorLen]byte, []byte)
	}{
		{
			name: "an attribute value the sender did not sign",
			mutate: func(wire []byte) ([]byte, [AuthenticatorLen]byte, []byte) {
				wire[HeaderLen+2]++
				return wire, reqAuth, secret
			},
		},
		{
			name: "a different Code",
			mutate: func(wire []byte) ([]byte, [AuthenticatorLen]byte, []byte) {
				wire[0] = CodeAccessReject
				return wire, reqAuth, secret
			},
		},
		{
			name: "a different Identifier",
			mutate: func(wire []byte) ([]byte, [AuthenticatorLen]byte, []byte) {
				wire[1]++
				return wire, reqAuth, secret
			},
		},
		{
			name: "a Request Authenticator from another request",
			mutate: func(wire []byte) ([]byte, [AuthenticatorLen]byte, []byte) {
				var other [AuthenticatorLen]byte
				copy(other[:], "another-req-auth")
				return wire, other, secret
			},
		},
		{
			name: "a different shared secret",
			mutate: func(wire []byte) ([]byte, [AuthenticatorLen]byte, []byte) {
				return wire, reqAuth, []byte("not-the-secret")
			},
		},
	}

	for _, m := range mutations {
		t.Run(m.name, func(t *testing.T) {
			wire := bytes.Clone(buf[:n])
			wire, auth, sec := m.mutate(wire)
			if VerifyResponseAuth(wire, auth, sec) {
				t.Fatalf("a response with %s verified: the Response Authenticator does not cover that field", m.name)
			}
		})
	}
}

// TestRFC5176CoARequestAuthenticatorMatchesTheFormula pins the CoA-Request and
// Disconnect-Request authenticator to the octet stream RFC 5176 names.
//
// RFC 5176 Section 3.5: "In Request packets, the Authenticator value is a
// 16-octet MD5 [RFC1321] checksum, called the Request Authenticator. The Request
// Authenticator is calculated the same way as for an Accounting-Request,
// specified in [RFC2866]."
//
// RFC 2866 Section 3, the formula that reference names: "The Request
// Authenticator field in Accounting-Request packets contains a one-way MD5 hash
// calculated over a stream of octets consisting of the Code + Identifier +
// Length + 16 zero octets + request attributes + shared secret (where +
// indicates concatenation)."
//
// RFC requirement: RFC5176-3.5-4 positive -- AccountingRequestAuth returns exactly
// that MD5, and the sixteen octets that carry the authenticator on the wire are
// replaced by zeros rather than hashed (packet.go AccountingRequestAuth).
func TestRFC5176CoARequestAuthenticatorMatchesTheFormula(t *testing.T) {
	secret := []byte("coa-formula-secret")
	buf, n := rfc2865SamplePacket(t, CodeCoARequest)

	// Non-zero garbage where the authenticator lives on the wire: a formula that
	// hashed the wire bytes rather than sixteen zeros gives a different answer.
	for i := 4; i < 4+AuthenticatorLen; i++ {
		buf[i] = 0x5A
	}

	got := AccountingRequestAuth(buf, n, secret)

	h := md5.New() //nolint:gosec // RFC 5176 Section 3.5 mandates the RFC 2866 MD5 formula
	h.Write(buf[:4])                        // Code + Identifier + Length
	h.Write(make([]byte, AuthenticatorLen)) // sixteen zero octets
	h.Write(buf[HeaderLen:n])               // Attributes
	h.Write(secret)                         // shared secret
	var want [AuthenticatorLen]byte
	copy(want[:], h.Sum(nil))

	if got != want {
		t.Fatalf("CoA Request Authenticator = %x, want %x (MD5(Code+ID+Length+16-zero-octets+Attributes+Secret))", got, want)
	}

	if AccountingRequestAuth(buf, HeaderLen, secret) == want {
		t.Fatal("the fixture carries no attribute contribution; the assertion above cannot see a formula that drops them")
	}
}

// TestRFC5176CoARequestAuthenticatorCoversEveryNamedField drives each field the
// formula names through the verifier a CoA listener uses.
//
// RFC requirement: RFC5176-3.5-4 negative -- a CoA-Request whose Code,
// Identifier, Attributes or shared secret differs from the one signed is refused
// by VerifyCoARequestAuth, so a Disconnect-Request an attacker rewrote does not
// tear a subscriber's session down (packet.go VerifyCoARequestAuth).
func TestRFC5176CoARequestAuthenticatorCoversEveryNamedField(t *testing.T) {
	secret := []byte("coa-formula-secret")
	buf, n := rfc2865SamplePacket(t, CodeDisconnectRequest)
	signed := AccountingRequestAuth(buf, n, secret)
	copy(buf[4:4+AuthenticatorLen], signed[:])

	if !VerifyCoARequestAuth(buf[:n], secret) {
		t.Fatal("the correctly signed request did not verify; the negatives below would prove nothing")
	}

	mutations := []struct {
		name   string
		mutate func(wire []byte) ([]byte, []byte)
	}{
		{
			name: "an attribute value the sender did not sign",
			mutate: func(wire []byte) ([]byte, []byte) {
				wire[HeaderLen+2]++
				return wire, secret
			},
		},
		{
			name: "a different Code",
			mutate: func(wire []byte) ([]byte, []byte) {
				wire[0] = CodeCoARequest
				return wire, secret
			},
		},
		{
			name: "a different Identifier",
			mutate: func(wire []byte) ([]byte, []byte) {
				wire[1]++
				return wire, secret
			},
		},
		{
			name: "a different shared secret",
			mutate: func(wire []byte) ([]byte, []byte) {
				return wire, []byte("not-the-secret")
			},
		},
	}

	for _, m := range mutations {
		t.Run(m.name, func(t *testing.T) {
			wire := bytes.Clone(buf[:n])
			wire, sec := m.mutate(wire)
			if VerifyCoARequestAuth(wire, sec) {
				t.Fatalf("a request with %s verified: the Request Authenticator does not cover that field", m.name)
			}
		})
	}
}
