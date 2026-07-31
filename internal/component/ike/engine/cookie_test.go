package engine

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/wire"
)

// resetCookieSecret clears the process-wide cookie secret so one test cannot inherit
// another's rotation state. The secret is a package global because one responder has
// one secret; the tests share the process, so each resets it.
func resetCookieSecret(t *testing.T) {
	t.Helper()
	responderCookies.mu.Lock()
	defer responderCookies.mu.Unlock()
	responderCookies.current = [32]byte{}
	responderCookies.previous = [32]byte{}
	responderCookies.version = 0
	responderCookies.previousValid = false
	responderCookies.rotatedAt = time.Time{}
}

// withCookieThreshold sets the half-open tolerance for one test and restores it.
func withCookieThreshold(t *testing.T, n uint32) {
	t.Helper()
	old := CookieThreshold()
	SetCookieThreshold(n)
	t.Cleanup(func() { SetCookieThreshold(old) })
}

// admitWithoutCookieChallenge raises the half-open tolerance past anything a unit test
// produces, so an inbound IKE_SA_INIT is admitted without answering a challenge.
//
// The cookie challenge is an ADMISSION gate. A test whose subject is what the responder
// does AFTER admission -- the parallel half-open slot, the supersede rule, the inbound
// rate limiter -- has to get past it to reach its own subject. Raising the tolerance is
// how a monotone threshold is turned off; there is no magic disable value.
//
// The challenge path itself is proven by the tagged tests in rfc7296_cookie_test.go, so
// nothing is left uncovered by opting these tests out.
func admitWithoutCookieChallenge(t *testing.T) {
	t.Helper()
	withCookieThreshold(t, ^uint32(0))
}

func cookieTestInputs() ([8]byte, []byte, net.IP) {
	return [8]byte{1, 2, 3, 4, 5, 6, 7, 8}, []byte("nonce-material-32-octets-long!!!"), net.ParseIP("198.51.100.7")
}

// buildSAInitRaw encodes an IKE_SA_INIT request carrying the given payloads, so the
// pre-state walker is driven with bytes a peer could really send. It takes no
// *testing.T because it cannot fail: the buffer is sized from Len(), which WriteTo is
// contracted to match. That also lets the fuzz seed corpus call it.
func buildSAInitRaw(spiI [8]byte, payloads []wire.PayloadEntry) []byte {
	msg := wire.Message{
		Header: wire.Header{
			InitiatorSPI: spiI,
			MajorVersion: 2,
			ExchangeType: wire.ExchangeIKESAInit,
			Flags:        wire.FlagInitiator,
		},
		Payloads: payloads,
	}
	buf := make([]byte, msg.Len())
	n := msg.WriteTo(buf, 0)
	return buf[:n]
}

func cookiePayload(data []byte) wire.PayloadEntry {
	return wire.PayloadEntry{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyCookie, NotificationData: data}}
}

// VALIDATES: a cookie minted for one initiation verifies, and the same inputs mint the
// same value while the secret stands.
// PREVENTS: a cookie that changes per call, which would make every challenge fail and
// turn the feature into a handshake outage.
func TestMintedCookieVerifies(t *testing.T) {
	resetCookieSecret(t)
	spiI, ni, ip := cookieTestInputs()
	now := time.Now()

	first := mintCookie(spiI, ni, ip, now)
	if len(first) == 0 {
		t.Fatal("mintCookie returned nothing")
	}
	second := mintCookie(spiI, ni, ip, now)
	if !bytes.Equal(first, second) {
		t.Error("the same initiation minted two different cookies; the challenge could never be answered")
	}
	if !verifyCookie(first, spiI, ni, ip, now) {
		t.Error("a freshly minted cookie did not verify")
	}
}

// VALIDATES: the cookie binds to the address, the nonce and the initiator SPI, so a
// cookie issued to one sender cannot be replayed by another.
// PREVENTS: a cookie that proves nothing, which would leave the half-open slot open to
// exactly the spoofed datagram the challenge exists to stop.
func TestVerifyCookieRejectsSubstitutedInputs(t *testing.T) {
	resetCookieSecret(t)
	spiI, ni, ip := cookieTestInputs()
	now := time.Now()
	cookie := mintCookie(spiI, ni, ip, now)

	otherSPI := [8]byte{9, 9, 9, 9, 9, 9, 9, 9}
	cases := []struct {
		name string
		spiI [8]byte
		ni   []byte
		ip   net.IP
	}{
		{"another address", spiI, ni, net.ParseIP("203.0.113.9")},
		{"another nonce", spiI, []byte("a completely different nonce!!!!"), ip},
		{"another initiator spi", otherSPI, ni, ip},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if verifyCookie(cookie, tc.spiI, tc.ni, tc.ip, now) {
				t.Errorf("a cookie minted for other inputs verified against %s", tc.name)
			}
		})
	}
}

