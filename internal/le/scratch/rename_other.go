//go:build !linux

// Design: plan/spec-le-is-a-ze-binary.md -- collision-safe scratch migration
// Overview: move.go -- staged cross-device moves
package scratch

import (
	"fmt"
	"os"
)

func renameNoReplace(source, target string) error {
	if pathExists(target) {
		return fmt.Errorf("target already exists")
	}
	return os.Rename(source, target)
}
