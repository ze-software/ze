// Design: docs/architecture/core-design.md -- le's namespaces describe themselves
//
// Package cli owns the le cli namespace: it says what the namespace is for,
// and its areas register from the packages below it.
// One register.go, one init(): the namespace's one sentence, which the menu,
// the root page and `le help ai` show (leroot.RegisterNamespace).

package cli

import (
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.RegisterNamespace("cli", "gates and reports over Ze's command surface: grammar, dispatch, ownership, file arguments and the command catalog")
}
