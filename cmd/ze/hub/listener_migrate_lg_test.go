// Design: docs/architecture/pki/tls-listeners.md -- per-listener certificate rotation (R-1)

package hub

import (
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"maps"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	zepki "github.com/ze-software/ze/internal/component/pki"
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
	lm.setLGTransport(lgFake)

	require.NoError(t, lm.updateLGTransport(lgListenerConfig{certName: "lg-cert"}))
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
	lm.setLGTransport(lgFake)

	err := lm.updateLGTransport(lgListenerConfig{certName: "typo-cert"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "typo-cert")
	require.Zero(t, lgFake.calls, "unresolvable material must never reach the listener")
}

func TestListenerMigratorUpdateLGCertificateNoOp(t *testing.T) {
	// VALIDATES: with no handle no looking glass runs, in this build or on this
	// daemon, and a reload changes nothing on it and says nothing, whether or
	// not the config names a certificate.
	// PREVENTS: a reload failing on a daemon that runs no looking glass, and a
	// nil handle panicking the daemon on a build without the ze_lg tag, where
	// nothing ever calls setLGTransport.
	//
	// A looking glass that runs always holds the handle, whatever it serves
	// (register_lg.go): an empty name on one returns its TLS listeners to the
	// self-signed pair (TestReloadDroppingLGCertificateNameServesSelfSigned,
	// ze_lg), or moves an entry that left tls unset to HTTP when the daemon has
	// no store (TestReloadDroppingLGCertificateNameWithoutStore, ze_lg).
	loadTwoCertStore(t)

	var logged strings.Builder
	noHandle := &listenerMigrator{logger: slog.New(slog.NewTextHandler(&logged, nil))}
	require.NoError(t, noHandle.updateLGTransport(lgListenerConfig{certName: ""}))
	require.NoError(t, noHandle.updateLGTransport(lgListenerConfig{certName: "lg-cert"}))
	require.Empty(t, logged.String(), "a daemon that runs no looking glass has nothing to say about its certificate")
}
