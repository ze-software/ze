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
	"strconv"
	"strings"
	"testing"
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	zepki "github.com/ze-software/ze/internal/component/pki"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/configorder"
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
		"pki": pkiBlock(caB64, certB64, keyB64, certName),
		"environment": map[string]any{
			"web": map[string]any{
				"enabled":     "true",
				"certificate": reference,
			},
		},
	}
}

// treeFromMap turns a loaded-tree map into the parsed *config.Tree a reload
// also carries (runReload gets both), so a test can keep writing its fixture as
// a map literal.
//
// TEST FIXTURES ONLY. This conversion is LOSSY and the daemon must never take
// it: ToMap lowers a one-member leaf-list to a bare string and a longer one to
// a []string, so neither arm below can rebuild the leaf-list it came from.
// preparePKIConfig used to do exactly this and lost every
// `pki certificate <name> intermediate` (TestPreparePKIConfigKeepsEveryIntermediate,
// plan/journal/validated-value-discarded-by-its-caller.md). It is safe here only
// because these fixtures carry no leaf-list; a fixture that needs one builds its
// tree from config text with zeconfig.ParseTreeWithYANG instead.
func treeFromMap(m map[string]any) *zeconfig.Tree {
	if m == nil {
		return nil
	}
	t := zeconfig.NewTree()
	for k, v := range m {
		// A reserved order key is not config: it is how ToPluginMap carries a
		// list's entry order beside the list. Rebuilding it as a container
		// would put a node in this tree that no YANG module declares.
		if strings.HasPrefix(k, configorder.KeyPrefix) {
			continue
		}
		switch val := v.(type) {
		case string:
			t.Set(k, val)
		case float64:
			t.Set(k, strconv.FormatFloat(val, 'f', -1, 64))
		case bool:
			if val {
				t.Set(k, "true")
			} else {
				t.Set(k, "false")
			}
		case map[string]any:
			t.SetContainer(k, treeFromMap(val))
			if mapValuesAreMaps(val) {
				for entryKey, entryVal := range val {
					entryMap, ok := entryVal.(map[string]any)
					if !ok {
						continue
					}
					t.AddListEntry(k, entryKey, treeFromMap(entryMap))
				}
			}
		}
	}
	return t
}

// mapValuesAreMaps reports whether every value is a map, which is how a YANG
// list entry set is told from a container of leaves in a fixture literal.
func mapValuesAreMaps(m map[string]any) bool {
	if len(m) == 0 {
		return false
	}
	for _, v := range m {
		if _, ok := v.(map[string]any); !ok {
			return false
		}
	}
	return true
}

// pkiBlock builds the pki root a reload sees: a CA and certName under it.
func pkiBlock(caB64, certB64, keyB64, certName string) map[string]any {
	return map[string]any{
		"ca": map[string]any{
			certName + "-ca": map[string]any{"certificate": caB64},
		},
		"certificate": map[string]any{
			certName: map[string]any{
				"certificate": certB64,
				"private":     map[string]any{"key": keyB64},
			},
		},
	}
}

// pkiLGTree builds the loaded-tree map a reload sees for the looking glass: a
// pki block defining certName, and the environment.looking-glass leaves the
// caller names.
func pkiLGTree(caB64, certB64, keyB64, certName string, lg map[string]any) map[string]any {
	return map[string]any{
		"pki": pkiBlock(caB64, certB64, keyB64, certName),
		"environment": map[string]any{
			"looking-glass": lg,
		},
	}
}

