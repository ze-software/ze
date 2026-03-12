// Design: (none -- predates documentation)
// Overview: store.go -- BlobStore uses these fs.File/DirEntry wrappers

package zefs

import (
	"io"
	"io/fs"
	"time"
)

// storeFile implements fs.File for a leaf entry.
type storeFile struct {
	name string
	data []byte
	pos  int
}

func (f *storeFile) Read(p []byte) (int, error) {
	if f.pos >= len(f.data) {
		return 0, io.EOF
	}
	n := copy(p, f.data[f.pos:])
	f.pos += n
	return n, nil
}

func (f *storeFile) Stat() (fs.FileInfo, error) {
	return &storeFileInfo{name: f.name, size: int64(len(f.data))}, nil
}

func (f *storeFile) Close() error { return nil }

// storeDir implements fs.File for a directory node.
type storeDir struct {
	node *node
	name string
}

func (d *storeDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.name, Err: fs.ErrInvalid}
}

func (d *storeDir) Stat() (fs.FileInfo, error) {
	return &storeFileInfo{name: d.name, dir: true}, nil
}

func (d *storeDir) Close() error { return nil }

// storeFileInfo implements fs.FileInfo.
type storeFileInfo struct {
	name string
	size int64
	dir  bool
}

func (fi *storeFileInfo) Name() string       { return fi.name }
func (fi *storeFileInfo) Size() int64        { return fi.size }
func (fi *storeFileInfo) Mode() fs.FileMode  { return 0o444 }
func (fi *storeFileInfo) ModTime() time.Time { return time.Time{} }
func (fi *storeFileInfo) IsDir() bool        { return fi.dir }
func (fi *storeFileInfo) Sys() any           { return nil }

// storeDirEntry implements fs.DirEntry.
type storeDirEntry struct {
	entryName string
	isDir     bool
	size      int
}

func (e *storeDirEntry) Name() string      { return e.entryName }
func (e *storeDirEntry) IsDir() bool       { return e.isDir }
func (e *storeDirEntry) Type() fs.FileMode { return 0 }
func (e *storeDirEntry) Info() (fs.FileInfo, error) {
	return &storeFileInfo{name: e.entryName, size: int64(e.size), dir: e.isDir}, nil
}
