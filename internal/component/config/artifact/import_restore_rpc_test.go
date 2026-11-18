// Design: docs/architecture/storage-backends.md -- import and selected restore share native publication.
package artifact_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	storagecli "github.com/ze-software/ze/internal/component/config/storage/cli"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestArtifactConfigImportSharesTheNativePublication proves both new raw
// configuration imports and existing pointer-backed imports use the same
// semantics as the actual standalone --path adapter.
func TestArtifactConfigImportSharesTheNativePublication(t *testing.T) {
	fixture := newPolicyHTTP(t, nil, false)
	id := artifactOperationUpload(t, fixture)
	require.NoError(t, fixture.owner.ChangeStore(t.Context(), "alice", id, func(target storage.Storage) error {
		_, err := storage.RestoreConfig(target, "router.conf", []byte("authoritative original"))
		if err != nil {
			return err
		}
		return target.WriteFile("router.conf", []byte("stale raw mirror"), 0o600)
	}))
	var priorVersions []storage.VersionInfo
	priorBytes := make(map[string][]byte)
	require.NoError(t, fixture.owner.ViewStore(t.Context(), "alice", id, func(source storage.Storage) error {
		var err error
		priorVersions, err = source.ListVersions("router.conf")
		if err != nil {
			return err
		}
		for _, version := range priorVersions {
			priorBytes[version.Stamp], err = source.ReadVersion("router.conf", version.Stamp)
			if err != nil {
				return err
			}
		}
		return nil
	}))
	for _, name := range []string{"new.conf", "router.conf"} {
		content := []byte("unvalidated imported bytes\n\x00\xff")
		input := artifactBinarySource(t, fixture, content)
		result := artifactOperationData(t, fixture, "request config artifact import artifact "+id+" name "+name+" transfer "+input)
		require.Equal(t, name, result["name"])
		require.EqualValues(t, len(content), result["bytes"])
		require.NoError(t, fixture.owner.ViewStore(t.Context(), "alice", id, func(source storage.Storage) error {
			got, err := storage.ReadActiveConfig(source, name)
			if err != nil {
				return err
			}
			require.Equal(t, content, got)
			if name == "router.conf" {
				versions, err := source.ListVersions(name)
				if err != nil {
					return err
				}
				require.Len(t, versions, len(priorVersions)+1)
				for _, prior := range priorVersions {
					require.Contains(t, versions, prior)
					retained, err := source.ReadVersion(name, prior.Stamp)
					if err != nil {
						return err
					}
					require.Equal(t, priorBytes[prior.Stamp], retained)
				}
			}
			binary, err := source.ReadKey("unknown/binary")
			if err != nil {
				return err
			}
			require.Equal(t, []byte{0, 255, 128, 0, 1}, binary)
			return nil
		}))

		path := filepath.Join(t.TempDir(), name)
		require.NoError(t, os.WriteFile(path, content, 0o600))
		localPath := filepath.Join(t.TempDir(), "local.zefs")
		local, err := storage.CreateBlobPopulated(localPath, func(source storage.Storage) error {
			if name == "router.conf" {
				_, err := storage.RestoreConfig(source, name, []byte("authoritative original"))
				return err
			}
			return nil
		}, false, zefs.Spare(0))
		require.NoError(t, err)
		localVersions, err := local.ListVersions(name)
		require.NoError(t, err)
		localBytes := make(map[string][]byte)
		for _, version := range localVersions {
			localBytes[version.Stamp], err = local.ReadVersion(name, version.Stamp)
			require.NoError(t, err)
		}
		require.NoError(t, local.Close())
		require.Zero(t, storagecli.Run([]string{"--path", localPath, "import", path}))
		local, err = storage.OpenBlob(localPath, false)
		require.NoError(t, err)
		got, err := storage.ReadActiveConfig(local, name)
		require.NoError(t, err)
		require.Equal(t, content, got)
		if name == "router.conf" {
			versions, err := local.ListVersions(name)
			require.NoError(t, err)
			require.Len(t, versions, len(localVersions)+1)
			for _, prior := range localVersions {
				require.Contains(t, versions, prior)
				retained, err := local.ReadVersion(name, prior.Stamp)
				require.NoError(t, err)
				require.Equal(t, localBytes[prior.Stamp], retained)
			}
		}
		require.NoError(t, local.Close())
	}
}

// TestArtifactConfigImportPreservesPendingAndOwnership proves a refused
// import cannot replace saved candidate state or consume another account's
// completed private source.
func TestArtifactConfigImportPreservesPendingAndOwnership(t *testing.T) {
	fixture := newPolicyHTTP(t, nil, false)
	id := artifactOperationUpload(t, fixture)
	input := artifactBinarySource(t, fixture, []byte("replacement"))
	require.NoError(t, fixture.owner.ChangeStore(t.Context(), "alice", id, func(target storage.Storage) error {
		_, err := storage.RestoreConfig(target, "router.conf", []byte("current"))
		if err != nil {
			return err
		}
		_, err = storage.WriteCandidateVersion(target, "router.conf", []byte("pending other edit"), time.Now())
		return err
	}))
	before, err := fixture.owner.Inspect(t.Context(), "alice", id)
	require.NoError(t, err)
	response := artifactOperationCall(t, fixture, "request config artifact import artifact "+id+" name router.conf transfer "+input)
	require.Equal(t, plugin.StatusError, response.Status)
	require.Contains(t, response.Error, "staged")
	after, err := fixture.owner.Inspect(t.Context(), "alice", id)
	require.NoError(t, err)
	require.Equal(t, before.Digest, after.Digest)
	response, err = fixture.dispatch(t.Context(), plugin.CallerIdentity{Username: "bob"}, "request config artifact import artifact "+id+" name new.conf transfer "+input)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, response.Status)
	after, err = fixture.owner.Inspect(t.Context(), "alice", id)
	require.NoError(t, err)
	require.Equal(t, before.Digest, after.Digest)
}

