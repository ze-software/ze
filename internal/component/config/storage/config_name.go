// Design: docs/architecture/zefs-format.md -- the ze data CLI and its daemon forms
// Related: restore.go -- RestoreConfig commits a config under the name judged here.

package storage

import (
	"strings"

	"github.com/ze-software/ze/internal/core/cliio"
)

// PlainConfigName reports whether name names a config in the store folder
// itself: not a directory step, not a path, and not the stdin marker. Every
// command that names a config by its file name alone judges the name here
// (`ze config import`, and `update data config`, which the local `ze data
// import` sends while a daemon owns the store), so they agree on what a config
// name is.
func PlainConfigName(name string) bool {
	switch name {
	case ".", "..":
		return false
	}
	if strings.ContainsAny(name, "/\\") {
		return false
	}
	return !cliio.IsStdin(name)
}
