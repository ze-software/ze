// VALIDATES: fleet-1 AC-8, AC-9, AC-26, AC-39, AC-41 through the live hub reload
// path (runReload -> prepareManagedReload -> ApplyManagedListeners).
// PREVENTS: managed listener declarations or certificates ignoring config reloads,
// or an invalid managed certificate leaving the PKI store altered on refusal.

package hub

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io/fs"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	zepki "github.com/ze-software/ze/internal/component/pki"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/zefs"
)

func reloadManagedHubConfig(t *testing.T, srv *pluginserver.Server, cp *zeconfig.Provider, lm *listenerMigrator, text string) error {
	t.Helper()
	tree, err := zeconfig.ParseTreeWithYANG(text, nil)
	require.NoError(t, err)
	load := func() (map[string]any, *zeconfig.Tree, reloadSelection, error) {
		return tree.ToPluginMap(), tree, nil, nil
	}
	return runReload(srv, cp, load, lm)
}

func dialManagedPresentedCert(t *testing.T, addr string) *x509.Certificate {
	t.Helper()
	conn, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS13}) //nolint:gosec // test inspects presented leaf directly
	require.NoError(t, err)
	defer conn.Close()
	state := conn.ConnectionState()
	require.NotEmpty(t, state.PeerCertificates)
	return state.PeerCertificates[0]
}

// VALIDATES: AC-39 -- a reload naming an undefined managed certificate, or
// certificate material that does not parse, is refused before any consumer
// applies, restores the prior PKI store, and returns a three-part operator error.
func TestReloadRefusesUnresolvableManagedCertificate(t *testing.T) {
	t.Cleanup(func() { _ = zepki.Load(nil) })
	oldCA, oldChain, oldCert, oldKey := chainB64(t, "old-fleet-hub", 1)
	initial := pkiChainConfig(oldCA, oldChain, oldCert, oldKey, "fleet-hub") + `
bus { hub {
	server central { ip 127.0.0.1; port 0; certificate fleet-hub; client edge-01 { secret edge-01-secret-at-least-32-characters; } }
} }`
	bootTree, err := zeconfig.ParseTreeWithYANG(initial, nil)
	require.NoError(t, err)
	bootPKI, err := preparePKIConfig(bootTree)
	require.NoError(t, err)
	require.NoError(t, zepki.Load(bootPKI))
	require.Equal(t, "old-fleet-hub", zepki.CertCN("fleet-hub"))

	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	hubCfg, err := zeconfig.ExtractHubConfig(bootTree)
	require.NoError(t, err)
	managed := startManagedServer(t.Context(), store, &hubCfg)
	require.NotNil(t, managed)
	t.Cleanup(managed.Stop)
	require.Len(t, managed.Addrs(), 1)
	addr := managed.Addrs()[0].String()
	initialLeaf := dialManagedPresentedCert(t, addr)

	srv, cp := reloadDriver(t, bootTree.ToPluginMap())
	priorPKIRoot, ok := bootTree.ToPluginMap()["pki"].(map[string]any)
	require.True(t, ok)
	cp.SetRoot("pki", priorPKIRoot)
	lm := newListenerMigrator()
	lm.setManaged(managed)

	// Undefined certificate name in candidate config that also changes fleet-hub in pki:
	// the refusal must restore the prior PKI store (old-fleet-hub, not new-fleet-hub).
	newCA, newChain, newCert, newKey := chainB64(t, "new-fleet-hub", 1)
	missingCfg := pkiChainConfig(newCA, newChain, newCert, newKey, "fleet-hub") + `
bus { hub {
	server central { ip 127.0.0.1; port 0; certificate missing-cert; client edge-01 { secret edge-01-secret-at-least-32-characters; } }
} }`
	err = reloadManagedHubConfig(t, srv, cp, lm, missingCfg)
	var typed *opererr.Error
	require.ErrorAs(t, err, &typed)
	require.Contains(t, typed.Problem(), "bus hub server central certificate missing-cert")
	require.NotEmpty(t, typed.Why())
	require.Equal(t, "edit pki certificate missing-cert", typed.Fix())

	// Prior PKI store was restored and the running listener keeps serving the original leaf.
	require.Equal(t, "old-fleet-hub", zepki.CertCN("fleet-hub"))
	require.Equal(t, initialLeaf.Raw, dialManagedPresentedCert(t, addr).Raw)

	// Unparseable material (certificate leaf without matching private key) is also refused
	// and keeps the prior PKI store and serving leaf intact.
	unparseableCfg := pkiChainConfig(newCA, newChain, newCert, oldKey, "fleet-hub") + `
bus { hub {
	server central { ip 127.0.0.1; port 0; certificate fleet-hub; client edge-01 { secret edge-01-secret-at-least-32-characters; } }
} }`
	err = reloadManagedHubConfig(t, srv, cp, lm, unparseableCfg)
	require.Error(t, err)
	require.Equal(t, "old-fleet-hub", zepki.CertCN("fleet-hub"))
	require.Equal(t, initialLeaf.Raw, dialManagedPresentedCert(t, addr).Raw)
}

