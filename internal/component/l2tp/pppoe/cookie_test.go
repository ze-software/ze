package pppoe_test

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
)

var (
	serverMAC = []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	clientMAC = []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	relayID   = []byte{0x01, 0x02, 0x03, 0x04}
	timeout   = 5 * time.Second
)

func fixedClock(ts int64) func() {
	return pppoe.SetNowUnix(func() int64 { return ts })
}

func mustKey(t *testing.T) pppoe.CookieKey {
	t.Helper()
	k, err := pppoe.NewCookieKey()
	if err != nil {
		t.Fatalf("NewCookieKey: %v", err)
	}
	return k
}

func TestACCookieGenVerify(t *testing.T) {
	restore := fixedClock(1000)
	defer restore()

	key := mustKey(t)
	cookie := pppoe.GenerateCookie(key, serverMAC, clientMAC, nil)

	if len(cookie) != pppoe.CookieLen {
		t.Fatalf("cookie length = %d, want %d", len(cookie), pppoe.CookieLen)
	}
	if !pppoe.VerifyCookie(key, cookie, serverMAC, clientMAC, nil, timeout) {
		t.Fatal("VerifyCookie returned false for valid cookie")
	}
}

func TestACCookieReplay(t *testing.T) {
	key := mustKey(t)

	restore := fixedClock(1000)
	cookie := pppoe.GenerateCookie(key, serverMAC, clientMAC, nil)
	restore()

	restore = fixedClock(1000 + int64(timeout.Seconds()) + 1)
	defer restore()

	if pppoe.VerifyCookie(key, cookie, serverMAC, clientMAC, nil, timeout) {
		t.Fatal("VerifyCookie should reject expired cookie")
	}
}

func TestACCookieWrongMAC(t *testing.T) {
	restore := fixedClock(1000)
	defer restore()

	key := mustKey(t)
	cookie := pppoe.GenerateCookie(key, serverMAC, clientMAC, nil)

	otherMAC := []byte{0xFF, 0xEE, 0xDD, 0xCC, 0xBB, 0xAA}
	if pppoe.VerifyCookie(key, cookie, serverMAC, otherMAC, nil, timeout) {
		t.Fatal("VerifyCookie should reject cookie with wrong client MAC")
	}
	if pppoe.VerifyCookie(key, cookie, otherMAC, clientMAC, nil, timeout) {
		t.Fatal("VerifyCookie should reject cookie with wrong server MAC")
	}
}

func TestACCookieDifferentKey(t *testing.T) {
	restore := fixedClock(1000)
	defer restore()

	key1 := mustKey(t)
	key2 := mustKey(t)
	cookie := pppoe.GenerateCookie(key1, serverMAC, clientMAC, nil)

	if pppoe.VerifyCookie(key2, cookie, serverMAC, clientMAC, nil, timeout) {
		t.Fatal("VerifyCookie should reject cookie from different key")
	}
}

func TestACCookieWithRelaySessionID(t *testing.T) {
	restore := fixedClock(1000)
	defer restore()

	key := mustKey(t)
	cookie := pppoe.GenerateCookie(key, serverMAC, clientMAC, relayID)

	if !pppoe.VerifyCookie(key, cookie, serverMAC, clientMAC, relayID, timeout) {
		t.Fatal("VerifyCookie should accept cookie with matching relay-session-id")
	}
	if pppoe.VerifyCookie(key, cookie, serverMAC, clientMAC, nil, timeout) {
		t.Fatal("VerifyCookie should reject cookie when relay-session-id is missing")
	}
	otherRelay := []byte{0x05, 0x06, 0x07, 0x08}
	if pppoe.VerifyCookie(key, cookie, serverMAC, clientMAC, otherRelay, timeout) {
		t.Fatal("VerifyCookie should reject cookie with wrong relay-session-id")
	}
}

func TestACCookieWithoutRelaySessionID(t *testing.T) {
	restore := fixedClock(1000)
	defer restore()

	key := mustKey(t)
	cookie := pppoe.GenerateCookie(key, serverMAC, clientMAC, nil)

	if !pppoe.VerifyCookie(key, cookie, serverMAC, clientMAC, nil, timeout) {
		t.Fatal("VerifyCookie should accept cookie without relay-session-id")
	}
}

func TestACCookieTruncated(t *testing.T) {
	restore := fixedClock(1000)
	defer restore()

	key := mustKey(t)
	cookie := pppoe.GenerateCookie(key, serverMAC, clientMAC, nil)

	for _, length := range []int{0, 1, pppoe.CookieLen - 1, pppoe.CookieLen + 1} {
		bad := make([]byte, length)
		copy(bad, cookie)
		if pppoe.VerifyCookie(key, bad, serverMAC, clientMAC, nil, timeout) {
			t.Fatalf("VerifyCookie should reject cookie of length %d", length)
		}
	}
}

func TestACCookieAtExactTimeout(t *testing.T) {
	key := mustKey(t)

	restore := fixedClock(1000)
	cookie := pppoe.GenerateCookie(key, serverMAC, clientMAC, nil)
	restore()

	restore = fixedClock(1000 + int64(timeout.Seconds()))
	defer restore()

	if !pppoe.VerifyCookie(key, cookie, serverMAC, clientMAC, nil, timeout) {
		t.Fatal("VerifyCookie should accept cookie at exact timeout boundary")
	}
}

func TestACCookieFutureClock(t *testing.T) {
	key := mustKey(t)

	restore := fixedClock(2000)
	cookie := pppoe.GenerateCookie(key, serverMAC, clientMAC, nil)
	restore()

	// Clock went backwards (monotonic in practice, but test the guard)
	restore = fixedClock(1999)
	defer restore()

	if pppoe.VerifyCookie(key, cookie, serverMAC, clientMAC, nil, timeout) {
		t.Fatal("VerifyCookie should reject cookie when elapsed < 0")
	}
}
