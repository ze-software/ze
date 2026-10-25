// Design: docs/architecture/core-design.md -- le's namespaces describe themselves
//
// Package config owns the le config namespace: it says what the namespace is for,
// and its areas register from the packages below it.
// One register.go, one init(): the namespace's one sentence, which the menu,
// the root page and `le help ai` show (leroot.RegisterNamespace).

package config

import (
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.RegisterNamespace("config",
		"gates over Ze's YANG config and how code reads it",
		"Gates over Ze's YANG config: every subtree delivered, every leaf coerced and read, port defaults agreed.")
}
