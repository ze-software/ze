// Design: docs/architecture/storage-backends.md -- interrupted import publication refuses both adapters.
package artifact_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	storagecli "github.com/ze-software/ze/internal/component/config/storage/cli"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestArtifactImportRetainsInterruptedFilePublication proves the standalone
// producer refuses its own interrupted file-commit record before either
// adapter changes active bytes, history or any retained intent.
func TestArtifactImportRetainsInterruptedFilePublication(t *testing.T) {
	for _, route := range []string{"local", "token"} {
		t.Run(route, func(t *testing.T) {
			fixture := newPolicyHTTP(t, nil, false)
			id := artifactOperationUpload(t, fixture)
			require.NoError(t, fixture.owner.ChangeStore(t.Context(), "alice", id, func(target storage.Storage) error {
				_, err := storage.RestoreConfig(target, "router.conf", []byte("current config"))
				if err != nil {
					return err
				}
				return target.WriteKey(zefs.KeyConfigFileCommit.Key("router.conf"), []byte("interrupted file commit"))
			}))
			before, err := fixture.owner.Inspect(t.Context(), "alice", id)
			require.NoError(t, err)
			if route == "local" {
				path := filepath.Join(t.TempDir(), "input.zefs")
				output, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
				require.NoError(t, err)
				require.NoError(t, fixture.owner.ExportTo(t.Context(), "alice", id, output))
				require.NoError(t, output.Close())
				original, err := os.ReadFile(path)
				require.NoError(t, err)
				input := filepath.Join(t.TempDir(), "router.conf")
				require.NoError(t, os.WriteFile(input, []byte("replacement"), 0o600))
				require.NotZero(t, storagecli.Run([]string{"--path", path, "import", input}))
				after, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Equal(t, original, after)
			} else {
				input := artifactBinarySource(t, fixture, []byte("replacement"))
				response := artifactOperationCall(t, fixture, "request config artifact import artifact "+id+" name router.conf transfer "+input)
				require.Equal(t, plugin.StatusError, response.Status)
				require.Contains(t, response.Error, "file commit")
			}
			after, err := fixture.owner.Inspect(t.Context(), "alice", id)
			require.NoError(t, err)
			require.Equal(t, before.Digest, after.Digest)
		})
	}
}
