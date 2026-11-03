// Design: docs/architecture/zefs-format.md -- ordinary artifact path aliases.
package zefs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// blobAlias retains the resolved target directory and leaf name, not the
// observation descriptor, accepted by an ordinary path constructor.
// Rooted constructors never accept aliases. A full rewrite replaces the link
// itself, so it closes this directory and resumes reads from the publication name.
type blobAlias struct {
	folder *os.Root
	name   string
}

func newBlobPath(folder *os.Root, name string, opts []Option) (*BlobStore, error) {
	s, err := newBlobIn(folder, name, opts)
	if err != nil {
		return nil, err
	}
	if err := s.bindAlias(); err != nil {
		return nil, errors.Join(err, s.Close())
	}
	return s, nil
}

func (s *BlobStore) bindAlias() error {
	info, err := s.folder.Lstat(s.name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil // Create publishes a new artifact at this absent name.
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return nil // No alias: reads use the retained publication directory.
	}
	accepted, err := openBlobAlias(s.folder, s.name)
	if err != nil {
		return err
	}
	defer accepted.Close() //nolint:errcheck // Identity observation only.
	want, err := accepted.Stat()
	if err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(s.path)
	if err != nil {
		return err
	}
	target, err := os.OpenRoot(filepath.Dir(resolved))
	if err != nil {
		return err
	}
	file, err := openBlobFile(target, filepath.Base(resolved), os.O_RDONLY)
	if err != nil {
		return errors.Join(err, target.Close())
	}
	got, statErr := file.Stat()
	if err := errors.Join(statErr, file.Close()); err != nil {
		return errors.Join(err, target.Close())
	}
	if !os.SameFile(want, got) {
		return errors.Join(fmt.Errorf("zefs: artifact link changed: %s: retry using its current path", s.path), target.Close())
	}
	s.alias = &blobAlias{folder: target, name: filepath.Base(resolved)}
	return nil
}

func (s *BlobStore) backingName() (*os.Root, string) {
	if s.alias != nil {
		return s.alias.folder, s.alias.name
	}
	return s.folder, s.name
}

func (s *BlobStore) closeAlias() error {
	if s.alias == nil {
		return nil // No alias is owned after publication or Close.
	}
	err := s.alias.folder.Close()
	s.alias = nil
	return err
}
