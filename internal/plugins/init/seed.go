// Design: docs/architecture/storage-backends.md -- descriptor-bound init entry.
package init

import (
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// runSeedChild is reached only after descriptor admission and confined crash
// initialization. It never calls the ordinary path resolver or importer.
func runSeedChild(root *os.Root, folder *os.File, importing bool) int {
	if !importing {
		publish := func(populate func(storage.Storage) error) (storage.Storage, error) {
			return storage.CreateSeedIn(root, folder, populate)
		}
		return runInitPublished(os.Stdin, nil, filepath.Join(folder.Name(), "database.zefs"), false, "", "", true, publish)
	}
	store, err := storage.ImportSeedIn(root, folder)
	if err == nil {
		err = store.Close()
	}
	if err != nil {
		var out textbuf.Buffer
		out.Str("session import failed: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // Command diagnostic.
		return 1
	}
	return 0
}
