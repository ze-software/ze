// VALIDATES: a managed client authenticates the hub before it sends its token --
// a pinned certificate fingerprint is honored, a wrong certificate ends the
// handshake, and the default (no pin, no tls-insecure) fails closed
// (spec-managed-server-hardening AC-1/AC-2).
// PREVENTS: the shipping posture this replaced, where the only way to reach a hub
// serving a self-signed certificate was ze.managed.tls.insecure, which sends the
// client's token to whatever answered on the hub address.

package managed

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/pem"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/selfcert"
)

const testHubToken = "0123456789abcdef0123456789abcdef"

// recordingHub is a TLS listener that reports what each connection produced.
// It records the first line an accepted connection sent after a successful
// handshake, or the handshake error. That answers the one question this file
// asks: did the client's token reach a server it had not authenticated?
type recordingHub struct {
	addr        string
	fingerprint string
	firstLine   chan string
	handshakeNG chan error
}

func startRecordingHub(t *testing.T) *recordingHub {
	t.Helper()

	certPEM, keyPEM, err := selfcert.GenerateWebCertWithNames("127.0.0.1:0", nil, time.Hour)
	if err != nil {
		t.Fatalf("generate hub certificate: %v", err)
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("hub key pair: %v", err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("hub certificate PEM did not decode")
	}
	sum := sha256.Sum256(block.Bytes)

	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{pair},
		MinVersion:   tls.VersionTLS13,
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() }) //nolint:errcheck // test cleanup

	hub := &recordingHub{
		addr:        ln.Addr().String(),
		fingerprint: hex.EncodeToString(sum[:]),
		firstLine:   make(chan string, 4),
		handshakeNG: make(chan error, 4),
	}

	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return // listener closed by cleanup
			}
			go hub.serve(conn)
		}
	}()
	return hub
}

// serve completes the handshake and reads one line. Both outcomes are recorded:
// a handshake failure means the client refused this hub, and a line means the
// client trusted it enough to write.
func (h *recordingHub) serve(conn net.Conn) {
	defer conn.Close() //nolint:errcheck // test cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	tc, ok := conn.(*tls.Conn)
	if !ok {
		return
	}
	if err := tc.HandshakeContext(ctx); err != nil {
		h.handshakeNG <- err
		return
	}
	buf := make([]byte, 512)
	n, err := tc.Read(buf)
	if err != nil {
		h.handshakeNG <- err
		return
	}
	h.firstLine <- string(buf[:n])
}

// wroteToken reports whether the client sent its auth frame within the timeout.
func (h *recordingHub) wroteToken(t *testing.T, timeout time.Duration) (string, bool) {
	t.Helper()
	select {
	case line := <-h.firstLine:
		return line, true
	case <-h.handshakeNG:
		return "", false
	case <-time.After(timeout):
		return "", false
	}
}

func runOnce(t *testing.T, cfg *ClientConfig) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return runConnection(ctx, cfg, newBackoff(time.Millisecond, time.Second, 0))
}

// TestManagedClientPinsCertificate: AC-1 -- a client carrying the hub's
// fingerprint completes the handshake against a certificate no CA issued. It
// sends its token over that connection.
//
// MUTATION: delete the pin branch in clientTLSConfig and this fails -- the
// system CA pool cannot verify the hub certificate, so the handshake ends
// before any token is written.
func TestManagedClientPinsCertificate(t *testing.T) {
	hub := startRecordingHub(t)
	cfg := &ClientConfig{
		Name:                   "edge-01",
		Server:                 hub.addr,
		Token:                  testHubToken,
		CertificateFingerprint: hub.fingerprint,
		Handler:                &Handler{Validate: func([]byte) error { return nil }},
	}

	_ = runOnce(t, cfg) // the hub closes after one line, so this always ends in an error

	line, ok := hub.wroteToken(t, 3*time.Second)
	if !ok {
		t.Fatal("client with the correct fingerprint did not reach the hub")
	}
	if !strings.Contains(line, testHubToken) {
		t.Fatalf("auth frame does not carry the token: %q", line)
	}
}

