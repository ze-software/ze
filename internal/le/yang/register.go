// Design: docs/architecture/core-design.md -- le's namespaces describe themselves
//
// Package yang owns the le yang namespace: it says what the namespace is for,
// and its areas register from the packages below it.
// One register.go, one init(): the namespace's one sentence, which the menu,
// the root page and `le help ai` show (leroot.RegisterNamespace).

package yang

import (
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.RegisterNamespace("yang",
		"YANG modules, their generated glue and path migrations",
		"The YANG modules: the generated glue beside them, and repository-wide ownership and path migrations.")
}
