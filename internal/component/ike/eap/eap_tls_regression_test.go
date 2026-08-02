// Design: plan/learned/805-ipsec-11-interop-eap.md -- EAP-TLS transport + peer fixes
// RFC: rfc/short/rfc5216.md -- EAP-TLS server certificate validation (Section 5.3)
//
// Focused regression tests for four EAP-TLS fixes that the end-to-end handshake
// harness (eap_tls_handshake_test.go) only exercises indirectly (a dropped
// wakeup shows up there as a deadlock, not a clean assertion):
//   1. notifyCh must never drop a wakeup when the buffered channel has space
//      (the old `case <-time.After(0)` fallback fired ~immediately, so the
//      select picked at random and dropped roughly half of all wakeups, parking
//      a blocked Read and deadlocking the handshake).
//   2. feedPeerData must wake a Read that is blocked on an empty buffer.
//   3. verifyServerChain validates the authenticator chain against the trust
//      anchor with no hostname check (EAP-TLS has no server name).
//   4. deriveTLSMSK fail-closes (all-zero MSK, no panic) on an incomplete handshake.
//
// This file is deliberately SELF-CONTAINED: it builds its own tiny PKI rather
// than reusing the handshake harness's helpers, so the fixes and their tests
// land as one coherent, independently-compilable unit.
//
// VALIDATES: the wakeup path delivers every signal when the buffer is empty and
// never blocks when it is full; a blocked Read is woken by feedPeerData; the
// peer's server-chain verification accepts a trusted chain and rejects an
// untrusted one and an empty presentation; deriveTLSMSK never panics on an
// incomplete handshake and yields an unusable all-zero MSK.
// PREVENTS: regressing notifyCh back to a lossy select (handshake deadlock),
// weakening the peer's RFC 5216 Section 5.3 server-certificate validation, or
// re-introducing the ExportKeyingMaterial panic on a failed handshake.

package eap

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"
)

// TestNotifyChDeliversWakeupWhenBufferEmpty asserts notifyCh always queues a
// token on an empty buffered channel. The pre-fix `case <-time.After(0)` select
// dropped the send at random even when the buffer had room; over 1000 iterations
// that lossy path is a statistical certainty to fail, while the `default`-based
// fix delivers every time.
func TestNotifyChDeliversWakeupWhenBufferEmpty(t *testing.T) {
	ch := make(chan struct{}, 1)
	for i := range 1000 {
		// Drain to guarantee the channel starts empty (buffer has space).
		select {
		case <-ch:
		default:
		}
		notifyCh(ch)
		select {
		case <-ch:
			// Wakeup delivered, as required.
		default:
			t.Fatalf("iteration %d: notifyCh dropped the wakeup on an empty buffered channel", i)
		}
	}
}

// TestNotifyChNonBlockingWhenBufferFull asserts notifyCh returns immediately
// (coalescing) when a token is already queued, and never blocks -- the `default`
// branch. A blocking notifyCh would stall the TLS goroutine that feeds data.
func TestNotifyChNonBlockingWhenBufferFull(t *testing.T) {
	ch := make(chan struct{}, 1)
	ch <- struct{}{} // buffer already full: a wakeup is pending

	done := make(chan struct{})
	go func() {
		notifyCh(ch)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("notifyCh blocked on a full channel")
	}
	if len(ch) != 1 {
		t.Fatalf("channel len = %d, want 1 (the pending wakeup is coalesced, not doubled)", len(ch))
	}
}

