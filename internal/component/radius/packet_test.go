package radius

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestEncodeAccessRequest(t *testing.T) {
	auth, err := RandomAuthenticator()
	if err != nil {
		t.Fatal(err)
	}

	pkt := &Packet{
		Code:          CodeAccessRequest,
		Identifier:    42,
		Authenticator: auth,
		Attrs: []Attr{
			{Type: AttrUserName, Value: AttrString("alice")},
		},
	}

	buf := make([]byte, MaxPacketLen)
	n, err := pkt.EncodeTo(buf, 0)
	if err != nil {
		t.Fatal(err)
	}

	if buf[0] != CodeAccessRequest {
		t.Errorf("code: got %d, want %d", buf[0], CodeAccessRequest)
	}
	if buf[1] != 42 {
		t.Errorf("id: got %d, want 42", buf[1])
	}

	wireLen := binary.BigEndian.Uint16(buf[2:4])
	if int(wireLen) != n {
		t.Errorf("length field %d != written %d", wireLen, n)
	}

	// Header(20) + Attr(Type=1, Len=7, "alice"=5) = 27
	if n != 27 {
		t.Errorf("total length: got %d, want 27", n)
	}
}

func TestDecodeAccessAccept(t *testing.T) {
	auth, _ := RandomAuthenticator()
	pkt := &Packet{
		Code:          CodeAccessAccept,
		Identifier:    7,
		Authenticator: auth,
		Attrs: []Attr{
			{Type: AttrFramedIPAddress, Value: []byte{10, 0, 0, 1}},
			{Type: AttrReplyMessage, Value: AttrString("welcome")},
		},
	}

	buf := make([]byte, MaxPacketLen)
	n, err := pkt.EncodeTo(buf, 0)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := Decode(buf[:n])
	if err != nil {
		t.Fatal(err)
	}

	if decoded.Code != CodeAccessAccept {
		t.Errorf("code: got %d, want %d", decoded.Code, CodeAccessAccept)
	}
	if decoded.Identifier != 7 {
		t.Errorf("id: got %d, want 7", decoded.Identifier)
	}

	ip := decoded.FindAttr(AttrFramedIPAddress)
	if !bytes.Equal(ip, []byte{10, 0, 0, 1}) {
		t.Errorf("Framed-IP-Address: got %v, want [10 0 0 1]", ip)
	}

	msg := decoded.FindAttr(AttrReplyMessage)
	if string(msg) != "welcome" {
		t.Errorf("Reply-Message: got %q, want %q", msg, "welcome")
	}
}

func TestDecodeAccessReject(t *testing.T) {
	auth, _ := RandomAuthenticator()
	pkt := &Packet{
		Code:          CodeAccessReject,
		Identifier:    3,
		Authenticator: auth,
		Attrs: []Attr{
			{Type: AttrReplyMessage, Value: AttrString("bad password")},
		},
	}

	buf := make([]byte, MaxPacketLen)
	n, err := pkt.EncodeTo(buf, 0)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := Decode(buf[:n])
	if err != nil {
		t.Fatal(err)
	}

	if decoded.Code != CodeAccessReject {
		t.Errorf("code: got %d, want %d", decoded.Code, CodeAccessReject)
	}

	msg := decoded.FindAttr(AttrReplyMessage)
	if string(msg) != "bad password" {
		t.Errorf("Reply-Message: got %q, want %q", msg, "bad password")
	}
}

func TestPacketRoundTrip(t *testing.T) {
	auth, _ := RandomAuthenticator()
	original := &Packet{
		Code:          CodeAccessRequest,
		Identifier:    99,
		Authenticator: auth,
		Attrs: []Attr{
			{Type: AttrUserName, Value: AttrString("bob")},
			{Type: AttrNASIPAddress, Value: []byte{192, 168, 1, 1}},
			{Type: AttrServiceType, Value: AttrUint32(ServiceTypeFramed)},
			{Type: AttrFramedProtocol, Value: AttrUint32(FramedProtocolPPP)},
		},
	}

	buf := make([]byte, MaxPacketLen)
	n, err := original.EncodeTo(buf, 0)
	if err != nil {
		t.Fatal(err)
	}

	// RFC requirement: RFC2865-3-1 positive -- a packet whose Length lies within the
	// 20..4096 bound decodes successfully.
	decoded, err := Decode(buf[:n])
	if err != nil {
		t.Fatal(err)
	}

	if decoded.Code != original.Code {
		t.Errorf("code mismatch")
	}
	if decoded.Identifier != original.Identifier {
		t.Errorf("id mismatch")
	}
	if decoded.Authenticator != original.Authenticator {
		t.Errorf("authenticator mismatch")
	}
	if len(decoded.Attrs) != len(original.Attrs) {
		t.Fatalf("attr count: got %d, want %d", len(decoded.Attrs), len(original.Attrs))
	}
	for i, a := range decoded.Attrs {
		if a.Type != original.Attrs[i].Type {
			t.Errorf("attr[%d] type: got %d, want %d", i, a.Type, original.Attrs[i].Type)
		}
		if !bytes.Equal(a.Value, original.Attrs[i].Value) {
			t.Errorf("attr[%d] value mismatch", i)
		}
	}
}

