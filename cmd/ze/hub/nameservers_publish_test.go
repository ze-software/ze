// Design: docs/architecture/diagnostics/path-mtu.md -- the reference address defaults to the first name server
// Related: main_system.go -- publishNameServers, the publication these tests drive
// Related: main_reload.go -- runReloadContext, which publishes only once the candidate is promoted

package hub

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/system"
	zepki "github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/pkg/zefs"
)

// keepPublishedNameServers saves the process-wide name-server publication and
// puts it back when the test ends, so no other test in this binary reads the
// servers these tests publish. A list that was never published cannot be
// unpublished, so it is restored as an empty publication: no reader in this
// package tells the two apart.
func keepPublishedNameServers(t *testing.T) {
	t.Helper()
	saved, _ := system.NameServers()
	t.Cleanup(func() { system.PublishNameServers(saved) })
}

// nameServerTree builds the config tree a daemon runs with the given system
// name servers. resolv-conf-path points under dir, so the resolv.conf write the
// reload performs beside the publication never reaches the host's file.
func nameServerTree(dir string, servers ...string) *zeconfig.Tree {
	dns := zeconfig.NewTree()
	dns.Set("resolv-conf-path", filepath.Join(dir, "resolv.conf"))
	sys := zeconfig.NewTree()
	sys.SetContainer("dns", dns)
	sys.SetContainer("authentication", zeconfig.NewTree())
	if len(servers) > 0 {
		sys.SetSlice("name-server", servers)
	}
	tree := zeconfig.NewTree()
	tree.SetContainer("system", sys)
	return tree
}

// requirePublishedNameServers asserts the published list and that a list was
// published at all, which is a different answer from an empty one.
func requirePublishedNameServers(t *testing.T, want []string, msg string) {
	t.Helper()
	servers, ok := system.NameServers()
	require.True(t, ok, "%s: the name servers must be published", msg)
	assert.Equal(t, want, servers, msg)
}

// VALIDATES: startup publishes the name servers of the loaded tree, in config
// order, and a nil tree keeps the list already published.
// PREVENTS: show mtu finding the list unpublished at startup, or a load that
// carries no tree erasing the running config's servers.
func TestPublishNameServersAtStartup(t *testing.T) {
	keepPublishedNameServers(t)

	publishNameServers(nameServerTree(t.TempDir(), "192.0.2.1", "192.0.2.2"))
	requirePublishedNameServers(t, []string{"192.0.2.1", "192.0.2.2"}, "startup")

	publishNameServers(nil)
	requirePublishedNameServers(t, []string{"192.0.2.1", "192.0.2.2"}, "a nil tree")
}

// VALIDATES: a reload publishes the name servers of the config it accepted,
// whether or not a candidate was promoted; a reload whose promotion fails keeps
// the list the running config holds; an accepted reload that drops the
// leaf-list clears it and still reports a publication.
// PREVENTS: the list going live before promotion, so a rolled-back commit
// leaves show mtu measuring a server the running config does not name.
// METHOD: drive runReloadContext three times over one server: a candidate
// that promotes, a candidate whose store refuses the promotion lock, and a
// reload with no candidate at all.
func TestRunReloadPublishesNameServersOnlyWhenAccepted(t *testing.T) {
	keepPublishedNameServers(t)
	resetAAABundleForTest(t)
	clearAPIEnv(t)
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))

	dir := t.TempDir()
	running := nameServerTree(dir, "192.0.2.1", "192.0.2.2")
	srv, cp := reloadDriver(t, running.ToPluginMap())
	cp.SetRoot("system", map[string]any{"authentication": map[string]any{}})
	resolveCandidate := liveLocalUsers(nil, func() ([]authz.UserConfig, error) {
		return liveConfigUsers(cp)
	}, nil)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, nil, resolveCandidate, "accepted-token"))
	publishNameServers(running)
	requirePublishedNameServers(t, []string{"192.0.2.1", "192.0.2.2"}, "startup")

	configPath := filepath.Join(dir, "router.conf")
	store := newTestStore(t, dir)
	oldStamp := "20261017-090000.000"
	require.NoError(t, store.WriteFile(configPath, []byte("active-file"), 0o600))
	require.NoError(t, store.WriteVersion(configPath, []byte("active-version"), mustParseReloadStamp(t, oldStamp)))
	setConfigPointer(t, store, configPath, zefs.KeyConfigActive, oldStamp)
	loadTree := func(tree *zeconfig.Tree) func() (map[string]any, *zeconfig.Tree, error) {
		return func() (map[string]any, *zeconfig.Tree, error) {
			lowered := tree.ToPluginMap()
			return lowered, tree, nil
		}
	}

	// An accepted commit: the candidate promotes and its servers go live.
	_, err := storage.WriteCandidateVersion(store, configPath, []byte("accepted"), mustParseReloadStamp(t, "20261017-100000.000"))
	require.NoError(t, err)
	err = runReloadContext(t.Context(), reloadSourceCommit, srv, nil, cp, store, configPath,
		loadTree(nameServerTree(dir, "198.51.100.1")), nil)
	require.NoError(t, err)
	requirePublishedNameServers(t, []string{"198.51.100.1"}, "an accepted reload")

	// A refused promotion: the reload rolls back, so the running servers stay.
	_, err = storage.WriteCandidateVersion(store, configPath, []byte("refused"), mustParseReloadStamp(t, "20261017-110000.000"))
	require.NoError(t, err)
	err = runReloadContext(t.Context(), reloadSourceCommit, srv, nil, cp, failAcquireStorage{Storage: store}, configPath,
		loadTree(nameServerTree(dir, "203.0.113.1")), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "promote candidate")
	requirePublishedNameServers(t, []string{"198.51.100.1"}, "a reload whose promotion failed")

	// An accepted reload with no candidate and no name-server clears the list.
	err = runReloadContext(t.Context(), reloadSourceCommit, srv, nil, cp, nil, "",
		loadTree(nameServerTree(dir)), nil)
	require.NoError(t, err)
	requirePublishedNameServers(t, nil, "an accepted reload with no name-server")
}
