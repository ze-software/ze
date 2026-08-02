// RFC: rfc/short/rfc5216.md -- EAP-TLS runs a TLS engine underneath the EAP exchange
//
// VALIDATES: an EAP-TLS exchange that ends before the TLS handshake completes
// releases the goroutine running the TLS engine, on the authenticator side
// (eap.Session) and on the peer side (eap.PeerSession), and Close is idempotent,
// safe before the method starts, and safe on a method that runs no goroutine.
// PREVENTS: the unbounded goroutine leak that exists when nothing calls Close.
// eapTLSTransport.Read parks on <-t.peerCh and only a close releases it, so every
// abandoned, failed, or refused EAP-TLS exchange stranded one goroutine plus the
// tls.Conn it held. The peer chooses how many exchanges it abandons, and it is
// unauthenticated while it does so.

package eap

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

// eapTLSEngineFrame is the frame every parked EAP-TLS engine goroutine carries.
// The TLS engine reaches it through Conn.HandshakeContext on both sides, so one
// pattern counts the authenticator's runTLSServer and the peer's handshake
// goroutine alike.
const eapTLSEngineFrame = "ike/eap.(*eapTLSTransport).Read"

// countEAPTLSEngines reports how many goroutines are inside the EAP-TLS
// transport read.
//
// It counts stack frames rather than runtime.NumGoroutine, because the test
// binary runs other goroutines whose count moves for reasons that have nothing
// to do with this transport. A frame match names exactly the goroutine under
// test, so the assertion is exact instead of a delta above noise.
func countEAPTLSEngines() int {
	buf := make([]byte, 1<<16)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return strings.Count(string(buf[:n]), eapTLSEngineFrame)
		}
		// The dump filled the buffer, so it may be truncated and the count short.
		buf = make([]byte, 2*len(buf))
	}
}

// waitEAPTLSEngines waits for the engine count to reach want and returns what it
// last observed. Starting and releasing a goroutine are both asynchronous, so
// the count is polled until it settles rather than read once.
func waitEAPTLSEngines(t *testing.T, want int) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		got := countEAPTLSEngines()
		if got == want || time.Now().After(deadline) {
			return got
		}
		// Poll interval. The loop returns as soon as the count reaches want.
		time.Sleep(2 * time.Millisecond)
	}
}

// The zero baseline each test below asserts is deliberately a package-wide
// invariant: no test in this package may leave an EAP-TLS exchange open. It was
// red when written, because the suite's own harness abandoned five of them, and
// the fix was to close those at their sites rather than to measure a delta here.
// A delta would have passed over exactly the defect this file exists to catch.

// startAbandonedAuthenticator drives an authenticator session as far as the
// EAP-TLS Start request and then stops answering, which is what an EAP-TLS
// exchange that the peer abandons looks like from this side.
func startAbandonedAuthenticator(t *testing.T, pki *eapTLSPKI) *Session {
	t.Helper()

	sess, err := NewSession(TypeTLS, pki.serverConfig())
	if err != nil {
		t.Fatalf("create authenticator session: %v", err)
	}

	identityReq := sess.Begin()
	next := sess.Process(&Packet{
		Code:       CodeResponse,
		Identifier: identityReq.Identifier,
		Type:       TypeIdentity,
		TypeData:   []byte("leak-test-user"),
	})
	if next == nil || next.Type != TypeTLS {
		t.Fatalf("authenticator did not reach the EAP-TLS Start request, got %+v", next)
	}
	return sess
}

// startAbandonedPeer drives a peer session as far as its ClientHello and then
// stops receiving, which is what an authenticator that goes away looks like.
func startAbandonedPeer(t *testing.T, pki *eapTLSPKI) *PeerSession {
	t.Helper()

	peer := NewPeerSessionTLS("leak-test-user", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
	})

	identity := peer.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity})
	if identity.Err != nil {
		t.Fatalf("peer identity round: %v", identity.Err)
	}

	start := peer.Process(&Packet{
		Code:       CodeRequest,
		Identifier: 2,
		Type:       TypeTLS,
		TypeData:   []byte{eapTLSFlagS},
	})
	if start.Err != nil {
		t.Fatalf("peer EAP-TLS Start round: %v", start.Err)
	}
	return peer
}

