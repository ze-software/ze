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
	_, err := Decode(make([]byte, 19))
	if err == nil {
		t.Fatal("expected error for packet < 20 bytes")
	}
}

func TestDecodeTooLong(t *testing.T) {
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
	_, err := Decode(buf)
	if err == nil {
		t.Fatal("expected error for invalid length field")
	}
}

func TestResponseAuthenticator(t *testing.T) {
	var reqAuth [AuthenticatorLen]byte
	copy(reqAuth[:], "0123456789abcdef")
	secret := []byte("testing123")

	auth1 := ResponseAuthenticator(CodeAccessAccept, 1, 20, reqAuth, nil, secret)
	auth2 := ResponseAuthenticator(CodeAccessAccept, 1, 20, reqAuth, nil, secret)
	if auth1 != auth2 {
		t.Error("same inputs should produce same authenticator")
	}

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

	if !VerifyResponseAuth(buf[:n], reqAuth, secret) {
		t.Error("valid response auth should verify")
	}

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
