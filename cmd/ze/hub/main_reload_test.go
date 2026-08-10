// Design: docs/architecture/hub-architecture.md -- SIGHUP config reload orchestration

package hub

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type reloadTestReactor struct {
	tree    map[string]any
	setTree map[string]any
}

type failingReconfigurable struct {
	addrs []string
	err   error
}

func (f *failingReconfigurable) Addresses() []string { return f.addrs }
func (f *failingReconfigurable) Reconfigure(context.Context, []string) error {
	return f.err
}

func (r *reloadTestReactor) Peers() []plugin.PeerInfo { return nil }
func (r *reloadTestReactor) Stats() plugin.ReactorStats {
	return plugin.ReactorStats{}
}
func (r *reloadTestReactor) GetPeerProcessBindings(netip.Addr) []plugin.PeerProcessBinding {
	return nil
}
func (r *reloadTestReactor) GetPeerCapabilityConfigs() []plugin.PeerCapabilityConfig { return nil }
func (r *reloadTestReactor) PeerNegotiatedCapabilities(netip.Addr) *plugin.PeerCapabilitiesInfo {
	return nil
}
func (r *reloadTestReactor) Stop()                                        {}
func (r *reloadTestReactor) TeardownPeer(netip.Addr, uint8, string) error { return nil }
func (r *reloadTestReactor) Reload() error                                { return nil }
func (r *reloadTestReactor) DrainPeerSync(_ context.Context) error        { return nil }
func (r *reloadTestReactor) VerifyConfig(map[string]any) error            { return nil }
func (r *reloadTestReactor) ApplyConfigDiff(map[string]any) error         { return nil }
func (r *reloadTestReactor) RemovePeer(netip.Addr) error                  { return nil }
func (r *reloadTestReactor) AddDynamicPeer(netip.Addr, map[string]any) error {
	return nil
}
func (r *reloadTestReactor) GetConfigTree() map[string]any { return r.tree }
func (r *reloadTestReactor) SetConfigTree(tree map[string]any) {
	r.setTree = tree
	r.tree = tree
}
func (r *reloadTestReactor) SignalAPIReady()                    {}
func (r *reloadTestReactor) AddAPIProcessCount(int)             {}
func (r *reloadTestReactor) SignalPluginStartupComplete()       {}
func (r *reloadTestReactor) SignalPeerAPIReady(string)          {}
func (m *reloadTestReactor) SetPeerUpBarrier(_ string, _ int)   {}
func (m *reloadTestReactor) SignalPeerUpBarrier(_ string)       {}
func (r *reloadTestReactor) PausePeer(netip.Addr) error         { return nil }
func (r *reloadTestReactor) ResumePeer(netip.Addr) error        { return nil }
func (r *reloadTestReactor) RegisterCacheConsumer(string, bool) {}
func (r *reloadTestReactor) UnregisterCacheConsumer(string)     {}
func (r *reloadTestReactor) FlushForwardPool(context.Context) error {
	return nil
}
func (r *reloadTestReactor) FlushForwardPoolPeer(context.Context, string) error {
	return nil
}
func (r *reloadTestReactor) ForwardUpdatesDirect([]uint64, []netip.AddrPort, string) error {
	return nil
}

// RelayStoredRoute satisfies plugin.ReactorRelayCoordinator; this stub relays
// nothing because these tests exercise SIGHUP reload, not the forward rail.
func (r *reloadTestReactor) RelayStoredRoute(netip.Addr, []rpc.StoredRoute) error {
	return nil
}
func (r *reloadTestReactor) ReleaseUpdates([]uint64, string) error { return nil }

func TestDoReloadPromotesCandidateOnSuccess(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewFilesystem()
	configPath := filepath.Join(dir, "router.conf")
	reactor := &reloadTestReactor{tree: map[string]any{"bgp": map[string]any{"router-id": "1.1.1.1"}}}
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
	require.NoError(t, err)
	t.Cleanup(func() { server.Stop() })

	oldStamp := "20260524-090000.000"
	require.NoError(t, store.WriteFile(configPath, []byte("active-file"), 0o600))
	require.NoError(t, store.WriteVersion(configPath, []byte("active-version"), mustParseReloadStamp(t, oldStamp)))
	require.NoError(t, storage.WritePointer(store, configPath, storage.PointerActive, oldStamp))
	newStamp, err := storage.WriteCandidateVersion(store, configPath, []byte("candidate-version"), mustParseReloadStamp(t, "20260524-100000.000"))
	require.NoError(t, err)

	load := func() (map[string]any, *zeconfig.Tree, error) {
		data, _, ok, readErr := storage.ReadCandidateConfig(store, configPath)
		require.NoError(t, readErr)
		require.True(t, ok, "doReload should load the staged candidate")
		require.Equal(t, "candidate-version", string(data))
		return map[string]any{"bgp": map[string]any{"router-id": "2.2.2.2"}}, nil, nil
	}

	// VALIDATES: AC-1 promotes candidate only after runtime reload succeeds.
	// PREVENTS: accepted candidate remaining transient after a successful commit.
	require.NoError(t, doReload(server, nil, nil, store, configPath, load, nil))

	active, ok, err := storage.ReadPointer(store, configPath, storage.PointerActive)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, newStamp, active)

	rollback, ok, err := storage.ReadPointer(store, configPath, storage.PointerRollback)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, oldStamp, rollback)

	_, ok, err = storage.ReadPointer(store, configPath, storage.PointerCandidate)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, map[string]any{"bgp": map[string]any{"router-id": "2.2.2.2"}}, reactor.setTree)
}

