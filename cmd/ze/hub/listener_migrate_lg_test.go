// Design: docs/architecture/pki/tls-listeners.md -- per-listener certificate rotation (R-1)

package hub

import (
	"crypto/tls"
	"crypto/x509"
	"maps"
	"testing"

	zepki "github.com/ze-software/ze/internal/component/pki"

	"github.com/stretchr/testify/require"
)

// mergePKIBlocks returns one pki root holding every ca and certificate of the
// blocks given, so a single store can define the web entry and the
// looking-glass entry the isolation test rotates between.
func mergePKIBlocks(t *testing.T, blocks ...map[string]any) map[string]any {
	t.Helper()

	cas := map[string]any{}
	certs := map[string]any{}
	for _, block := range blocks {
		blockCAs, ok := block["ca"].(map[string]any)
		require.True(t, ok)
		maps.Copy(cas, blockCAs)

		blockCerts, ok := block["certificate"].(map[string]any)
		require.True(t, ok)
		maps.Copy(certs, blockCerts)
	}
	return map[string]any{"ca": cas, "certificate": certs}
}

// installedLeafCN returns the common name of the leaf a rotation handed to the
// listener, which is what says WHICH certificate reached it.
func installedLeafCN(t *testing.T, f *fakeTLSUpdatable) string {
	t.Helper()

	pair, err := tls.X509KeyPair(f.certPEM, f.keyPEM)
	require.NoError(t, err)
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	require.NoError(t, err)
	return leaf.Subject.CommonName
}

// loadTwoCertStore installs a store defining web-cert and lg-cert, whose leaves
// carry distinguishable common names.
func loadTwoCertStore(t *testing.T) {
	t.Helper()

	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	webCA, webCert, webKey := caSignedB64(t, "web leaf")
	lgCA, lgCert, lgKey := caSignedB64(t, "lg leaf")
	cfg, err := preparePKIConfig(treeFromMap(map[string]any{
		"pki": mergePKIBlocks(t,
			pkiBlock(webCA, webCert, webKey, "web-cert"),
			pkiBlock(lgCA, lgCert, lgKey, "lg-cert"),
		),
	}))
	require.NoError(t, err)
	require.NoError(t, zepki.Load(cfg))
}

func TestListenerMigratorUpdateLGCertificate(t *testing.T) {
	// VALIDATES: R-1 -- one migrator carries both TLS handles, and each rotation
	// reads its OWN handle and its OWN name.
	// PREVENTS: the web listener presenting the looking glass's identity, or the
	// reverse. The looking glass is a public read-only surface and the web
	// listener carries the operator's own session, so a crossed chain is both a
	// broken listener and a leaked identity.
	loadTwoCertStore(t)

	webFake := &fakeTLSUpdatable{}
	lgFake := &fakeTLSUpdatable{}
	lm := &listenerMigrator{}
	lm.setWebTLS(webFake)
	lm.setLGTLS(lgFake)

	require.NoError(t, lm.updateLGCertificate("lg-cert"))
	require.Equal(t, 1, lgFake.calls)
	require.Zero(t, webFake.calls, "rotating the looking glass must not touch the web listener")
	require.Equal(t, "lg leaf", installedLeafCN(t, lgFake))

	require.NoError(t, lm.updateWebCertificate("web-cert"))
	require.Equal(t, 1, webFake.calls)
	require.Equal(t, 1, lgFake.calls, "rotating the web listener must not touch the looking glass")
	require.Equal(t, "web leaf", installedLeafCN(t, webFake))
	require.Equal(t, "lg leaf", installedLeafCN(t, lgFake),
		"the looking glass must still hold the chain its own rotation installed")
}

func TestListenerMigratorUpdateLGCertificateFailsClosed(t *testing.T) {
	// VALIDATES: a looking-glass name that does not resolve is an error, and the
	// listener is left holding what it had.
	// PREVENTS: a silent downgrade to the self-signed certificate a public
	// listener may still be holding, which looks healthy and is refused by every
	// visitor's browser.
	loadTwoCertStore(t)

	lgFake := &fakeTLSUpdatable{}
	lm := &listenerMigrator{}
	lm.setLGTLS(lgFake)

	err := lm.updateLGCertificate("typo-cert")
	require.Error(t, err)
	require.Contains(t, err.Error(), "typo-cert")
	require.Zero(t, lgFake.calls, "unresolvable material must never reach the listener")
}

func TestListenerMigratorUpdateLGCertificateNoOp(t *testing.T) {
	// VALIDATES: the two cases that are not a rotation -- the operator named no
	// certificate, and the looking glass is not in this build.
	// PREVENTS: a reload failing on a deployment that never asked for a named
	// certificate, and a nil handle panicking the daemon on a build without the
	// ze_lg tag, where nothing ever calls setLGTLS.
	loadTwoCertStore(t)

	lgFake := &fakeTLSUpdatable{}
	lm := &listenerMigrator{}
	lm.setLGTLS(lgFake)
	require.NoError(t, lm.updateLGCertificate(""))
	require.Zero(t, lgFake.calls)

	noHandle := &listenerMigrator{}
	require.NoError(t, noHandle.updateLGCertificate("lg-cert"))
}
