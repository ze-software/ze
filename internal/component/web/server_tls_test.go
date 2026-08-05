// Design: plan/spec-pki-full-chain.md -- web TLS chain + rotation tests

package web

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// testChain builds an intermediate CA and a leaf it issued, returning the
// leaf+intermediate PEM concatenation (what pki.ServerTLSMaterial produces) and
// the leaf key PEM. cn names the leaf so a rotation test can tell two
// certificates apart.
func testChain(t *testing.T, cn string) (chainPEM, keyPEM []byte) {
	t.Helper()

	interKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	interTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: cn + " intermediate"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	interDER, err := x509.CreateCertificate(rand.Reader, interTmpl, interTmpl, &interKey.PublicKey, interKey)
	require.NoError(t, err)
	interCert, err := x509.ParseCertificate(interDER)
	require.NoError(t, err)

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, interCert, &leafKey.PublicKey, interKey)
	require.NoError(t, err)

	keyDER, err := x509.MarshalPKCS8PrivateKey(leafKey)
	require.NoError(t, err)

	chainPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	chainPEM = append(chainPEM, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: interDER})...)
	return chainPEM, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

// startServerWithMaterial starts a WebServer on one loopback port with the given
// TLS material and returns it once it is bound.
func startServerWithMaterial(t *testing.T, certPEM, keyPEM []byte) *WebServer {
	t.Helper()

	srv, err := NewWebServer(WebConfig{
		ListenAddrs: []string{"127.0.0.1:0"},
		CertPEM:     certPEM,
		KeyPEM:      keyPEM,
	})
	require.NoError(t, err)

	go func() {
		if serveErr := srv.ListenAndServe(context.Background()); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			t.Logf("ListenAndServe: %v", serveErr)
		}
	}()

	readyCtx, readyCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer readyCancel()
	require.NoError(t, srv.WaitReady(readyCtx))

	t.Cleanup(func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutCancel()
		if shutErr := srv.Shutdown(shutCtx); shutErr != nil {
			t.Logf("shutdown: %v", shutErr)
		}
	})
	return srv
}

// handshakePeerCerts opens a TLS connection to addr and returns the certificate
// chain the server sent.
func handshakePeerCerts(t *testing.T, addr string) []*x509.Certificate {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dialer := &tls.Dialer{Config: &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // the test asserts on the chain the server sends, not on trust
		MinVersion:         tls.VersionTLS12,
	}}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	tlsConn, ok := conn.(*tls.Conn)
	require.True(t, ok, "expected a TLS connection")
	return tlsConn.ConnectionState().PeerCertificates
}

func TestWebServerServesPKIChain(t *testing.T) {
	// VALIDATES: AC-1 and AC-4 -- an HTTPS listener given leaf+intermediate PEM
	// sends BOTH certificates in the handshake, which is what `openssl s_client`
	// shows the operator and what lets a browser build the path to the anchor.
	// PREVENTS: a listener that accepts chain material and then serves the leaf
	// alone, which looks correct in every test that only checks the connection
	// succeeds.
	chainPEM, keyPEM := testChain(t, "web chain leaf")
	srv := startServerWithMaterial(t, chainPEM, keyPEM)

	peers := handshakePeerCerts(t, srv.Address())
	require.Len(t, peers, 2, "server must send the leaf and its intermediate")
	require.Equal(t, "web chain leaf", peers[0].Subject.CommonName, "the leaf must be sent first")
	require.Equal(t, "web chain leaf intermediate", peers[1].Subject.CommonName)
}

func TestWebServerUpdateTLSCertificate(t *testing.T) {
	// VALIDATES: AC-9 -- rotating the configured certificate changes what new
	// handshakes are served WITHOUT rebinding the listener, so long-lived SSE
	// sessions survive a reload that rotates the certificate.
	// PREVENTS: a rotation path that swaps a field the already-built tls.Config
	// no longer reads, leaving the old certificate on the wire until restart.
	firstPEM, firstKey := testChain(t, "before rotation")
	srv := startServerWithMaterial(t, firstPEM, firstKey)

	addrBefore := srv.Addresses()
	peers := handshakePeerCerts(t, srv.Address())
	require.Equal(t, "before rotation", peers[0].Subject.CommonName)

	secondPEM, secondKey := testChain(t, "after rotation")
	require.NoError(t, srv.UpdateTLSCertificate(secondPEM, secondKey))

	require.Equal(t, addrBefore, srv.Addresses(), "rotation must not rebind the listeners")

	peers = handshakePeerCerts(t, srv.Address())
	require.Equal(t, "after rotation", peers[0].Subject.CommonName, "new handshakes must serve the rotated certificate")
	require.Len(t, peers, 2, "the rotated certificate must keep serving its chain")
}

func TestUpdateTLSCertificateRejectsBadMaterial(t *testing.T) {
	// VALIDATES: fail-closed rotation -- unparseable material is refused and the
	// PREVIOUS certificate keeps serving. A rotation that cleared the served
	// certificate on bad input would take the listener down; one that installed
	// it would break every handshake.
	firstPEM, firstKey := testChain(t, "keep me")
	srv := startServerWithMaterial(t, firstPEM, firstKey)

	require.Error(t, srv.UpdateTLSCertificate([]byte("not pem"), firstKey))
	require.Error(t, srv.UpdateTLSCertificate(nil, nil))

	peers := handshakePeerCerts(t, srv.Address())
	require.Equal(t, "keep me", peers[0].Subject.CommonName, "a refused rotation must leave the previous certificate serving")
}
