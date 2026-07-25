// VALIDATES: DefaultConfig derives "<instance>.conf" from the stored instance
// name, falling back to "ze.conf" when the name is missing, empty, or fails the
// validInstanceName guard (which blocks path traversal in blob keys).
// PREVENTS: an unsanitized instance name flowing into a config filename, and a
// regression in the missing/empty/invalid fallbacks.

package resolve

import (
	"errors"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// errStubUnused is returned by fakeStore methods that DefaultConfig never calls.
var errStubUnused = errors.New("fakeStore: method not used by these tests")

// fakeStore implements storage.Storage; only ReadFile carries behavior.
type fakeStore struct{ data map[string][]byte }

func (f fakeStore) ReadFile(name string) ([]byte, error) {
	if b, ok := f.data[name]; ok {
		return b, nil
	}
	return nil, os.ErrNotExist
}
func (fakeStore) WriteFile(string, []byte, fs.FileMode) error        { return nil }
func (fakeStore) Remove(string) error                                { return nil }
func (fakeStore) Exists(string) bool                                 { return false }
func (fakeStore) List(string) ([]string, error)                      { return nil, nil }
func (fakeStore) AcquireLock(string) (storage.WriteGuard, error)     { return nil, errStubUnused }
func (fakeStore) Stat(string) (storage.FileMeta, error)              { return storage.FileMeta{}, nil }
func (fakeStore) Rename(string, string) error                        { return nil }
func (fakeStore) Close() error                                       { return nil }
func (fakeStore) WriteVersion(string, []byte, time.Time) error       { return nil }
func (fakeStore) ListVersions(string) ([]storage.VersionInfo, error) { return nil, nil }

func TestDefaultConfig(t *testing.T) {
	key := zefs.KeyInstanceName.Pattern

	for _, tc := range []struct {
		name   string
		stored map[string][]byte
		want   string
	}{
		{"valid name", map[string][]byte{key: []byte("edge-01\n")}, "edge-01.conf"},
		{"missing key", map[string][]byte{}, "ze.conf"},
		{"empty value", map[string][]byte{key: []byte("  \n")}, "ze.conf"},
		{"path traversal blocked", map[string][]byte{key: []byte("../evil")}, "ze.conf"},
		{"leading hyphen blocked", map[string][]byte{key: []byte("-bad")}, "ze.conf"},
	} {
		if got := DefaultConfig(fakeStore{data: tc.stored}); got != tc.want {
			t.Errorf("%s: DefaultConfig = %q, want %q", tc.name, got, tc.want)
		}
	}
}