func TestDecodeTooShort(t *testing.T) {
	// RFC requirement: RFC2865-3-1 negative -- a 19-octet packet (below the 20-octet
	// minimum) is rejected.
	_, err := Decode(make([]byte, 19))
	if err == nil {
		t.Fatal("expected error for packet < 20 bytes")
	}
}

func TestDecodeTooLong(t *testing.T) {
	// RFC requirement: RFC2865-3-1 negative -- a 4097-octet packet (above the 4096-octet
	// maximum) is rejected.
	_, err := Decode(make([]byte, 4097))
	if err == nil {
		t.Fatal("expected error for packet > 4096 bytes")
	}
}

func TestDecodeBadLength(t *testing.T) {
	buf := make([]byte, 20)
	buf[0] = CodeAccessAccept
	// Set length to 5000, which exceeds data length.
	binary.BigEndian.PutUint16(buf[2:4], 5000)
	// RFC requirement: RFC2865-3-1 negative -- a Length field pointing past the datagram
	// (outside the valid 20..len bound) is rejected.
	_, err := Decode(buf)
	if err == nil {
		t.Fatal("expected error for invalid length field")
	}
}

func TestResponseAuthenticator(t *testing.T) {
	var reqAuth [AuthenticatorLen]byte
	copy(reqAuth[:], "0123456789abcdef")
	secret := []byte("testing123")

	// RFC requirement: RFC2865-3-3 positive -- the Response Authenticator is the deterministic
	// MD5(Code+ID+Length+RequestAuth+Attrs+Secret): identical inputs yield an identical value.
	auth1 := ResponseAuthenticator(CodeAccessAccept, 1, 20, reqAuth, nil, secret)
	auth2 := ResponseAuthenticator(CodeAccessAccept, 1, 20, reqAuth, nil, secret)
	if auth1 != auth2 {
		t.Error("same inputs should produce same authenticator")
	}

	// RFC requirement: RFC2865-3-3 negative -- the shared secret is folded into the hash, so
	// a different secret produces a different authenticator (not a value independent of it).
	auth3 := ResponseAuthenticator(CodeAccessAccept, 1, 20, reqAuth, nil, []byte("different"))
	if auth1 == auth3 {
		t.Error("different secrets should produce different authenticator")
	}
}

func TestVerifyResponseAuth(t *testing.T) {
	secret := []byte("testing123")
	reqAuth, _ := RandomAuthenticator()

	pkt := &Packet{
		Code:          CodeAccessAccept,
		Identifier:    5,
		Authenticator: reqAuth, // placeholder; will be overwritten
	}

	buf := make([]byte, MaxPacketLen)
	n, _ := pkt.EncodeTo(buf, 0)

	// Compute correct response authenticator.
	pktLen := binary.BigEndian.Uint16(buf[2:4])
	correct := ResponseAuthenticator(buf[0], buf[1], pktLen, reqAuth, buf[HeaderLen:n], secret)
	copy(buf[4:4+AuthenticatorLen], correct[:])

	// RFC requirement: RFC2865-3-3 positive -- a response signed with the RFC 2865 formula
	// verifies against the request authenticator and shared secret.
	if !VerifyResponseAuth(buf[:n], reqAuth, secret) {
		t.Error("valid response auth should verify")
	}

	// RFC requirement: RFC2865-3-3 negative -- a single-octet corruption of the computed
	// Response Authenticator fails verification.
	// Corrupt one byte.
	buf[4]++
	if VerifyResponseAuth(buf[:n], reqAuth, secret) {
		t.Error("corrupted response auth should fail verification")
	}
}

func TestAccountingRequestAuth(t *testing.T) {
	secret := []byte("accttest")

	buf := make([]byte, MaxPacketLen)
	buf[0] = CodeAccountingReq
	buf[1] = 1
	binary.BigEndian.PutUint16(buf[2:4], 20)

	auth := AccountingRequestAuth(buf, 20, secret)
	// Should be deterministic.
	auth2 := AccountingRequestAuth(buf, 20, secret)
	if auth != auth2 {
		t.Error("same inputs should produce same accounting auth")
	}
}

