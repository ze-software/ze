// Design: docs/architecture/zefs-format.md -- framed trees require secure Unix traversal.
// Related: check_tree_unix.go -- descriptor-relative tree implementation.

//go:build !linux && !darwin && !freebsd

package zefs

import "errors"

// errFrameTreeUnsupported is a package variable rather than a fresh error on
// each call so the caller's nil check stays a real branch on this platform.
var errFrameTreeUnsupported = errors.New("zefs: framed tree integrity and repair require Linux, Darwin or FreeBSD")

func walkFrameTree(_ string, _ func(string, []byte) error) error {
	return errFrameTreeUnsupported
}

func repairFrameTree(_, _ string) (*RepairReport, error) {
	return nil, errFrameTreeUnsupported
}
