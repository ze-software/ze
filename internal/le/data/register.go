// Design: docs/architecture/core-design.md -- le's namespaces describe themselves
//
// Package data owns the le data namespace: it says what the namespace is for,
// and its areas register from the packages below it.
// One register.go, one init(): the namespace's one sentence, which the menu,
// the root page and `le help ai` show (leroot.RegisterNamespace).

package data

import (
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.RegisterNamespace("data", "fetch and refresh the external data Ze ships or cites: RIR delegations, IANA registries and IETF reference texts")
}