// TestManagedClientRefusesWrongCertificate: AC-2 -- a client carrying a
// fingerprint the hub does not present refuses the connection and sends
// nothing. This is the impostor case: the token must not leave the client.
//
// MUTATION: return an InsecureSkipVerify config from the pin branch (no
// VerifyConnection) and this fails -- the handshake completes and the token
// arrives.
func TestManagedClientRefusesWrongCertificate(t *testing.T) {
	hub := startRecordingHub(t)
	wrong := strings.Repeat("ab", 32) // 64 hex digits, not this hub's certificate
	cfg := &ClientConfig{
		Name:                   "edge-01",
		Server:                 hub.addr,
		Token:                  testHubToken,
		CertificateFingerprint: wrong,
		Handler:                &Handler{Validate: func([]byte) error { return nil }},
	}

	err := runOnce(t, cfg)
	if err == nil {
		t.Fatal("client accepted a hub whose certificate it did not pin")
	}
	if !strings.Contains(err.Error(), "tls handshake") {
		t.Fatalf("error = %v, want a TLS handshake failure", err)
	}
	if line, ok := hub.wroteToken(t, time.Second); ok {
		t.Fatalf("client sent %q to a hub it did not authenticate", line)
	}
}

// TestManagedClientDefaultFailsClosed: with no fingerprint and no tls-insecure,
// the trust anchor is the system CA pool, which cannot verify a self-signed hub.
// The connection must fail rather than proceed.
//
// MUTATION: set InsecureSkipVerify in the default branch of clientTLSConfig and
// this fails -- the handshake completes and the token arrives.
func TestManagedClientDefaultFailsClosed(t *testing.T) {
	hub := startRecordingHub(t)
	cfg := &ClientConfig{
		Name:    "edge-01",
		Server:  hub.addr,
		Token:   testHubToken,
		Handler: &Handler{Validate: func([]byte) error { return nil }},
	}

	if err := runOnce(t, cfg); err == nil {
		t.Fatal("client accepted an unverifiable hub certificate by default")
	}
	if line, ok := hub.wroteToken(t, time.Second); ok {
		t.Fatalf("client sent %q to an unverified hub", line)
	}
}

// TestManagedClientFingerprintSources: the environment variable overrides the
// configured leaf (ai/rules/config.md precedence), and hex case does not matter
// because crypto/hex writes lowercase and an operator can paste uppercase.
//
// MUTATION: drop the env lookup in certificateFingerprint and the override case
// fails. Drop strings.ToLower and the uppercase case fails.
func TestManagedClientFingerprintSources(t *testing.T) {
	hub := startRecordingHub(t)
	upper := strings.ToUpper(hub.fingerprint)

	t.Run("uppercase-leaf", func(t *testing.T) {
		cfg := &ClientConfig{
			Name:                   "edge-01",
			Server:                 hub.addr,
			Token:                  testHubToken,
			CertificateFingerprint: upper,
			Handler:                &Handler{Validate: func([]byte) error { return nil }},
		}
		_ = runOnce(t, cfg)
		if _, ok := hub.wroteToken(t, 3*time.Second); !ok {
			t.Fatal("uppercase fingerprint did not match the hub certificate")
		}
	})

	t.Run("env-overrides-leaf", func(t *testing.T) {
		if err := env.Set(envCertificateFingerprint, hub.fingerprint); err != nil {
			t.Fatalf("set %s: %v", envCertificateFingerprint, err)
		}
		t.Cleanup(func() { _ = env.Set(envCertificateFingerprint, "") })

		cfg := &ClientConfig{
			Name:                   "edge-01",
			Server:                 hub.addr,
			Token:                  testHubToken,
			CertificateFingerprint: strings.Repeat("cd", 32), // wrong leaf, right env
			Handler:                &Handler{Validate: func([]byte) error { return nil }},
		}
		_ = runOnce(t, cfg)
		if _, ok := hub.wroteToken(t, 3*time.Second); !ok {
			t.Fatal("env fingerprint did not override the configured leaf")
		}
	})
}
