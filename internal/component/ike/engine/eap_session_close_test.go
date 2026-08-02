// RFC: rfc/short/rfc7296.md -- Section 2.12: a closed connection forgets its keys
//
// VALIDATES: SA.forgetKeys closes whichever EAP session the SA carries, for both
// roles (*eap.Session on the responder, *eap.PeerSession on the initiator), so the
// EAP-TLS engine goroutine is released on every path that closes an SA.
// PREVENTS: the wiring half of the EAP-TLS goroutine leak. eap.Session.Close and
// eap.PeerSession.Close can both be correct and still leak every goroutine if no
// SA-close path calls them, which was the state before this test: the transport's
// Close had zero non-test callers.

package engine

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log/slog"
	"math/big"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/eap"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// eapTLSEngineFrame is the frame a parked EAP-TLS engine goroutine carries on
// either side. Counting frames names the goroutine under test exactly, where
// runtime.NumGoroutine would only give a delta above the test binary's noise.
const eapTLSEngineFrame = "ike/eap.(*eapTLSTransport).Read"

func countEAPTLSEngines() int {
	buf := make([]byte, 1<<16)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return strings.Count(string(buf[:n]), eapTLSEngineFrame)
		}
		buf = make([]byte, 2*len(buf))
	}
}

// waitEAPTLSEngines polls until the engine count reaches want, then returns what
// it last saw. Starting and releasing a goroutine are both asynchronous.
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

// escPKI is an in-memory CA plus a leaf certificate usable by both EAP roles.
type escPKI struct {
	caPEM   []byte
	certPEM []byte
	keyPEM  []byte
}

func newESCPKI(t *testing.T) *escPKI {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ca key: %v", err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "eap-close-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("ca cert: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse ca: %v", err)
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("leaf key: %v", err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "eap-close-leaf"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("leaf cert: %v", err)
	}
	leafKeyDER, err := x509.MarshalPKCS8PrivateKey(leafKey)
	if err != nil {
		t.Fatalf("marshal leaf key: %v", err)
	}

	return &escPKI{
		caPEM:   pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
		certPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}),
		keyPEM:  pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: leafKeyDER}),
	}
}

// TestForgetKeysClosesResponderEAPSession proves the authenticator's EAP session,
// the shape sa.EAPSession carries on the responder, is closed when the SA closes.
func TestForgetKeysClosesResponderEAPSession(t *testing.T) {
	if got := waitEAPTLSEngines(t, 0); got != 0 {
		t.Fatalf("baseline: %d EAP-TLS engine goroutines, want 0", got)
	}
	pki := newESCPKI(t)

	sess, err := eap.NewSession(eap.TypeTLS, eap.MethodConfig{
		ServerCertPEM: pki.certPEM,
		ServerKeyPEM:  pki.keyPEM,
		CACertPEM:     pki.caPEM,
	})
	if err != nil {
		t.Fatalf("create authenticator session: %v", err)
	}

	identityReq := sess.Begin()
	next := sess.Process(&eap.Packet{
		Code:       eap.CodeResponse,
		Identifier: identityReq.Identifier,
		Type:       eap.TypeIdentity,
		TypeData:   []byte("close-test-user"),
	})
	if next == nil || next.Type != eap.TypeTLS {
		t.Fatalf("authenticator did not reach the EAP-TLS Start request, got %+v", next)
	}

	// The goroutine must exist before the close is asserted, or the assertion
	// below would pass against a build that never started one.
	if got := waitEAPTLSEngines(t, 1); got != 1 {
		t.Fatalf("after EAP-TLS Start: %d engine goroutines, want 1", got)
	}

	sa := &SA{EAPSession: sess}
	sa.forgetKeys()

	if got := waitEAPTLSEngines(t, 0); got != 0 {
		t.Fatalf("forgetKeys left %d EAP-TLS engine goroutines parked, want 0", got)
	}
}

// TestForgetKeysClosesInitiatorEAPSession is the initiator mirror: sa.EAPSession
// holds *eap.PeerSession there, and closeEAPSession must recover that shape too.
func TestForgetKeysClosesInitiatorEAPSession(t *testing.T) {
	if got := waitEAPTLSEngines(t, 0); got != 0 {
		t.Fatalf("baseline: %d EAP-TLS engine goroutines, want 0", got)
	}
	pki := newESCPKI(t)

	peer := eap.NewPeerSessionTLS("close-test-user", &eap.PeerTLSConfig{
		CertPEM:   pki.certPEM,
		KeyPEM:    pki.keyPEM,
		CACertPEM: pki.caPEM,
	})

	if res := peer.Process(&eap.Packet{Code: eap.CodeRequest, Identifier: 1, Type: eap.TypeIdentity}); res.Err != nil {
		t.Fatalf("peer identity round: %v", res.Err)
	}
	// eapTLSFlagS (0x20) is the EAP-TLS Start flag, RFC 5216 Section 2.1.
	if res := peer.Process(&eap.Packet{
		Code:       eap.CodeRequest,
		Identifier: 2,
		Type:       eap.TypeTLS,
		TypeData:   []byte{0x20},
	}); res.Err != nil {
		t.Fatalf("peer EAP-TLS Start round: %v", res.Err)
	}

	if got := waitEAPTLSEngines(t, 1); got != 1 {
		t.Fatalf("after EAP-TLS Start: %d engine goroutines, want 1", got)
	}

	sa := &SA{EAPSession: peer}
	sa.forgetKeys()

	if got := waitEAPTLSEngines(t, 0); got != 0 {
		t.Fatalf("forgetKeys left %d EAP-TLS engine goroutines parked, want 0", got)
	}
}

