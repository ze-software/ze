// Design: docs/architecture/pki/tls-listeners.md -- reload ordering (AC-10) and rollback (R-3)

package hub

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"testing"
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	zepki "github.com/ze-software/ze/internal/component/pki"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/env"

	"github.com/stretchr/testify/require"
)

// fakeTLSUpdatable records the material handed to the rotation seam. It lives
// in an untagged file because tlsUpdatable is always-on hub code.
type fakeTLSUpdatable struct {
	certPEM []byte
	keyPEM  []byte
	calls   int
	err     error
}

func (f *fakeTLSUpdatable) UpdateTLSCertificate(certPEM, keyPEM []byte) error {
	f.calls++
	if f.err != nil {
		return f.err
	}
	f.certPEM = append([]byte(nil), certPEM...)
	f.keyPEM = append([]byte(nil), keyPEM...)
	return nil
}

// caSignedB64 returns a CA certificate, a leaf it issued, and the leaf key, all
// base64 DER as the pki config block carries them. A CA is required, not
// decoration: pki.Validate refuses to install any device certificate that does
// not chain to a configured ca entry (store.go), so a store certificate always
// has a verifiable path.
func caSignedB64(t *testing.T, cn string) (caB64, certB64, keyB64 string) {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: cn + " ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano() + 1),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
	require.NoError(t, err)
	leafKeyDER, err := x509.MarshalPKCS8PrivateKey(leafKey)
	require.NoError(t, err)

	return base64.StdEncoding.EncodeToString(caDER),
		base64.StdEncoding.EncodeToString(leafDER),
		base64.StdEncoding.EncodeToString(leafKeyDER)
}

// pkiWebTree builds the loaded-tree map a reload sees: a pki block defining a CA
// and certName under it, and environment.web referencing `reference`.
func pkiWebTree(caB64, certB64, keyB64, certName, reference string) map[string]any {
	return map[string]any{
		"pki": map[string]any{
			"ca": map[string]any{
				certName + "-ca": map[string]any{"certificate": caB64},
			},
			"certificate": map[string]any{
				certName: map[string]any{
					"certificate": certB64,
					"private":     map[string]any{"key": keyB64},
				},
			},
		},
		"environment": map[string]any{
			"web": map[string]any{
				"enabled":     "true",
				"certificate": reference,
			},
		},
	}
}

// treeFromMap turns a loaded-tree map into the parsed *config.Tree a reload
// also carries (runReload gets both).
func treeFromMap(m map[string]any) *zeconfig.Tree {
	return configTreeFromMap(m)
}

func TestReloadInstallsPKIBeforePluginApply(t *testing.T) {
	// VALIDATES: AC-10 -- ONE commit that adds a pki certificate AND references
	// it from environment.web.certificate resolves within that same commit.
	// PREVENTS: the ordering defect this spec fixes. The store used to be
	// installed at the END of doReload, after plugin apply, so a consumer
	// resolving a reference during that commit looked it up in the PREVIOUS
	// store and failed. An operator then had to commit twice for a change they
	// wrote once.
	//
	// The observation point is the certificate gate in runReload, which runs
	// immediately before s.ReloadConfig (the plugin apply). "fresh-cert" exists
	// ONLY in the incoming commit, so the gate can only pass if the new store is
	// already installed by the time plugin apply is reached.
	//
	// This harness has no reactor, so ReloadConfig itself cannot complete; the
	// reload therefore stops one step AFTER the gate. Where it stops is the
	// assertion. TestReloadRejectsBrokenWebCertificateReference is the negative
	// control proving the gate does reject a name the store lacks, so "no error
	// about fresh-cert" is not vacuously true.
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	caB64, certB64, keyB64 := caSignedB64(t, "reload chain leaf")
	newTree := pkiWebTree(caB64, certB64, keyB64, "fresh-cert", "fresh-cert")

	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	cp := zeconfig.NewProvider()
	// The store starts EMPTY: "fresh-cert" exists only in the incoming commit.
	cp.SetRoot("pki", map[string]any{})

	fake := &fakeTLSUpdatable{}
	lm := &listenerMigrator{}
	lm.setWebTLS(fake)

	load := func() (map[string]any, *zeconfig.Tree, error) {
		return newTree, treeFromMap(newTree), nil
	}

	err = runReload(srv, cp, load, lm)
	require.Error(t, err, "this harness has no reactor, so the reload cannot complete")
	require.ErrorContains(t, err, "no reactor configured",
		"the reload must reach plugin apply, which means it passed the certificate gate")
	require.NotContains(t, err.Error(), "fresh-cert",
		"the commit's own certificate must resolve during that same commit")
}

