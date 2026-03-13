// Design: (none -- predates documentation)
// Overview: store.go -- BlobStore uses these fs.File/DirEntry wrappers

package zefs

import (
	"io"
	"io/fs"
	"sort"
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

// storeDir implements fs.File and fs.ReadDirFile for a directory node.
type storeDir struct {
	node    *node
	name    string
	entries []fs.DirEntry // lazily built, sorted
	pos     int           // streaming position for ReadDir(n>0)
}

func (d *storeDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.name, Err: fs.ErrInvalid}
}

func (d *storeDir) Stat() (fs.FileInfo, error) {
	return &storeFileInfo{name: d.name, dir: true}, nil
}

func (d *storeDir) Close() error { return nil }

// ReadDir implements fs.ReadDirFile. When n > 0, it returns up to n entries
// per call, advancing an internal cursor; io.EOF signals no more entries.
// When n <= 0, it returns all entries in one shot.
func (d *storeDir) ReadDir(n int) ([]fs.DirEntry, error) {
	if d.entries == nil {
		d.entries = make([]fs.DirEntry, 0, len(d.node.children))
		for childName, child := range d.node.children {
			d.entries = append(d.entries, &storeDirEntry{
				entryName: childName,
				isDir:     child.children != nil,
				size:      len(child.data),
			})
		}
		sort.Slice(d.entries, func(i, j int) bool {
			return d.entries[i].Name() < d.entries[j].Name()
		})
	}

	if n <= 0 {
		remaining := d.entries[d.pos:]
		d.pos = len(d.entries)
		return remaining, nil
	}

	if d.pos >= len(d.entries) {
		return nil, io.EOF
	}
	end := min(d.pos+n, len(d.entries))
	result := d.entries[d.pos:end]
	d.pos = end
	if d.pos >= len(d.entries) {
		return result, io.EOF
	}
	return result, nil
}

// storeFileInfo implements fs.FileInfo.
type storeFileInfo struct {
	name string
	size int64
	dir  bool
}

func (fi *storeFileInfo) Name() string { return fi.name }
func (fi *storeFileInfo) Size() int64  { return fi.size }
func (fi *storeFileInfo) Mode() fs.FileMode {
	if fi.dir {
		return fs.ModeDir | 0o555
	}
	return 0o444
}
func (fi *storeFileInfo) ModTime() time.Time { return time.Time{} }
func (fi *storeFileInfo) IsDir() bool        { return fi.dir }
func (fi *storeFileInfo) Sys() any           { return nil }

// storeDirEntry implements fs.DirEntry.
type storeDirEntry struct {
	entryName string
	isDir     bool
	size      int
}

func (e *storeDirEntry) Name() string { return e.entryName }
func (e *storeDirEntry) IsDir() bool  { return e.isDir }
func (e *storeDirEntry) Type() fs.FileMode {
	if e.isDir {
		return fs.ModeDir
	}
	return 0
}
func (e *storeDirEntry) Info() (fs.FileInfo, error) {
	return &storeFileInfo{name: e.entryName, size: int64(e.size), dir: e.isDir}, nil
}