// VALIDATES: verifyCookie denies on every miss -- empty data, a wrong length, an
// unknown version octet, a nil address, an empty nonce.
// PREVENTS: the zero-value trap, where an absent or malformed cookie reads as a valid
// answer and takes the half-open slot (ai/rules/fail-closed-guards.md).
func TestVerifyCookieFailsClosed(t *testing.T) {
	resetCookieSecret(t)
	spiI, ni, ip := cookieTestInputs()
	now := time.Now()
	cookie := mintCookie(spiI, ni, ip, now)

	tampered := append([]byte(nil), cookie...)
	tampered[0] ^= 0xff
	truncated := cookie[:len(cookie)-1]
	extended := append(append([]byte(nil), cookie...), 0)

	cases := []struct {
		name string
		data []byte
		ni   []byte
		ip   net.IP
	}{
		{"nil data", nil, ni, ip},
		{"empty data", []byte{}, ni, ip},
		{"truncated", truncated, ni, ip},
		{"extended", extended, ni, ip},
		{"unknown version octet", tampered, ni, ip},
		{"nil address", cookie, ni, nil},
		{"empty nonce", cookie, nil, ip},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if verifyCookie(tc.data, spiI, tc.ni, tc.ip, now) {
				t.Errorf("verifyCookie accepted %s", tc.name)
			}
		})
	}
}

// VALIDATES: a cookie minted under the previous secret still verifies for one rotation
// interval, and stops verifying after it.
// PREVENTS: a rotation that invalidates every in-flight challenge, and a responder that
// accepts an old cookie forever, which RFC 7296 Section 2.6 says "would defeat part of
// the DoS protection".
func TestVerifyCookieAcceptsThePreviousSecretForOneInterval(t *testing.T) {
	resetCookieSecret(t)
	spiI, ni, ip := cookieTestInputs()
	start := time.Now()

	old := mintCookie(spiI, ni, ip, start)
	// Mint again past the interval, which rotates the secret.
	rotated := start.Add(cookieRotateInterval + time.Second)
	fresh := mintCookie(spiI, ni, ip, rotated)
	if bytes.Equal(old, fresh) {
		t.Fatal("the secret did not rotate past the interval")
	}
	if !verifyCookie(old, spiI, ni, ip, rotated) {
		t.Error("a cookie minted under the previous secret was refused straight after rotation")
	}
	expired := rotated.Add(cookieRotateInterval + time.Second)
	if verifyCookie(old, spiI, ni, ip, expired) {
		t.Error("a cookie minted under the previous secret still verified a whole interval later")
	}
}

// VALIDATES: the pre-state walker finds the first COOKIE notify and the Nonce in a real
// IKE_SA_INIT.
// PREVENTS: a walker that reads neither, which would make every challenge unanswerable.
func TestScanSAInitPreStateReadsCookieAndNonce(t *testing.T) {
	spiI, ni, _ := cookieTestInputs()
	want := []byte{0xaa, 0xbb, 0xcc}
	raw := buildSAInitRaw(spiI, []wire.PayloadEntry{
		cookiePayload(want),
		{Payload: &wire.PayloadNonce{NonceData: ni}},
	})
	cookie, nonce, ok := scanSAInitPreState(raw)
	if !ok {
		t.Fatal("a well-formed IKE_SA_INIT was refused by the walker")
	}
	if !bytes.Equal(cookie, want) {
		t.Errorf("cookie = %x, want %x", cookie, want)
	}
	if !bytes.Equal(nonce, ni) {
		t.Errorf("nonce = %x, want %x", nonce, ni)
	}
}

// VALIDATES: a message with no COOKIE notify yields no cookie but still parses.
// PREVENTS: treating a first, cookie-less initiation as malformed, which would answer
// it with silence instead of a challenge.
func TestScanSAInitPreStateReportsAnAbsentCookie(t *testing.T) {
	spiI, ni, _ := cookieTestInputs()
	raw := buildSAInitRaw(spiI, []wire.PayloadEntry{
		{Payload: &wire.PayloadNonce{NonceData: ni}},
	})
	cookie, nonce, ok := scanSAInitPreState(raw)
	if !ok {
		t.Fatal("a cookie-less IKE_SA_INIT was refused by the walker")
	}
	if cookie != nil {
		t.Errorf("cookie = %x, want none", cookie)
	}
	if !bytes.Equal(nonce, ni) {
		t.Error("the nonce was not read from a cookie-less message")
	}
}

