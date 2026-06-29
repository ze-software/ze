package pppoeclient

import (
	"crypto/md5" //nolint:gosec // testing CHAP-MD5
	"encoding/binary"
	"encoding/hex"
	"net/netip"
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

// TestIPCPAddressAssignment validates the IPCP Configure-Request with
// IP-Address option 3 per RFC 1332 Section 3.3.
//
// VALIDATES: AC-5 - IPv4 address assigned by server, applied to ppp interface.
func TestIPCPAddressAssignment(t *testing.T) {
	// Client requests 0.0.0.0 (RFC 1332: "peer, assign me an address").
	pkt := buildIPCPRequest(0x01, netip.IPv4Unspecified())

	require.True(t, len(pkt) >= 10)
	assert.Equal(t, byte(1), pkt[0], "Code = Configure-Request")
	assert.Equal(t, byte(0x01), pkt[1], "Identifier")

	pktLen := int(binary.BigEndian.Uint16(pkt[2:4]))
	assert.Equal(t, 10, pktLen)

	assert.Equal(t, byte(ipcpOptionIPAddress), pkt[4], "Option type = IP-Address")
	assert.Equal(t, byte(6), pkt[5], "Option length = 6")
	assert.Equal(t, []byte{0, 0, 0, 0}, pkt[6:10], "IP = 0.0.0.0")
}

// TestIPCPAddressAssignmentWithAddr validates IPCP with a specific address
// (used for the second request after server Naks with the assigned address).
func TestIPCPAddressAssignmentWithAddr(t *testing.T) {
	addr := netip.MustParseAddr("10.0.0.42")
	pkt := buildIPCPRequest(0x02, addr)

	assert.Equal(t, []byte{10, 0, 0, 42}, pkt[6:10])
}

// TestIPCPNakAddressParsing validates extraction of server-assigned IP
// from an IPCP Configure-Nak containing option 3.
func TestIPCPNakAddressParsing(t *testing.T) {
	// Simulate a Nak payload with IP-Address option 3.
	data := []byte{
		ipcpOptionIPAddress, 6, // type=3, len=6
		192, 168, 1, 100, // 192.168.1.100
	}
	addr := parseIPCPNakAddress(data)
	assert.Equal(t, netip.MustParseAddr("192.168.1.100"), addr)
}

// TestIPCPNakAddressParsingMultipleOptions tests parsing with extra options.
func TestIPCPNakAddressParsingMultipleOptions(t *testing.T) {
	data := []byte{
		2, 6, 0x00, 0x2d, 0x0f, 0x01, // IP-Compression-Protocol (option 2)
		ipcpOptionIPAddress, 6, 10, 20, 30, 40, // IP-Address = 10.20.30.40
	}
	addr := parseIPCPNakAddress(data)
	assert.Equal(t, netip.MustParseAddr("10.20.30.40"), addr)
}

// TestIPCPNakAddressParsingEmpty tests parsing with no option 3.
func TestIPCPNakAddressParsingEmpty(t *testing.T) {
	addr := parseIPCPNakAddress([]byte{})
	assert.False(t, addr.IsValid())

	addr = parseIPCPNakAddress([]byte{2, 4, 0x00, 0x2d}) // only option 2
	assert.False(t, addr.IsValid())
}

// TestIPCPNakAddressParsingTruncated tests parsing with truncated data.
func TestIPCPNakAddressParsingTruncated(t *testing.T) {
	data := []byte{ipcpOptionIPAddress, 6, 10, 20} // truncated: only 2 of 4 IP bytes
	addr := parseIPCPNakAddress(data)
	assert.False(t, addr.IsValid())
}
