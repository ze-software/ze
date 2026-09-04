// Design: docs/architecture/pki/tls-listeners.md -- the config route into the PKI store

package hub

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"strconv"
	"testing"
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	zepki "github.com/ze-software/ze/internal/component/pki"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	"github.com/stretchr/testify/require"
)

// chainB64 builds a trust chain of the requested depth and returns every value
// base64 DER, which is the encoding the pki block takes. The root signs the
// first intermediate, each intermediate signs the next, and the last one signs
// the leaf. interB64 is ordered from the issuer of the leaf toward the trust
// anchor, which is the order the YANG leaf-list documents.
func chainB64(t *testing.T, commonName string, intermediates int) (caB64 string, interB64 []string, certB64, keyB64 string) {
	t.Helper()

	caCert, caKey, caDER := issueCA(t, commonName+" root", nil, nil)

	issuer, issuerKey := caCert, caKey
	chain := make([]string, 0, intermediates)
	for i := range intermediates {
		name := commonName + " intermediate " + strconv.Itoa(i+1)
		cert, key, der := issueCA(t, name, issuer, issuerKey)
		// The leaf's issuer comes first, so each new intermediate is prepended.
		chain = append([]string{base64.StdEncoding.EncodeToString(der)}, chain...)
		issuer, issuerKey = cert, key
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano() + int64(intermediates) + 1),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, issuer, &leafKey.PublicKey, issuerKey)
	require.NoError(t, err)
	leafKeyDER, err := x509.MarshalPKCS8PrivateKey(leafKey)
	require.NoError(t, err)

	return base64.StdEncoding.EncodeToString(caDER),
		chain,
		base64.StdEncoding.EncodeToString(leafDER),
		base64.StdEncoding.EncodeToString(leafKeyDER)
}

// issueCA signs one CA certificate. A nil parent makes it self-signed, which is
// the root case.
func issueCA(t *testing.T, commonName string, parent *x509.Certificate, parentKey *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	signer, signerKey := parent, parentKey
	if signer == nil {
		signer, signerKey = tmpl, key
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, signer, &key.PublicKey, signerKey)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert, key, der
}

// pkiChainConfig renders the config text an operator writes for a stored chain:
// the root under `pki ca`, and the leaf with one `intermediate` statement for
// each intermediate certificate.
func pkiChainConfig(caB64 string, interB64 []string, certB64, keyB64, name string) string {
	text := "pki {\n\tca " + name + "-root {\n\t\tcertificate " + caB64 + "\n\t}\n"
	text += "\tcertificate " + name + " {\n\t\tcertificate " + certB64 + "\n"
	for _, inter := range interB64 {
		text += "\t\tintermediate " + inter + "\n"
	}
	text += "\t\tprivate {\n\t\t\tkey " + keyB64 + "\n\t\t}\n\t}\n}\n"
	return text
}

func TestPreparePKIConfigKeepsEveryIntermediate(t *testing.T) {
	// VALIDATES: a `pki certificate <name> intermediate` statement reaches the
	// store, so a leaf issued by an intermediate CA validates and the listener
	// can serve the whole path.
	// PREVENTS: the config route dropping the chain. preparePKIConfig used to
	// rebuild a tree from the plugin-facing map, where a one-member leaf-list is
	// a bare string and a two-member one is a []string, so every intermediate
	// was lost and a real CA-issued chain could not boot.
	for _, intermediates := range []int{1, 2, 3} {
		name := "chain-" + strconv.Itoa(intermediates)
		t.Run(name, func(t *testing.T) {
			caB64, interB64, certB64, keyB64 := chainB64(t, name, intermediates)
			tree, err := zeconfig.ParseTreeWithYANG(pkiChainConfig(caB64, interB64, certB64, keyB64, name), nil)
			require.NoError(t, err)

			cfg, err := preparePKIConfig(tree)
			require.NoError(t, err, "a chain the operator wrote in full must validate")

			entry := cfg.Certificates[name]
			require.NotNil(t, entry)
			require.Len(t, entry.Intermediates, intermediates,
				"every intermediate the config names must reach the store")
			require.Len(t, entry.RawIntermediates, intermediates)
		})
	}
}

func TestReloadInstallsEveryIntermediate(t *testing.T) {
	// VALIDATES: the SIGHUP reload installs a store built from the whole config,
	// intermediates included, through the loader the daemon itself uses.
	// PREVENTS: a reload refusing a chain it should accept. The reload lowered
	// the tree to a map and rebuilt it, so `pki.Validate` saw an empty
	// intermediate pool and failed the commit with "certificate signed by
	// unknown authority".
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	const name = "reload-chain"
	caB64, interB64, certB64, keyB64 := chainB64(t, name, 2)
	tree, err := zeconfig.ParseTreeWithYANG(pkiChainConfig(caB64, interB64, certB64, keyB64, name), nil)
	require.NoError(t, err)

	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	cp := zeconfig.NewProvider()
	cp.SetRoot("pki", map[string]any{})

	// The same pair diskConfigLoaders returns: the plugin-facing map, and the
	// tree it was lowered from.
	lowered := tree.ToPluginMap()
	load := func() (map[string]any, *zeconfig.Tree, error) {
		return lowered, tree, nil
	}

	// This harness has no reactor, so the reload always ends there. WHICH error
	// comes back is the assertion: the pki stage runs first, so a reload that
	// reaches the reactor is a reload whose chain validated. Reading the store
	// afterwards would prove nothing, because the failed apply restores the
	// prior store on the way out.
	err = runReload(srv, cp, load, &listenerMigrator{})
	require.Error(t, err)
	require.ErrorContains(t, err, "no reactor configured",
		"the reload must get past the pki stage, which means every intermediate reached the store")
	require.NotContains(t, err.Error(), "unknown authority",
		"a chain the operator wrote in full must not be refused for lacking its own issuer")
}
