// Design: docs/architecture/core-design.md -- le's namespaces describe themselves
//
// Package plugin owns the le plugin namespace: it says what the namespace is for,
// and its areas register from the packages below it.
// One register.go, one init(): the namespace's one sentence, which the menu,
// the root page and `le help ai` show (leroot.RegisterNamespace).

package plugin

import (
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.RegisterNamespace("plugin",
		"plugin gates: process boundary, declarations, tables",
		"Plugin gates: the process boundary, build-gate coupling, declarations and the generated plugin tables.")
}
