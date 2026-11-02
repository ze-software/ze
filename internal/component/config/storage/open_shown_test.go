package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

// TestOpenRefusalNamesTheFailingComponent proves a folder walk that fails
// names the component the operator must fix, in the words of the caller.
//
// VALIDATES: a symbolic link on the path to the store, or the store folder
// itself as a link, is named: on the local route (one spelling) a folder on
// the path keeps zefs's own refusal, byte for byte what the open answered
// before refusals named a shown folder; with a shown spelling the refusal
// says the failure is on the path to the shown folder, or the folder itself.
// PREVENTS: the operator reading only an errno with no hint of which folder
// to fix, and the resolved path leaking into a remote caller's refusal.
// Method: a link stands on the path, so every lstat follows it and only the
// no-follow walk refuses it; the superuser is refused the same way.
func TestOpenRefusalNamesTheFailingComponent(t *testing.T) {
	const shown = "spelled/as/typed"
	// The walk names folders by the path it opens from /, which on darwin
	// crosses /var -> /private/var: resolve the root so the expected words
	// are the ones the walk builds.
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "real")
	if err := os.MkdirAll(filepath.Join(target, "store", treeName), 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	below := filepath.Join(link, "store")

	t.Run("local route keeps the walk's refusal", func(t *testing.T) {
		_, want := openFolder(below)
		if want == nil {
			t.Fatalf("the walk opened %s through a link", below)
		}
		for name, open := range map[string]func(string) (Storage, error){"Open": Open, "OpenReadOnly": openReadOnly} {
			store, err := open(below)
			if err == nil {
				t.Fatalf("%s(%s) succeeded (close: %v)", name, below, store.Close())
			}
			if err.Error() != want.Error() {
				t.Errorf("%s refusal %q, want the walk's %q", name, err, want)
			}
			if !strings.Contains(err.Error(), "unsafe component "+link+":") {
				t.Errorf("%s refusal %q does not name the link %s", name, err, link)
			}
			if !errors.Is(err, ErrPermissions) {
				t.Errorf("%s refusal %q is not %v", name, err, ErrPermissions)
			}
		}
	})

	t.Run("shown route names a folder on the path", func(t *testing.T) {
		store, err := OpenReadOnly(below, shown)
		if err == nil {
			t.Fatalf("open of %s succeeded (close: %v)", below, store.Close())
		}
		if !strings.Contains(err.Error(), "store folder "+shown+" cannot be opened: a folder on the path to it cannot be opened: ") {
			t.Errorf("refusal %q does not say a folder on the path to %s failed", err, shown)
		}
		if strings.Contains(err.Error(), root) {
			t.Errorf("refusal %q names the opened path under %s", err, root)
		}
		if !errors.Is(err, ErrPermissions) {
			t.Errorf("refusal %q is not %v", err, ErrPermissions)
		}
	})

	t.Run("the folder itself is named as spelled", func(t *testing.T) {
		itself := filepath.Join(root, "itself")
		if err := os.Symlink(filepath.Join(target, "store"), itself); err != nil {
			t.Fatal(err)
		}
		for spelled, want := range map[string]string{itself: itself, shown: shown} {
			store, err := OpenReadOnly(itself, spelled)
			if err == nil {
				t.Fatalf("open of %s succeeded (close: %v)", itself, store.Close())
			}
			if !strings.Contains(err.Error(), "store folder "+want+" cannot be opened: ") {
				t.Errorf("refusal %q does not name the store folder %s", err, want)
			}
			if spelled == shown && strings.Contains(err.Error(), root) {
				t.Errorf("refusal %q names the opened path under %s", err, root)
			}
		}
	})
}

// TestWalkRefusalNamesNoBuiltPath proves a walk failure that holds no
// component reaches another route by the shown folder and its bare causes.
//
// VALIDATES: the close of a folder the walk holds, which fails with a path
// error naming an absolute ancestor of the store folder, is refused to a
// caller on another route with the shown folder and the errno only, and a
// permission failure there still matches ErrPermissions.
// PREVENTS: an absolute path the daemon built leaking into a remote caller's
// refusal through the one branch with no component to name.
// Method: the errors openFolder builds for a failed close are constructed
// directly, because no test can make close(2) fail on demand.
func TestWalkRefusalNamesNoBuiltPath(t *testing.T) {
	const built = "/srv/ze/built"
	folder := storeFolder{path: built + "/store", shown: "spelled/as/typed"}
	closeErr := errors.Join(
		&fs.PathError{Op: "close", Path: built, Err: syscall.EIO},
		&fs.PathError{Op: "close", Path: built + "/store", Err: syscall.EBADF},
	)

	err := folder.walkRefusal(closeErr)
	want := "store folder spelled/as/typed cannot be opened: " + syscall.EIO.Error() + "\n" + syscall.EBADF.Error() +
		"; check that ze's working directory exists and the file system holding the store folder answers reads"
	if err.Error() != want {
		t.Errorf("refusal %q, want %q", err, want)
	}
	if !errors.Is(err, syscall.EIO) {
		t.Errorf("refusal %q does not match its cause %v", err, syscall.EIO)
	}

	denied := fmt.Errorf("%w: %w", ErrPermissions, &fs.PathError{Op: "close", Path: built, Err: syscall.EACCES})
	err = folder.walkRefusal(denied)
	want = ErrPermissions.Error() + ": store folder spelled/as/typed cannot be opened: " + syscall.EACCES.Error() +
		"; give the user ze runs search access to ze's working directory and to every folder on the path to the store folder"
	if err.Error() != want {
		t.Errorf("refusal %q, want %q", err, want)
	}
	if !errors.Is(err, ErrPermissions) {
		t.Errorf("refusal %q is not %v", err, ErrPermissions)
	}
}
