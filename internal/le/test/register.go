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
		"Ze's test suites and the tools that drive them",
		"Ze's test suites and the tools that drive them: functional, interop, integration, unit, fuzz and mutation suites, and the fixture, peer, MCP and replay tools a test runs. Mock servers are under le mock.")
}
