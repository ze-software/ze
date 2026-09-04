// Design: docs/architecture/pki/tls-listeners.md -- looking-glass TLS chain and rotation

package lg

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"
)

// testChain builds an intermediate CA and a leaf it issued, and returns the
// leaf-then-intermediate PEM concatenation that pki.ServerTLSMaterial produces,
// plus the leaf key PEM. The common name identifies the leaf, so a rotation
// test can tell two certificates apart.
func testChain(t *testing.T, commonName string) (chainPEM, keyPEM []byte) {
	t.Helper()

	interKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate intermediate key: %v", err)
	}
	interTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: commonName + " intermediate"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	interDER, err := x509.CreateCertificate(rand.Reader, interTmpl, interTmpl, &interKey.PublicKey, interKey)
	if err != nil {
		t.Fatalf("create intermediate certificate: %v", err)
	}
	interCert, err := x509.ParseCertificate(interDER)
	if err != nil {
		t.Fatalf("parse intermediate certificate: %v", err)
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, interCert, &leafKey.PublicKey, interKey)
	if err != nil {
		t.Fatalf("create leaf certificate: %v", err)
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(leafKey)
	if err != nil {
		t.Fatalf("marshal leaf key: %v", err)
	}

	chainPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	chainPEM = append(chainPEM, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: interDER})...)
	return chainPEM, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

// startTLSTestServer starts a looking-glass server on one loopback port with
// the given TLS material and returns it once every listener is bound.
func startTLSTestServer(t *testing.T, certPEM, keyPEM []byte) *LGServer {
	t.Helper()

	srv, err := NewLGServer(LGConfig{
		ListenAddrs: []string{"127.0.0.1:0"},
		TLS:         true,
		CertPEM:     certPEM,
		KeyPEM:      keyPEM,
		Dispatch:    mockDispatch(),
	})
	if err != nil {
		t.Fatalf("NewLGServer: %v", err)
	}

	go func() {
		_ = srv.ListenAndServe(context.Background())
	}()

	readyCtx, readyCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer readyCancel()
	if err := srv.WaitReady(readyCtx); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}

	t.Cleanup(func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutCancel()
		if shutErr := srv.Shutdown(shutCtx); shutErr != nil {
			t.Logf("shutdown: %v", shutErr)
		}
	})
	return srv
}

// dialTLS opens a TLS connection to addr and returns it after the handshake.
// The caller closes it.
func dialTLS(t *testing.T, addr string) *tls.Conn {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dialer := &tls.Dialer{Config: &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // the test asserts on the chain the server sends, not on trust
		MinVersion:         tls.VersionTLS12,
	}}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		t.Fatalf("expected a TLS connection, got %T", conn)
	}
	return tlsConn
}

// handshakePeerCerts opens a TLS connection to addr and returns the certificate
// chain the server sent.
func handshakePeerCerts(t *testing.T, addr string) []*x509.Certificate {
	t.Helper()

	conn := dialTLS(t, addr)
	defer func() { _ = conn.Close() }()
	return conn.ConnectionState().PeerCertificates
}

// requireStatusOK sends one HTTP/1.1 request over an already-open connection
// and fails unless the looking glass answers 200. The body is drained so the
// connection stays usable for the next request.
func requireStatusOK(t *testing.T, conn net.Conn, reader *bufio.Reader, when string) {
	t.Helper()

	if _, err := io.WriteString(conn, "GET /api/looking-glass/status HTTP/1.1\r\nHost: lg.test\r\n\r\n"); err != nil {
		t.Fatalf("write request %s: %v", when, err)
	}
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatalf("read response %s: %v", when, err)
	}
	defer resp.Body.Close() //nolint:errcheck // test cleanup
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body %s: %v", when, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %s: got %d, want 200", when, resp.StatusCode)
	}
	if len(body) == 0 {
		t.Fatalf("body %s: got no data", when)
	}
}

func TestLGServerServesPKIChain(t *testing.T) {
	// VALIDATES: AC-1 -- the looking glass given leaf+intermediate PEM sends
	// BOTH certificates in the handshake, leaf first, which is what lets a
	// stranger's browser build the path to its trust anchor.
	// PREVENTS: a listener that accepts chain material and then serves the leaf
	// alone, which looks correct to every test that only checks the connection
	// succeeds.
	chainPEM, keyPEM := testChain(t, "lg chain leaf")
	srv := startTLSTestServer(t, chainPEM, keyPEM)

	peers := handshakePeerCerts(t, srv.Address())
	if len(peers) != 2 {
		t.Fatalf("served chain: got %d certificates, want leaf and intermediate", len(peers))
	}
	if peers[0].Subject.CommonName != "lg chain leaf" {
		t.Errorf("first certificate: got %q, want the leaf", peers[0].Subject.CommonName)
	}
	if peers[1].Subject.CommonName != "lg chain leaf intermediate" {
		t.Errorf("second certificate: got %q, want the intermediate", peers[1].Subject.CommonName)
	}
}

