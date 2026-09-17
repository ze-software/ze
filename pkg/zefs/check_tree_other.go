// Design: docs/architecture/zefs-format.md -- framed trees require secure Unix traversal.
// Related: check_tree_unix.go -- descriptor-relative tree implementation.

//go:build !linux && !darwin

package zefs

import "fmt"

func walkFrameTree(path string, _ func(string, []byte) error) error {
	return fmt.Errorf("zefs: framed tree integrity at %s requires Linux or Darwin", path)
}

func repairFrameTree(srcPath, _ string) (*RepairReport, error) {
	return nil, fmt.Errorf("zefs: framed tree repair at %s requires Linux or Darwin", srcPath)
}
