// Design: docs/architecture/core-design.md -- le's namespaces describe themselves
//
// Package test owns the le test namespace: it says what the namespace is for,
// and its areas register from the packages below it.
// One register.go, one init(): the namespace's one sentence, which the menu,
// the root page and `le help ai` show (leroot.RegisterNamespace).

package test

import (
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.RegisterNamespace("test",
		"Ze's test suites and the helpers and mocks they run",
		"Ze's test suites and the helpers they run: functional, interop, integration, unit, fuzz and mutation suites, and mock servers.")
}
