// Design: docs/architecture/web-interface.md -- backup folder changes follow accepted config commits.
package artifact_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/artifact"
	configcli "github.com/ze-software/ze/internal/component/config/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	storagecli "github.com/ze-software/ze/internal/component/config/storage/cli"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/env"
)

func backupDirectoryCandidate(parent string) map[string]any {
	return map[string]any{"environment": map[string]any{"backup": map[string]any{"directory": parent}}}
}

// TestBackupDirectoryCommitSelection reaches actual reload acceptance while an
// uploaded workspace and immutable download remain open in the prior folder.
// Preparation must check/create the candidate folder; only acceptance can make
// it effective, and rejection must retain useful old workspace/download bytes.
func TestBackupDirectoryCommitSelection(t *testing.T) {
	for _, outcome := range []string{"accepted", "unusable folder", "outer abort", "removed setting", "live namespace", "reserved namespace"} {
		t.Run(outcome, func(t *testing.T) {
			backupDirectoryEnvironment(t, "")
			oldParent := t.TempDir()
			liveDirectory := t.TempDir()
			live, err := storage.Create(liveDirectory)
			require.NoError(t, err)
			require.NoError(t, live.WriteKey("test/commit/kept", []byte("live bytes")))
			storagecli.InstallDataRPC(&storagecli.DataRPCTarget{Store: live})
			t.Cleanup(func() {
				storagecli.InstallDataRPC(nil)
				require.NoError(t, live.Close())
			})
			server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, plugin.NewCoordinator(backupDirectoryCandidate(oldParent)))
			require.NoError(t, err)
			t.Cleanup(server.Stop)
			owner := artifact.New()
			// The existing owner's accepted root is seeded independently of OS
			// overrides, which must not mask a later committed YANG selection.
			owner.FixtureParent(oldParent)
			require.NoError(t, configcli.AttachArtifactOwner(server, owner))
			content := newPolicyHTTP(t, nil, false).source
			upload, err := owner.Begin(t.Context(), "alice", "original.zefs", int64(len(content)), false)
			require.NoError(t, err)
			_, err = owner.Append(t.Context(), "alice", upload.ID, 0, content)
			require.NoError(t, err)
			workspace, _, err := owner.Finish(t.Context(), "alice", upload.ID)
			require.NoError(t, err)
			download, err := owner.Snapshot(t.Context(), "alice", workspace.ID, "")
			require.NoError(t, err)
			_, before, err := owner.Read(t.Context(), "alice", download.ID, 0)
			require.NoError(t, err)
			oldRoot := owner.FixtureState().Root
			next := filepath.Join(t.TempDir(), "committed-backups")
			unsafe := outcome == "live namespace" || outcome == "reserved namespace"
			if outcome == "live namespace" {
				next = filepath.Join(liveDirectory, storage.TreeName, "new-backup")
			}
			if outcome == "reserved namespace" {
				next = filepath.Join(liveDirectory, storage.BlobName)
			}
			candidate := backupDirectoryCandidate(next)
			if outcome == "removed setting" {
				data := t.TempDir()
				t.Setenv("ze.data.dir", data)
				t.Setenv("ZE_DATA_DIR", data)
				env.ResetCache()
				next = filepath.Join(data, "backups")
				candidate = map[string]any{"environment": map[string]any{}}
			}
			if outcome == "unusable folder" {
				require.NoError(t, os.WriteFile(next, []byte("keep this file"), 0o600))
			}
			ctx, settle := server.DeferReloadAcceptance(t.Context())
			t.Cleanup(func() { require.NoError(t, settle(false)) })
			liveBefore := backupDirectoryPhysicalTree(t, liveDirectory)
			reloadErr := server.ReloadConfig(ctx, candidate)
			if outcome == "unusable folder" || unsafe {
				require.Error(t, reloadErr, "commit preparation must refuse an unusable folder")
				require.NoError(t, settle(false))
				if unsafe {
					require.Equal(t, liveBefore, backupDirectoryPhysicalTree(t, liveDirectory), "refused commit must not create or alter live store entries")
				}
			} else {
				require.NoError(t, reloadErr)
				info, err := os.Stat(next)
				require.NoError(t, err, "commit preparation must create/check the folder before any new upload")
				require.True(t, info.IsDir())
				require.Equal(t, oldRoot, owner.FixtureState().Root, "inner reload must not publish before outer acceptance")
				accept := outcome != "outer abort"
				require.NoError(t, settle(accept))
			}
			newUpload, err := owner.Begin(t.Context(), "alice", "next.zefs", -1, false)
			require.NoError(t, err)
			want := oldParent
			if outcome == "accepted" {
				want = next
			}
			if outcome == "removed setting" {
				want = next
			}
			currentRoot := owner.FixtureState().Root
			require.Equal(t, want, filepath.Dir(currentRoot))
			if outcome == "accepted" {
				var snapshotPath string
				systemDownload, err := owner.SnapshotSystem(t.Context(), "alice", func(path string, limit int64) error {
					snapshotPath = path
					return storagecli.BackupSystemTo(t.Context(), path, limit)
				})
				require.NoError(t, err)
				require.Equal(t, currentRoot, filepath.Dir(filepath.Dir(snapshotPath)), "system backups must allocate under the newly accepted folder")
				require.NoError(t, owner.Abort(t.Context(), "alice", systemDownload.ID))
			}
			_, after, err := owner.Read(t.Context(), "alice", download.ID, 0)
			require.NoError(t, err)
			require.True(t, bytes.Equal(before, after), "a live download must survive the directory decision")
			kept, err := owner.Inspect(t.Context(), "alice", workspace.ID)
			require.NoError(t, err)
			require.Equal(t, workspace.ID, kept.ID)
			require.NoError(t, owner.Abort(t.Context(), "alice", newUpload.ID))
			require.NoError(t, owner.Abort(t.Context(), "alice", download.ID))
			require.NoError(t, owner.Remove(t.Context(), "alice", workspace.ID))
			require.NoError(t, owner.Close())
			_, err = os.Stat(oldRoot)
			require.ErrorIs(t, err, os.ErrNotExist)
			_, err = os.Stat(currentRoot)
			require.ErrorIs(t, err, os.ErrNotExist)
			if outcome == "unusable folder" {
				keptFile, err := os.ReadFile(next)
				require.NoError(t, err)
				require.Equal(t, "keep this file", string(keptFile))
			}
		})
	}
}
