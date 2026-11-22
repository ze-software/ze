// Design: docs/architecture/hub-architecture.md -- reload host mutation deferral and SIGHUP worker shutdown
// Related: main_reload.go -- runReloadContext, handleSIGHUPReload

package hub

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	zepki "github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestRunReloadDefersHostMutationsUntilAfterPromotion proves that host-side
// reload effects (such as applyResolvConf) run only after candidate promotion
// succeeds, never before.
//
// VALIDATES: a failed promoteReloadCandidate leaves host-side state (resolv.conf)
// on the accepted configuration instead of the rejected candidate.
// PREVENTS: a rolled-back commit mutating host state (resolv.conf, tuning,
// hostname, schedulers) before promotion confirms acceptance.
func TestRunReloadDefersHostMutationsUntilAfterPromotion(t *testing.T) {
	keepPublishedNameServers(t)
	resetAAABundleForTest(t)
	clearAPIEnv(t)
	t.Cleanup(func() {
		stopArchiveScheduler()
		stopSmartManager()
		_ = zepki.Load(nil)
	})
	require.NoError(t, zepki.Load(nil))

	dir := t.TempDir()
	resolvPath := filepath.Join(dir, "resolv.conf")
	running := nameServerTree(dir, "192.0.2.1")
	srv, cp := reloadDriver(t, running.ToPluginMap())
	cp.SetRoot("system", map[string]any{"authentication": map[string]any{}})
	resolveCandidate := liveLocalUsers(nil, func() ([]authz.UserConfig, error) {
		return liveConfigUsers(cp)
	}, nil)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, nil, resolveCandidate, "accepted-token"))

	configPath := filepath.Join(dir, "router.conf")
	store := newTestStore(t, dir)
	oldStamp := "20261017-090000.000"
	require.NoError(t, store.WriteFile(configPath, []byte("active-file"), 0o600))
	require.NoError(t, store.WriteVersion(configPath, []byte("active-version"), mustParseReloadStamp(t, oldStamp)))
	setConfigPointer(t, store, configPath, zefs.KeyConfigActive, oldStamp)
	loadTree := func(tree *zeconfig.Tree) func() (map[string]any, *zeconfig.Tree, error) {
		return func() (map[string]any, *zeconfig.Tree, error) {
			return tree.ToPluginMap(), tree, nil
		}
	}

	// Accepted reload promotes and applies resolv.conf.
	_, err := storage.WriteCandidateVersion(store, configPath, []byte("accepted"), mustParseReloadStamp(t, "20261017-100000.000"))
	require.NoError(t, err)
	err = runReloadContext(t.Context(), reloadSourceCommit, srv, nil, cp, store, configPath,
		loadTree(nameServerTree(dir, "198.51.100.1")), nil)
	require.NoError(t, err)
	written, err := os.ReadFile(resolvPath)
	require.NoError(t, err, "accepted reload must write resolv.conf")
	assert.Contains(t, string(written), "nameserver 198.51.100.1")

	// Failed promotion rolls back and must NOT overwrite resolv.conf with candidate.
	_, err = storage.WriteCandidateVersion(store, configPath, []byte("refused"), mustParseReloadStamp(t, "20261017-110000.000"))
	require.NoError(t, err)
	err = runReloadContext(t.Context(), reloadSourceCommit, srv, nil, cp, failAcquireStorage{Storage: store}, configPath,
		loadTree(nameServerTree(dir, "203.0.113.99")), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "promote candidate")

	afterFailure, err := os.ReadFile(resolvPath)
	require.NoError(t, err)
	assert.Contains(t, string(afterFailure), "nameserver 198.51.100.1", "accepted resolver entry must remain after failed promotion")
	assert.NotContains(t, string(afterFailure), "203.0.113.99", "rejected candidate must not mutate resolv.conf before promotion")
}

// TestHandleSIGHUPReloadExitsOnContextCancelAndChannelClose proves that
// handleSIGHUPReload terminates cleanly and closes done when either its context
// is canceled or reloadCh is closed, preventing goroutine leaks on early
// startup failure or normal shutdown.
//
// VALIDATES: handleSIGHUPReload selects on ctx.Done() as well as reloadCh.
// PREVENTS: an idle SIGHUP worker leaking when startup fails before waitLoop
// closes reloadCh.
func TestHandleSIGHUPReloadExitsOnContextCancelAndChannelClose(t *testing.T) {
	t.Run("context cancel while idle", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		reloadCh := make(chan os.Signal, 1)
		done := make(chan struct{})
		var loads atomic.Int32
		load := func() (map[string]any, *zeconfig.Tree, error) {
			loads.Add(1)
			return nil, nil, nil
		}

		go handleSIGHUPReload(ctx, reloadCh, done, &sync.Mutex{}, nil, nil, nil, nil, "", load, nil, nil)
		cancel()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("handleSIGHUPReload did not exit after context cancellation")
		}
		assert.Equal(t, int32(0), loads.Load())
	})

	t.Run("channel close while context live", func(t *testing.T) {
		reloadCh := make(chan os.Signal, 1)
		done := make(chan struct{})
		var loads atomic.Int32
		load := func() (map[string]any, *zeconfig.Tree, error) {
			loads.Add(1)
			return nil, nil, nil
		}

		go handleSIGHUPReload(t.Context(), reloadCh, done, &sync.Mutex{}, nil, nil, nil, nil, "", load, nil, nil)
		close(reloadCh)

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("handleSIGHUPReload did not exit after reloadCh close")
		}
		assert.Equal(t, int32(0), loads.Load())
	})

	t.Run("canceled context ignores queued signal", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		reloadCh := make(chan os.Signal, 1)
		reloadCh <- syscall.SIGHUP
		done := make(chan struct{})
		var loads atomic.Int32
		load := func() (map[string]any, *zeconfig.Tree, error) {
			loads.Add(1)
			return nil, nil, nil
		}

		go handleSIGHUPReload(ctx, reloadCh, done, &sync.Mutex{}, nil, nil, nil, nil, "", load, nil, nil)

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("handleSIGHUPReload did not exit with canceled context")
		}
		assert.Equal(t, int32(0), loads.Load())
	})
}
