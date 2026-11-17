//go:build !linux

// Design: docs/architecture/container/network.md -- the namespace a link is made in
// Related: attach.go -- the created call that sends the inode

package network

import "errors"

// namespaceInode refuses: a network namespace exists on Linux only, and a
// container runs only there.
func namespaceInode(_ int) (uint64, error) {
	return 0, errors.New("a container network namespace exists on Linux only")
}
