package radius

import (
	"bytes"
	"crypto/md5" //nolint:gosec // Testing RFC 2865 MD5-based encoding
	"testing"
)

func TestEncodeUserPassword(t *testing.T) {
	secret := []byte("testing123")
	var auth [AuthenticatorLen]byte
	copy(auth[:], "0123456789abcdef")

	password := []byte("mypassword")
	encoded := EncodeUserPassword(password, secret, auth)

	// RFC 2865 Section 5.2: result must be multiple of 16 bytes.
	if len(encoded)%16 != 0 {
		t.Errorf("encoded length %d not multiple of 16", len(encoded))
	}

	// Verify we can decode it back by reversing the XOR.
	h := md5.New() //nolint:gosec // Testing RFC 2865
	h.Write(secret)
	h.Write(auth[:])
	block := h.Sum(nil)

	decoded := make([]byte, len(encoded))
	for i := range 16 {
		decoded[i] = encoded[i] ^ block[i]
	}

	// First 10 bytes should be "mypassword", rest should be zero padding.
	if string(decoded[:10]) != "mypassword" {
		t.Errorf("decoded first block: got %q, want %q", decoded[:10], "mypassword")
	}
	for i := 10; i < 16; i++ {
		if decoded[i] != 0 {
			t.Errorf("decoded[%d] = %d, want 0 (padding)", i, decoded[i])
		}
	}
}

func TestEncodeUserPasswordEmpty(t *testing.T) {
	secret := []byte("secret")
	var auth [AuthenticatorLen]byte

	encoded := EncodeUserPassword(nil, secret, auth)
	if len(encoded) != 16 {
		t.Errorf("empty password should pad to 16 bytes, got %d", len(encoded))
	}
}

func TestEncodeUserPasswordMultiBlock(t *testing.T) {
	secret := []byte("secret")
	var auth [AuthenticatorLen]byte
	copy(auth[:], "authenticator123")

	// 20-char password needs 2 blocks (32 bytes).
	password := []byte("12345678901234567890")
	encoded := EncodeUserPassword(password, secret, auth)

	if len(encoded) != 32 {
		t.Errorf("20-byte password should produce 32 bytes, got %d", len(encoded))
	}
}

func TestEncodeCHAPPassword(t *testing.T) {
	response := make([]byte, 16)
	for i := range response {
		response[i] = byte(i)
	}

	val := EncodeCHAPPassword(42, response)

	// RFC 2865 Section 5.3: 1-byte Ident + 16-byte response = 17 bytes.
	if len(val) != 17 {
		t.Fatalf("CHAP-Password length: got %d, want 17", len(val))
	}
	if val[0] != 42 {
		t.Errorf("CHAP identifier: got %d, want 42", val[0])
	}
	if !bytes.Equal(val[1:], response) {
		t.Error("CHAP response mismatch")
	}
}

func TestEncodeMSCHAPv2Response(t *testing.T) {
	peerChallenge := make([]byte, 16)
	ntResponse := make([]byte, 24)
	for i := range peerChallenge {
		peerChallenge[i] = byte(i)
	}
	for i := range ntResponse {
		ntResponse[i] = byte(i + 100)
	}

	vsaValue, err := EncodeMSCHAP2Response(7, peerChallenge, ntResponse)
	if err != nil {
		t.Fatal(err)
	}

	// Outer VSA: Type(1=26) + Length(1) + VendorID(4=311) + VendorType(1=25) + VendorLength(1) + Value(50).
	if vsaValue[0] != AttrVendorSpecific {
		t.Errorf("VSA type: got %d, want %d", vsaValue[0], AttrVendorSpecific)
	}

	vendorID, vendorType, value, err := DecodeVSA(vsaValue[2:])
	if err != nil {
		t.Fatal(err)
	}
	if vendorID != VendorMicrosoft {
		t.Errorf("vendor ID: got %d, want %d", vendorID, VendorMicrosoft)
	}
	if vendorType != MSCHAP2Response {
		t.Errorf("vendor type: got %d, want %d", vendorType, MSCHAP2Response)
	}

	// Value: Ident(1) + Flags(1) + PeerChallenge(16) + Reserved(8) + Response(24) = 50.
	if len(value) != 50 {
		t.Fatalf("MS-CHAP2-Response value length: got %d, want 50", len(value))
	}
	if value[0] != 7 {
		t.Errorf("ident: got %d, want 7", value[0])
	}
	if value[1] != 0 {
		t.Errorf("flags: got %d, want 0", value[1])
	}
	if !bytes.Equal(value[2:18], peerChallenge) {
		t.Error("peer challenge mismatch")
	}
	if !bytes.Equal(value[26:50], ntResponse) {
		t.Error("NT response mismatch")
	}
}

func TestEncodeMSCHAP2ResponseBadSizes(t *testing.T) {
	_, err := EncodeMSCHAP2Response(0, make([]byte, 15), make([]byte, 24))
	if err == nil {
		t.Error("expected error for 15-byte peer challenge")
	}

	_, err = EncodeMSCHAP2Response(0, make([]byte, 16), make([]byte, 23))
	if err == nil {
		t.Error("expected error for 23-byte NT response")
	}
}

func TestEncodeVSARoundTrip(t *testing.T) {
	data := []byte("test-value")
	encoded, err := EncodeVSA(12345, 99, data)
	if err != nil {
		t.Fatal(err)
	}

	vendorID, vendorType, value, err := DecodeVSA(encoded[2:])
	if err != nil {
		t.Fatal(err)
	}
	if vendorID != 12345 {
		t.Errorf("vendor ID: got %d, want 12345", vendorID)
	}
	if vendorType != 99 {
		t.Errorf("vendor type: got %d, want 99", vendorType)
	}
	if !bytes.Equal(value, data) {
		t.Error("value mismatch")
	}
}

func TestDecodeUint32(t *testing.T) {
	val := AttrUint32(0x01020304)
	decoded, err := DecodeUint32(val)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != 0x01020304 {
		t.Errorf("got %08x, want 01020304", decoded)
	}
}

func TestDecodeUint32BadSize(t *testing.T) {
	_, err := DecodeUint32([]byte{1, 2, 3})
	if err == nil {
		t.Error("expected error for 3-byte value")
	}
}
