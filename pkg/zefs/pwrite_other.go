// Design: docs/architecture/zefs-format.md -- in-place write fallback for non-unix
// Overview: store.go -- flush() falls back to full rewrite on non-unix

//go:build !unix

package zefs

import "errors"

type writeRegion struct {
	offset int
	data   []byte
}

var errPwriteUnsupported = errors.New("zefs: pwrite not supported on this platform")

func pwriteRegions(_ string, _ []writeRegion) error {
	return errPwriteUnsupported
}
