// Design: docs/architecture/core-design.md -- le's namespaces describe themselves
//
// Package dns owns the le dns namespace: it says what the namespace is for,
// and its tools register from the packages below it.
// One register.go, one init(): the namespace's one sentence, which the menu,
// the root page and `le help ai` show (leroot.RegisterNamespace).

package dns

import (
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.RegisterNamespace("dns",
		"DNS clients a functional test drives",
		"DNS clients a functional test drives against a Ze DNS service: `dns query` sends one query with the header bits and OPT a test names and prints every field of the reply, header bits included.")
}