func TestLGCertificateEnvWins(t *testing.T) {
	// VALIDATES: AC-10 -- ze.looking-glass.certificate beats the config file on
	// the startup path and on the reload path.
	// PREVENTS: a config edit re-pointing a certificate an operator pinned
	// through the environment, which would serve one identity after a reload and
	// another after the next restart.
	//
	// Both paths call lgCertificateName (main_reload.go): the startup gate in
	// cmd/ze/hub/main.go takes its value from it, and the reload subtest below
	// drives runReload, which reaches the same function through the real reload.
	require.NoError(t, env.Set("ze.looking-glass.certificate", "pinned"))
	t.Cleanup(func() { _ = env.Set("ze.looking-glass.certificate", "") })

	t.Run("the startup path serves the environment value", func(t *testing.T) {
		tree := map[string]any{
			"environment": map[string]any{
				"looking-glass": map[string]any{"enabled": "true", "certificate": "from-config"},
			},
		}
		require.Equal(t, "pinned", lgCertificateName(treeFromMap(tree)))
	})

	t.Run("the reload path serves the environment value", func(t *testing.T) {
		t.Cleanup(func() { _ = zepki.Load(nil) })
		require.NoError(t, zepki.Load(nil))

		// The config names a certificate the store DOES define, the environment
		// names one it does not. The reload is refused over the environment's
		// name, which no reload reading the config file could produce.
		caB64, certB64, keyB64 := caSignedB64(t, "config lg cert")
		newTree := pkiLGTree(caB64, certB64, keyB64, "from-config", map[string]any{
			"enabled":     "true",
			"certificate": "from-config",
		})

		srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
		require.NoError(t, err)

		cp := zeconfig.NewProvider()
		cp.SetRoot("pki", map[string]any{})

		fake := &fakeTLSUpdatable{}
		lm := &listenerMigrator{}
		lm.setLGTLS(fake)

		load := func() (map[string]any, *zeconfig.Tree, error) {
			return newTree, treeFromMap(newTree), nil
		}

		err = runReload(srv, cp, load, lm)
		require.Error(t, err)
		// The refusal names the certificate it FAILED to resolve. "from-config"
		// appears too, in the list of names the store does hold, so the
		// assertion is on the failing name rather than on its absence.
		require.Contains(t, err.Error(), "certificate pinned not found",
			"the reload must resolve the environment value, not the config file's")
		require.Zero(t, fake.calls)
	})
}

func TestLGCertificateNameReadsSettingsNotAddresses(t *testing.T) {
	// VALIDATES: A-3 on the certificate leaf -- the name is a SETTING, so a
	// looking glass started by ze.looking-glass.enabled or --lg still serves the
	// operator's certificate.
	// PREVENTS: an `enabled` gate discarding the reference for exactly the
	// deployments most likely to set one.
	tree := map[string]any{
		"environment": map[string]any{
			"looking-glass": map[string]any{"enabled": "false", "certificate": "still-mine"},
		},
	}
	require.Equal(t, "still-mine", lgCertificateName(treeFromMap(tree)),
		"a disabled block still states which certificate the operator wants served")

	require.Empty(t, lgCertificateName(nil))
	require.Empty(t, lgCertificateName(treeFromMap(map[string]any{})))
}

func TestReloadPlaintextLGKeepsCertificateInert(t *testing.T) {
	// VALIDATES: a looking glass running plaintext serves no certificate, so a
	// reload neither checks the name nor rotates anything.
	// PREVENTS: the two paths disagreeing about one config. Startup resolves the
	// name only when TLS is on (service_lg.go), so a daemon that boots this
	// config happily must not have its next reload refused over the same leaf --
	// and the lg server refuses a rotation it could never serve, which would
	// turn that disagreement into a rejected commit.
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	plaintext := map[string]any{
		"environment": map[string]any{
			"looking-glass": map[string]any{
				"enabled":     "true",
				"tls":         "false",
				"certificate": "absent-cert",
			},
		},
	}
	require.Empty(t, lgCertificateName(treeFromMap(plaintext)),
		"a plaintext looking glass presents no certificate, so it names none")

	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	cp := zeconfig.NewProvider()
	cp.SetRoot("pki", map[string]any{})

	fake := &fakeTLSUpdatable{}
	lm := &listenerMigrator{}
	lm.setLGTLS(fake)

	load := func() (map[string]any, *zeconfig.Tree, error) {
		return plaintext, treeFromMap(plaintext), nil
	}

	err = runReload(srv, cp, load, lm)
	require.Error(t, err, "this harness has no reactor, so the reload cannot complete")
	require.ErrorContains(t, err, "no reactor configured",
		"the reload must reach plugin apply, which means the certificate gate let it through")
	require.NotContains(t, err.Error(), "absent-cert",
		"a name a plaintext listener never reads must not refuse the commit")
	require.Zero(t, fake.calls, "nothing may be rotated onto a plaintext looking glass")
}