// TestEAPTLSAuthenticatorCloseReleasesEngineGoroutine proves the authenticator's
// TLS engine goroutine outlives an abandoned exchange until Close runs, and does
// not outlive Close.
func TestEAPTLSAuthenticatorCloseReleasesEngineGoroutine(t *testing.T) {
	if got := waitEAPTLSEngines(t, 0); got != 0 {
		t.Fatalf("baseline: %d EAP-TLS engine goroutines already running, want 0", got)
	}
	pki := newEAPTLSPKI(t)

	const sessions = 4
	var open []*Session
	for range sessions {
		open = append(open, startAbandonedAuthenticator(t, pki))
	}

	// Assert the goroutine EXISTS before asserting Close removes it. Without this
	// the test would pass just as well against a build that never started one,
	// which would make the release assertion below vacuous.
	if got := waitEAPTLSEngines(t, sessions); got != sessions {
		t.Fatalf("after %d abandoned exchanges: %d engine goroutines, want %d", sessions, got, sessions)
	}

	for _, sess := range open {
		sess.Close()
	}

	if got := waitEAPTLSEngines(t, 0); got != 0 {
		t.Fatalf("after Close: %d engine goroutines still parked, want 0 (each holds a tls.Conn and its handshake secrets)", got)
	}
}

// TestEAPTLSPeerCloseReleasesEngineGoroutine is the peer-side mirror: the
// initiator builds its own transport in startTLSClient, and it leaked the same
// way.
func TestEAPTLSPeerCloseReleasesEngineGoroutine(t *testing.T) {
	if got := waitEAPTLSEngines(t, 0); got != 0 {
		t.Fatalf("baseline: %d EAP-TLS engine goroutines already running, want 0", got)
	}
	pki := newEAPTLSPKI(t)

	const sessions = 4
	var open []*PeerSession
	for range sessions {
		open = append(open, startAbandonedPeer(t, pki))
	}

	if got := waitEAPTLSEngines(t, sessions); got != sessions {
		t.Fatalf("after %d abandoned exchanges: %d engine goroutines, want %d", sessions, got, sessions)
	}

	for _, peer := range open {
		peer.Close()
	}

	if got := waitEAPTLSEngines(t, 0); got != 0 {
		t.Fatalf("after Close: %d engine goroutines still parked, want 0", got)
	}
}

// TestEAPTLSCloseIsSafeOnEveryShape covers the calls a close-on-every-path rule
// makes unavoidable: an exchange closed twice, one closed before its method ever
// started, and a method that runs no goroutine at all. Each must be a no-op
// rather than a panic, because the SA close path cannot know which case it holds.
func TestEAPTLSCloseIsSafeOnEveryShape(t *testing.T) {
	pki := newEAPTLSPKI(t)

	t.Run("close before start", func(t *testing.T) {
		sess, err := NewSession(TypeTLS, pki.serverConfig())
		if err != nil {
			t.Fatalf("create session: %v", err)
		}
		sess.Close()
		if got := countEAPTLSEngines(); got != 0 {
			t.Fatalf("closing an unstarted session left %d engine goroutines", got)
		}
	})

	t.Run("close twice", func(t *testing.T) {
		sess := startAbandonedAuthenticator(t, pki)
		sess.Close()
		sess.Close()
		if got := waitEAPTLSEngines(t, 0); got != 0 {
			t.Fatalf("after a double Close: %d engine goroutines, want 0", got)
		}
	})

	t.Run("peer close before start", func(t *testing.T) {
		peer := NewPeerSessionTLS("leak-test-user", &PeerTLSConfig{
			CertPEM:   pki.clientCertPEM,
			KeyPEM:    pki.clientKeyPEM,
			CACertPEM: pki.trustedCAPEM,
		})
		peer.Close()
		peer.Close()
	})

	t.Run("mschapv2 method holds no goroutine", func(t *testing.T) {
		sess, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
		if err != nil {
			t.Fatalf("create session: %v", err)
		}
		sess.Close()

		peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
		peer.Close()
	})
}

// TestEAPTLSCloseAfterCompletedHandshakeIsHarmless proves the close-on-every-path
// rule costs a successful exchange nothing. The engine goroutine has already
// returned by then, and the MSK is exported from the connection state rather than
// read through the transport, so it stays available after the close.
func TestEAPTLSCloseAfterCompletedHandshakeIsHarmless(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("leak-test-user", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
	})

	res := runEAPTLSHandshake(t, pki.serverConfig(), peer)
	if !res.peerDone || !res.serverEAPSuccess {
		t.Fatalf("handshake did not succeed: peerDone=%v serverSuccess=%v err=%v", res.peerDone, res.serverEAPSuccess, res.peerErr)
	}
	if res.serverMSK != res.peerMSK {
		t.Fatal("the two sides derived different MSKs")
	}

	peer.Close()
	res.server.Close()

	msk, err := peer.deriveTLSMSK()
	if err != nil {
		t.Fatalf("MSK export after Close: %v", err)
	}
	if msk != res.peerMSK {
		t.Fatal("the MSK changed after Close")
	}
	if got := waitEAPTLSEngines(t, 0); got != 0 {
		t.Fatalf("after a completed handshake and Close: %d engine goroutines, want 0", got)
	}
}
