// Design: docs/architecture/core-design.md -- le's namespaces describe themselves
//
// Package lab owns the le lab namespace: it says what the namespace is for,
// and its areas register from the packages below it.
// One register.go, one init(): the namespace's one sentence, which the menu,
// the root page and `le help ai` show (leroot.RegisterNamespace).

package lab

import (
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.RegisterNamespace("lab",
		"the interop and integration labs and their peer pins",
		"The interop and integration labs: their pinned peer releases against upstream.")
}
