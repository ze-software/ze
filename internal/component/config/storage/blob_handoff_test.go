// Design: docs/architecture/storage-backends.md -- artifact lock handoff.
package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/zefs"
)

func TestOpenBlobCooperativeHandoff(t *testing.T) {
	for _, writable := range []bool{false, true} {
		name := "reader"
		if writable {
			name = "writer"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "backup.zefs")
			writer, err := CreateBlob(path)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, writer.Close()) })
			require.NoError(t, writer.WriteFile("router.conf", []byte("A"), 0))
			before, err := os.Stat(path)
			require.NoError(t, err)
			latest := strings.Repeat("new layout B\n", 128)
			first := true
			acquire := func(folder *os.File, leaf string, directory bool) (*os.File, error) {
				file, err := openNode(folder, leaf, directory)
				if err != nil || !first {
					return file, err
				}
				first = false
				// The real writer still holds the sibling flock while publishing B.
				require.NoError(t, writer.WriteFile("router.conf", []byte(latest), 0))
				require.NoError(t, writer.WriteFile("only-b.conf", []byte("keep B"), 0))
				after, err := os.Stat(path)
				require.NoError(t, err)
				require.False(t, os.SameFile(before, after), "must publish a new inode")
				require.NoError(t, writer.Close())
				return file, nil
			}
			next, err := openBlob(path, writable, acquire)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, next.Close()) })
			got, err := next.ReadFile("router.conf")
			assert.NoError(t, err)
			assert.Equal(t, latest, string(got), "handoff consumed stale A")
			got, err = next.ReadFile("only-b.conf")
			assert.NoError(t, err)
			assert.Equal(t, "keep B", string(got))
			if writable {
				require.NoError(t, next.WriteFile("reader.conf", []byte("next writer"), 0))
			}
			require.NoError(t, next.Close())
			report, err := zefs.Check(path)
			require.NoError(t, err)
			require.True(t, report.ContainerOK, "published layout must stay valid")
			reopened, err := OpenBlob(path, false)
			require.NoError(t, err)
			defer reopened.Close() //nolint:errcheck // Read-only test artifact.
			got, err = reopened.ReadFile("router.conf")
			assert.NoError(t, err)
			assert.Equal(t, latest, string(got), "later publication overwrote B")
			got, err = reopened.ReadFile("only-b.conf")
			assert.NoError(t, err)
			assert.Equal(t, "keep B", string(got))
		})
	}
}