// TestTransportFeedWakesBlockedRead drives the real transport: a Read blocked on
// an empty peer buffer must be woken by feedPeerData and return the fed bytes.
// This is the unit-level form of the handshake deadlock the notifyCh fix cures.
func TestTransportFeedWakesBlockedRead(t *testing.T) {
	tr := newEAPTLSTransport()
	got := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 32)
		n, err := tr.Read(buf) // blocks until feedPeerData signals
		if err != nil {
			got <- nil
			return
		}
		got <- buf[:n]
	}()

	// Let the reader reach its blocking <-peerCh before any data is fed.
	time.Sleep(20 * time.Millisecond)
	// feedPeerData now refuses data past the unread-backlog ceiling, so its error
	// is checked here. Five bytes on an empty transport is nowhere near it.
	if err := tr.feedPeerData([]byte("hello")); err != nil {
		t.Fatalf("feedPeerData refused 5 bytes on an empty transport: %v", err)
	}

	select {
	case b := <-got:
		if string(b) != "hello" {
			t.Fatalf("Read returned %q, want %q", b, "hello")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("feedPeerData did not wake the blocked Read (dropped wakeup)")
	}
}

// regrCA generates a self-signed CA certificate and its key.
func regrCA(t *testing.T, cn string, serial int64) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ca key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(serial),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("ca cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse ca: %v", err)
	}
	return cert, key
}

// regrServerLeafDER returns the DER of a server-auth leaf signed by caCert/caKey.
func regrServerLeafDER(t *testing.T, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, cn string, serial int64) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("leaf key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("leaf cert: %v", err)
	}
	return der
}

// TestVerifyServerChain covers the peer's RFC 5216 Section 5.3 server-chain
// validation callback directly: a leaf signed by the configured trust anchor is
// accepted, one signed by an untrusted CA is rejected, and an empty presentation
// is rejected. The end-to-end counterpart is TestEAPTLSPeerRejectsUntrustedServerChain.
func TestVerifyServerChain(t *testing.T) {
	trustedCA, trustedKey := regrCA(t, "regr-trusted-ca", 1)
	untrustedCA, untrustedKey := regrCA(t, "regr-untrusted-ca", 2)

	roots := x509.NewCertPool()
	roots.AddCert(trustedCA)
	verify := verifyServerChain(roots)

	trustedLeaf := regrServerLeafDER(t, trustedCA, trustedKey, "regr-server", 3)
	untrustedLeaf := regrServerLeafDER(t, untrustedCA, untrustedKey, "regr-rogue-server", 4)

	if err := verify([][]byte{trustedLeaf}, nil); err != nil {
		t.Fatalf("trusted server chain rejected: %v", err)
	}
	if err := verify([][]byte{untrustedLeaf}, nil); err == nil {
		t.Fatal("untrusted server chain accepted: peer server-cert validation is not enforced")
	}
	if err := verify(nil, nil); err == nil {
		t.Fatal("empty certificate presentation accepted: a server that sends no cert must be rejected")
	}
}

// TestDeriveTLSMSKFailsClosedOnIncompleteHandshake pins the fail-closed guard: on
// a tlsConn whose handshake did not complete (e.g. the authenticator's cert was
// rejected, which still sets tlsDone), deriveTLSMSK must report an error and must
// NOT panic. crypto/tls' ExportKeyingMaterial panics on an incomplete handshake,
// so without the cs.HandshakeComplete guard this test panics.
//
// The error is the load-bearing half. A zero MSK returned with no error is a
// valid-looking answer the caller cannot tell from a real key, so the caller
// authenticates over 64 zero octets instead of refusing.
func TestDeriveTLSMSKFailsClosedOnIncompleteHandshake(t *testing.T) {
	// A freshly wrapped tls.Client has HandshakeComplete == false (no handshake
	// was ever driven), the same observable state as a failed handshake. The
	// config is never used because no handshake is performed.
	ps := &PeerSession{
		tlsConn: tls.Client(newEAPTLSTransport(), &tls.Config{MinVersion: tls.VersionTLS12}),
	}

	msk, err := ps.deriveTLSMSK() // must not panic (the fail-closed guard)
	if err == nil {
		t.Fatal("incomplete handshake must report an error, not a usable MSK")
	}
	if msk != ([64]byte{}) {
		t.Fatalf("incomplete handshake must yield an all-zero MSK, got %x", msk)
	}
}
