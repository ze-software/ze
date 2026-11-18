// Design: docs/architecture/web-interface.md -- current-system snapshots use private transfers.
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	storagecli "github.com/ze-software/ze/internal/component/config/storage/cli"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestArtifactSystemSnapshotRegistered reaches the modeled command, reads its
// normal chunk transport, reopens a full artifact and preserves every live key.
// Another account cannot read its transfer, and explicit abort removes it.
func TestArtifactSystemSnapshotRegistered(t *testing.T) {
	directory := t.TempDir()
	store, err := storage.Create(directory)
	require.NoError(t, err)
	path := filepath.Join(directory, "router.conf")
	_, _, err = storage.EnsureActiveVersion(store, path, []byte("bgp { router-id 192.0.2.1; }\n"), time.Now())
	require.NoError(t, err)
	require.NoError(t, store.WriteKey(zefs.KeyLocalAdminPassword.Key(), []byte("snapshot-private-credential")))
	require.NoError(t, store.WriteKey("test/system-snapshot/binary", bytes.Repeat([]byte{0, 255, 128, 1}, command.ContentOctetsMax)))
	storagecli.InstallDataRPC(&storagecli.DataRPCTarget{Store: store, ConfigPath: path})
	t.Cleanup(func() {
		storagecli.InstallDataRPC(nil)
		require.NoError(t, store.Close())
	})
	keys, err := store.ListKeys("")
	require.NoError(t, err)
	before := make(map[string][]byte, len(keys))
	for _, key := range keys {
		before[key], err = store.ReadKey(key)
		require.NoError(t, err)
	}
	server := privateConfigServer(t)
	response, err := privateConfigDispatch(t, server, "request config artifact system snapshot", true)
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, plugin.StatusDone, response.Status, response.Error)
	encoded, err := json.Marshal(response.Data)
	require.NoError(t, err)
	var snapshot struct {
		Transfer string `json:"transfer"`
		Total    int64  `json:"total"`
	}
	require.NoError(t, json.Unmarshal(encoded, &snapshot))
	require.Len(t, snapshot.Transfer, 64)
	require.Greater(t, snapshot.Total, int64(command.ContentOctetsMax), "the fixture must cross a real chunk boundary")
	refused, err := server.Dispatcher().Dispatch(&pluginserver.CommandContext{Server: server,
		Username: "other", Authorizer: privateConfigPolicy{edit: true}, RequestContext: t.Context(), Sender: plugin.OperatorSender()},
		"show config artifact transfer read transfer "+snapshot.Transfer+" offset 0")
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, refused.Status, "snapshots belong only to their authenticated creator")
	var content bytes.Buffer
	for offset := int64(0); offset < snapshot.Total; {
		response, err = privateConfigDispatch(t, server, fmt.Sprintf("show config artifact transfer read transfer %s offset %d", snapshot.Transfer, offset), true)
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, response.Status, response.Error)
		encoded, err = json.Marshal(response.Data)
		require.NoError(t, err)
		var chunk struct {
			Next    int64 `json:"next"`
			Content struct {
				Blob json.RawMessage `json:"blob"`
			} `json:"content"`
		}
		require.NoError(t, json.Unmarshal(encoded, &chunk))
		data, err := command.DecodeBlob(chunk.Content.Blob)
		require.NoError(t, err)
		require.Greater(t, chunk.Next, offset)
		_, err = content.Write(data)
		require.NoError(t, err)
		offset = chunk.Next
	}
	require.Equal(t, snapshot.Total, int64(content.Len()))
	artifactPath := filepath.Join(t.TempDir(), "system.zefs")
	require.NoError(t, os.WriteFile(artifactPath, content.Bytes(), 0o600))
	require.NoError(t, storage.CheckArtifact(artifactPath))
	backup, err := storage.OpenBlob(artifactPath, false)
	require.NoError(t, err)
	defer func() { require.NoError(t, backup.Close()) }()
	backupKeys, err := backup.ListKeys("")
	require.NoError(t, err)
	require.ElementsMatch(t, keys, backupKeys)
	for _, key := range keys {
		got, err := backup.ReadKey(key)
		require.NoError(t, err)
		require.Equal(t, before[key], got, key)
		live, err := store.ReadKey(key)
		require.NoError(t, err)
		require.Equal(t, before[key], live, key)
	}
	response, err = privateConfigDispatch(t, server, "request config artifact upload abort transfer "+snapshot.Transfer, true)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status, response.Error)
	response, err = privateConfigDispatch(t, server, "show config artifact transfer read transfer "+snapshot.Transfer+" offset 0", true)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, response.Status)
}

// TestArtifactSystemSnapshotRequiresEdit refuses a read-only caller before the
// native source can disclose any private backup bytes or transfer identity.
func TestArtifactSystemSnapshotRequiresEdit(t *testing.T) {
	server := privateConfigServer(t)
	response, err := privateConfigDispatch(t, server, "request config artifact system snapshot", false)
	require.ErrorIs(t, err, pluginserver.ErrUnauthorized)
	require.NotNil(t, response)
	require.Equal(t, plugin.StatusError, response.Status)
	require.Nil(t, response.Data)
}