func TestDoReloadClearsCandidateOnFailure(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewFilesystem()
	configPath := filepath.Join(dir, "router.conf")
	reactor := &reloadTestReactor{tree: map[string]any{"bgp": map[string]any{"router-id": "1.1.1.1"}}}
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
	require.NoError(t, err)
	t.Cleanup(func() { server.Stop() })

	activeStamp := "20260524-090000.000"
	candidateStamp := "20260524-100000.000"
	require.NoError(t, store.WriteVersion(configPath, []byte("active"), mustParseReloadStamp(t, activeStamp)))
	require.NoError(t, storage.WritePointer(store, configPath, storage.PointerActive, activeStamp))
	require.NoError(t, store.WriteVersion(configPath, []byte("candidate"), mustParseReloadStamp(t, candidateStamp)))
	require.NoError(t, storage.WritePointer(store, configPath, storage.PointerCandidate, candidateStamp))

	load := func() (map[string]any, *zeconfig.Tree, error) {
		return nil, nil, fmt.Errorf("candidate parse failed")
	}

	// VALIDATES: AC-2 clears candidate after a failed reload and leaves active unchanged.
	// PREVENTS: failed candidate being applied by the next reload or boot.
	err = doReload(server, nil, nil, store, configPath, load, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "candidate parse failed")

	active, ok, err := storage.ReadPointer(store, configPath, storage.PointerActive)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, activeStamp, active)

	_, ok, err = storage.ReadPointer(store, configPath, storage.PointerCandidate)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestDoReloadRollsBackOnListenerMigrationFailure(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewFilesystem()
	configPath := filepath.Join(dir, "router.conf")
	oldTree := map[string]any{"bgp": map[string]any{"router-id": "1.1.1.1"}}
	newTree := map[string]any{"bgp": map[string]any{"router-id": "2.2.2.2"}}
	reactor := &reloadTestReactor{tree: oldTree}
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
	require.NoError(t, err)
	t.Cleanup(func() { server.Stop() })
	cp := zeconfig.NewProvider()
	cp.SetRoot("bgp", map[string]any{"router-id": "1.1.1.1"})

	oldStamp := "20260524-090000.000"
	newStamp := "20260524-100000.000"
	require.NoError(t, store.WriteVersion(configPath, []byte("old"), mustParseReloadStamp(t, oldStamp)))
	require.NoError(t, storage.WritePointer(store, configPath, storage.PointerActive, oldStamp))
	require.NoError(t, store.WriteVersion(configPath, []byte("new"), mustParseReloadStamp(t, newStamp)))
	require.NoError(t, storage.WritePointer(store, configPath, storage.PointerCandidate, newStamp))

	lm := newListenerMigrator(nil)
	lm.web = &failingReconfigurable{addrs: []string{"127.0.0.1:3443"}, err: fmt.Errorf("listener refused")}
	load := func() (map[string]any, *zeconfig.Tree, error) {
		return newTree, reloadWebTree("127.0.0.1", "3444"), nil
	}

	// VALIDATES: listener migration failure rolls runtime/provider back before rejecting the candidate.
	// PREVENTS: failed commits leaving plugin runtime or ConfigProvider on the rejected config.
	err = doReload(server, nil, cp, store, configPath, load, lm)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listener refused")

	active, ok, err := storage.ReadPointer(store, configPath, storage.PointerActive)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, oldStamp, active)
	_, ok, err = storage.ReadPointer(store, configPath, storage.PointerCandidate)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, oldTree, reactor.tree)
	providerRoot, err := cp.Get("bgp")
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1", providerRoot["router-id"])
}

func reloadWebTree(ip, port string) *zeconfig.Tree {
	tree := zeconfig.NewTree()
	env := zeconfig.NewTree()
	web := zeconfig.NewTree()
	web.Set("enabled", "true")
	srv := zeconfig.NewTree()
	srv.Set("ip", ip)
	srv.Set("port", port)
	web.AddListEntry("server", "main", srv)
	env.SetContainer("web", web)
	tree.SetContainer("environment", env)
	return tree
}

