// Design: docs/architecture/core-design.md -- le's namespaces describe themselves
//
// Package golang owns the le go namespace: it says what the namespace is for,
// and its areas register from the packages below it.
// One register.go, one init(): the namespace's one sentence, which the menu,
// the root page and `le help ai` show (leroot.RegisterNamespace).

package golang

import (
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.RegisterNamespace("go",
		"the Go toolchain: lint, type checks, pins and code moves",
		"The Go toolchain: lint, type-checks across tags and platforms, the patched standard library, version pins and code moves.")
}
