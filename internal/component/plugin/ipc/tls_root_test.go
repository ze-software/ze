// VALIDATES: a client built by TLSConfigWithRoot validates the server's chain
// against the daemon's certificate authority root and nothing else, and refuses
// outright when it holds no usable root (spec-local-ca AC-2, AC-3, AC-4, R-2).
// PREVENTS: the pinning this replaced, whose empty fingerprint returned
// InsecureSkipVerify with no comparison, so a plugin with no anchor connected
// to whatever answered on the address.
//
// It is an EXTERNAL test package on purpose: internal/component/pki reaches
// this package (pki/show.go -> plugin/server -> plugin/ipc), so only a package
// outside ipc can drive the real certificate authority against it.
package ipc_test

import (
	"context"
	"crypto/tls"
	"errors"
	"io/fs"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
)

// memRootStore is the smallest thing that satisfies pki.RootStore. The
// certificate authority does not care where its two entries live, and a map
// keeps these tests off the disk.
type memRootStore map[string][]byte

func (m memRootStore) ReadFile(name string) ([]byte, error) {
	value, ok := m[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return value, nil
}

func (m memRootStore) WriteFile(name string, data []byte, _ fs.FileMode) error {
	m[name] = data
	return nil
}

func (m memRootStore) Exists(name string) bool {
	_, ok := m[name]
	return ok
}

// testRoot returns a freshly generated certificate authority.
func testRoot(t *testing.T) *pki.Root {
	t.Helper()
	root, err := pki.LoadOrGenerateRoot(memRootStore{})
	if err != nil {
		t.Fatalf("LoadOrGenerateRoot: %v", err)
	}
	return root
}

// serveLeaf starts a TLS listener presenting a leaf the given root issued, and
// returns its address.
func serveLeaf(t *testing.T, root *pki.Root) string {
	t.Helper()
	cert, err := root.IssueLeaf("ze-plugin-hub", []string{"127.0.0.1"})
	if err != nil {
		t.Fatalf("IssueLeaf: %v", err)
	}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { listener.Close() }) //nolint:errcheck // test cleanup

	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return // closed by cleanup
			}
			// Complete the handshake before closing. A server that closes on
			// accept resets the client mid-flight, and every case here would
			// then report a transport error rather than the verification
			// verdict it exists to read.
			go func(conn net.Conn) {
				defer conn.Close() //nolint:errcheck,gosec // test cleanup
				tc, ok := conn.(*tls.Conn)
				if !ok {
					return
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if handshakeErr := tc.HandshakeContext(ctx); handshakeErr != nil {
					return
				}
				_ = tc.SetReadDeadline(time.Now().Add(10 * time.Second))
				buf := make([]byte, 1)
				_, _ = tc.Read(buf) // returns when the client closes
			}(conn)
		}
	}()
	return listener.Addr().String()
}

// dialWithRoot dials addr with the config TLSConfigWithRoot builds from rootPEM
// and returns the handshake outcome.
func dialWithRoot(t *testing.T, addr string, rootPEM []byte) error {
	t.Helper()
	conf, err := ipc.TLSConfigWithRoot(rootPEM)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := (&tls.Dialer{Config: conf}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	return conn.Close()
}

// TestTLSConfigWithRootValidatesTheChain: AC-2 -- a leaf the root issued is
// accepted when the root is the client's only anchor.
//
// MUTATION: drop RootCAs from the returned config and this fails, because the
// system pool cannot verify a leaf a private root issued.
func TestTLSConfigWithRootValidatesTheChain(t *testing.T) {
	root := testRoot(t)
	addr := serveLeaf(t, root)

	if err := dialWithRoot(t, addr, root.CertificatePEM()); err != nil {
		t.Fatalf("a leaf the root issued was refused by a client holding that root: %v", err)
	}
}

// TestTLSConfigWithRootRefusesAnotherIssuer: AC-3 -- a certificate this root
// did not issue ends the handshake, so nothing is written to the peer.
//
// MUTATION: set InsecureSkipVerify in the returned config and this fails: the
// handshake completes against a stranger.
func TestTLSConfigWithRootRefusesAnotherIssuer(t *testing.T) {
	served := testRoot(t)
	trusted := testRoot(t)
	addr := serveLeaf(t, served)

	err := dialWithRoot(t, addr, trusted.CertificatePEM())
	if err == nil {
		t.Fatal("a client accepted a leaf its own root did not issue")
	}
	// Naming the verification failure keeps this from passing on a dial error,
	// a timeout, or a closed port.
	var verification *tls.CertificateVerificationError
	if !errors.As(err, &verification) {
		t.Fatalf("error = %v, want a certificate verification failure", err)
	}
}

// TestTLSConfigWithRootRefusesWithNoAnchor: AC-4 and R-2 -- an absent root is
// an error and no config. This is the defect the certificate authority
// replaces: the pin answered an empty fingerprint with InsecureSkipVerify.
//
// MUTATION: return a bare &tls.Config{} for an empty root and every case here
// reports a usable config.
func TestTLSConfigWithRootRefusesWithNoAnchor(t *testing.T) {
	for name, rootPEM := range map[string][]byte{
		"nil":        nil,
		"empty":      {},
		"whitespace": []byte("  \n\t "),
	} {
		t.Run(name, func(t *testing.T) {
			conf, err := ipc.TLSConfigWithRoot(rootPEM)
			if err == nil {
				t.Fatal("an absent trust anchor produced a usable TLS config")
			}
			if conf != nil {
				t.Fatal("a refused trust anchor must produce no config at all")
			}
		})
	}
}

// TestTLSConfigWithRootRefusesAnUnparsableAnchor: text that is not a
// certificate is refused too. An empty pool would otherwise verify nothing and
// read as a configured anchor.
//
// MUTATION: ignore the AppendCertsFromPEM result and this fails: a config comes
// back holding an empty pool.
func TestTLSConfigWithRootRefusesAnUnparsableAnchor(t *testing.T) {
	conf, err := ipc.TLSConfigWithRoot([]byte("-----BEGIN CERTIFICATE-----\nnot base64\n-----END CERTIFICATE-----\n"))
	if err == nil {
		t.Fatal("an unparsable trust anchor produced a usable TLS config")
	}
	if conf != nil {
		t.Fatal("a refused trust anchor must produce no config at all")
	}
}

// TestTLSConfigWithRootSetsNoServerName: crypto/tls fills ServerName from the
// dial address, so the leaf's SANs are what the peer is checked against. A
// ServerName set here would override every caller's address.
func TestTLSConfigWithRootSetsNoServerName(t *testing.T) {
	root := testRoot(t)

	conf, err := ipc.TLSConfigWithRoot(root.CertificatePEM())
	if err != nil {
		t.Fatalf("TLSConfigWithRoot: %v", err)
	}
	if conf.ServerName != "" {
		t.Fatalf("ServerName = %q, want it taken from the dial address", conf.ServerName)
	}
	if conf.InsecureSkipVerify {
		t.Fatal("the replacement for the pin must not skip verification")
	}
	if conf.MinVersion != tls.VersionTLS13 {
		t.Fatalf("MinVersion = %d, want TLS 1.3", conf.MinVersion)
	}
}