// VALIDATES: AC-3/AC-4 -- CoA-Request (code 43) round-trips through encode/decode.
func TestCoARequestRoundTrip(t *testing.T) {
	pkt := &Packet{
		Code:       CodeCoARequest,
		Identifier: 10,
		Attrs: []Attr{
			{Type: AttrAcctSessionID, Value: AttrString("sess-001")},
			{Type: AttrFilterID, Value: AttrString("10mbit")},
		},
	}

	buf := make([]byte, MaxPacketLen)
	n, err := pkt.EncodeTo(buf, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Set proper CoA authenticator.
	secret := []byte("coa-secret")
	auth := AccountingRequestAuth(buf, n, secret)
	copy(buf[4:4+AuthenticatorLen], auth[:])

	decoded, err := Decode(buf[:n])
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Code != CodeCoARequest {
		t.Errorf("code: got %d, want %d", decoded.Code, CodeCoARequest)
	}
	sessID := decoded.FindAttr(AttrAcctSessionID)
	if string(sessID) != "sess-001" {
		t.Errorf("Acct-Session-Id: got %q, want %q", sessID, "sess-001")
	}
}

// VALIDATES: AC-6/AC-7 -- Disconnect-Request (code 40) round-trips.
func TestDisconnectRequestRoundTrip(t *testing.T) {
	pkt := &Packet{
		Code:       CodeDisconnectRequest,
		Identifier: 20,
		Attrs: []Attr{
			{Type: AttrAcctSessionID, Value: AttrString("sess-002")},
		},
	}

	buf := make([]byte, MaxPacketLen)
	n, err := pkt.EncodeTo(buf, 0)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := Decode(buf[:n])
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Code != CodeDisconnectRequest {
		t.Errorf("code: got %d, want %d", decoded.Code, CodeDisconnectRequest)
	}
}

// VALIDATES: AC-3 -- CoA request authenticator verification.
func TestVerifyCoARequestAuth(t *testing.T) {
	secret := []byte("coa-test-secret")
	pkt := &Packet{
		Code:       CodeCoARequest,
		Identifier: 5,
		Attrs: []Attr{
			{Type: AttrAcctSessionID, Value: AttrString("sess-100")},
		},
	}

	buf := make([]byte, MaxPacketLen)
	n, err := pkt.EncodeTo(buf, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Set correct authenticator.
	auth := AccountingRequestAuth(buf, n, secret)
	copy(buf[4:4+AuthenticatorLen], auth[:])

	// RFC requirement: RFC5176-3.5-4 positive -- a Request Authenticator computed as
	// MD5(Code+Id+Length+16-zero-octets+Attrs+Secret) verifies for a correctly signed packet.
	if !VerifyCoARequestAuth(buf[:n], secret) {
		t.Error("valid CoA auth should verify")
	}

	// RFC requirement: RFC5176-3.5-4 negative -- a tampered authenticator (wrong MD5) is rejected.
	// Corrupt one byte.
	buf[4]++
	if VerifyCoARequestAuth(buf[:n], secret) {
		t.Error("corrupted CoA auth should fail verification")
	}
}

// VALIDATES: AC-4 -- invalid authenticator on short packet.
func TestVerifyCoARequestAuthTooShort(t *testing.T) {
	if VerifyCoARequestAuth(make([]byte, 10), []byte("secret")) {
		t.Error("short packet should fail verification")
	}
}

func TestEncodeAtOffset(t *testing.T) {
	pkt := &Packet{
		Code:       CodeAccessRequest,
		Identifier: 1,
	}

	buf := make([]byte, MaxPacketLen)
	n, err := pkt.EncodeTo(buf, 10)
	if err != nil {
		t.Fatal(err)
	}

	if n != HeaderLen {
		t.Errorf("header-only packet: got %d, want %d", n, HeaderLen)
	}
	if buf[10] != CodeAccessRequest {
		t.Error("code should be at offset 10")
	}
}

// rfc3579SignVector is a hand-built Access-Request and the HMAC-MD5 an OUTSIDE
// tool computed over it. The digest came from
//
//	openssl dgst -md5 -mac HMAC -macopt key:testing123 <the 45 octets below>
//
// so nothing in this file, and nothing in packet.go, produced the expected
// value. A signer that hashed the wrong octet range, keyed on the wrong string,
// or forgot to zero the signature field would agree with a self-computed
// expectation and disagree with this one.
//
// The packet: Code 1 (Access-Request), Identifier 0x2a, Length 45, a fixed
// 16-octet Request Authenticator, a User-Name of "alice", and a
// Message-Authenticator whose sixteen value octets are zero.
var rfc3579SignVector = struct {
	packet []byte
	secret []byte
	digest [AuthenticatorLen]byte
	maOff  int
}{
	packet: []byte{
		0x01, 0x2a, 0x00, 0x2d,
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
		0x01, 0x07, 'a', 'l', 'i', 'c', 'e',
		0x50, 0x12,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	},
	secret: []byte("testing123"),
	digest: [AuthenticatorLen]byte{
		0xef, 0xa1, 0xd9, 0x82, 0xee, 0x5f, 0x38, 0x59,
		0x9f, 0x66, 0xdf, 0x64, 0x86, 0xec, 0x00, 0x15,
	},
	maOff: 29,
}

// TestSignMessageAuthenticatorMatchesRFC3579 proves the signer computes the
// value RFC 3579 Section 3.2 defines, against a digest computed outside ze.
//
// VALIDATES: AC-3 -- the HMAC covers Type, Identifier, Length, the Request
// Authenticator and every attribute, with the signature field zeroed.
// PREVENTS: a signer that hashes only the attributes, that substitutes zeros
// for the Request Authenticator (the RFC 5176 rule, which is a DIFFERENT
// packet's rule), or that hashes the value octets as the caller left them.
//
// RFC requirement: RFC3579-3.2-1 positive -- SignMessageAuthenticator writes
// the HMAC-MD5 an independent implementation computes over the same octets
// (packet.go SignMessageAuthenticator).
func TestSignMessageAuthenticatorMatchesRFC3579(t *testing.T) {
	v := rfc3579SignVector
	buf := make([]byte, MaxPacketLen)
	copy(buf, v.packet)

	signed, err := SignMessageAuthenticator(buf, len(v.packet), v.secret)
	if err != nil {
		t.Fatalf("SignMessageAuthenticator: %v", err)
	}
	if !signed {
		t.Fatal("the packet carries a Message-Authenticator, so the signer must report it signed one")
	}
	got := buf[v.maOff : v.maOff+AuthenticatorLen]
	if !bytes.Equal(got, v.digest[:]) {
		t.Fatalf("Message-Authenticator = %x, openssl says %x", got, v.digest)
	}
	// Everything outside the sixteen value octets is untouched: the signer
	// writes a value, never a packet.
	if !bytes.Equal(buf[:v.maOff], v.packet[:v.maOff]) {
		t.Error("the signer rewrote the packet before the attribute value")
	}
}

// TestSignMessageAuthenticatorZeroesTheSignatureField proves the "sixteen
// octets of zero" rule holds whatever the caller left in the value.
//
// VALIDATES: AC-3 -- the value the caller supplied does not enter the hash.
// PREVENTS: a signer that hashes the buffer as it stands, which would produce a
// value no server can recompute unless the caller happened to pass zeros. The
// vector test above cannot catch it, because its placeholder IS zeros.
//
// RFC requirement: RFC3579-3.2-1 negative -- a non-zero placeholder in the
// signature field produces the SAME digest, so the field is excluded from the
// hash (packet.go SignMessageAuthenticator).
func TestSignMessageAuthenticatorZeroesTheSignatureField(t *testing.T) {
	v := rfc3579SignVector
	buf := make([]byte, MaxPacketLen)
	copy(buf, v.packet)
	for i := range AuthenticatorLen {
		buf[v.maOff+i] = 0xff
	}

	if _, err := SignMessageAuthenticator(buf, len(v.packet), v.secret); err != nil {
		t.Fatalf("SignMessageAuthenticator: %v", err)
	}
	if !bytes.Equal(buf[v.maOff:v.maOff+AuthenticatorLen], v.digest[:]) {
		t.Fatalf("Message-Authenticator = %x, want %x: the placeholder entered the hash",
			buf[v.maOff:v.maOff+AuthenticatorLen], v.digest)
	}
}

// TestSignMessageAuthenticatorReportsAbsence proves a packet without the
// attribute is left alone and says so.
//
// VALIDATES: the signer never invents an attribute, so a caller that owes a
// Message-Authenticator learns it does not have one.
// PREVENTS: a silent no-op reading as a successful signature, which would put
// an unprotected EAP Access-Request on the wire (ai/rules/principles.md).
func TestSignMessageAuthenticatorReportsAbsence(t *testing.T) {
	pkt := &Packet{
		Code:       CodeAccessRequest,
		Identifier: 7,
		Attrs:      []Attr{{Type: AttrUserName, Value: AttrString("alice")}},
	}
	buf := make([]byte, MaxPacketLen)
	n, err := pkt.EncodeTo(buf, 0)
	if err != nil {
		t.Fatal(err)
	}
	before := append([]byte{}, buf[:n]...)

	signed, err := SignMessageAuthenticator(buf, n, []byte("testing123"))
	if err != nil {
		t.Fatalf("SignMessageAuthenticator: %v", err)
	}
	if signed {
		t.Error("no Message-Authenticator is present, so the signer signed nothing")
	}
	if !bytes.Equal(buf[:n], before) {
		t.Error("the signer modified a packet carrying no Message-Authenticator")
	}
}

// TestSignMessageAuthenticatorRefusesAMalformedPacket proves the signer fails
// rather than answering over octets it could not walk.
//
// VALIDATES: a length outside the buffer, and an attribute list that runs off
// the end, both produce an error.
// PREVENTS: a signature computed over a truncated packet, which a server would
// reject and which the caller would read as a signed request.
func TestSignMessageAuthenticatorRefusesAMalformedPacket(t *testing.T) {
	buf := make([]byte, MaxPacketLen)
	copy(buf, rfc3579SignVector.packet)

	if _, err := SignMessageAuthenticator(buf, MinPacketLen-1, rfc3579SignVector.secret); err == nil {
		t.Error("a packet shorter than the header must be refused")
	}
	if _, err := SignMessageAuthenticator(buf, len(buf)+1, rfc3579SignVector.secret); err == nil {
		t.Error("a length past the buffer must be refused")
	}

	// An attribute whose Length runs past the packet: the walk fails and the
	// signer has no offset to write to.
	torn := make([]byte, MaxPacketLen)
	copy(torn, rfc3579SignVector.packet)
	torn[21] = 0xff // User-Name Length, now longer than the whole packet
	if _, err := SignMessageAuthenticator(torn, len(rfc3579SignVector.packet), rfc3579SignVector.secret); err == nil {
		t.Error("an attribute list that does not walk must be refused")
	}
}

// TestSignAndVerifyMessageAuthenticatorAgree holds the signer against the
// verifier ze already shipped, over a REPLY, which is where the two meet.
//
// VALIDATES: AC-3 -- a reply this signer produced verifies under
// verifyResponseMessageAuthenticator, and one octet of drift does not.
// PREVENTS: R-4, the signer and the verifier disagreeing about which octets are
// covered. Each is written from the RFC independently, so agreement is
// evidence; the interop scenario against a real server is the rest of it.
//
// The reply's own Authenticator field is overwritten with the Request
// Authenticator before the HMAC runs, because RFC 3579 Section 3.2 computes a
// reply's Message-Authenticator "using the Request-Authenticator from the
// Access-Request this packet is in reply to". That substitution is the one
// difference between the two directions, and doing it here is what makes this
// test a check of the SIGNER rather than of a shared helper.
func TestSignAndVerifyMessageAuthenticatorAgree(t *testing.T) {
	secret := []byte("s3cr3t")
	var requestAuth [AuthenticatorLen]byte
	for i := range requestAuth {
		requestAuth[i] = byte(0xa0 + i)
	}

	reply := &Packet{
		Code:       CodeAccessChallenge,
		Identifier: 9,
		Attrs: []Attr{
			{Type: AttrEAPMessage, Value: []byte{0x01, 0x09, 0x00, 0x05, 0x01}},
			{Type: AttrMessageAuthenticator, Value: make([]byte, AuthenticatorLen)},
			{Type: AttrState, Value: []byte("opaque")},
		},
	}
	buf := make([]byte, MaxPacketLen)
	n, err := reply.EncodeTo(buf, 0)
	if err != nil {
		t.Fatal(err)
	}
	copy(buf[4:4+AuthenticatorLen], requestAuth[:])
	signed, err := SignMessageAuthenticator(buf, n, secret)
	if err != nil {
		t.Fatal(err)
	}
	if !signed {
		t.Fatal("the reply carries a Message-Authenticator")
	}
	// A real server writes the Response Authenticator over the field the HMAC
	// read. The verifier substitutes the Request Authenticator back, so put
	// something else there to prove it does.
	copy(buf[4:4+AuthenticatorLen], bytes.Repeat([]byte{0x5a}, AuthenticatorLen))

	if !verifyResponseMessageAuthenticator(buf[:n], requestAuth, secret) {
		t.Error("the verifier rejected a reply the signer produced")
	}

	tampered := append([]byte{}, buf[:n]...)
	tampered[HeaderLen+2] ^= 0x01 // one octet of the EAP-Message value
	if verifyResponseMessageAuthenticator(tampered, requestAuth, secret) {
		t.Error("the verifier accepted a reply whose attributes moved under the signature")
	}
}
