// Design: docs/architecture/core-design.md -- le's namespaces describe themselves
//
// Package web owns the le web namespace: it says what the namespace is for,
// and its areas register from the packages below it.
// One register.go, one init(): the namespace's one sentence, which the menu,
// the root page and `le help ai` show (leroot.RegisterNamespace).

package web

import (
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.RegisterNamespace("web", "the web UI's assets: per-page asset sets, vendored copies and the htmx upgrade findings")
}
