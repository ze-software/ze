// VALIDATES: a managed client authenticates the hub before it sends its token --
// it validates the chain against the pki ca entry it names, refuses a hub that
// entry did not anchor, refuses outright when the named entry resolves to
// nothing, and keeps working when the hub reissues its leaf under the same root
// (spec-local-ca AC-3/AC-4/AC-7/AC-12).
// PREVENTS: the pin this replaced, which named one certificate and died with it,
// and the posture before that, where reaching a hub serving a certificate no
// public CA issued meant ze.managed.tls.insecure and a token sent to whatever
// answered on the hub address.

package managed

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/pkg/zefs"
)

const testHubToken = "0123456789abcdef0123456789abcdef"

// testRoot generates one certificate authority in a temporary blob store, the
// way a daemon does on first start.
func testRoot(t *testing.T) *pki.Root {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.NewBlob(dir+"/database.zefs", dir)
	if err != nil {
		t.Fatalf("open blob store: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := store.Close(); closeErr != nil {
			t.Errorf("close blob store: %v", closeErr)
		}
	})

	root, err := pki.LoadOrGenerateRoot(store)
	if err != nil {
		t.Fatalf("LoadOrGenerateRoot: %v", err)
	}
	if _, readErr := store.ReadFile(zefs.KeyCACert.Pattern); readErr != nil {
		t.Fatalf("the generated root was not stored: %v", readErr)
	}
	return root
}

// installCA puts root into the pki store under name, which is what an operator
// does by pasting the exported root into a pki ca block. The store is package
// state, so the test restores it.
func installCA(t *testing.T, name string, root *pki.Root) {
	t.Helper()
	cert := root.Certificate()
	if err := pki.Load(&pki.PKIConfig{
		CACerts: map[string]*pki.CACertEntry{
			name: {Name: name, Certificate: cert, Raw: cert.Raw},
		},
		Certificates: map[string]*pki.CertificateEntry{},
	}); err != nil {
		t.Fatalf("load pki ca %s: %v", name, err)
	}
	t.Cleanup(func() {
		if err := pki.Load(nil); err != nil {
			t.Errorf("clear pki store: %v", err)
		}
	})
}

// issuingHub is a TLS listener that issues a FRESH leaf from root for every
// handshake, which is what a hub does across a restart: the leaf changes and
// the root does not. It records the first line an accepted connection sent
// after a successful handshake, or the handshake error, which answers the one
// question this file asks: did the client's token reach a server it had not
// authenticated?
type issuingHub struct {
	addr        string
	firstLine   chan string
	handshakeNG chan error
	serials     chan *big.Int
}

