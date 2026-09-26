package cli

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestDaemonAddress drives the address part of the refusal an offline command
// prints while a daemon owns the store, over each of its answers: the recorded
// address as host:port, a malformed record, a store that records none, and a
// store it cannot read.
// VALIDATES: AC-3 (the refusal names the daemon's SSH address, or says why not).
// PREVENTS: an empty hint that reads as a refusal which forgot the address.
func TestDaemonAddress(t *testing.T) {
	dir := t.TempDir()
	writeBlob := func(name string, values map[string]string) string {
		path := filepath.Join(dir, name)
		blob, err := storage.CreateBlobPopulated(path, func(seed storage.Storage) error {
			for key, value := range values {
				if err := seed.WriteKey(key, []byte(value)); err != nil {
					return err
				}
			}
			return nil
		}, false, zefs.Spare(0))
		require.NoError(t, err)
		require.NoError(t, blob.Close())
		return path
	}

	recorded := writeBlob("recorded.zefs", map[string]string{zefs.KeySSHDefault.Pattern: "127.0.0.1/2222"})
	assert.Equal(t, " at 127.0.0.1:2222", daemonAddress(recorded))

	malformed := writeBlob("malformed.zefs", map[string]string{zefs.KeySSHDefault.Pattern: "127.0.0.1"})
	assert.Equal(t, ` (its recorded SSH address "127.0.0.1" is not host/port)`, daemonAddress(malformed))

	absent := writeBlob("absent.zefs", map[string]string{zefs.KeyInstanceName.Pattern: "edge"})
	assert.Equal(t, " (the store records no SSH address at "+zefs.KeySSHDefault.Pattern+")", daemonAddress(absent))

	unreadable := daemonAddress(filepath.Join(dir, "missing.zefs"))
	assert.Contains(t, unreadable, " (its SSH address is unreadable: ")
	assert.Contains(t, unreadable, "missing.zefs")
}
