//go:build !linux

// Design: docs/architecture/storage-backends.md -- ownership transfer requires Linux descriptors.
// Related: ownership.go implements locked, descriptor-pinned ownership maintenance.
package storage

import "fmt"

// TransferOwnership is unsupported outside Linux because ownership maintenance
// requires pinned nofollow descriptors through validation, transfer, and rollback.
func TransferOwnership(dir string, uid, gid int) error {
	return fmt.Errorf("transfer ownership %s to %d:%d: supported only on Linux", dir, uid, gid)
}
