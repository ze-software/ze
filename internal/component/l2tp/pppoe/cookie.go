// Design: docs/architecture/l2tp/bng-5-pppoe.md -- AC-Cookie anti-DoS protection
// Related: server.go -- InterfaceServer calls GenerateCookie/VerifyCookie
// RFC 2516 Section 5.2: AC-Cookie for DoS protection.
// The Access Concentrator includes an AC-Cookie tag in PADO replies.
// The client echoes it back in PADR. The AC verifies the cookie
// before allocating a session, preventing PADR floods from spoofed MACs.

package pppoe

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"time"
)

// CookieLen is the on-wire size of an AC-Cookie tag value:
// 16 bytes truncated HMAC-SHA256 + 4 bytes Unix timestamp.
const CookieLen = 20

const hmacTruncLen = 16

// nowUnix returns the current Unix timestamp. Tests override this
// via export_test.go to control time without sleeping.
var nowUnix = func() int64 { return time.Now().Unix() }

// CookieKey holds the per-interface secret used to generate and
// verify AC-Cookies. Each access interface gets its own key so
// a cookie from one interface cannot be replayed on another.
type CookieKey struct {
	secret [32]byte
}

// NewCookieKey generates a fresh random key from crypto/rand.
func NewCookieKey() (CookieKey, error) {
	var k CookieKey
	if _, err := rand.Read(k.secret[:]); err != nil {
		return CookieKey{}, err
	}
	return k, nil
}

// GenerateCookie produces a CookieLen-byte AC-Cookie that binds the
// server MAC, client MAC, and optional relay-session-id to a timestamp.
func GenerateCookie(key CookieKey, serverMAC, clientMAC, relaySessionID []byte) []byte {
	ts := nowUnix()

	mac := computeMAC(key, serverMAC, clientMAC, relaySessionID, ts)

	cookie := make([]byte, CookieLen)
	copy(cookie, mac[:hmacTruncLen])
	binary.BigEndian.PutUint32(cookie[hmacTruncLen:], uint32(ts))
	return cookie
}

// VerifyCookie checks that cookie was produced by GenerateCookie with
// the same key, MACs, and relay-session-id, and that the embedded
// timestamp has not expired. Returns false for any failure (wrong
// key, wrong MACs, expired, truncated).
func VerifyCookie(key CookieKey, cookie, serverMAC, clientMAC, relaySessionID []byte, timeout time.Duration) bool {
	if len(cookie) != CookieLen {
		return false
	}

	ts := int64(binary.BigEndian.Uint32(cookie[hmacTruncLen:])) // uint32 Unix seconds; overflows 2106

	elapsed := nowUnix() - ts
	if elapsed < 0 || elapsed > int64(timeout.Seconds()) {
		return false
	}

	mac := computeMAC(key, serverMAC, clientMAC, relaySessionID, ts)

	return hmac.Equal(cookie[:hmacTruncLen], mac[:hmacTruncLen])
}

func computeMAC(key CookieKey, serverMAC, clientMAC, relaySessionID []byte, ts int64) [sha256.Size]byte {
	h := hmac.New(sha256.New, key.secret[:])
	h.Write(serverMAC)
	h.Write(clientMAC)
	if len(relaySessionID) > 0 {
		h.Write(relaySessionID)
	}
	var tsBuf [8]byte
	binary.BigEndian.PutUint64(tsBuf[:], uint64(ts))
	h.Write(tsBuf[:])

	var out [sha256.Size]byte
	h.Sum(out[:0])
	return out
}