func TestReloadRejectsBrokenLGCertificateReference(t *testing.T) {
	// VALIDATES: AC-5 and R-5 -- a commit whose looking-glass certificate
	// reference does not resolve is rejected, and the PRIOR store is put back.
	// PREVENTS: a config edit silently downgrading a public looking glass to its
	// self-signed certificate at reload time, and the subtler half: a rejected
	// commit leaving the daemon serving the NEW material under the OLD config.
	//
	// Both stores define "shared" and the material behind it DIFFERS, so the
	// restore assertion cannot pass by the name merely surviving.
	t.Cleanup(func() { _ = zepki.Load(nil) })

	oldCA, oldCert, oldKey := caSignedB64(t, "old lg cert")
	newCA, newCert, newKey := caSignedB64(t, "new lg cert")

	priorTree := pkiLGTree(oldCA, oldCert, oldKey, "shared", map[string]any{"enabled": "true"})
	priorPKI, err := preparePKIConfig(treeFromMap(priorTree))
	require.NoError(t, err)
	require.NoError(t, zepki.Load(priorPKI))
	require.Equal(t, "old lg cert", zepki.CertCN("shared"), "precondition: the prior store is installed")

	newTree := pkiLGTree(newCA, newCert, newKey, "shared", map[string]any{
		"enabled":     "true",
		"certificate": "absent-cert",
	})

	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	cp := zeconfig.NewProvider()
	priorPKIRoot, ok := priorTree["pki"].(map[string]any)
	require.True(t, ok)
	cp.SetRoot("pki", priorPKIRoot)

	fake := &fakeTLSUpdatable{}
	lm := &listenerMigrator{}
	lm.setLGTLS(fake)

	load := func() (map[string]any, *zeconfig.Tree, error) {
		return newTree, treeFromMap(newTree), nil
	}

	err = runReload(srv, cp, load, lm)
	require.Error(t, err, "a reload naming an absent certificate must fail")
	require.Contains(t, err.Error(), "absent-cert")
	require.Contains(t, err.Error(), "environment.looking-glass.certificate")
	require.Zero(t, fake.calls, "nothing may be installed on the listener by a rejected reload")
	require.Equal(t, "old lg cert", zepki.CertCN("shared"),
		"the refused commit must leave the prior store installed, not the one it was rejected for")
}

func TestReloadRotatesLGCertificate(t *testing.T) {
	// VALIDATES: AC-6 -- the material the running looking glass is handed comes
	// from the commit being applied, with no rebind.
	// PREVENTS: a reload that installs a new store while the listener keeps
	// serving the chain it was built with, which looks healthy and presents an
	// identity the config no longer describes.
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	caB64, certB64, keyB64 := caSignedB64(t, "incoming lg leaf")
	incoming, err := preparePKIConfig(treeFromMap(pkiLGTree(caB64, certB64, keyB64, "lg-cert", map[string]any{
		"enabled":     "true",
		"certificate": "lg-cert",
	})))
	require.NoError(t, err)
	require.NoError(t, zepki.Load(incoming))

	fake := &fakeTLSUpdatable{}
	lm := &listenerMigrator{}
	lm.setLGTLS(fake)

	require.NoError(t, lm.updateLGCertificate("lg-cert"))
	require.Equal(t, 1, fake.calls)

	pair, err := tls.X509KeyPair(fake.certPEM, fake.keyPEM)
	require.NoError(t, err)
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	require.NoError(t, err)
	require.Equal(t, "incoming lg leaf", leaf.Subject.CommonName)
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
	incoming, err := preparePKIConfig(treeFromMap(pkiWebTree(caB64, certB64, keyB64, "fresh-cert", "fresh-cert")))
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

	priorPKI, err := preparePKIConfig(treeFromMap(pkiWebTree(oldCA, oldCert, oldKey, "rolled-back", "")))
	require.NoError(t, err)
	newPKI, err := preparePKIConfig(treeFromMap(pkiWebTree(newCA, newCert, newKey, "rolled-back", "")))
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
