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
//
// VALIDATES: the wakeup path delivers every signal when the buffer is empty and
// never blocks when it is full; a blocked Read is woken by feedPeerData; the
// peer's server-chain verification accepts a trusted chain and rejects an
// untrusted one and an empty presentation.
// PREVENTS: regressing notifyCh back to a lossy select (handshake deadlock), or
// weakening the peer's RFC 5216 Section 5.3 server-certificate validation.

package eap

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
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
	tr.feedPeerData([]byte("hello"))

	select {
	case b := <-got:
		if string(b) != "hello" {
			t.Fatalf("Read returned %q, want %q", b, "hello")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("feedPeerData did not wake the blocked Read (dropped wakeup)")
	}
}

// certDER decodes a single PEM CERTIFICATE block to its DER bytes.
func certDER(t *testing.T, certPEM []byte) []byte {
	t.Helper()
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatalf("certDER: input is not a PEM CERTIFICATE block")
	}
	return block.Bytes
}

// TestVerifyServerChain covers the peer's RFC 5216 Section 5.3 server-chain
// validation callback directly: a leaf signed by the configured trust anchor is
// accepted, one signed by an untrusted CA is rejected, and an empty presentation
// is rejected. The end-to-end counterpart is TestEAPTLSPeerRejectsUntrustedServerChain.
func TestVerifyServerChain(t *testing.T) {
	pki := newEAPTLSPKI(t)
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pki.trustedCAPEM) {
		t.Fatal("failed to load trusted CA into the roots pool")
	}
	verify := verifyServerChain(roots)

	if err := verify([][]byte{certDER(t, pki.serverCertPEM)}, nil); err != nil {
		t.Fatalf("trusted server chain rejected: %v", err)
	}
	if err := verify([][]byte{certDER(t, pki.untrustedServerCertPEM)}, nil); err == nil {
		t.Fatal("untrusted server chain accepted: peer server-cert validation is not enforced")
	}
	if err := verify(nil, nil); err == nil {
		t.Fatal("empty certificate presentation accepted: a server that sends no cert must be rejected")
	}
}

// TestDeriveTLSMSKFailsClosedOnIncompleteHandshake pins the fail-closed guard: on
// a tlsConn whose handshake did not complete (e.g. the authenticator's cert was
// rejected, which still sets tlsDone), deriveTLSMSK must return an all-zero MSK
// and must NOT panic. crypto/tls' ExportKeyingMaterial panics on an incomplete
// handshake, so without the cs.HandshakeComplete guard this test panics; the
// all-zero result is an invalid key that cannot yield a passing EAP-Success.
func TestDeriveTLSMSKFailsClosedOnIncompleteHandshake(t *testing.T) {
	// A freshly wrapped tls.Client has HandshakeComplete == false (no handshake
	// was ever driven), which is the same observable state as a failed handshake.
	// The config is never used: no handshake is driven, so tls.Client just wraps
	// the transport and ConnectionState() reports HandshakeComplete == false.
	ps := &PeerSession{
		tlsConn: tls.Client(newEAPTLSTransport(), &tls.Config{MinVersion: tls.VersionTLS12}),
	}

	msk := ps.deriveTLSMSK() // must not panic (the fail-closed guard)
	if msk != ([64]byte{}) {
		t.Fatalf("incomplete handshake must yield an all-zero MSK, got %x", msk)
	}
}
