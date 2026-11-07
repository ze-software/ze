// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- client-mode PPP authentication

package pppoeclient

import (
	"crypto/md5" //nolint:gosec // CHAP-MD5 per RFC 1994 Section 4; protocol requirement
)

// chapMD5Response computes the CHAP-MD5 response per RFC 1994 Section 4.1:
// Response = MD5(ID || secret || challenge-value).
func chapMD5Response(id byte, secret string, challenge []byte) [md5.Size]byte {
	h := md5.New() //nolint:gosec // CHAP-MD5 per RFC 1994; protocol mandates MD5
	h.Write([]byte{id})
	h.Write([]byte(secret))
	h.Write(challenge)
	var digest [md5.Size]byte
	copy(digest[:], h.Sum(nil))
	return digest
}

// buildPAPAuthRequest builds a PAP Authenticate-Request packet per
// RFC 1334 Section 2.2.1.
func buildPAPAuthRequest(id byte, username, password string) []byte {
	peerIDLen := len(username)
	passwdLen := len(password)
	dataLen := 1 + peerIDLen + 1 + passwdLen
	pktLen := 4 + dataLen
	buf := make([]byte, pktLen)
	buf[0] = 1 // Authenticate-Request
	buf[1] = id
	buf[2] = byte(pktLen >> 8)   //nolint:gosec // pktLen < 65536
	buf[3] = byte(pktLen & 0xff) //nolint:gosec // low byte
	buf[4] = byte(peerIDLen)     //nolint:gosec // username length < 256 in practice
	copy(buf[5:5+peerIDLen], username)
	buf[5+peerIDLen] = byte(passwdLen) //nolint:gosec // password length < 256 in practice
	copy(buf[6+peerIDLen:], password)
	return buf
}