func TestReloadCertificateGateSeesTheIncomingStore(t *testing.T) {
	// VALIDATES: AC-10 at the seam, without a plugin server -- the material the
	// web listener is handed comes from the commit being applied.
	// Complements the test above: that one proves WHERE the gate sits relative
	// to plugin apply, this one proves the gate resolves the right material.
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	caB64, certB64, keyB64 := caSignedB64(t, "incoming leaf")
	incoming, err := preparePKIConfig(pkiWebTree(caB64, certB64, keyB64, "fresh-cert", "fresh-cert"))
	require.NoError(t, err)
	require.NoError(t, zepki.Load(incoming))

	fake := &fakeTLSUpdatable{}
	lm := &listenerMigrator{}
	lm.setWebTLS(fake)

	require.NoError(t, lm.updateWebCertificate("fresh-cert"))
	require.Equal(t, 1, fake.calls)

	pair, err := tls.X509KeyPair(fake.certPEM, fake.keyPEM)
	require.NoError(t, err)
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	require.NoError(t, err)
	require.Equal(t, "incoming leaf", leaf.Subject.CommonName)
}

func TestReloadRejectsBrokenWebCertificateReference(t *testing.T) {
	// VALIDATES: AC-3 and R-5 on the RELOAD path -- a commit whose web
	// certificate reference does not resolve is rejected, and the running
	// listener keeps whatever it was serving.
	// PREVENTS: a config edit silently downgrading a production HTTPS listener
	// to self-signed at reload time, which no startup check can catch.
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	caB64, certB64, keyB64 := caSignedB64(t, "present cert")
	newTree := pkiWebTree(caB64, certB64, keyB64, "present-cert", "absent-cert")

	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	cp := zeconfig.NewProvider()
	cp.SetRoot("pki", map[string]any{})

	fake := &fakeTLSUpdatable{}
	lm := &listenerMigrator{}
	lm.setWebTLS(fake)

	load := func() (map[string]any, *zeconfig.Tree, error) {
		return newTree, treeFromMap(newTree), nil
	}

	err = runReload(srv, cp, load, lm)
	require.Error(t, err, "a reload naming an absent certificate must fail")
	require.Contains(t, err.Error(), "absent-cert")
	require.Zero(t, fake.calls, "nothing may be installed on the listener by a rejected reload")
}

func TestRollbackReloadRestoresPriorPKIStore(t *testing.T) {
	// VALIDATES: R-3 -- a rejected reload puts the PREVIOUS store back, so the
	// daemon never runs the old config against the new commit's certificates.
	// PREVENTS: store/config drift, where a rolled-back reload leaves consumers
	// resolving names into material the active config never described.
	t.Cleanup(func() { _ = zepki.Load(nil) })

	oldCA, oldCert, oldKey := caSignedB64(t, "old cert")
	newCA, newCert, newKey := caSignedB64(t, "new cert")

	priorPKI, err := preparePKIConfig(pkiWebTree(oldCA, oldCert, oldKey, "rolled-back", ""))
	require.NoError(t, err)
	newPKI, err := preparePKIConfig(pkiWebTree(newCA, newCert, newKey, "rolled-back", ""))
	require.NoError(t, err)

	require.NoError(t, zepki.Load(priorPKI))
	require.NoError(t, zepki.Load(newPKI))
	require.Equal(t, "new cert", zepki.CertCN("rolled-back"), "precondition: the new store is installed")

	cp := zeconfig.NewProvider()
	cp.SetRoot("bgp", map[string]any{"marker": "old"})
	prior, err := snapshotProvider(cp)
	require.NoError(t, err)

	require.NoError(t, rollbackReload(context.Background(), nil, nil, cp, prior, priorPKI))

	require.Equal(t, "old cert", zepki.CertCN("rolled-back"),
		"rollback must reinstall the certificate material the prior config described")
}

func TestReloadWebCertificateReadsSettingsNotAddresses(t *testing.T) {
	// VALIDATES: the reload reads environment.web.certificate as a SETTING, so a
	// listener started by --web or ze.web.enabled still rotates onto the
	// operator's certificate.
	// PREVENTS: an `enabled` gate that discards the certificate SETTING on the
	// reload path, where it would silently strand the operator's reference.
	tree := map[string]any{
		"environment": map[string]any{
			"web": map[string]any{
				"enabled":     "false",
				"certificate": "still-mine",
			},
		},
	}
	require.Equal(t, "still-mine", reloadWebCertificate(treeFromMap(tree)),
		"a disabled block still states which certificate the operator wants served")

	require.Empty(t, reloadWebCertificate(nil))
	require.Empty(t, reloadWebCertificate(treeFromMap(map[string]any{})))
}

func TestReloadWebCertificateEnvWins(t *testing.T) {
	// The env var pins the certificate at startup; a config edit must not
	// re-point it at reload time, matching the precedence every other web
	// setting uses.
	require.NoError(t, env.Set("ze.web.certificate", "pinned"))
	t.Cleanup(func() { _ = env.Set("ze.web.certificate", "") })

	tree := map[string]any{
		"environment": map[string]any{
			"web": map[string]any{"enabled": "true", "certificate": "from-config"},
		},
	}
	require.Equal(t, "pinned", reloadWebCertificate(treeFromMap(tree)))
}
