// Design: docs/architecture/storage-backends.md -- stored configuration name admission.
// Related: restore.go -- RestoreConfig commits a config under the name judged here.
// Related: rename_config.go, rollback_config.go -- shared owner transactions.

package storage

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/ze-software/ze/internal/core/cliio"
)

// StoredConfigName validates a configuration filename against an open source
// handle and returns its basename. Raw storage keys, control characters and
// paths outside that store are refused. Safe for concurrent use.
func StoredConfigName(source Storage, name string) (string, error) {
	if isNamespaced(name) {
		return "", fmt.Errorf("stored configuration name %q is a storage key, not a config filename", name)
	}
	if err := source.CheckName(name); err != nil {
		return "", err
	}
	base := filepath.Base(name)
	if !PlainConfigName(base) {
		return "", fmt.Errorf("stored configuration name %q is not a config filename", name)
	}
	for _, character := range base {
		if unicode.IsControl(character) {
			return "", fmt.Errorf("stored configuration name %q contains a control character", name)
		}
	}
	return base, nil
}

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