func startIssuingHub(t *testing.T, root *pki.Root) *issuingHub {
	t.Helper()

	hub := &issuingHub{
		firstLine:   make(chan string, 4),
		handshakeNG: make(chan error, 4),
		serials:     make(chan *big.Int, 4),
	}

	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		MinVersion: tls.VersionTLS13,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			cert, issueErr := root.IssueLeaf("ze-managed-hub", []string{"127.0.0.1"})
			if issueErr != nil {
				return nil, issueErr
			}
			leaf, parseErr := x509.ParseCertificate(cert.Certificate[0])
			if parseErr != nil {
				return nil, parseErr
			}
			select {
			case hub.serials <- leaf.SerialNumber:
			default:
			}
			return &cert, nil
		},
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { listener.Close() }) //nolint:errcheck // test cleanup
	hub.addr = listener.Addr().String()

	go func() {
		for {
			conn, acceptErr := listener.Accept()
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
func (h *issuingHub) serve(conn net.Conn) {
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
func (h *issuingHub) wroteToken(t *testing.T, timeout time.Duration) (string, bool) {
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

// TestManagedClientValidatesAgainstConfiguredRoot: AC-12 -- a client naming a
// pki ca entry validates the hub's chain against that root and sends its token
// over the connection.
//
// MUTATION: drop the RootCAs assignment in clientTLSConfig and this fails: the
// system pool cannot verify a leaf a private root issued.
func TestManagedClientValidatesAgainstConfiguredRoot(t *testing.T) {
	root := testRoot(t)
	installCA(t, "fleet-hub", root)
	hub := startIssuingHub(t, root)

	cfg := &ClientConfig{
		Name:    "edge-01",
		Server:  hub.addr,
		Token:   testHubToken,
		CA:      "fleet-hub",
		Handler: &Handler{Validate: func([]byte) error { return nil }},
	}

	_ = runOnce(t, cfg) // the hub closes after one line, so this always ends in an error

	line, ok := hub.wroteToken(t, 3*time.Second)
	if !ok {
		t.Fatal("a client holding the hub root did not reach the hub")
	}
	if !strings.Contains(line, testHubToken) {
		t.Fatalf("auth frame does not carry the token: %q", line)
	}
}

// TestManagedClientRefusesAnotherIssuer: AC-3 -- a hub whose leaf the
// configured root did not issue ends the handshake and gets no token. This is
// the impostor case.
//
// MUTATION: set InsecureSkipVerify in the ca branch of clientTLSConfig and this
// fails: the handshake completes and the token arrives.
func TestManagedClientRefusesAnotherIssuer(t *testing.T) {
	trusted := testRoot(t)
	stranger := testRoot(t)
	installCA(t, "fleet-hub", trusted)
	hub := startIssuingHub(t, stranger)

	cfg := &ClientConfig{
		Name:    "edge-01",
		Server:  hub.addr,
		Token:   testHubToken,
		CA:      "fleet-hub",
		Handler: &Handler{Validate: func([]byte) error { return nil }},
	}

	err := runOnce(t, cfg)
	if err == nil {
		t.Fatal("a client accepted a hub its configured root did not anchor")
	}
	if !strings.Contains(err.Error(), "tls handshake") {
		t.Fatalf("error = %v, want a TLS handshake failure", err)
	}
	if line, ok := hub.wroteToken(t, time.Second); ok {
		t.Fatalf("client sent %q to a hub it did not authenticate", line)
	}
}

// TestManagedClientRefusesWithNoAnchor: AC-4 -- there is no configuration in
// which an absent anchor produces a successful connection. A named ca entry
// that resolves to nothing is an error and no config at all, and a client that
// names none falls to the system pool, which cannot verify a private hub.
//
// MUTATION: return the system-pool config when the named entry is missing, and
// the first case fails: a broken reference becomes a working connection
// attempt against an anchor the operator never chose.
func TestManagedClientRefusesWithNoAnchor(t *testing.T) {
	root := testRoot(t)

	t.Run("named-entry-does-not-resolve", func(t *testing.T) {
		installCA(t, "fleet-hub", root)
		hub := startIssuingHub(t, root)

		cfg := &ClientConfig{
			Name:    "edge-01",
			Server:  hub.addr,
			Token:   testHubToken,
			CA:      "no-such-ca",
			Handler: &Handler{Validate: func([]byte) error { return nil }},
		}

		err := runOnce(t, cfg)
		if err == nil {
			t.Fatal("a client whose named ca entry does not exist connected anyway")
		}
		if !strings.Contains(err.Error(), "no-such-ca") {
			t.Fatalf("error = %v, want the unresolved ca name", err)
		}
		if line, ok := hub.wroteToken(t, time.Second); ok {
			t.Fatalf("client sent %q with no trust anchor at all", line)
		}
	})

	t.Run("no-ca-configured", func(t *testing.T) {
		hub := startIssuingHub(t, root)

		cfg := &ClientConfig{
			Name:    "edge-01",
			Server:  hub.addr,
			Token:   testHubToken,
			Handler: &Handler{Validate: func([]byte) error { return nil }},
		}

		if err := runOnce(t, cfg); err == nil {
			t.Fatal("a client accepted an unverifiable hub certificate by default")
		}
		if line, ok := hub.wroteToken(t, time.Second); ok {
			t.Fatalf("client sent %q to an unverified hub", line)
		}
	})
}

// TestManagedClientSurvivesAHubRestart: AC-7 -- the hub issues a fresh leaf,
// which is what a restart produces, and the same client config still connects.
// The anchor is the issuer, so nothing about the client changes.
//
// The hub here issues on every handshake, so the second connection meets a leaf
// with a different serial. That is the property, and it needs no port reuse to
// demonstrate.
//
// MUTATION: put the leaf rather than the root in the pool built by
// clientTLSConfig and the second connection fails: the leaf it pinned is gone.
func TestManagedClientSurvivesAHubRestart(t *testing.T) {
	root := testRoot(t)
	installCA(t, "fleet-hub", root)
	hub := startIssuingHub(t, root)

	cfg := &ClientConfig{
		Name:    "edge-01",
		Server:  hub.addr,
		Token:   testHubToken,
		CA:      "fleet-hub",
		Handler: &Handler{Validate: func([]byte) error { return nil }},
	}

	serials := make([]*big.Int, 0, 2)
	for attempt := range 2 {
		_ = runOnce(t, cfg)
		if _, ok := hub.wroteToken(t, 3*time.Second); !ok {
			t.Fatalf("connection %d did not reach the hub", attempt+1)
		}
		select {
		case serial := <-hub.serials:
			serials = append(serials, serial)
		case <-time.After(time.Second):
			t.Fatalf("connection %d recorded no served certificate", attempt+1)
		}
	}

	if serials[0].Cmp(serials[1]) == 0 {
		t.Fatal("the hub served the same leaf twice, so no reissue was exercised")
	}
}
