// Design: docs/architecture/core-design.md -- le's namespaces describe themselves
//
// Package arch owns the le arch namespace: it says what the namespace is for,
// and its areas register from the packages below it.
// One register.go, one init(): the namespace's one sentence, which the menu,
// the root page and `le help ai` show (leroot.RegisterNamespace).

package arch

import (
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.RegisterNamespace("arch",
		"architecture gates over the Go source",
		"Architecture gates over the Go source: import tiers, registries, guards, refusals and persisted state.")
}
