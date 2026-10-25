// Design: docs/architecture/core-design.md -- le's namespaces describe themselves
//
// Package ai owns the le ai namespace: it says what the namespace is for,
// and its areas register from the packages below it.
// One register.go, one init(): the namespace's one sentence, which the menu,
// the root page and `le help ai` show (leroot.RegisterNamespace).

package ai

import (
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.RegisterNamespace("ai",
		"agent tooling: hooks, rules, generated files, tokens",
		"The agent tooling: native hooks, the rule corpus, generated agent files, digest anchors and token use.")
}