// VALIDATES: the walker denies a chain that cannot advance, one that is too long, and a
// COOKIE outside the 1..64 octets RFC 7296 Section 2.6 allows.
// PREVENTS: an unbounded CPU loop reachable by one unauthenticated datagram, and a
// 600-octet cookie being reflected back to its sender.
func TestScanSAInitPreStateDeniesMalformedChains(t *testing.T) {
	spiI, ni, _ := cookieTestInputs()

	zeroLength := buildSAInitRaw(spiI, []wire.PayloadEntry{
		{Payload: &wire.PayloadNonce{NonceData: ni}},
	})
	// Rewrite the first payload's length to zero: the chain can never advance.
	zeroLength[wire.HeaderLen+2] = 0
	zeroLength[wire.HeaderLen+3] = 0

	runOff := buildSAInitRaw(spiI, []wire.PayloadEntry{
		{Payload: &wire.PayloadNonce{NonceData: ni}},
	})
	// A payload that claims to end past the message.
	runOff[wire.HeaderLen+2] = 0xff
	runOff[wire.HeaderLen+3] = 0xff

	oversized := buildSAInitRaw(spiI, []wire.PayloadEntry{
		cookiePayload(bytes.Repeat([]byte{0x41}, maxCookieLen+1)),
		{Payload: &wire.PayloadNonce{NonceData: ni}},
	})

	long := make([]wire.PayloadEntry, 0, maxPreStatePayloads+2)
	for range maxPreStatePayloads + 2 {
		long = append(long, wire.PayloadEntry{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyInitialContact}})
	}
	longChain := buildSAInitRaw(spiI, long)

	cases := []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"header only", make([]byte, wire.HeaderLen-1)},
		{"zero-length payload", zeroLength},
		{"payload past the end", runOff},
		{"cookie over 64 octets", oversized},
		{"chain longer than the bound", longChain},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, ok := scanSAInitPreState(tc.data); ok {
				t.Errorf("the walker accepted %s", tc.name)
			}
		})
	}
}

// VALIDATES: a 64-octet cookie is accepted and a 65-octet one is not.
// PREVENTS: an off-by-one that reads the RFC 7296 Section 2.6 bound as "< 64" rather
// than "<= 64" (ai/rules/tdd.md, boundary testing).
func TestScanSAInitPreStateCookieLengthBoundary(t *testing.T) {
	spiI, ni, _ := cookieTestInputs()
	for _, tc := range []struct {
		length int
		want   bool
	}{{maxCookieLen - 1, true}, {maxCookieLen, true}, {maxCookieLen + 1, false}} {
		raw := buildSAInitRaw(spiI, []wire.PayloadEntry{
			cookiePayload(bytes.Repeat([]byte{0x42}, tc.length)),
			{Payload: &wire.PayloadNonce{NonceData: ni}},
		})
		_, _, ok := scanSAInitPreState(raw)
		if ok != tc.want {
			t.Errorf("a %d-octet cookie: accepted = %v, want %v", tc.length, ok, tc.want)
		}
	}
}

// VALIDATES: cookieRequired reads true with no session, and honors the configured
// half-open tolerance monotonically.
// PREVENTS: a threshold whose default lets the first initiation through unchallenged,
// which is the exact datagram that wedges a peer's only half-open slot.
func TestCookieRequiredIsMonotoneAndFailsClosed(t *testing.T) {
	withCookieThreshold(t, 0)
	if !cookieRequired(nil) {
		t.Error("cookieRequired must fail closed on a nil session")
	}
	ps := &PeerSession{peerName: "p1"}
	if !cookieRequired(ps) {
		t.Error("at threshold 0 with no half-open handshakes the first initiation must still be challenged")
	}
	withCookieThreshold(t, 1)
	if cookieRequired(ps) {
		t.Error("at threshold 1 with no half-open handshakes no challenge is due")
	}
}

// FuzzScanSAInitPreState drives the hand-rolled walker with arbitrary bytes.
// ai/rules/tdd.md makes a fuzz target mandatory for wire-format parsing of external
// input, and this walker runs on unauthenticated input before any state exists.
func FuzzScanSAInitPreState(f *testing.F) {
	spiI, ni, _ := cookieTestInputs()
	f.Add(buildSAInitRaw(spiI, []wire.PayloadEntry{
		cookiePayload([]byte{1, 2, 3}),
		{Payload: &wire.PayloadNonce{NonceData: ni}},
	}))
	f.Add([]byte{})
	f.Add(make([]byte, wire.HeaderLen))
	f.Fuzz(func(t *testing.T, data []byte) {
		cookie, nonce, ok := scanSAInitPreState(data)
		if !ok {
			return
		}
		if len(cookie) > maxCookieLen {
			t.Errorf("the walker returned a %d-octet cookie, over the RFC 7296 Section 2.6 bound", len(cookie))
		}
		if ok && len(data) < wire.HeaderLen {
			t.Error("the walker accepted a message shorter than the IKE header")
		}
		_ = nonce
	})
}