// TestRunResponderStopReleasesAbandonedEAPSession drives the leak from its entry
// point rather than from forgetKeys.
//
// A responder parked mid-EAP when the operator reconfigures leaves runResponder
// through its stopCh case, which is the one exit the loop's own state switch
// never reaches: the SA is never marked StateDead and never reaped, so before the
// defer existed its keys and its EAP-TLS engine goroutine both survived the
// session. Closing forgetKeys alone would not have caught that, because nothing
// on this path called it.
func TestRunResponderStopReleasesAbandonedEAPSession(t *testing.T) {
	if got := waitEAPTLSEngines(t, 0); got != 0 {
		t.Fatalf("baseline: %d EAP-TLS engine goroutines, want 0", got)
	}
	pki := newESCPKI(t)

	sess, err := eap.NewSession(eap.TypeTLS, eap.MethodConfig{
		ServerCertPEM: pki.certPEM,
		ServerKeyPEM:  pki.keyPEM,
		CACertPEM:     pki.caPEM,
	})
	if err != nil {
		t.Fatalf("create authenticator session: %v", err)
	}
	identityReq := sess.Begin()
	if next := sess.Process(&eap.Packet{
		Code:       eap.CodeResponse,
		Identifier: identityReq.Identifier,
		Type:       eap.TypeIdentity,
		TypeData:   []byte("close-test-user"),
	}); next == nil || next.Type != eap.TypeTLS {
		t.Fatalf("authenticator did not reach the EAP-TLS Start request, got %+v", next)
	}
	if got := waitEAPTLSEngines(t, 1); got != 1 {
		t.Fatalf("after EAP-TLS Start: %d engine goroutines, want 1", got)
	}

	// A half-open responder SA mid-EAP, exactly as the dispatch goroutine
	// publishes it before the peer has finished authenticating.
	ps := &PeerSession{peerName: "eap-close-peer", stopCh: make(chan struct{})}
	ps.setSA(&SA{PeerName: "eap-close-peer", State: StateEAPInProgress, EAPSession: sess})

	// The operator stops the session. runResponder must not outlive it, and must
	// not leave the abandoned exchange behind.
	close(ps.stopCh)
	if err := ps.runResponder(ipsec.SiteToSitePeer{}, ipsec.IKEGroup{}, NewSATable(), nil, nil, slog.New(slog.DiscardHandler)); err == nil {
		t.Fatal("runResponder returned nil on a stopped session, want errStopped")
	}

	if got := waitEAPTLSEngines(t, 0); got != 0 {
		t.Fatalf("the stopped responder left %d EAP-TLS engine goroutines parked, want 0", got)
	}
}

// TestRunInitiatorClosesItsSAOnExit proves the initiator's other half of the same
// hole.
//
// An initiator handshake that fails leaves runInitiator through errSADead,
// errTimeout or errStopped, and none of those exits passed through forgetKeys
// before the defer existed: the partial handshake's DH private value and nonces
// stayed on the SA, and an EAP-TLS exchange it had reached stayed open with it.
//
// The assertion is on the erase rather than on an EAP session, because the SA is
// built inside runInitiator and no EAP session can be attached from outside.
// closeEAPSession runs on the same call, and the two tests above prove that half.
func TestRunInitiatorClosesItsSAOnExit(t *testing.T) {
	ps := &PeerSession{peerName: "test-peer", stopCh: make(chan struct{})}
	// Stopped before it starts, so the loop takes its first exit and the deferred
	// close is the only thing that can erase the SA.
	close(ps.stopCh)

	err := ps.runInitiator(testPeer(), testIKEGroup(), NewSATable(), nil, nil, slog.New(slog.DiscardHandler))
	if err == nil {
		t.Fatal("runInitiator returned nil on a stopped session, want errStopped")
	}

	sa := ps.getSA()
	if sa == nil {
		t.Fatal("runInitiator published no SA, so this test proves nothing")
	}
	// The SA reached IKE_SA_INIT, so it held a DH private value and a nonce.
	if sa.LocalNonce != nil {
		t.Error("runInitiator returned with the initiator nonce still on the SA: the deferred close did not run")
	}
	// DHExchange.Clear zeroes the public key alongside the private value, so an
	// all-zero public key is the observable trace that the erase ran.
	if sa.LocalDH != nil {
		for _, b := range sa.LocalDH.PublicKey {
			if b != 0 {
				t.Error("runInitiator returned with the DH key material still on the SA: the deferred close did not run")
				break
			}
		}
	}
}

// TestForgetKeysHandlesEveryEAPSessionShape covers the SAs that reach the close
// path carrying no EAP session at all, which is every PSK and certificate peer.
// closeEAPSession must ignore them rather than panic: forgetKeys runs on the
// close path of every SA, not only the EAP ones.
func TestForgetKeysHandlesEveryEAPSessionShape(t *testing.T) {
	(&SA{}).forgetKeys()
	(&SA{EAPSession: nil}).forgetKeys()

	// An MS-CHAPv2 session holds no goroutine, and the close path cannot know that.
	sess, err := eap.NewSession(eap.TypeMSCHAPv2, eap.MethodConfig{Password: "secret"})
	if err != nil {
		t.Fatalf("create MS-CHAPv2 session: %v", err)
	}
	(&SA{EAPSession: sess}).forgetKeys()
	(&SA{EAPSession: eap.NewPeerSession(eap.TypeMSCHAPv2, "user", "secret")}).forgetKeys()

	if got := countEAPTLSEngines(); got != 0 {
		t.Fatalf("%d EAP-TLS engine goroutines after closing non-TLS sessions, want 0", got)
	}
}
