package storage

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// openReadOnly is OpenReadOnly for a caller holding one spelling of the
// folder, in the shape the opener tables take.
func openReadOnly(dir string) (Storage, error) { return OpenReadOnly(dir, dir) }

// TestOpenReadOnlyRefusalsNameTheShownFolder proves every refusal of the
// read-only open names the folder as the caller spelled it, never the path
// the open used.
//
// VALIDATES: a daemon handler that opens the store beside the path it
// resolved answers a refusal naming the path the caller typed, the words the
// local command prints, for each refusal the open path produces before the
// tree is opened: a pending import, a canonical blob, no store, a folder
// whose entries cannot be checked, and a folder that cannot be opened.
// PREVENTS: the resolved absolute path leaking into a remote caller's
// refusal, and the two routes answering one request with different words.
// Method: each case prepares a temporary folder and opens it under a shown
// spelling that shares no text with it.
func TestOpenReadOnlyRefusalsNameTheShownFolder(t *testing.T) {
	const shown = "spelled/as/typed"
	unreadable := os.Geteuid() == 0
	cases := []struct {
		name    string
		prepare func(t *testing.T, dir string)
		is      error
		names   []string
		root    bool // a mode the superuser ignores
	}{
		{
			name:    "no store",
			prepare: func(*testing.T, string) {},
			is:      ErrNoStore,
			names:   []string{shown + "/" + treeName},
		},
		{
			name: "canonical blob",
			prepare: func(t *testing.T, dir string) {
				writeShownFixture(t, filepath.Join(dir, blobName))
			},
			names: []string{"artifact " + shown + "/" + blobName, "--from " + shown + "/" + blobName},
		},
		{
			name: "unreadable intent",
			prepare: func(t *testing.T, dir string) {
				writeShownFixture(t, filepath.Join(dir, importIntentName))
			},
			is:    ErrImportPending,
			names: []string{shown + "/" + importIntentName, shown + "/" + treeName},
		},
		{
			name: "entries cannot be checked",
			prepare: func(t *testing.T, dir string) {
				chmodShown(t, dir, 0o000)
			},
			is:    fs.ErrPermission,
			names: []string{shown + "/" + importIntentName},
			root:  true,
		},
		{
			name: "folder cannot be opened",
			prepare: func(t *testing.T, dir string) {
				if err := os.Mkdir(filepath.Join(dir, treeName), 0o700); err != nil {
					t.Fatal(err)
				}
				// Search access lets every lstat succeed, and without read
				// access the folder itself cannot be opened.
				chmodShown(t, dir, 0o300)
			},
			is:    ErrPermissions,
			names: []string{"store folder " + shown + " "},
			root:  true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.root && unreadable {
				t.Skip("the superuser ignores the folder mode this case sets")
			}
			dir := filepath.Join(t.TempDir(), "store")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			tc.prepare(t, dir)

			store, err := OpenReadOnly(dir, shown)
			if err == nil {
				t.Fatalf("open of %s succeeded; want a refusal (close: %v)", dir, store.Close())
			}
			if tc.is != nil && !errors.Is(err, tc.is) {
				t.Errorf("refusal %q is not %v", err, tc.is)
			}
			for _, name := range tc.names {
				if !strings.Contains(err.Error(), name) {
					t.Errorf("refusal %q does not name %q", err, name)
				}
			}
			if strings.Contains(err.Error(), dir) {
				t.Errorf("refusal %q names the opened path %s", err, dir)
			}
		})
	}
}

// writeShownFixture writes bytes that are no valid blob and no valid intent.
func writeShownFixture(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("not a store file"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// chmodShown sets the folder mode and restores it at cleanup, so the
// temporary directory can be removed.
func chmodShown(t *testing.T, dir string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(dir, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Errorf("restore mode of %s: %v", dir, err)
		}
	})
}
