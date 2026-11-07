package pppoeclient

import (
	"crypto/md5" //nolint:gosec // testing CHAP-MD5
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCHAPAuthentication validates the CHAP-MD5 response computation
// per RFC 1994 Section 4.1: Response = MD5(ID || secret || challenge).
//
// VALIDATES: AC-4 - Credentials sent per agreed method.
func TestCHAPAuthentication(t *testing.T) {
	id := byte(0x01)
	secret := "testing123"
	challenge := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE, 0xBA, 0xBE,
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}

	got := chapMD5Response(id, secret, challenge)

	// Compute expected independently.
	h := md5.New() //nolint:gosec // test
	h.Write([]byte{id})
	h.Write([]byte(secret))
	h.Write(challenge)
	var want [md5.Size]byte
	copy(want[:], h.Sum(nil))

	assert.Equal(t, want, got)
	assert.Equal(t, md5.Size, len(got))
}

// TestCHAPAuthenticationKnownVector validates against a known CHAP-MD5 vector.
func TestCHAPAuthenticationKnownVector(t *testing.T) {
	// RFC 1994 doesn't provide test vectors, so we use a computed one.
	// ID=0x42, secret="password", challenge=16 zero bytes.
	id := byte(0x42)
	secret := "password"
	challenge := make([]byte, 16)

	got := chapMD5Response(id, secret, challenge)

	// MD5(0x42 || "password" || 16*0x00)
	h := md5.New() //nolint:gosec // test
	h.Write([]byte{0x42})
	h.Write([]byte("password"))
	h.Write(challenge)
	want := hex.EncodeToString(h.Sum(nil))

	assert.Equal(t, want, hex.EncodeToString(got[:]))
}

// TestPAPAuthentication validates the PAP Authenticate-Request packet format
// per RFC 1334 Section 2.2.1.
//
// VALIDATES: AC-4 - PAP auth-request format.
func TestPAPAuthentication(t *testing.T) {
	pkt := buildPAPAuthRequest(0x01, "user@isp.net", "secret123")

	require.True(t, len(pkt) >= 4)
	assert.Equal(t, byte(1), pkt[0], "Code should be Authenticate-Request")
	assert.Equal(t, byte(0x01), pkt[1], "Identifier")

	pktLen := int(binary.BigEndian.Uint16(pkt[2:4]))
	assert.Equal(t, len(pkt), pktLen, "Length field must match actual packet length")

	peerIDLen := int(pkt[4])
	assert.Equal(t, len("user@isp.net"), peerIDLen)
	assert.Equal(t, "user@isp.net", string(pkt[5:5+peerIDLen]))

	passwdLen := int(pkt[5+peerIDLen])
	assert.Equal(t, len("secret123"), passwdLen)
	assert.Equal(t, "secret123", string(pkt[6+peerIDLen:6+peerIDLen+passwdLen]))
}

// TestPAPAuthenticationEmpty validates PAP with empty username/password.
func TestPAPAuthenticationEmpty(t *testing.T) {
	pkt := buildPAPAuthRequest(0x00, "", "")

	assert.Equal(t, byte(1), pkt[0])
	pktLen := int(binary.BigEndian.Uint16(pkt[2:4]))
	assert.Equal(t, 6, pktLen)       // header(4) + peerIDLen(1) + passwdLen(1)
	assert.Equal(t, byte(0), pkt[4]) // peerID length = 0
	assert.Equal(t, byte(0), pkt[5]) // passwd length = 0
}
