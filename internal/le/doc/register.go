// Design: docs/architecture/core-design.md -- le's namespaces describe themselves
//
// Package doc owns the le doc namespace: it says what the namespace is for,
// and its areas register from the packages below it.
// One register.go, one init(): the namespace's one sentence, which the menu,
// the root page and `le help ai` show (leroot.RegisterNamespace).

package doc

import (
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.RegisterNamespace("doc",
		"documentation gates and generated indexes",
		"Documentation gates and generated indexes: links, code and doc agreement, change wiring and writing style.")
}