// VALIDATES: AC-39 -- removing the certificate name on a serving hub when the
// local authority cannot issue the replacement leaf refuses the reload whole
// and leaves the running listener presenting the named chain.
func TestReloadRefusesUnissuableManagedLeafOnNameRemoval(t *testing.T) {
	t.Cleanup(func() { _ = zepki.Load(nil) })
	ca, chain, cert, key := chainB64(t, "fleet-hub", 1)
	initial := pkiChainConfig(ca, chain, cert, key, "fleet-hub") + `
bus { hub {
	server central { ip 127.0.0.1; port 0; certificate fleet-hub; client edge-01 { secret edge-01-secret-at-least-32-characters; } }
} }`
	bootTree, err := zeconfig.ParseTreeWithYANG(initial, nil)
	require.NoError(t, err)
	bootPKI, err := preparePKIConfig(bootTree)
	require.NoError(t, err)
	require.NoError(t, zepki.Load(bootPKI))

	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	// Corrupt local CA (cert present without key) so authority load fails.
	require.NoError(t, store.WriteFile(zefs.KeyCACert.Pattern, []byte("original root"), 0o600))

	hubCfg, err := zeconfig.ExtractHubConfig(bootTree)
	require.NoError(t, err)
	managed := startManagedServer(t.Context(), store, &hubCfg)
	require.NotNil(t, managed)
	t.Cleanup(managed.Stop)
	require.Len(t, managed.Addrs(), 1)
	addr := managed.Addrs()[0].String()
	initialLeaf := dialManagedPresentedCert(t, addr)

	srv, cp := reloadDriver(t, bootTree.ToPluginMap())
	priorPKIRoot, ok := bootTree.ToPluginMap()["pki"].(map[string]any)
	require.True(t, ok)
	cp.SetRoot("pki", priorPKIRoot)
	lm := newListenerMigrator()
	lm.setManaged(managed)

	// Candidate removes certificate leaf, asking for local CA issuance.
	removedCert := pkiChainConfig(ca, chain, cert, key, "fleet-hub") + `
bus { hub {
	server central { ip 127.0.0.1; port 0; client edge-01 { secret edge-01-secret-at-least-32-characters; } }
} }`
	err = reloadManagedHubConfig(t, srv, cp, lm, removedCert)
	var typed *opererr.Error
	require.ErrorAs(t, err, &typed)
	require.ErrorIs(t, err, fs.ErrNotExist)
	require.Equal(t, "set bus hub server central certificate fleet-hub", typed.Fix())
	require.Equal(t, initialLeaf.Raw, dialManagedPresentedCert(t, addr).Raw)
}