func TestLGServerUpdateTLSCertificate(t *testing.T) {
	// VALIDATES: AC-6 -- rotating the certificate serves new material on the
	// next handshake WITHOUT rebinding a listener, so a viewer holding an open
	// connection keeps it across a reload that renews the certificate.
	// PREVENTS: a rotation that swaps a field the bound listener no longer
	// reads, and a rotation that rebinds and drops every open connection.
	firstPEM, firstKey := testChain(t, "before rotation")
	srv := startTLSTestServer(t, firstPEM, firstKey)

	addrBefore := srv.Addresses()

	open := dialTLS(t, srv.Address())
	defer func() { _ = open.Close() }()
	openReader := bufio.NewReader(open)
	if name := open.ConnectionState().PeerCertificates[0].Subject.CommonName; name != "before rotation" {
		t.Fatalf("open connection leaf: got %q, want the pre-rotation leaf", name)
	}
	requireStatusOK(t, open, openReader, "before rotation")

	secondPEM, secondKey := testChain(t, "after rotation")
	if err := srv.UpdateTLSCertificate(secondPEM, secondKey); err != nil {
		t.Fatalf("UpdateTLSCertificate: %v", err)
	}

	requireStatusOK(t, open, openReader, "after rotation")

	addrAfter := srv.Addresses()
	if len(addrAfter) != len(addrBefore) {
		t.Fatalf("listener count: got %d, want %d", len(addrAfter), len(addrBefore))
	}
	for i, addr := range addrBefore {
		if addrAfter[i] != addr {
			t.Errorf("listener %d: got %q, want %q -- rotation must not rebind", i, addrAfter[i], addr)
		}
	}

	peers := handshakePeerCerts(t, srv.Address())
	if len(peers) != 2 {
		t.Fatalf("rotated chain: got %d certificates, want leaf and intermediate", len(peers))
	}
	if peers[0].Subject.CommonName != "after rotation" {
		t.Errorf("rotated leaf: got %q, want the post-rotation leaf", peers[0].Subject.CommonName)
	}
}

func TestLGUpdateTLSCertificateRejectsBadMaterial(t *testing.T) {
	// VALIDATES: fail-closed rotation -- material that does not parse is
	// refused and the PREVIOUS certificate keeps serving.
	// PREVENTS: a rotation that clears the served certificate on bad input,
	// which takes down a listener that was working a moment earlier.
	firstPEM, firstKey := testChain(t, "keep me")
	srv := startTLSTestServer(t, firstPEM, firstKey)

	if err := srv.UpdateTLSCertificate([]byte("not pem"), firstKey); err == nil {
		t.Error("unparseable certificate material: got no error, want a refusal")
	}
	if err := srv.UpdateTLSCertificate(nil, nil); err == nil {
		t.Error("empty material: got no error, want a refusal")
	}

	peers := handshakePeerCerts(t, srv.Address())
	if len(peers) == 0 {
		t.Fatal("served chain: got nothing, want the previous certificate")
	}
	if peers[0].Subject.CommonName != "keep me" {
		t.Errorf("served leaf: got %q, want the previous leaf", peers[0].Subject.CommonName)
	}
}

func TestLGUpdateTLSCertificateWithoutTLS(t *testing.T) {
	// VALIDATES: a plaintext looking glass refuses a rotation instead of
	// accepting material it will never serve.
	// PREVENTS: a silent success that reads, to the hub's reload path, exactly
	// like a certificate that reached the wire.
	srv, err := NewLGServer(LGConfig{
		ListenAddrs: []string{"127.0.0.1:0"},
		Dispatch:    mockDispatch(),
	})
	if err != nil {
		t.Fatalf("NewLGServer: %v", err)
	}

	chainPEM, keyPEM := testChain(t, "never served")
	if err := srv.UpdateTLSCertificate(chainPEM, keyPEM); err == nil {
		t.Error("rotation on a plaintext server: got no error, want a refusal")
	}
}
