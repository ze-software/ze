// Design: docs/architecture/storage-backends.md -- retained import source directory.
package storage

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/pkg/zefs"
)

// importSource retains the accepted source directory for one import or replay.
// path is the durable label, never a descriptor path. Only openImportSource
// constructs it; its zero value is uninitialized. The owner MUST call close
// after the import, and MUST NOT use it concurrently with close.
type importSource struct {
	path   string
	folder *os.File
	root   *os.Root
}

func openImportSource(path string) (*importSource, error) {
	dir, name, err := splitStorePath(path)
	if err != nil {
		return nil, err
	}
	folder, err := openFolder(dir)
	if err != nil {
		return nil, err
	}
	root, err := blobRoot(folder)
	if err != nil {
		return nil, errors.Join(err, folder.Close())
	}
	return &importSource{path: filepath.Join(folder.Name(), name), folder: folder, root: root}, nil
}

func (s *importSource) close() error {
	return errors.Join(s.root.Close(), s.folder.Close())
}

func (s *importSource) open() (*zefs.BlobStore, error) {
	name := filepath.Base(s.path)
	file, err := openNode(s.folder, name, false)
	if err != nil {
		return nil, err
	}
	report, err := zefs.CheckFile(file)
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	if err := checkArtifactReport(s.path, report); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return zefs.OpenFileIn(s.root, name, file)
}
