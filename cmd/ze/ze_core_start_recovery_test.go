//go:build ze_core

// Design: docs/architecture/hub-architecture.md -- restore after every daemon borrower returns.
package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

func ownedDaemonRecoveryFixture(t *testing.T) (storage.Storage, string, string) {
	t.Helper()
	dir := t.TempDir()
	live, err := storage.Create(dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, live.Close()) })
	require.NoError(t, live.WriteKey("meta/test/current", []byte("original")))
	path := filepath.Join(t.TempDir(), "backup.zefs")
	artifact, err := storage.CreateBlob(path, zefs.Spare(0))
	require.NoError(t, err)
	require.NoError(t, artifact.WriteKey("meta/test/restored", []byte("replacement")))
	require.NoError(t, artifact.Close())
	return storage.BindConfigSource(live, "router.conf", storage.ConfigSourceStored), dir, path
}

// TestOwnedDaemonRecoveryWaitsForBorrowerDefers proves the launcher releases
// the live store after the runtime's deferred writes, then applies its accepted
// recovery and retains those writes in the retired tree.
func TestOwnedDaemonRecoveryWaitsForBorrowerDefers(t *testing.T) {
	source, dir, path := ownedDaemonRecoveryFixture(t)
	code := runOwnedDaemonStore(source, func(borrowed storage.Storage) int {
		admission := storage.RecoveryFor(borrowed)
		require.NotNil(t, admission)
		defer func() { require.NoError(t, borrowed.WriteKey("meta/test/drained", []byte("last borrower finished"))) }()
		_, err := admission.StagePath(context.Background(), path)
		require.NoError(t, err)
		return rpc.DaemonRestartExitStatus
	})
	require.Equal(t, rpc.DaemonRestartExitStatus, code)
	restored, err := storage.Open(dir)
	require.NoError(t, err)
	data, err := restored.ReadKey("meta/test/restored")
	require.NoError(t, err)
	require.Equal(t, "replacement", string(data))
	require.NoError(t, restored.Close())
	retired, err := filepath.Glob(filepath.Join(dir, storage.TreeName+".replaced-*"))
	require.NoError(t, err)
	require.Len(t, retired, 1)
	frame, err := os.ReadFile(filepath.Join(retired[0], "meta", "test", "drained"))
	require.NoError(t, err)
	drained, _, next, err := zefs.DecodeNetcapstringRef(frame, 0)
	require.NoError(t, err)
	require.Equal(t, len(frame), next)
	require.Equal(t, "last borrower finished", string(drained))
}

// TestOwnedDaemonRecoveryDiscardsAfterUncleanExit proves a failed runtime
// cannot install its accepted snapshot. Method: stage, return failure, reopen
// the unchanged store and verify the private staging directory was removed.
func TestOwnedDaemonRecoveryDiscardsAfterUncleanExit(t *testing.T) {
	source, dir, path := ownedDaemonRecoveryFixture(t)
	code := runOwnedDaemonStore(source, func(borrowed storage.Storage) int {
		_, err := storage.RecoveryFor(borrowed).StagePath(context.Background(), path)
		require.NoError(t, err)
		return 1
	})
	require.Equal(t, 1, code)
	reopened, err := storage.Open(dir)
	require.NoError(t, err)
	data, err := reopened.ReadKey("meta/test/current")
	require.NoError(t, err)
	require.Equal(t, "original", string(data))
	_, err = reopened.ReadKey("meta/test/restored")
	require.ErrorIs(t, err, os.ErrNotExist)
	require.NoError(t, reopened.Close())
	stages, err := filepath.Glob(filepath.Join(dir, "database.recovery-tmp-*"))
	require.NoError(t, err)
	require.Empty(t, stages)
}

// TestOwnedDaemonRecoveryKeepsTheRecoveryCommand proves a second cleanup
// failure cannot move the accepted source's manual recovery command from the
// final diagnostic line. Method: render the joined constructor/close outcome.
func TestOwnedDaemonRecoveryKeepsTheRecoveryCommand(t *testing.T) {
	output, err := os.CreateTemp(t.TempDir(), "stderr")
	require.NoError(t, err)
	previous := os.Stderr
	os.Stderr = output
	t.Cleanup(func() { os.Stderr = previous })
	const fix = "ze data restore /private/source.zefs full"
	primary := opererr.New("the verified source remains", "the import did not complete", fix)
	code := daemonStoreFailure(errors.Join(primary, errors.New("the store handle could not close")))
	require.Equal(t, 1, code)
	require.NoError(t, output.Close())
	content, err := os.ReadFile(output.Name())
	require.NoError(t, err)
	require.Contains(t, string(content), "the verified source remains")
	require.Contains(t, string(content), "the store handle could not close")
	require.True(t, strings.HasSuffix(string(content), "fix: "+fix+"\n"), string(content))
}