func selectedRestoreTarget(t *testing.T) (storage.Storage, *storagecli.DataRPCTarget) {
	t.Helper()
	store := storage.CreateMemory()
	_, err := storage.RestoreConfig(store, "device.conf", []byte("original runtime"))
	require.NoError(t, err)
	target := &storagecli.DataRPCTarget{Store: store, ConfigPath: "device.conf"}
	storagecli.InstallDataRPC(target)
	t.Cleanup(func() { storagecli.InstallDataRPC(nil); require.NoError(t, store.Close()) })
	return store, target
}

// TestArtifactSelectedRestoreUsesNativeReload proves selection is complete
// and source ownership is released before the live owner's normal reload.
func TestArtifactSelectedRestoreUsesNativeReload(t *testing.T) {
	fixture := newPolicyHTTP(t, nil, false)
	id := artifactOperationUpload(t, fixture)
	store, target := selectedRestoreTarget(t)
	calls := 0
	target.Reload = func(context.Context) error {
		calls++
		current, err := storage.ReadActiveConfig(store, "device.conf")
		if err != nil {
			return err
		}
		require.Equal(t, "original runtime", string(current))
		candidate, _, present, err := storage.ReadCandidateConfig(store, "device.conf")
		if err != nil {
			return err
		}
		require.True(t, present)
		require.Equal(t, policyConfig, string(candidate))
		_, err = fixture.owner.List(t.Context(), "alice")
		if err != nil {
			return err
		}
		return storage.PromoteCandidate(store, "device.conf")
	}
	before, err := fixture.owner.Inspect(t.Context(), "alice", id)
	require.NoError(t, err)
	answer := artifactOperationData(t, fixture, "request config artifact restore config artifact "+id+" name router.conf")
	require.Equal(t, "router.conf", answer["source-name"])
	require.Equal(t, "device.conf", answer["config-name"])
	require.NotEmpty(t, answer["version"])
	require.Equal(t, 1, calls)
	current, err := storage.ReadActiveConfig(store, "device.conf")
	require.NoError(t, err)
	require.Equal(t, policyConfig, string(current))
	after, err := fixture.owner.Inspect(t.Context(), "alice", id)
	require.NoError(t, err)
	require.Equal(t, before.Digest, after.Digest)
}

// TestArtifactSelectedRestoreRefusesUnsafeTargets preserves selection,
// pending-state, reload, window and managed-client policy at the native owner.
func TestArtifactSelectedRestoreRefusesUnsafeTargets(t *testing.T) {
	for _, state := range []string{"ambiguous", "pending", "no reload", "reload failure", "window", "unserved client"} {
		t.Run(state, func(t *testing.T) {
			fixture := newPolicyHTTP(t, nil, false)
			id := artifactOperationUpload(t, fixture)
			store, target := selectedRestoreTarget(t)
			calls := 0
			target.Reload = func(context.Context) error { calls++; return storage.PromoteCandidate(store, "device.conf") }
			line := "request config artifact restore config artifact " + id + " name router.conf"
			switch state {
			case "ambiguous":
				line = "request config artifact restore config artifact " + id
			case "pending":
				_, err := storage.WriteCandidateVersion(store, "device.conf", []byte("pending"), time.Now())
				require.NoError(t, err)
			case "no reload":
				target.Reload = nil
			case "reload failure":
				target.Reload = func(context.Context) error {
					calls++
					require.NoError(t, storage.ClearCandidate(store, "device.conf"))
					return errors.New("native reload refusal")
				}
			case "window":
				window := confirm.NewWindow(func([]byte) error { return nil }, confirm.NewStoreRecorder(store, "device.conf"))
				t.Cleanup(window.Stop)
				require.NoError(t, window.Confirmed("window-owner", time.Minute, false, confirm.Commit{Snapshot: func() ([]byte, error) { return storage.ReadActiveConfig(store, "device.conf") }, Apply: func() error { return nil }}))
				target.Window = func() *confirm.Window { return window }
			case "unserved client":
				line += " client edge-a"
			}
			response := artifactOperationCall(t, fixture, line)
			require.Equal(t, plugin.StatusError, response.Status)
			if state != "reload failure" {
				require.Zero(t, calls)
			}
			current, err := storage.ReadActiveConfig(store, "device.conf")
			require.NoError(t, err)
			require.Equal(t, "original runtime", string(current))
		})
	}
}

// TestArtifactSelectedRestoreUsesTheNativeManagedTarget proves a serving hub
// writes the selected client version without reloading its own configuration.
func TestArtifactSelectedRestoreUsesTheNativeManagedTarget(t *testing.T) {
	fixture := newPolicyHTTP(t, nil, false)
	id := artifactOperationUpload(t, fixture)
	store, target := selectedRestoreTarget(t)
	clientName := pluginserver.ClientConfigKey("edge-a")
	_, err := storage.RestoreConfig(store, clientName, []byte("client original"))
	require.NoError(t, err)
	target.ServesClient = func(client string) bool { return client == "edge-a" }
	target.Reload = func(context.Context) error { t.Fatal("client restore reloaded its hub"); return nil }
	answer := artifactOperationData(t, fixture, "request config artifact restore config artifact "+id+" name router.conf client edge-a")
	require.Equal(t, clientName, answer["config-name"])
	require.Equal(t, "edge-a", answer["client"])
	current, err := storage.ReadActiveConfig(store, clientName)
	require.NoError(t, err)
	require.Equal(t, policyConfig, string(current))
	current, err = storage.ReadActiveConfig(store, "device.conf")
	require.NoError(t, err)
	require.Equal(t, "original runtime", string(current))
}
