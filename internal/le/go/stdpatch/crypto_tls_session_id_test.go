package stdpatch

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"
)

// recordingConn keeps every byte one side of a connection writes.
// Safe for concurrent use.
type recordingConn struct {
	net.Conn
	mu      sync.Mutex
	written bytes.Buffer
}

func (c *recordingConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	c.written.Write(p)
	c.mu.Unlock()
	return c.Conn.Write(p)
}

func (c *recordingConn) bytes() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return bytes.Clone(c.written.Bytes())
}

// helloSessionID reads the session id of the first handshake message in a
// flight: a ClientHello or a ServerHello (record header 5 octets, handshake
// header 4, version 2, random 32, then the length-prefixed id).
func helloSessionID(t *testing.T, flight []byte) []byte {
	t.Helper()
	const offset = 5 + 4 + 2 + 32
	if len(flight) <= offset || flight[0] != 22 {
		t.Fatalf("the flight does not open with a handshake record: % x", flight[:min(len(flight), 16)])
	}
	length := int(flight[offset])
	if len(flight) < offset+1+length {
		t.Fatalf("the hello is shorter than its session id length %d", length)
	}
	return flight[offset+1 : offset+1+length]
}

// selfSigned answers a server certificate for the handshakes below, and the
// pool a client trusts it through.
func selfSigned(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "stdpatch"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"stdpatch"},
		IsCA:         true,
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},

		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(parsed)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, pool
}

// handshake runs one handshake over an in-memory pipe and answers what the
// client and the server wrote.
func handshake(t *testing.T, client, server *tls.Config) (clientFlight, serverFlight []byte) {
	t.Helper()
	clientEnd, serverEnd := net.Pipe()
	recordedClient := &recordingConn{Conn: clientEnd}
	recordedServer := &recordingConn{Conn: serverEnd}
	deadline := time.Now().Add(10 * time.Second)
	if err := clientEnd.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	if err := serverEnd.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}

	serverConn := tls.Server(recordedServer, server)
	var wg sync.WaitGroup
	var serverErr error
	wg.Go(func() {
		serverErr = serverConn.HandshakeContext(context.Background())
		if serverErr == nil {
			// Drain until the client closes, so a post-handshake message
			// the server writes is never blocked by the pipe.
			_, _ = serverConn.Read(make([]byte, 1)) //nolint:errcheck // the client's close ends the read
		}
	})
	clientConn := tls.Client(recordedClient, client)
	clientErr := clientConn.HandshakeContext(context.Background())
	_ = clientConn.Close() //nolint:errcheck // the handshake result is what the test judges
	wg.Wait()
	_ = serverConn.Close() //nolint:errcheck // the handshake result is what the test judges
	if clientErr != nil {
		t.Fatalf("client handshake: %v", clientErr)
	}
	if serverErr != nil {
		t.Fatalf("server handshake: %v", serverErr)
	}
	return recordedClient.bytes(), recordedServer.bytes()
}

// VALIDATES: AC-19, the patched crypto/tls hunk on the wire.
// PREVENTS: a tree that carries the marker but not the hunk, or a hunk that
// also changed the ticket-issuing or TLS 1.3 handshakes.
//
// TestPatchedCryptoTLSServerSessionID proves the patched crypto/tls (AC-19)
// on a plain TLS server outside EAP. It reads the ServerHello off the wire:
// a TLS 1.2 server that issues a ticket keeps the empty session_id (RFC 5077
// Section 3.4); one that issues none sends 32 octets that are not the
// client's (RFC 5216 Section 2.1.1); TLS 1.3 echoes the client's
// legacy_session_id unchanged. Run under the unpatched GOROOT, or a tree
// that carries the marker without the hunk, the no-ticket case goes red.
func TestPatchedCryptoTLSServerSessionID(t *testing.T) {
	certificate, roots := selfSigned(t)
	server := func(maxVersion uint16, ticketsOff bool) *tls.Config {
		return &tls.Config{Certificates: []tls.Certificate{certificate}, MaxVersion: maxVersion, SessionTicketsDisabled: ticketsOff}
	}
	client := func(maxVersion uint16, ticketsOff bool) *tls.Config {
		config := &tls.Config{RootCAs: roots, ServerName: "stdpatch", MaxVersion: maxVersion, SessionTicketsDisabled: ticketsOff}
		// A crypto/tls client offers the session ticket extension only when
		// it has a cache to keep the ticket in.
		if !ticketsOff {
			config.ClientSessionCache = tls.NewLRUClientSessionCache(1)
		}
		return config
	}

	t.Run("tls12-ticket-issued", func(t *testing.T) {
		_, serverFlight := handshake(t, client(tls.VersionTLS12, false), server(tls.VersionTLS12, false))
		if id := helloSessionID(t, serverFlight); len(id) != 0 {
			t.Errorf("a server that issues a ticket sent a %d-octet session_id, want empty (RFC 5077 Section 3.4)", len(id))
		}
	})

	t.Run("tls12-no-ticket", func(t *testing.T) {
		for name, configs := range map[string][2]*tls.Config{
			"client offers none": {client(tls.VersionTLS12, true), server(tls.VersionTLS12, false)},
			"server issues none": {client(tls.VersionTLS12, false), server(tls.VersionTLS12, true)},
		} {
			clientFlight, serverFlight := handshake(t, configs[0], configs[1])
			id := helloSessionID(t, serverFlight)
			if len(id) != 32 {
				t.Errorf("%s: the ServerHello session_id is %d octets, want 32 (RFC 5216 Section 2.1.1)", name, len(id))
				continue
			}
			if bytes.Equal(id, helloSessionID(t, clientFlight)) {
				t.Errorf("%s: the server echoed the client's session_id instead of choosing one", name)
			}
		}
	})

	t.Run("tls13-unchanged", func(t *testing.T) {
		clientFlight, serverFlight := handshake(t, client(tls.VersionTLS13, false), server(tls.VersionTLS13, false))
		if sent, echoed := helloSessionID(t, clientFlight), helloSessionID(t, serverFlight); !bytes.Equal(sent, echoed) {
			t.Errorf("TLS 1.3 legacy_session_id_echo % x differs from the client's % x", echoed, sent)
		}
	})
}