// VALIDATES: AC-41 -- a hub started with an undefined managed certificate starts
// with nothing bound, refuses a reload that still names the undefined certificate,
// and binds and serves the certificate once a reload defines it in pki.
func TestReloadRefusedWhileBootCertificateUnresolved(t *testing.T) {
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	bootCfg := &zePlugin.HubConfig{Servers: []zePlugin.HubServerConfig{{
		Name: "central", Host: "127.0.0.1", Certificate: "fleet-hub",
		Clients: map[string]string{"edge-01": "edge-01-secret-at-least-32-characters"},
	}}}
	managed := startManagedServer(t.Context(), store, bootCfg)
	require.NotNil(t, managed)
	t.Cleanup(managed.Stop)
	require.Empty(t, managed.Addrs())

	srv, cp := reloadDriver(t, map[string]any{})
	cp.SetRoot("pki", map[string]any{})
	lm := newListenerMigrator()
	lm.setManaged(managed)

	// Reload still naming the undefined certificate is refused.
	stillUnresolved := `bus { hub {
	server central { ip 127.0.0.1; port 0; certificate fleet-hub; client edge-01 { secret edge-01-secret-at-least-32-characters; } }
} }`
	err = reloadManagedHubConfig(t, srv, cp, lm, stillUnresolved)
	var typed *opererr.Error
	require.ErrorAs(t, err, &typed)
	require.Equal(t, "edit pki certificate fleet-hub", typed.Fix())
	require.Empty(t, managed.Addrs())

	// Reload defining fleet-hub in pki succeeds and binds the listener.
	ca, chain, cert, key := chainB64(t, "fleet-hub", 1)
	repaired := pkiChainConfig(ca, chain, cert, key, "fleet-hub") + stillUnresolved
	require.NoError(t, reloadManagedHubConfig(t, srv, cp, lm, repaired))
	require.Len(t, managed.Addrs(), 1)
	certPEM, _, err := zepki.ServerTLSMaterial("fleet-hub")
	require.NoError(t, err)
	wantBlock, _ := pem.Decode(certPEM)
	require.NotNil(t, wantBlock)
	require.Equal(t, wantBlock.Bytes, dialManagedPresentedCert(t, managed.Addrs()[0].String()).Raw)
}

// VALIDATES: AC-8, AC-9, AC-26 -- live reload on an idle hub binds a new serving
// block, adds clients, and removes a client entry while keeping its stored history.
func TestReloadAppliesManagedListenerClientsLive(t *testing.T) {
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.WriteFile(pluginserver.ClientConfigKey("edge-01"), []byte("cfg"), 0o600))

	managed := startManagedServer(t.Context(), store, &zePlugin.HubConfig{})
	require.NotNil(t, managed)
	t.Cleanup(managed.Stop)
	require.Empty(t, managed.Addrs())

	srv, cp := reloadDriver(t, map[string]any{})
	cp.SetRoot("pki", map[string]any{})
	lm := newListenerMigrator()
	lm.setManaged(managed)

	const secret = "edge-01-secret-at-least-32-characters"
	added := `bus { hub {
	server central { ip 127.0.0.1; port 0; client edge-01 { secret ` + secret + `; label site { value lon; } } }
} }`
	require.NoError(t, reloadManagedHubConfig(t, srv, cp, lm, added))
	require.Len(t, managed.Addrs(), 1)
	require.True(t, managed.Serves("edge-01"))

	client := dialHubClient(t, managed.Addrs()[0].String(), "edge-01", secret)
	defer client.Close()
	require.Eventually(t, func() bool {
		dev, devErr := pluginserver.ManagedDeviceByName("edge-01")
		return devErr == nil && dev.Connected && dev.Labels["site"] == "lon"
	}, 2*time.Second, 10*time.Millisecond)

	// Reload removing edge-01 closes its session and retains its history as removed.
	removed := `bus { hub { server central { ip 127.0.0.1; port 0; } } }`
	require.NoError(t, reloadManagedHubConfig(t, srv, cp, lm, removed))
	require.False(t, managed.Serves("edge-01"))
	dev, err := pluginserver.ManagedDeviceByName("edge-01")
	require.NoError(t, err)
	require.False(t, dev.Declared)
	require.False(t, dev.Connected)
	require.False(t, dev.RemovedAt.IsZero())
}
