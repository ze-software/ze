// Design: docs/architecture/zefs-format.md -- in-place write fallback for non-unix
// Overview: store.go -- flush() falls back to full rewrite on non-unix

//go:build !unix

package zefs

import (
	"errors"
	"os"
	"path/filepath"
)

type writeRegion struct {
	offset int
	data   []byte
}

var errPwriteUnsupported = errors.New("zefs: pwrite not supported on this platform")

func openBlobAlias(folder *os.Root, name string) (*os.File, error) {
	// #nosec G304 -- Ordinary path constructors intentionally follow the caller's alias; rooted constructors never call this opener.
	return os.Open(filepath.Join(folder.Name(), name))
}

func openBlobFile(folder *os.Root, name string, flags int) (*os.File, error) {
	return folder.OpenFile(name, flags, 0)
}

func pwriteRegions(_ *os.Root, _ string, _ []writeRegion, _ bool) error {
	return errPwriteUnsupported
}