// VALIDATES: AC-11 -- SIGHUP daemon reload emits an audit record with actor, surface, and action.
// PREVENTS: Signal-triggered lifecycle operations bypassing the unified audit trail.
func TestRecordDaemonReloadAudit(t *testing.T) {
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)

	recordDaemonReloadAudit(recorder, "system", "signal", audit.System, "SIGHUP")

	entries := recorder.Query(audit.Filter{Action: audit.ActionDaemonReload})
	require.Len(t, entries, 1)
	assert.Equal(t, "system", entries[0].Actor)
	assert.Equal(t, "signal", entries[0].RemoteAddr)
	assert.Equal(t, audit.System, entries[0].Surface)
	assert.Equal(t, audit.OutcomeSuccess, entries[0].Outcome)
	assert.Equal(t, "SIGHUP", entries[0].Detail)
}

func TestStageSIGHUPCandidateRejectsExistingCandidate(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewFilesystem()
	configPath := filepath.Join(dir, "router.conf")
	require.NoError(t, store.WriteFile(configPath, []byte("edited-file"), 0o600))
	_, err := storage.WriteCandidateVersion(store, configPath, []byte("in-flight"), mustParseReloadStamp(t, "20260524-100000.000"))
	require.NoError(t, err)

	// VALIDATES: AC-13 rejects a second staged commit instead of overwriting or reusing it.
	// PREVENTS: SIGHUP applying another entry point's already staged candidate.
	err = stageSIGHUPCandidate(store, configPath)
	require.Error(t, err)
	assert.True(t, errors.Is(err, storage.ErrCandidateExists))

	data, _, ok, readErr := storage.ReadCandidateConfig(store, configPath)
	require.NoError(t, readErr)
	require.True(t, ok)
	assert.Equal(t, "in-flight", string(data))
}

func TestClearStaleCandidateOnBoot(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewFilesystem()
	configPath := filepath.Join(dir, "router.conf")
	activeStamp := "20260524-090000.000"

	require.NoError(t, store.WriteVersion(configPath, []byte("active"), mustParseReloadStamp(t, activeStamp)))
	require.NoError(t, storage.WritePointer(store, configPath, storage.PointerActive, activeStamp))
	_, err := storage.WriteCandidateVersion(store, configPath, []byte("stale"), mustParseReloadStamp(t, "20260524-100000.000"))
	require.NoError(t, err)

	// VALIDATES: AC-12 boot cleanup leaves active authoritative and clears stale candidate state.
	// PREVENTS: a crash-left candidate blocking the next transactional commit.
	require.NoError(t, clearStaleCandidateOnBoot(store, configPath))

	_, ok, err := storage.ReadPointer(store, configPath, storage.PointerCandidate)
	require.NoError(t, err)
	assert.False(t, ok)

	data, err := storage.ReadActiveConfig(store, configPath)
	require.NoError(t, err)
	assert.Equal(t, "active", string(data))
}

func TestEnsureActivePointerProtectsFailedFirstSIGHUP(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewFilesystem()
	configPath := filepath.Join(dir, "router.conf")
	reactor := &reloadTestReactor{tree: map[string]any{"bgp": map[string]any{"router-id": "1.1.1.1"}}}
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
	require.NoError(t, err)
	t.Cleanup(func() { server.Stop() })

	require.NoError(t, store.WriteFile(configPath, []byte("bad edited file"), 0o600))
	_, wrote, err := storage.EnsureActiveVersion(store, configPath, []byte("known good"), mustParseReloadStamp(t, "20260524-090000.000"))
	require.NoError(t, err)
	require.True(t, wrote)
	_, err = storage.WriteCandidateVersion(store, configPath, []byte("bad edited file"), mustParseReloadStamp(t, "20260524-100000.000"))
	require.NoError(t, err)

	load := func() (map[string]any, *zeconfig.Tree, error) {
		return nil, nil, fmt.Errorf("candidate parse failed")
	}

	// VALIDATES: AC-8 and AC-12 keep active pointer on the known-good version after failed SIGHUP.
	// PREVENTS: first failed SIGHUP making the next boot load the bad edited config file.
	err = doReload(server, nil, nil, store, configPath, load, nil)
	require.Error(t, err)

	data, err := storage.ReadActiveConfig(store, configPath)
	require.NoError(t, err)
	assert.Equal(t, "known good", string(data))

	_, ok, err := storage.ReadPointer(store, configPath, storage.PointerCandidate)
	require.NoError(t, err)
	assert.False(t, ok)
}

func mustParseReloadStamp(t *testing.T, stamp string) time.Time {
	t.Helper()
	parsed, err := storage.ParseVersionStamp(stamp)
	require.NoError(t, err)
	return parsed
}
