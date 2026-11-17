// Design: docs/architecture/container/image-store.md -- the store root's access check

package imagestore

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestStoreRootOpenToOthersRefused proves a store root that group or others
// can reach is not used, and that the refusal is tied to the mode: the same
// folder closed to them is used.
//
// Method: for each mode that opens the root to group or others, make the root
// with that mode before the store starts. The listing must be refused with an
// operator error whose fix is the chmod, and the store must not have used
// the folder: it creates none of its folders there. Then close the folder to
// group and others and list again in the same run: the store now uses it.
func TestStoreRootOpenToOthersRefused(t *testing.T) {
	for _, mode := range []fs.FileMode{0o755, 0o750, 0o705, 0o701, 0o710} {
		t.Run(mode.String(), func(t *testing.T) {
			dataDir := t.TempDir()
			storePath := filepath.Join(dataDir, storeFolder)
			if err := os.Mkdir(storePath, 0o700); err != nil {
				t.Fatal(err)
			}
			// Chmod after Mkdir: the umask would remove the bits under test.
			if err := os.Chmod(storePath, mode); err != nil {
				t.Fatal(err)
			}

			store := openStore(dataDir, nil)
			_, err := store.ListImages(context.Background())
			if err == nil {
				t.Fatalf("a store root of mode %v was used, want a refusal", mode)
			}
			var refusal *opererr.Error
			if !errors.As(err, &refusal) {
				t.Fatalf("the refusal is %T, want an operator error: %v", err, err)
			}
			if !strings.Contains(refusal.Problem(), "open to group or others") {
				t.Errorf("the refusal says %q, want that the store is open to group or others", refusal.Problem())
			}
			if refusal.Fix() != "chmod 0700 "+storePath {
				t.Errorf("the fix is %q, want chmod 0700 %s", refusal.Fix(), storePath)
			}
			// The doctor check container-store reads the same refusal.
			checked := store.CheckStore()
			if !errors.As(checked, &refusal) {
				t.Fatalf("the store check answered %v, want the operator error of the refused root", checked)
			}
			if refusal.Fix() != "chmod 0700 "+storePath {
				t.Errorf("the store check's fix is %q, want chmod 0700 %s", refusal.Fix(), storePath)
			}
			if _, err := os.Stat(filepath.Join(storePath, stagingFolder)); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("the refused store root was used: its staging folder exists (%v)", err)
			}

			if err := os.Chmod(storePath, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := store.CheckStore(); err != nil {
				t.Errorf("the store check refused the root closed to group and others: %v", err)
			}
			if _, err := store.ListImages(context.Background()); err != nil {
				t.Fatalf("the store root closed to group and others was refused: %v", err)
			}
			if _, err := os.Stat(filepath.Join(storePath, stagingFolder)); err != nil {
				t.Errorf("the store root closed to group and others was not prepared: %v", err)
			}
		})
	}
}
