//go:build ze_web

// Design: docs/architecture/web-interface.md -- standalone recovery uses the outer store owner.
package hub

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	storagecli "github.com/ze-software/ze/internal/component/config/storage/cli"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestWebOnlyFullRestoreKeepsTheOuterBoundary drives the same binding and stop
// verdict as standalone startup. Response completion only requests its existing
// stop loop; actual replacement waits for every clean owner, including Close.
func TestWebOnlyFullRestoreKeepsTheOuterBoundary(t *testing.T) {
	for _, failedCleanup := range []bool{false, true} {
		name := "clean restart"
		if failedCleanup {
			name = "failed cleanup"
		}
		t.Run(name, func(t *testing.T) {
			stopRestartCause.Store(nil)
			t.Cleanup(func() {
				stopRestartCause.Store(nil)
				storagecli.InstallDataRPC(nil)
			})
			dir := t.TempDir()
			live, err := storage.Create(dir)
			require.NoError(t, err)
			require.NoError(t, live.WriteKey("unknown/live", []byte("original")))
			owner, err := storage.NewDaemonRecovery(storage.BindConfigSource(live, "router.conf", storage.ConfigSourceStored))
			require.NoError(t, err)
			t.Cleanup(func() {
				if owner.Borrow() != nil {
					closed, closeErr := owner.Close()
					require.NoError(t, closeErr)
					require.NoError(t, closed.Discard())
				}
			})
			sourcePath := filepath.Join(t.TempDir(), "full.zefs")
			source, err := storage.CreateBlobPopulated(sourcePath, func(seed storage.Storage) error {
				return seed.WriteKey("unknown/restored", []byte{0, 255, 128, 1})
			}, false, zefs.Spare(0))
			require.NoError(t, err)
			require.NoError(t, source.Close())
			commands, dispatch := standaloneCommandFixture(t)
			stop := make(chan os.Signal, 2)
			bindWebOnlyRecovery(owner.Borrow(), commands, stop)
			if failedCleanup {
				_, err = commands.OnStop(func() error { return os.ErrPermission })
				require.NoError(t, err)
			}
			response, err := dispatch(t.Context(), plugin.CallerIdentity{Username: "alice"}, "request data restore path "+sourcePath+" full")
			require.NoError(t, err)
			require.Equal(t, plugin.StatusDone, response.Status, response.Error)
			data, ok := response.Data.(plugin.Map)
			require.True(t, ok)
			require.Equal(t, "accepted", data["state"])
			require.NotEmpty(t, data["digest"])
			select {
			case <-stop:
				t.Fatal("restart reached the stop loop before the response completed")
			default:
			}
			original, err := owner.Borrow().ReadKey("unknown/live")
			require.NoError(t, err)
			require.Equal(t, "original", string(original))
			response.TransportComplete()
			response.TransportComplete()
			request := <-stop
			require.IsType(t, requestedStop{}, request)
			select {
			case <-stop:
				t.Fatal("one accepted response requested restart twice")
			default:
			}
			original, err = owner.Borrow().ReadKey("unknown/live")
			require.NoError(t, err)
			require.Equal(t, "original", string(original))
			cleanupError := shutdownWebOnly(context.Background(), nil, commands)
			exit := webOnlyExitStatus(cleanupError)
			if failedCleanup {
				require.ErrorIs(t, cleanupError, os.ErrPermission)
				require.Equal(t, 1, exit)
			} else {
				require.NoError(t, cleanupError)
				require.Equal(t, rpc.DaemonRestartExitStatus, exit)
			}
			refused, err := dispatch(t.Context(), plugin.CallerIdentity{Username: "alice"}, "request data backup path "+filepath.Join(t.TempDir(), "after-stop.zefs"))
			require.NoError(t, err)
			require.Equal(t, plugin.StatusError, refused.Status)
			require.Contains(t, refused.Error, "serves no store")
			closed, err := owner.Close()
			require.NoError(t, err)
			if exit == rpc.DaemonRestartExitStatus {
				require.NoError(t, closed.Apply())
			} else {
				require.NoError(t, closed.Discard())
			}
			result, err := storage.Open(dir)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, result.Close()) })
			if failedCleanup {
				content, readErr := result.ReadKey("unknown/live")
				require.NoError(t, readErr)
				require.Equal(t, "original", string(content))
			} else {
				content, readErr := result.ReadKey("unknown/restored")
				require.NoError(t, readErr)
				require.Equal(t, []byte{0, 255, 128, 1}, content)
			}
		})
	}
}

// TestWebOnlyConfigRestoreRefusesAbsentReload protects the store-only binding:
// a native live-config operation cannot stage bytes then call a missing reload.
func TestWebOnlyConfigRestoreRefusesAbsentReload(t *testing.T) {
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.WriteFile("router.conf", []byte("original"), 0o600))
	storagecli.InstallDataRPC(&storagecli.DataRPCTarget{Store: store, ConfigPath: "router.conf"})
	t.Cleanup(func() { storagecli.InstallDataRPC(nil) })
	path := filepath.Join(t.TempDir(), "config.zefs")
	artifact, err := storage.CreateBlobPopulated(path, func(seed storage.Storage) error {
		return seed.WriteFile("router.conf", []byte("replacement"), 0o600)
	}, false, zefs.Spare(0))
	require.NoError(t, err)
	require.NoError(t, artifact.Close())
	_, dispatch := standaloneCommandFixture(t)
	var response *plugin.Response
	require.NotPanics(t, func() {
		response, err = dispatch(t.Context(), plugin.CallerIdentity{Username: "alice"}, "request data restore path "+path+" config")
	})
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, response.Status)
	require.Contains(t, response.Error, "reload")
	_, _, staged, err := storage.ReadCandidateConfig(store, "router.conf")
	require.NoError(t, err)
	require.False(t, staged)
	content, err := storage.ReadActiveConfig(store, "router.conf")
	require.NoError(t, err)
	require.Equal(t, "original", string(content))
}

// TestWebOnlyForceSignalOwnerEndsOnRelease proves the existing second-signal
// worker finishes when its owner releases it and ignores command stop values.
func TestWebOnlyForceSignalOwnerEndsOnRelease(t *testing.T) {
	for _, test := range []struct {
		name   string
		signal os.Signal
		forced bool
	}{{name: "released"}, {name: "command request", signal: requestedStop{}}, {name: "second OS signal", signal: syscall.SIGTERM, forced: true}} {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				previous := stopForceExit
				forced := false
				stopForceExit = func() { forced = true }
				t.Cleanup(func() { stopForceExit = previous })
				signals := make(chan os.Signal, 1)
				finished := make(chan struct{})
				go func() {
					defer close(finished)
					forceExitOnSignal(signals)
				}()
				if test.signal != nil {
					signals <- test.signal
				}
				synctest.Wait()
				close(signals)
				<-finished
				require.Equal(t, test.forced, forced)
			})
		})
	}
}
