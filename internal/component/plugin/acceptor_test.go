// VALIDATES: the hub acceptor serves a leaf the daemon's certificate authority
// root issued, so a peer holding the root validates the chain instead of
// comparing a fingerprint. Also that an acceptor built without an issuer is an
// error, never a self-signed fallback.
// PREVENTS: an acceptor that mints its own anchor (nothing can validate it and
// nothing can rotate it), and a nil issuer that silently reintroduces one.
package plugin_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/pki"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/pkg/zefs"
)

// caRoot opens a blob store in a temporary directory, generates the daemon's
// root in it, and returns the root with the parsed root certificate and a pool
// holding it. The pool is what a peer builds from the exported root PEM.
func caRoot(t *testing.T) (*pki.Root, *x509.Certificate, *x509.CertPool) {
	t.Helper()

	dir := t.TempDir()
	store, err := storage.NewBlob(filepath.Join(dir, "database.zefs"), dir)
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

	rootPEM, err := store.ReadFile(zefs.KeyCACert.Pattern)
	if err != nil {
		t.Fatalf("read stored root: %v", err)
	}
	block, _ := pem.Decode(rootPEM)
	if block == nil {
		t.Fatal("the stored root is not PEM")
	}
	rootCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse stored root: %v", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(rootPEM) {
		t.Fatal("the stored root PEM is not usable as a trust anchor")
	}
	return root, rootCert, pool
}

func TestHubAcceptorServesAnIssuedLeaf(t *testing.T) {
	root, rootCert, pool := caRoot(t)

	acceptor, err := plugin.NewHubAcceptor(nil, root, clock.RealClock{})
	if err != nil {
		t.Fatalf("NewHubAcceptor: %v", err)
	}
	t.Cleanup(acceptor.Stop)

	// The only anchor this client holds is the root. A handshake that completes
	// is the assertion: the leaf chains to it.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dialer := &tls.Dialer{Config: &tls.Config{
		RootCAs:    pool,
		MinVersion: tls.VersionTLS13,
	}}
	raw, err := dialer.DialContext(ctx, "tcp", acceptor.Addr().String())
	if err != nil {
		t.Fatalf("dial the acceptor with the root as the only anchor: %v", err)
	}
	defer raw.Close() //nolint:errcheck // the test fails on an assertion, not on close

	conn, ok := raw.(*tls.Conn)
	if !ok {
		t.Fatalf("tls.Dialer returned a %T, want a *tls.Conn", raw)
	}
	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		t.Fatal("the acceptor presented no certificate")
	}
	leaf := state.PeerCertificates[0]
	if leaf.IsCA {
		t.Fatal("the acceptor presented a CA certificate, not a leaf issued from one")
	}
	if err := leaf.CheckSignatureFrom(rootCert); err != nil {
		t.Fatalf("the presented leaf was not signed by the root: %v", err)
	}

	if len(state.VerifiedChains) == 0 {
		t.Fatal("the handshake completed with no verified chain")
	}
	chain := state.VerifiedChains[0]
	anchor := chain[len(chain)-1]
	if !bytes.Equal(anchor.Raw, rootCert.Raw) {
		t.Fatal("the verified chain ends at a different certificate than the stored root")
	}
}

func TestHubAcceptorRefusesWithoutAnIssuer(t *testing.T) {
	acceptor, err := plugin.NewHubAcceptor(nil, nil, clock.RealClock{})
	if err == nil {
		acceptor.Stop()
		t.Fatal("an acceptor with no certificate authority must be an error, not a self-signed fallback")
	}
	if acceptor != nil {
		t.Fatal("a failed NewHubAcceptor must return no acceptor")
	}
}

// rootlessAuthority issues leaves but publishes no root, which is the state a
// plugin process cannot recover from: it validates the chain against the root
// the hub hands it, so an empty one refuses every connect-back in another
// process with no cause named there.
type rootlessAuthority struct{ *pki.Root }

func (rootlessAuthority) CertificatePEM() []byte { return nil }

func TestHubAcceptorRefusesAnAuthorityWithNoRoot(t *testing.T) {
	root, _, _ := caRoot(t)

	acceptor, err := plugin.NewHubAcceptor(nil, rootlessAuthority{root}, clock.RealClock{})
	if err == nil {
		acceptor.Stop()
		t.Fatal("an acceptor whose authority publishes no root must be an error")
	}
	if acceptor != nil {
		t.Fatal("a failed NewHubAcceptor must return no acceptor")
	}
}

// TestHubAcceptorCarriesTheRootForPlugins: the acceptor holds the root PEM
// startExternal writes into every child's environment. Without it a plugin has
// no anchor and refuses to connect, which is the whole rail this replaces.
//
// MUTATION: pass nil instead of the authority root PEM in NewHubAcceptor and
// this fails on the length check.
func TestHubAcceptorCarriesTheRootForPlugins(t *testing.T) {
	root, rootCert, _ := caRoot(t)

	acceptor, err := plugin.NewHubAcceptor(nil, root, clock.RealClock{})
	if err != nil {
		t.Fatalf("NewHubAcceptor: %v", err)
	}
	t.Cleanup(acceptor.Stop)

	block, _ := pem.Decode(acceptor.RootPEM())
	if block == nil {
		t.Fatal("the acceptor carries no PEM root for a plugin process to validate against")
	}
	if !bytes.Equal(block.Bytes, rootCert.Raw) {
		t.Fatal("the acceptor carries a different root than the one that issued its leaf")
	}
}
